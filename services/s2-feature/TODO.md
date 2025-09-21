# S2 Feature Generator — 製作清單（補充版 / 可直接對照開發）

> 本清單在你現有的 S2 規格上「只新增」說明，不刪改任何既有定義與數學公式；內容對齊 Project Chimera 白皮書 v3 與功能規格書。S2 的原始待辦與描述已彙整於你提供的 MD（見「參考」）。

---

## 0) 目標與邊界

* **目標**：把「行情→特徵→信號事件」這段資料鏈做穩、做準，為 S3 決策提供**無未來洩漏**、可追溯、可回放的特徵快照。
* **涵蓋**：FUT/SPOT 通用行情特徵（ATR/RV/ρ/Spread/Depth）、日級 Regime、特徵回補（Recompute）、事件與持久化。
* **不做**（由其他服務負責）：下單（S4）、倉位治理（S6）、復盤（S8）、配置 Promote（S10）。

---

## 1) 事件與資料契約（I/O）

### 1.1 輸入（Redis Streams）

* `mkt:events:<venue>:<market>:<symbol>`

  * 來源：S1
  * **必要欄位**（摘錄）：`ts`, `source`, `symbol`, `mid`, `best_bid`, `best_ask`, `funding_next`, `depth_top1_usdt`, `kline{open,high,low,close,volume,open_time,close_time}`
  * **約束**：`ts` 單調遞增；同 symbol 的成對 K 線必有 `open_time < close_time`。

### 1.2 輸出（Redis Streams / KV）

* `feat:events:<symbol>`

  * 載荷（節選）：`signal_id`, `t0`, `symbol`, `features{...}`, `config_rev`, `dqc{...}`, `source="s2"`
* `feat:last:<symbol>`（KV）

  * 存放最近一次特徵快照（JSON），**TTL 15 分鐘**，供 S3 快速拉取。

### 1.3 輸出（ArangoDB Collections）

* `signals`：新建或補寫 `features`, `t0`, `symbol`, `config_rev`
* `strategy_events`：記錄 `FEATURE_RECOMPUTE`、`FEATURE_DQC_FAIL` 等事件

> **寫入時機**：見 §5「處理管線與持久化節點」。

---

## 2) 特徵集合（新增欄位清單）

> 公式沿用你既有文件（ATR / RV / ρ 等），此處只列**欄位命名、單位與約束**。

* **波動與幅度**

  * `atr_14`, `atr_14_pct`（相對 mid，單位「比率」）
  * `rv_1d`, `rv_4h`, `rv_30d`（年化，單位「比率」）
  * `rv_pctile_30d` ∈ \[0,1]
* **價差與深度**

  * `spread_bps`（(ask-bid)/mid \* 1e4）
  * `depth_top1_usdt`（雙邊合計或分邊，請在 `meta.calc` 標記）
  * `liq_score` ∈ \[0,1]（深度正規化）
* **相關性與交互**

  * `rho_usdttwd_14`（USDT/TWD 與 BTCUSDT）
  * `rho_fx_btc_roll_z`（z-score 選配；若使用請由 S2 預先產出）
* **資金費／微結構**

  * `funding_next`, `funding_next_abs`
  * `micro_imbalance_top1`（(bid\_qty-ask\_qty)/(bid\_qty+ask\_qty)）
* **Regime 與派生**

  * `regime_label` ∈ {`FROZEN`,`NORMAL`,`EXTREME`}（影子欄位；正式值寫入 Redis `prod:regime:market:state`）

---

## 3) DQC（資料品質檢查）與處置

* **時間一致性**：`ts` 不可回跳；K 線 `open_time < close_time`；違反→丟棄並記 `FEATURE_DQC_FAIL`。
* **完整性**：特徵依賴的欄位缺失→該次特徵標示 `dqc.missing=true`、**不推送** `feat:events`。
* **異常跳變**：對 `mid` 施以 IQR 或中位數±N×MAD 門檻；觸發→標 `dqc.jump=true`，仍可寫 DB（供後續分析）但**不對外事件**。
* **未來洩漏保證**：所有滑窗僅使用 `t0` 及其**之前**資料；回補時以 `close_time <= t0` 為硬約束。
* **追蹤欄位**：`dqc{missing:boolean, jump:boolean, note:string}`

---

## 4) 配置與熱載（S10 對接）

* **拉取**：開機與 `cfg:events` 時執行 `GET /active`，以 RCU 原子替換本地 `active_cfg`。
* **使用點**：特徵清單、滑窗長度、Regime 分位閾值、`rho_min`、`liq_threshold` 等均來自 `config_bundles`。
* **降級**：`/active` 不可用時，使用本地快取 rev，並將 `/health` 狀態降為 `DEGRADED`。

---

## 5) 處理管線與寫點（Hop-by-Hop）

1. **Consume**：`XREADGROUP` 自 `mkt:events:*` 拉批（批量 ≤ 200，超時 200ms）
2. **Cache**：寫入 `feat:{cache}:<symbol>` 的滑窗（環形緩衝／LRU）
3. **Compute**：依 active\_cfg 計算特徵；生成 `features{...}` 與 `dqc{...}`
4. **Persist(1)**：若 `dqc.pass=true` **且** `mode.realtime=true` → upsert `signals(signal_id,t0,symbol,features,config_rev)`
5. **Emit**：同條件下發佈 `feat:events:<symbol>`；更新 `feat:last:<symbol>`（TTL 15m）
6. **Persist(2)**：若 `mode.persist_all=true`（可選）則**無論 DQC**均寫 DB（但 `dqc` 標記保留）
7. **Metrics**：記 `s2.compute.latency_ms`、`s2.emit.count`、`s2.dqc.fail.count`
8. **Ack**：成功後 `XACK`；失敗轉 `pending`，由重試工人處理（退避 + 上限）

---

## 6) 排程與回補（Jobs）

* **特徵回補**（每小時 / 手動）

  * 讀資料湖/K 線 API → 按 `from_ts~to_ts` 重算 → `signals.features` upsert
  * 產生 `strategy_events(kind=FEATURE_RECOMPUTE, scope=window)`
* **Regime 計算**（每日 00:05）

  * 計 RV 序列與分位 → Redis `prod:regime:market:state`（含 `rev`, `ts`, `expiry_ts`）

---

## 7) 觀測性（SLI / 日誌 / 追蹤）

* **SLI**：`compute_latency_p50/p95`、`stream_lag_ms`、`emit_rate`、`dqc_fail_rate`、`recompute_throughput`
* **Logs（結構化）**：`event=compute_ok|dqc_fail|emit_ok|emit_skip`，附 `signal_id`, `rev`, `symbol`, `reasons[]`
* **Trace**：每一次 `signal_id` 產生一條 trace（span：fetch→compute→persist→emit）

---

## 8) 背壓與容錯

* **Streams 背壓**：使用消費者群組，`pending_idle > T` 的項目交由死信處理（告警 + 重試 N 次）
* **冪等**：以 `signal_id` 為 upsert 鍵；事件重放不會導致重複寫入
* **批量**：計算/寫入/推送皆支援批處理（對 DB 使用 batch upsert）
* **邊界值**：當 `spread_bps` 或 `depth_top1_usdt` 超出物理上限（配置），僅計記錄，不對外事件

---

## 9) 安全與資源

* **Least-Privilege**：只授予 Arango `WRITE signals/strategy_events` 權限；Redis 使用具名 ACL
* **資源建議**：`CPU 0.5–1 vCPU / MEM 512–2048MB`；GC Tunings：`GOGC=100~200`
* **限流**：對重計算 API 設單實例 QPS 限制（如 5 rps）與窗口大小限制（≤ 90 天）

---

## 10) API（對外）

### `POST /features/recompute`

```json
{
  "symbols": ["BTCUSDT"],
  "windows": ["1h","4h","1d"],
  "from_ts": 1735660800000,
  "to_ts": 1736265600000,
  "force": false
}
```

**回應**

```json
{
  "accepted": true,
  "task_id": "recomp-20250105-0001",
  "estimated_batches": 42
}
```

**校驗**（新增補充）

* `symbols`：必填、≥1；正則 `^[A-Z0-9]{3,}$`
* `windows`：必填，枚舉 `{1m,5m,1h,4h,1d}`
* `from_ts < to_ts`；跨度 ≤ 90 天
* 超額或參數錯誤 → `400/422` 與 `errors[]` 明細

### `GET /health`

* 內含相依健康：Arango、Redis、Config（rev staleness）、Stream Lag

---

## 11) 測試與驗收（DoD）

* **單元**：ATR/RV/ρ/Spread/Depth 純函數測試（門檻上下 ±ε）
* **契約**：對 `mkt:events` 假資料→`feat:events` 與 `signals` 內容一致；重放冪等
* **效能**：在 1k msg/s、3 symbols、`windows={1m,5m,1h}` 下 `compute_p95 ≤ 100ms`
* **韌性**：Redis 模擬 slot 移轉、WS 抖動，`pending` 能清空且無資料遺失
* **DQC**：缺欄/回跳/跳變案例可觸發預期動作（不發事件、仍入庫、記標記）

---

## 12) 交付物

* **程式目錄**

  * `/cmd/s2-feature/main.go`、`/internal/compute/*`、`/internal/streams/*`、`/internal/store/*`
* **設定**

  * Helm/ConfigMap：`S2_DB_ARANGO_URI`, `S2_REDIS_ADDRESSES`, `S2_SYMBOLS`, `S2_WINDOWS`, `S2_DQC_*`, `S2_REGIME_*`
* **儀表板**

  * Grafana：`S2 Overview`（SLI、Lag、DQC、Throughput）
* **Runbook**

  * Recompute 操作步驟、常見 DQC 告警與處置、回放與重試指南

---

## 13) Redis 與 DB 鍵位（補充對照）

* **Redis Keys**

  * `feat:last:<symbol>`（JSON，TTL 15m）
  * `prod:regime:market:state`（JSON：`regime`,`rev`,`ts`,`expiry_ts`）
  * `feat:{cache}:<symbol>`（滑窗快取，實作用本機記憶體＋可選 Redis 備援）
* **Redis Streams**

  * `mkt:events:<venue>:<market>:<symbol>`（in）
  * `feat:events:<symbol>`（out）
* **Arango Collections**

  * `signals`（含 `features`, `t0`, `symbol`, `config_rev`, `dqc`）
  * `strategy_events`（`FEATURE_RECOMPUTE`, `FEATURE_DQC_FAIL`）

---

## 14) 新增特徵的流程（工程視角）

1. **定義**：增補到 `factor_registry`（定義、頻率、lag、依賴）
2. **實作**：`internal/compute/<factor>.go`（純函數 + 單元測試）
3. **註冊**：`initializeFeatureCalculators()` 掛載
4. **配置**：在 `config_bundles` 的 factors 清單啟用
5. **驗證**：Recompute 一段樣本窗口；核對 `signals.features` 與事件
6. **上線**：灰度在 Canary，觀測 `dqc_fail_rate` 與 `compute_p95`

---

## 15) 風險與緩解

* **行情間歇/延遲**：決策心跳（由 S3 實作）會兜底，但 S2 需暴露 `stream_lag_ms` 指標
* **Redis Cluster 變更**：使用官方 cluster client，所有 `XREADGROUP/XADD` 具重試與 slot 重新定址
* **未來洩漏**：以 `t0` 為硬邊界寫防護；回補亦不跨界
* **大量回補**：限制每請求跨度 ≤ 90 天；分批任務＋進度回報

---

### 參考

* 你提供的 S2 README 與待辦清單（已合流到本文件的章節結構與待辦補充）。

> **備註**：以上為「增補版」製作清單，與你既有 S2 文件相容；工程可据此直接落 PRD、排期與任務分解。
