# Project Chimera — S6 Position Manager 補充說明（工程版）

> 本文件補充 S6 在整體系統中的**角色、資料流、演算法、API、Redis/DB 操作、排程與觀測性**，可直接對照落地實作。

---

## 1. 職責與邊界

**S6 的核心任務：**

* 監控並治理**期貨/現貨**持倉：移動停損、分批止盈、加倉與強制風險削減。
* 驅動**執行動作**（委託 S4 下單/撤單），確保冪等與補償。
* 管理**資金側**補充（自動/半自動劃轉，透過 S1/S12）。
* 維護**持倉快照**（DB）與**即時狀態**（Redis）。
* 產出**事件**（pos\:events）供 S8/S11 等下游使用。

**不做的事：**

* 不直接與交易所下單（委派 S4）。
* 不訓練或推理 ML 模型（僅消費 S3/Config 的規則/閾值）。

---

## 2. 上游/下游接口與數據流

* **輸入（讀）**

  * 行情/深度：`mkt:*`（Redis Streams，來源 S1）
  * 特徵：`feat:events:*`（S2）
  * 訂單結果：`ord:result:*`（S4）
  * 配置：S10 `GET /active` + `cfg:events`（Redis BroadCast）
  * 健康與門檻：S11 聚合的 health 狀態（`prod:health:*`）

* **輸出（寫/呼叫）**

  * **下單/撤單**：S4 `POST /orders`、`POST /cancel`
  * **資金劃轉（自動/半自動）**：S1 私有 `/xchg/treasury/transfer`（由 S6 直呼或透過 S12 審批）
  * **持倉快照**：Arango `positions_snapshots`
  * **事件**：`pos:events:{SYMBOL}`（Redis Streams）
  * **Redis 狀態**：`pos:{sl}/pos:{tp}/pos:{adds}` 系列 Key

---

## 3. 主要演算法與判斷（S6 內純函式）

### 3.1 ROE/強平距離/盈虧

* **方向內生 PnL**：`PnL = (P_mark - P_entry_avg) * Q * dir`
* **ROE**：`ROE = PnL / isolated_margin`
* **強平緩衝**：`LB = |P_mark - P_liq| / P_mark`（低於閾值觸發**降風險**）

### 3.2 移動停損（Trailing Stop）

* ATR 型（多倉）：

  * `SL = max(SL_prev, P_entry - k_regime * ATR)`（空倉對稱）
* **鎖利 ROE 型**（多倉）：

  * 達門檻 `ρ*` 時，`SL = max(SL_prev, P_entry + (ρ* * isolated_margin) / Q)`
* 進入更高鎖利階 `θ_k` → 撤舊掛新（reduceOnly）

### 3.3 分批止盈（Ladder TP）

* 例：`ROE ≥ ρ_1` → 平 `β_1 * Q`；`ROE ≥ ρ_2` → 再平 `β_2 * Q_remaining` …
* 每次平倉後需**重算均價、保證金**，並同步**更新 SL/TP 階梯**。

### 3.4 加倉（順勢）

* 觸發：同向新信號 + `ROE ≥ gate` + `add_on_count < cap`
* 數量：`Q_add(j) = Q0 * γ^j`（`0 < γ < 1`，幾何遞減）
* 上限：`Σ Q_add(j) ≤ η * Q0`（總加倉倍數限制）

### 3.5 SPOT 會計（WAC）

* **買入**：`WAC' = (Q * WAC + Q_buy * P_buy) / (Q + Q_buy)`
* **賣出已實現損益**：`PnL_real = (P_sell - WAC) * Q_sell`
* **未實現損益**：`(P_mark - WAC) * Q_remain`

### 3.6 資金劃轉（Auto Treasury）

* `need = max(0, min_free_fut - free_fut)`
* 若 `free_spot ≥ need + spot_buffer` → 造 `transfer_id` → （半自動）送 S12 審批 → S1 執行
* **冪等**：`transfer_id` 作去重

---

## 4. API（入向）與範例

### 4.1 `GET /health`

* 內容：Redis/Arango/依賴服務探活、WS Lag、管理 Tick 延遲

### 4.2 `POST /positions/manage`

* 用途：立刻執行一次治理（外部/維運觸發）
* Request（示例）

```json
{
  "symbol": "BTCUSDT",
  "market": "FUT",
  "position_id": "pos_001",
  "actions": ["EVAL"], 
  "reason": "manual-eval"
}
```

* Response（示例）

```json
{
  "plan": {
    "plan_id": "plan_001",
    "actions": [
      {"type": "STOP_MOVE", "old": 43000.0, "new": 44000.0},
      {"type": "TP_PARTIAL", "qty": 0.33}
    ]
  },
  "orders": [
    {"kind":"CANCEL","target":"SL#abcd"},
    {"kind":"NEW","type":"STOP_MARKET","reduceOnly":true,"stopPx":44000.0}
  ]
}
```

### 4.3 `POST /auto-transfer/trigger`

* 用途：外部（S12）或策略（S6）觸發一次資金補充的流程（可配置自動/半自動）
* 參照前述 `TransferRequest/Response` 型別

---

## 5. 與 S4 的實作契約（出向）

* **下單/撤單**：`POST /orders` / `POST /cancel`

  * 冪等鍵：`intent_id` / `client_order_id`（由 S6 統一生成）
  * **STOP\_MOVE 流程**：先 `/cancel` 舊 SL（等待回報）→ `/orders` 新 SL（`reduceOnly=true`）
  * **TP\_PARTIAL**：直接 `/orders` 市價（`reduceOnly=true`）
  * **ADD\_ON**：`/orders` 新單，`add_on_count++`

* **錯誤與補償**

  * 5xx/429 退避重試（固定+抖動），重試仍失敗 → 記 `alerts(ERROR)` 並標記 **部分成功**
  * 未決定義務：重試佇列（ZSET：到期時間）

---

## 6. DB 寫入與索引（ArangoDB）

* `positions_snapshots`

  * **寫入時機**：每次治理前後、接獲 `ord:result` 後
  * **必填**：`symbol, side, entry_price_avg, isolated_margin_usdt, unrealized_pnl_usdt, roe, add_on_count, ts`
  * **索引**（建議）

    * hash：`symbol`
    * skiplist：`ts`, `symbol+ts`（時間序查詢）

* `treasury_transfers`（若採半自動審批）

  * **狀態**：`PENDING`→`APPROVED`→`EXECUTED`/`FAILED`
  * **索引**：hash(`transfer_id`), skiplist(`ts`)

> 初始化 Collections/索引已在 Init Job 規範，S6 僅需按契約寫入。

---

## 7. Redis Key/Streams（狀態與事件）

* **狀態**

  * `pos:sl:level:{pos_id}` → 當前 SL 階段（k、價格、時間）
  * `pos:tp:ladder:{pos_id}` → TP 階梯配置與進度
  * `pos:adds:{pos_id}` → 已加倉次數、時間戳
  * `risk:budget:*` / `risk:concurrency:*` → 配額釋放（倉位關閉時）

* **事件**

  * `pos:events:{SYMBOL}`（XADD）

    * `ENTRY`/`EXIT`/`ADD`/`STOP_MOVE`/`TP_PARTIAL`/`STAGNATED`/`RISK_DOWNGRADE`
  * **重試佇列**（ZSET）

    * `pos:retry:z`：到期後重新執行未完成的 STOP\_MOVE/TP\_PARTIAL

* **鎖**

  * `lock:pos:{pos_id}`（SET NX PX ttl）→ 單筆治理臨界區
  * `lock:treasury:{from}:{to}` → 劃轉互斥

---

## 8. 排程與事件觸發

* **管理 tick**（每 10–30s）：全量巡檢 + 事件優先（行情觸發）
* **自動劃轉 tick**（每 5m）：餘額與風險巡檢
* **冷啟**：拉取 S10 `/active`、回補各 `pos:*` 狀態（必要時從 DB 重建）

---

## 9. 觀測性（SLI/SLO）與告警

* **SLI**

  * `/orders` 端到端 p95（含等待/撤舊掛新）
  * `stop_move_success_rate`、`tp_partial_success_rate`
  * `maker_wait_timeout_rate`（來自 S4 回報匯總）
  * `stagnated_count`（停滯筆數）
  * `treasury_autoxfer_count/success_rate`

* **SLO（示例）**

  * `p95_exec ≤ 800ms`、`stop_move_success_rate ≥ 99%`
  * `stagnated_count` 7d MA 不上升

* **告警**

  * 連續 StopMove 失敗 → `ERROR`
  * 退避達上限或鎖持有超時 → `FATAL`（降級/暫停加倉）

---

## 10. 測試計畫（契約/單元/整合）

* **單元（純函式）**

  * `next_stop_price(ATR, regime, SL_prev, ...)`
  * `tp_ladder_decision(roe, ladder_state, ...)`
  * `add_on_decision(roe, add_on_count, gates, ...)`

* **契約（S4/S1/S12）**

  * `/orders`：冪等與退避；`STOP_MOVE` 的撤舊掛新順序與一致性
  * `/xchg/treasury/transfer`：Idempotency-Key 重放一致

* **整合（E2E）**

  * FUT 入場 → SL 掛單 → ROE 達門檻 → StopMove → TP\_PARTIAL → 退出
  * SPOT 成交 → WAC 更新 → 守護停損觸發 → 市價對沖

---

## 11. 失效情境與補償機制

* **WS/行情中斷**：降級為固定頻率輪詢；守護停損使用本地 mid 快照
* **Redis Cluster slot 移轉**：採用官方 cluster client；關鍵寫操作具重試與觀察 TTL
* **S4 不可用**：將治理動作入重試 ZSET；觸發降級（只做風險削減，不加倉）
* **配置不一致**：收到 `cfg:events` 但 `/active` 拉取失敗 → Backoff 重拉，保留舊版執行

---

## 12. 實作優先順序（S6 內）

1. **Stop/TP 基礎治理**（含撤舊掛新、冪等、重試）
2. **分批止盈 & 加倉**（含配額釋放與併發守門）
3. **SPOT 會計（WAC）與守護停損**
4. **自動資金劃轉（半自動→全自動）**
5. **觀測性/告警與回歸測試套件**

---

## 13. 最小可交付（MVP）核查清單

* [ ] `POST /positions/manage` 能產出正確 `ManagePlan` 與對應 `/orders` 輸出
* [ ] 新倉成立後**秒級**掛上 `STOP_MARKET reduceOnly`
* [ ] ROE 達門檻自動**撤舊掛新**，並寫入 `pos:sl:level:*`
* [ ] `positions_snapshots` 能回放單筆交易全生命週期
* [ ] 觀測面板可見 `stop_move_success_rate / p95_exec / stagnated_count`

---

> 備註：本補充與先前 **白皮書 v3 / Hop-by-Hop 規格 / 功能規格書** 一致，僅將 S6 的**介面與責任**具體化到可開發 + 可測試的層級；各公式與門檻值由 S10 的 **Active Bundle** 提供，並以 RCU 熱載。
