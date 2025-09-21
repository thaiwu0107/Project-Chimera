# S7 — Label Backfill（補充規格 v1.1）

> 本補充在你現有的 S7 說明之上 **新增** 更細的資料契約、演算法、排程、併發與觀測性規範，方便直接落地實作與契約測試。原有內容不變。

---

## 1) 任務範圍與責任邊界

* **職責**：為每筆 `signal` 計算 12/24/36 小時 **淨利標籤**（`net_roi` 與 `label ∈ {pos,neg,neutral}`），落庫並（可選）發事件，供迴歸訓練、Autopsy 與監控使用。
* **資料來源**：`signals`, `fills`, `funding_records`, `positions_snapshots`（FUT）、`holdings_spot_snapshots`（SPOT）等。
* **資料落點**：`labels_12h`, `labels_24h`, `labels_36h`（Upsert），以及 `strategy_events(kind=LABEL_WRITE)` 审计。
* **對外介面**：`POST /labels/backfill`（批次/單筆）、`GET /labels/{trade_id}`, `GET /labels/history`。
* **事件**（可選）：回填完畢後推送 `labels:ready` 事件；命中復盤觸發條件時，S8 可被喚起生成報告。

---

## 2) 時間窗與樣本選取

* **信號時間**：`t0 = signals.t0 (epoch ms)`。
* **回填視窗**：對每個 `H ∈ {12h,24h,36h}`，定義區間 `[t0, t0+H)`。
* **到期樣本**：`now() ≥ t0 + H` 且尚未存在對應 `labels_H(signal_id)`。
* **邊界（含等於）**：期末點採 **左閉右開**（不含 `t0+H` 之後的任何事件）。

---

## 3) PnL / ROI 計算（FUT 與 SPOT）

### 3.1 FUT（逐倉、USDT 結算）

* **方向**：`dir ∈ {+1,-1}`（多/空）；在計算 PnL 時以 `dir` 內生。
* **成交彙總**（於 `[t0, t0+H)`）：

  * 市價/限價成交序列 `fills`：記錄 `(price_i, qty_i, fee_i)`，方向以訂單側推得。
  * **已實現 PnL**：

    $$
    \text{PnL}_{\text{real}} = \sum_i dir \cdot (P_{\text{fill},i} - P_{\text{entry,lot},i}) \cdot q_i
    $$

    > 實作中通常直接由交易所回傳之每筆 realized PnL 累計，避免自行逐筆對沖。
* **未實現 PnL**（期末）：

  * 取 `P_mark@t0+H−`（期末前最新）

    $$
    \text{PnL}_{\text{unreal}} = dir \cdot (P_{\text{mark}} - P_{\text{entry,remain}})\cdot Q_{\text{remain}}
    $$
* **Funding**：聚合 `funding_records.amount_usdt` 於區間內。
* **費用**：聚合 `∑ fee_i`（含 maker/taker）。
* **分母（名目或保證金）**：逐倉以 **實際動用保證金總額** 近似：

  $$
  \text{Margin}_{[t0,t0+H]} = \sum_{\text{開倉/加倉}} \text{isolated\_margin}_{\text{lot}}
  $$
* **淨利 / ROI**：

  $$
  \text{net\_pnl} = \text{PnL}_{\text{real}} + \text{PnL}_{\text{unreal}} - \sum \text{Fees} - \sum \text{Funding}
  $$

  $$
  \text{net\_roi} = \frac{\text{net\_pnl}}{\text{Margin}_{[t0,t0+H]}}
  $$

### 3.2 SPOT（WAC 會計）

* **WAC 更新**（買入）：

  $$
  \text{WAC}'=\frac{Q\cdot\text{WAC}+Q_{\text{buy}}\cdot P_{\text{buy}}}{Q+Q_{\text{buy}}}
  $$
* **賣出已實現**：

  $$
  \text{PnL}_{\text{real}}=(P_{\text{sell}}-\text{WAC})\cdot Q_{\text{sell}}
  $$
* **未實現**（期末）：

  $$
  \text{PnL}_{\text{unreal}}=(P_{\text{mark}}-\text{WAC})\cdot Q_{\text{remain}}
  $$
* **費用**：聚合交易所費用。
* **分母**：採 **成本基礎**（倉位啟動後投入之 USDT 成本），或以初始名目近似；專案預設：**成本基礎**。
* **ROI**：同 FUT 公式，把 Funding 項移除。

> **精度**：金額保留 1e-8、ROI 保留 1e-6（統一四捨五入規則）。

---

## 4) 標籤規則（可配置）

* 預設門檻（可於 `config_bundles.flags.labels` 調整）：

  * `pos`：`net_roi ≥ +0.005`
  * `neg`：`net_roi ≤ -0.005`
  * `neutral`：其他
* **邊界一致性測試**：在 `±0.005 ± ε` 皆需覆蓋。

---

## 5) 資料契約（讀/寫）

### 5.1 讀（主要）

* `signals(signal_id, t0, symbol, decision, config_rev, ...)`
* `fills(order_id, fill_id, price, qty, fee_usdt, mid_at_send, ts, side, market)`（需以 `trade_id/signal_id` 或時窗關聯）
* `funding_records(symbol, funding_time, amount_usdt, funding_rate)`（FUT）
* `positions_snapshots(symbol, ts, roe, isolated_margin_usdt, ...)`（FUT）
* `holdings_spot_snapshots(asset, ts, total_qty, wac_usdt, ...)`（SPOT，可選）

### 5.2 寫（Upsert）

* `labels_12h / 24h / 36h`（既有 JSON Schema，鍵為 `signal_id + horizon_h`）
* `strategy_events(kind=LABEL_WRITE, detail={signal_id,horizon,net_roi,label,calc_ms})`

> **冪等鍵**：`(signal_id, horizon_h)`；重算以 UPSERT 與審計事件新建一筆版本化 detail。

---

## 6) Redis 佈局（鍵/Streams）

* `labels:last_backfill_ts`：上次批次的完成時間（ms）
* `labels:queue`（ZSET，score=due\_ts）：到期待計算佇列
* `labels:lock:<signal_id>:<H>`：細粒度鎖（NX，TTL=2×批次逾時）
* `labels:ready`（Stream，可選）：`{signal_id,horizon,net_roi,label,ts}`—供 S8 監聽觸發 Autopsy
* `health:labels:stats`（HASH）：S7 健康快照（成功/失敗、p95 延遲等）

---

## 7) API 介面（擴充）

* `POST /labels/backfill`
  **Body**：

  ```json
  {
    "horizon_h": [12,24,36],
    "from_ts": 0,
    "to_ts": 0,
    "limit": 500,
    "dry_run": false,
    "emit_ready_event": true
  }
  ```

  **200**：

  ```json
  {
    "scheduled": 123,
    "computed": 117,
    "skipped": 6,
    "duration_ms": 8421,
    "errors": []
  }
  ```
* `POST /labels/recalculate`（單/批信號重算；以 `(signal_id,h)` 指定）
* `GET /labels/{signal_id}`（彙整 12/24/36h）
* `GET /labels/history?symbol=BTCUSDT&h=24&from_ts=...&to_ts=...`

> **權限**：`/recalculate` 限管理者；其餘唯讀。

---

## 8) 排程/工作流

* **掃描器**（每 15 分鐘）：

  1. 查詢 `signals`：`t0+H ≤ now` 且 `labels_H` 不存在，批量入 `labels:queue`。
  2. 觀察 `labels:queue`，對 `score ≤ now` 的項逐一鎖定並計算。
* **事件觸發**：可額外監聽 `pos:events(EXIT)`—提早對「已平倉」交易嘗試寫 12h Label（若到期）；其餘 H 仍跟隨排程。
* **退避**：Arango/交易所 API 錯誤 → `backoff = min(max, base·2^k)+jitter`，入重試佇列（ZSET）。

---

## 9) 併發/鎖/冪等

* **批次鎖**：`labels:lock:batch`（避免多副本同時掃描同一批到期集合）
* **單樣本鎖**：`labels:lock:<signal_id>:<H>`（確保單樣本單工作執行）
* **冪等**：Upsert 以 `(signal_id,horizon_h)` 作唯一鍵；重入不重複寫。
* **超時與補償**：計算任務超時 → 釋放鎖並入重試佇列；連續 N 次失敗 → `alerts(ERROR)`。

---

## 10) 觀測性（SLI/SLO / 指標）

* **核心 SLI**

  * `backfill_success_rate = success / (success+fail)`
  * `compute_latency_p50/p95`（單樣本）
  * `queue_lag_ms`（now - due\_ts）
  * `roi_label_shift`（本批 vs 近 7 日的 label 比例漂移，KS 或 χ²）
* **SLO（建議）**

  * 成功率 ≥ 99.0%
  * 單樣本 p95 ≤ 200ms（不含外部 I/O）
  * 平均 queue lag ≤ 5 分鐘
* **導出**：Prometheus 指標 + `health:labels:stats`（HASH 快照）+ `strategy_events` 審計。

---

## 11) 錯誤矩陣與降級

| 類別     | 例                              | 動作                                                       |
| ------ | ------------------------------ | -------------------------------------------------------- |
| 資料缺失   | `fills` 空、`funding_records` 缺段 | 記 `LABEL_INCOMPLETE`，以可得資料估算；標記 `label_source="partial"` |
| API 錯誤 | Arango timeout                 | 退避重試、溢出入重試佇列                                             |
| 邏輯衝突   | FUT/ SPOT 同信號歧義                | 以 `decision.market` 優先；衝突入審計                             |
| 漂移異常   | 本批 label 分佈偏離 7 日均值 > 門檻       | `alerts(WARN)`；標記批次，供人工覆核                                |

---

## 12) 與其他服務的交互（落地順序）

1. **讀取**
   S7 → Arango：`signals`（掃描到期）→ `fills`、`funding_records`、`positions_snapshots`（FUT）/ `holdings_spot_snapshots`（SPOT，若啟用）。
2. **計算**
   依第 3 節公式產生 `net_roi` 與 `label`。
3. **寫入**
   Upsert 至 `labels_12h/24h/36h`；寫 `strategy_events(kind=LABEL_WRITE)`。
4. **事件（可選）**
   推 `labels:ready`（Stream），由 S8 訂閱觸發 Autopsy（若符合觸發器）。
5. **監控**
   S7 → S11：推送計量指標（成功率、延遲、漂移）；S11 合成健康度。

---

## 13) 契約測試（Happy/Edge）

* **Happy**

  * FUT：單向多筆 fills + Funding，`ROI_net` 正確；Upsert 成功且冪等。
  * SPOT：多筆買/賣，WAC 正確，期末未實現計算準確。
* **Edge**

  * 視窗邊界：`ts = t0+H−ε` / `ts = t0+H+ε`；僅前者納入。
  * 缺資料：`funding_records` 缺失 → 以 0 估算並標記 `partial`。
  * 重入：相同 `(signal_id,H)` 多次提交 → 僅一次落庫。
  * 大量樣本：`labels:queue` 高水位 → 批次/並行正確分片與鎖。

---

## 14) 範例：回填請求與結果

**POST /labels/backfill**

```json
{
  "horizon_h": [12, 24],
  "from_ts": 1737072000000,
  "to_ts": 1737158400000,
  "limit": 1000,
  "dry_run": false,
  "emit_ready_event": true
}
```

**200 OK**

```json
{
  "scheduled": 432,
  "computed": 417,
  "skipped": 15,
  "duration_ms": 9640,
  "errors": []
}
```

**labels\:ready（Stream 條目，選用）**

```json
{
  "signal_id": "sig_20250117_000123",
  "horizon_h": 24,
  "symbol": "BTCUSDT",
  "net_roi": 0.0063,
  "label": "pos",
  "ts": 1737244800123
}
```

---

## 15) 實作提示

* **查詢優化**：對 `fills.timestamp`, `funding_time`, `signals.t0` 建索引；以 `symbol + 時窗` 抽取，減少全掃。
* **批次大小**：`batch_size` 與 `worker_concurrency` 可在 Config Bundle flags 中配置；建議初期 200–500。
* **精度統一**：金額 1e-8；ROI 1e-6；避免跨服務呈現差異。
* **審計**：所有回填操作落 `strategy_events`，含 `calc_ms`, `partial`, `source_rev`（config\_rev）以備追溯。

---

> 以上補充與你既有的 S7 說明相互對齊，新增了可直接實作的演算法細節、鍵位命名、排程/鎖/冪等與觀測性規範，確保能以最少反覆打磨出穩定的標籤服務。
