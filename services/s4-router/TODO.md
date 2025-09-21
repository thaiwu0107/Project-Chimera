# Project Chimera — S4 Order Router 補充規格（工程版 v1.0 / 未實作）

> 本章彙整並細化 S4 的行為、資料落地與與他服務互動，作為直接開發與契約測試的依據。內容與你先前的 S4 草案對齊並擴充缺漏（API、狀態機、Redis/DB 寫點、排程、觀測性、回退策略）。

---

## 1) 服務定位與範疇

* **職責**：將上游（S3 策略、S6 倉位管理、S5 對帳）下達的**訂單意圖**轉為實際交易所指令，並管理訂單完整生命週期（建立、部分成交、成交、撤單、OCO/守護停損、TWAP）。
* **市場**：Binance **FUT**（USDT 永續）與 **SPOT**（現貨）。
* **執行策略**：Maker→Taker 回退、TWAP 拆單、OCO（若不支援則守護停損）。
* **狀態**：❌ 未實作（本文件做為實作藍圖）。

---

## 2) 對外 API（HTTP）

### 2.1 `GET /health`

* **回傳**：`HealthResponse{status: OK|DEGRADED|DOWN, checks: [redis, arango, exch, cb_state, route_rev]}`
* **用於**：K8s liveness/readiness、S11 健康巡檢。

### 2.2 `POST /orders`

* **用途**：接受上游**訂單意圖**，執行 FUT/SPOT 下單、掛 SL/TP 或 OCO、啟動守護、或進入 TWAP 排程。
* **請求（要點）**：

  * `intent.intent_id`（冪等鍵，必要）
  * `market` ∈ {`FUT`,`SPOT`}
  * `symbol`（例：`BTCUSDT`）
  * `side` ∈ {`BUY`,`SELL`}
  * `qty`（依 `stepSize` 四捨五入）
  * `exec_policy`：

    * `maker_then_taker.wait_ms`（路由表給定或覆蓋）
    * `twap.{slices,gap_ms,jitter_ms?}`
    * `oco`（SPOT）：`tp_px`,`sl_px`
    * `guard_stop.enable=true`（SPOT fallback）
  * `reduce_only`（平倉/減倉）
  * **FUT 附帶**：`post_fill_actions.stop_loss`, `take_profit`（`STOP_MARKET` / `TAKE_PROFIT_MARKET`）
* **回應**：`OrderResult{status, order_id|oco_legs[], avg_price?, filled_qty?, slippage_bps?, message?}`
* **冪等**：相同 `intent_id` 必須回同結果（或查回先前結果）。

### 2.3 `POST /cancel`

* **用途**：取消**任一**掛單（單腿 / OCO 組）。
* **請求**：`{ order_id? | client_order_id?, cascade_oco?: true, reason?: "REBALANCE|RISK|ORPHAN|..." }`
* **回應**：`CancelResponse{status, canceled_ids[], message?}`
* **冪等**：重複取消需回相同結果或「已取消」。

---

## 3) 服務內部狀態機（Orders）

```
NEW → PARTIALLY_FILLED → FILLED → CLOSED
  ↘ (CANCEL_REQ) → CANCELED
  ↘ (REJECTED) 
```

* **OCO 組**：`OCO_GROUP[LEG_TP, LEG_SL]`，任一 `FILLED` → 另一腿自動 `CANCELED`。
* **守護停損**：Client-side watcher 觸發 → 即刻下保護單（或先撤入場）。

---

## 4) 寫入點（ArangoDB / Redis）

### 4.1 ArangoDB Collections（寫入時機）

* `orders`：建立/狀態變更（`NEW|PARTIALLY_FILLED|FILLED|CANCELED|REJECTED`），含 `rcu_rev`、`client_order_id`、`exec_policy` 快照。
* `fills`：每筆成交回報（含 `mid_at_send`, `book_top3`, `slippage_bps`）。
* `strategy_events`：

  * `ROUTER_ORDER_ACCEPTED/EXECUTED/CANCELED/REJECTED`
  * `OCO_ARMED|OCO_FALLBACK|GUARD_ARMED|GUARD_TRIGGERED`
  * `TWAP_SLICE_SENT|TWAP_DONE`
* **索引（建議）**：

  * `orders`: hash(`order_id`), hash(`client_order_id`), skiplist(`symbol`,`created_at`)
  * `fills`: hash(`fill_id`), skiplist(`timestamp`,`order_id`)

### 4.2 Redis（鍵與 Streams / ZSet）

* **冪等鍵**：`idem:order:{client_order_id}` → TTL 24h
* **TWAP 佇列**（ZSet）：`prod:exec:twap:queue`；score = `due_ts`；value = JSON 切片任務
* **守護停損狀態**（Hash/Key）：`guard:spot:{symbol}:{intent_id}`（`armed=1, armed_at`）
* **執行結果 Stream**：`ord:results`（下發回報給 S6/S5）
* **路由參數表**（Hash/JSON）：`router:curves:{symbol}`（`maker_wait_ms vs notional`、`twap_slices vs notional` …）
* **熔斷狀態**：`router:cb:state=OPEN|HALF|CLOSED`（附 rolling error rate）
* **臨時行情快照**（由 S1/S2 寫）：`mkt:snap:{symbol}`（mid/spread/depth\_topN）

---

## 5) 執行算法與邏輯

### 5.1 Maker→Taker 回退

* **步驟**：

  1. 依路由表計算 `wait_ms = f(notional, spread, depth)`；
  2. 先掛 **被動限價**（Post-Only / 不吃單）；
  3. 視窗內若部分成交率 `φ<φ_min` 或超時 → 撤剩餘、以 `MARKET` 完成。
* **滑價紀錄**：`slip_bps = sign(side) * (VWAP - mid_at_send)/mid_at_send * 1e4`（寫 `fills`）。

### 5.2 TWAP 拆單

* **調度**：總量 `Q`、切片 `n=ceil(Q/slice)`、間隔 `Δt`；
  `t_i=t0+i·Δt`、`q_i=Q/n` 或 `q_i ∝ sqrt(i)`（正規化）。
* **執行**：每片視情況走 Maker→Taker；未完 → 重新入列 ZSet（下一 `due_ts`）。

### 5.3 SPOT：OCO / 守護停損

* **原生 OCO 支援**：直接建立雙腿（TP/SL）；
* **不支援** → **守護**：

  * 先下 `LIMIT_MAKER` 入場；
  * **停損先到**：撤入場（若未成），可選反手；
  * **成交後**：立刻掛 `STOP_MARKET` ；
  * 守護 Watcher 以 WS/輪巡監控中間價觸發。

### 5.4 交易規則與 Rounding

* 檢查 `minNotional`、`tickSize`、`stepSize`；`round_to_tick/step`。
* FUT：確保 `marginType=ISOLATED`、`leverage=20`；掛 SL/TP 使用 `workingType=MARK_PRICE` 且 `reduceOnly=true`。

---

## 6) 風險守門與回退

* **Kill-switch**：`kill_switch=ON` 時阻擋**新倉**，允許平倉/風險單。
* **滑價/流動性上限**：若預估滑價或 `spread` 超閾 → 改限價或縮片/延遲。
* **熔斷**：近 60s `5xx/429` 超閾 → `cb:OPEN`（暫停新開），觀察窗結束後半開再閉合。
* **失敗補償**：API 超時 → 按冪等鍵重試；最終失敗發 `alerts(ERROR|FATAL)`。

---

## 7) 定時任務（Cron / Ticker）

* **TWAP tick（1–3s）**：ZSet 取到期切片 → 下單 → 未完再入列。
* **殘單清理（1–5m）**：比對交易所 `openOrders` vs DB；孤兒單 → 撤；過期單 → 策略處置；計 Jaccard 一致率。
* **路由表滾動更新（每日）**：基於近 30 日 TCA（由 S11 提供）回灌 `router:curves:*`。

---

## 8) 觀測性與 SLO / 告警

* **核心 SLI**：

  * `router.submit.latency_ms`（p50/p95）
  * `maker.fill_ratio`、`taker.rate`
  * `twap.slice_fill_ratio`、`twap.backlog`
  * `slippage.bps.p50/p95`、`fee.usdt.total`
* **SLO（示例）**：

  * `/orders` 成功率 ≥ 99.5%
  * 端到端 p95 ≤ 800 ms（Maker 視窗外）
* **告警**：

  * 熔斷打開 / 長期 `maker_fill_ratio` 過低 / `insufficient_balance_rate` 過高
  * OCO 守護落入 fallback 比例異常

---

## 9) 交易所整合（Binance 重點）

* **FUT**：`/fapi/*` REST、私有 WS 訂閱 `ORDER_TRADE_UPDATE`；
* **SPOT**：`/api/*` REST、私有 WS `executionReport`；
* **記得**：REST 429/418 節流、簽名時鐘 `Δt` 修正、重放防重（`recvWindow` 與本地時鐘校正）。

---

## 10) 與他服務的互動（Call Matrix）

| 呼叫方 → S4      | 入口             | 行為                 | S4 對外落地                        |
| ------------- | -------------- | ------------------ | ------------------------------ |
| S3 Strategy   | `POST /orders` | 新倉 / 平倉 / 改價策略     | `orders`、`fills`、`ord:results` |
| S6 Position   | `POST /orders` | 移動停損、分批止盈、加倉       | 同上（附 `reduceOnly`）             |
| S5 Reconciler | `POST /cancel` | 孤兒單/過期單處置          | `orders(status=CANCELED)`、事件   |
| S12 Web/UI    | `POST /cancel` | 手動撤單               | 同上                             |
| S11 Metrics   | （無直呼）          | 拉 Prom/或 Stream 彙整 | `router:*` 指標事件（可選）            |

---

## 11) 契約與邊界檢核（必測）

* **/orders（FUT Maker→Taker）**：限價視窗內成交→`FILLED`；逾時回退→`FILLED`，並有 `slippage_bps`。
* **/orders（SPOT OCO）**：雙腿成功→`ACCEPTED` 並顯示 `legs[]`；一腿失敗→自動 fallback 守護或市價入場。
* **冪等性**：相同 `intent_id` 重送 → 回相同結果。
* **規則**：`qty<stepSize` / `price` 非 `tickSize` 整數倍 → `400`; `minNotional` 未達 → `422`; `reduceOnly=true` 新倉 → `422`。
* **/cancel**：不存在 `404`、已成 `409`、已撤回 `200`。

---

## 12) 組態與機密

* **環境變數（示例）**

  * `S4_EXCHANGE=binance` / `S4_TESTNET=true`
  * `S4_BINANCE_KEY/SECRET`（K8s Secret）
  * `S4_REDIS_ADDRESSES`、`S4_DB_ARANGO_*`
  * `S4_ROUTE_TABLE_PATH=/etc/chimera/route.json`（或改從 `router:curves:*`）
  * `S4_CB_ERROR_RATE_WINDOW=60s`、`S4_CB_ERROR_RATE_THRESH=0.2`
  * `S4_MAKER_MAX_WAIT_MS=3000`、`S4_TWAP_MAX_SLICES=4`
  * `S4_OCO_GUARDIAN=true`、`S4_WORKING_TYPE=MARK_PRICE`

---

## 13) 開發優先級（落地順序）

1. **基本下單與撤單（FUT/市價）** → 2) **Maker→Taker** → 3) **SPOT OCO + 守護** → 4) **TWAP** → 5) **殘單清理/對帳介面配合** → 6) **觀測性與熔斷**。

---

## 14) Ready 定義（驗收）

* **功能驗收**：連續 1000 筆意圖成功率 ≥ 99%，端到端 p95 ≤ 500ms；OCO 守護 100% 互斥；TWAP 無漏片。
* **資料驗收**：`orders/fills/strategy_events` 欄位完整；`ord:results` 與 DB 一致；`slippage_bps` 可信。
* **可靠性**：API 超時/重啟後，依 `intent_id` 能完全恢復訂單狀態；熔斷能阻擋新倉。

---

> 附註：本節已將你先前列出的待辦（TWAP 佇列、OCO 守護、冪等/熔斷、DB/Streams 寫點、契約測試條目、指標與告警等）整併為一份**可直接依序開工**的工程規格；細節（如各欄位與 JSON 例、錯誤碼行為）保持與既有白皮書/規格書一致，以利跨服務協作與後續自動化測試。&#x20;
