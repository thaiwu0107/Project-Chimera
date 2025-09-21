# S5 Reconciler — 補充規格（整合版 v3）

> 本節在你現有的 S5 草稿與我們前述白皮書 v3 的行為/資料流之上，拉齊**職責邊界、API 契約、數據讀寫點、演算法細節、排程與 Observability**，讓工程可直接落地。內容延伸自你提供的 S5 初稿清單（功能、API、對帳模式、孤兒處置、定時任務等）。

---

## 1) 服務定位與職責邊界

* **定位**：唯一權威的**對帳與事務狀態修復引擎**。負責比對「交易所真相」與「本地 DB」的**訂單/持倉/餘額**一致性，並按**保守回收原則**執行修復與風險降低（撤單、平倉、狀態修復與審計）。
* **只做三件事**：

  1. **發現差異**（集合/數量/狀態）
  2. **採取行動**（撤單/接管/平倉/修 DB）
  3. **留下痕跡**（`strategy_events`、審計、alerts、metrics）
* **與其他服務的邊界**：

  * **下單/撤單**一律透過 **S4 Order Router** 的 API（不可繞過）。
  * **行情/帳戶快照**走**交易所 REST**（標準化器在 S5 內），不依賴 S1 的 WS；必要時可利用 S1 的共享憑證與限流器。

---

## 2) 外部相依與資料讀寫

### 2.1 讀取（入向資料）

* **交易所 REST**（必要）

  * FUT：`GET /fapi/v2/openOrders`、`GET /fapi/v2/positionRisk`
  * SPOT：`GET /api/v3/openOrders`（必要時加資產餘額端點）
  * **目的**：取得「真相集」：`O_ex`（掛單集合）、`P_ex`（持倉集合）、`B_ex`（餘額）
* **ArangoDB（本地 DB）**

  * 讀 `orders`（狀態∈{NEW,PARTIALLY\_FILLED}）、`positions_snapshots`（最近 T 小時滾動快照）
  * **目的**：取得「本地集」：`O_db`、`P_db`

### 2.2 寫入（出向資料）

* **S4 API**

  * `POST /cancel`：撤孤兒掛單或過期掛單
  * `POST /orders`（僅限保守平倉/回收，用 `reduceOnly=true`）
* **ArangoDB**

  * `strategy_events(kind=RECONCILE_*)`：完整審計
  * 修正 `orders`/`positions_snapshots` 真相對齊（單向以交易所為準）
* **Redis（Cluster）**

  * `recon:last_run_ts`（ZSET/Key）
  * `recon:status:<reconcile_id>`（HASH：state/progress/counters）
  * `recon:events`（XADD）：對帳結果/動作摘要
  * `alerts`（XADD）：嚴重不一致或操作失敗告警（ERROR/FATAL）

---

## 3) API 契約（最小集合）

### 3.1 `GET /health`

* **200 OK**：`{"status":"UP","checks":[...]}`
* 檢查：Redis 可用、Arango 可用、交易所 REST 節點探活（可選）

### 3.2 `POST /reconcile`

**Request（建議版）**：

```json
{
  "mode": "ALL",                   // ALL | ORDERS | POSITIONS | HOLDINGS（擴充）
  "symbols": ["BTCUSDT"],          // 空=全
  "markets": ["FUT","SPOT"],       // 空=全
  "from_time": 0,                  // 可選：限定比對時間窗（ms）
  "to_time": 0,                    // 可選：預設 now()
  "dry_run": false,                // 只報告不動手
  "orphan_policy": "CONSERVATIVE", // RECLAIM_IF_SAFE | CONSERVATIVE
  "time_window_h": 72              // 比對窗口，預設 72h
}
```

**Response（建議版）**：

```json
{
  "reconcile_id": "rc_20250921_001",
  "status": "COMPLETED",
  "summary": {
    "orders_matched": 150,
    "orders_orphaned_api_only": 2,
    "orders_orphaned_db_only": 1,
    "positions_matched": 10,
    "positions_orphaned_api_only": 1,
    "positions_orphaned_db_only": 0,
    "discrepancies": 4,
    "jaccard_orders": 0.982,
    "jaccard_positions": 0.955,
    "fixed": 2,
    "adopted": 1,
    "closed": 1
  },
  "actions_taken": [
    {"type":"CANCEL_ORDER","order_id":"...","reason":"API-only"},
    {"type":"ADOPT_ORDER","order_id":"...","reason":"safe to reclaim"},
    {"type":"CLOSE_POSITION","symbol":"...","reason":"DB-only"}
  ]
}
```

> 上述結構與你草稿中的回應欄位概念一致；多補「雙向孤兒分類、Jaccard 指標、採取動作分類」以利可觀測性與審計。

**欄位校驗（摘）：**

* `mode` ∈ {ALL, ORDERS, POSITIONS, HOLDINGS}，預設 ALL
* `time_window_h` ∈ \[1, 168]，預設 72
* `orphan_policy` ∈ {RECLAIM\_IF\_SAFE, CONSERVATIVE}（預設保守）

---

## 4) 差異檢測與演算法（可直接實作）

### 4.1 集合層（是否存在）

* **集合定義**：

  * 訂單主鍵：`(symbol, client_order_id)` 或 `order_id`
  * 持倉主鍵：`(symbol, side)`（按交易所定義聚合）
* **集合差異**：

  * `API-only = O_ex \ O_db`（孤兒掛單：交易所有/本地無）
  * `DB-only  = O_db \ O_ex`（殘留掛單：本地有/交易所無）
* **一致率（Jaccard）**：

  * $J=\frac{|S_{ex}\cap S_{db}|}{|S_{ex}\cup S_{db}|}$（訂單、持倉各算）

### 4.2 參數層（數量/價格/狀態）

* **數量容差**：

  * $|qty_{ex}-qty_{db}|\le \epsilon_{qty}$ 視為一致（`ε_qty` 可隨品種/面額設定）
* **價格容差**：

  * $|px_{ex}-px_{db}|/px_{ex} \le \epsilon_{px}$
* **狀態一致**：

  * 例：DB=NEW/交易所無 → `DB-only`；DB=FILLED/交易所仍有殘單 → 應撤單

---

## 5) 處置策略（行為矩陣）

> **原則**：**風險優先**、**最小擾動**、**可追溯**。遇不確定 → 保守回收（小額 reduceOnly 平倉 / 撤單），禁止反向加倉。

| 類型          | 條件        | 動作（非 dry）                | 備註                           |
| ----------- | --------- | ------------------------ | ---------------------------- |
| 訂單 API-only | 交易所有/DB 無 | `POST S4 /cancel`        | 取消成功→記 `RECONCILE_CANCELLED` |
| 訂單 DB-only  | DB 有/交易所無 | DB 修正為 CLOSED            | 寫事件、審計                       |
| 訂單 參數不符     | 價格/數量不符   | 若可安全接管→補寫 DB；否則 `cancel` | `RECLAIM_IF_SAFE` 才接管        |
| 持倉 API-only | 交易所有/DB 無 | 接管（補寫 DB）或保守平倉           | 取決於 `orphan_policy`          |
| 持倉 DB-only  | DB 有/交易所無 | DB 清理（CLOSED）            |                              |
| 持倉 數量不符     | 容差外       | DB 對齊交易所；必要時保守平倉調整       |                              |

---

## 6) 事務狀態機修復（倉位/訂單）

* **狀態流**：`PENDING_ENTRY → ACTIVE → PENDING_CLOSING → CLOSED`
* **修復要點**：

  * `PENDING_ENTRY` 崩潰遺留：若交易所已有倉位 → **接管**並補齊 `orders/fills` 關聯與 `positions_snapshots`
  * `ACTIVE` 但交易所無倉：應轉 `CLOSED`，清理殘留掛單
  * 每一步都寫 `strategy_events`，含 `reconcile_id`、`action`、`reason`

---

## 7) 排程與觸發

* **開機必跑**：`POST /reconcile {mode:ALL, dry_run:false, time_window_h:72}`
* **定時**：每 **10–15 分鐘**全域對帳（可 per-symbol 並行分片）
* **事件觸發**：

  * S11 偵測 **Jaccard**/`maker_fill_ratio`/`ib_rate` 異常 → 立即觸發
  * S4 回報取消/下單異常連續 N 次 → 立即觸發

---

## 8) Redis Keys / Streams（最小閉環）

* `recon:last_run_ts`（STRING）：最近一次完成對帳的 epoch ms
* `recon:status:<reconcile_id>`（HASH）：`state`, `started_at`, `ended_at`, `actions_count_*`, `jaccard_*`
* `recon:events`（STREAM）：

  ```json
  {
    "reconcile_id":"rc_...",
    "severity":"INFO|WARN|ERROR",
    "phase":"FETCH|DIFF|ACT|WRITE",
    "summary":"orders_orphan_api_only=2,..."
  }
  ```
* `alerts`（STREAM）：ERROR/FATAL 告警訊息（含 `reconcile_id`、root cause）

---

## 9) DB Collections（寫入點）

* `strategy_events`：`{event_id, kind="RECONCILE_*", ts, detail{reconcile_id, action, before, after}}`
* `orders`：狀態/數量/價格修正（審計留痕）
* `positions_snapshots`：補齊/修正一筆最新快照（保留原始快照鏈）

---

## 10) 指標與 SLO（供 S11/Grafana）

* **Jaccard**（orders / positions）目標：≥ 0.98 / ≥ 0.95
* **孤兒處置成功率**：≥ 99%
* **對帳任務 p95 時延**：≤ 2s（單 symbol）
* **對帳失敗率**：≤ 0.5%
* Prom 指標：`recon_jobs_total{status}`, `recon_duration_seconds{mode}`, `recon_discrepancies{type}`, `recon_actions_total{action}`

---

## 11) 契約測試（Happy / Edge）

**Happy**

1. `mode=ORDERS`，API 與 DB 完全一致 → `discrepancies=0`、`jaccard≈1`
2. `mode=ALL`，發現 1 筆孤兒掛單（API-only）→ 取消成功，DB 寫事件

**Edge**

1. `dry_run=true`：只回報差異，不呼叫 S4
2. `RECLAIM_IF_SAFE`：DB 缺一筆交易所已存在且一致的倉位 → 接管
3. 無法取消（S4 連續 3 次失敗）→ `alerts(FATAL)`、列入下一輪重試
4. **大量差異**（>X%）：自動降級健康度（寫 `health:state`）、加密道歉頁（可選）

---

## 12) 參考偽代碼（核心流程）

```text
reconcile(req):
  id = newReconcileID()
  write(redis:recon:status:id, state="RUNNING", started_at=now)

  ex = fetchExchangeState(req.symbols, req.markets)     # O_ex, P_ex, B_ex
  db = fetchLocalState(req.symbols, req.markets, req.window)  # O_db, P_db

  diffs = diffSetsAndParams(ex, db, eps)
  summary = summarize(diffs)            # jaccard, orphan counts, etc.

  if !req.dry_run:
     actions = planActions(diffs, req.orphan_policy)
     for a in actions:
        exec(a) via S4 (cancel / reduceOnly close) or DB fix
        audit(strategy_events, a)

  finalize:
    write(redis:recon:status:id, state="COMPLETED", ended_at=now, counters=summary)
    xadd(redis:recon:events, {id, summary})
    return {id, status:"COMPLETED", summary, actions_taken}
```

---

## 13) 風險與回滾

* **防重/冪等**：以 `reconcile_id` 與請求參數 hash 做去重；同一請求短時間內重送 → 回舊結果
* **保守降級**：當撤單/平倉失敗或 REST 受限 → 進入 **DEGRADED**：只修 DB 與告警，不做交易動作
* **審計閉環**：所有動作必有 `strategy_events` 與 `alerts` 對應

---

## 14) 與既有文檔對齊（摘）

* **對帳模式**、**孤兒處置**、**定時任務**、**API 範本**等，均沿用你草稿主線並補上**雙向孤兒分類、Jaccard 指標、動作矩陣、Redis/DB 寫入點與格式**，便於觀測與回放。

---

**交付建議**：先完成 **ALL/ORDERS/POSITIONS** 模式 + **API-only/DB-only** 基本處置，再補參數容差與接管策略（`RECLAIM_IF_SAFE`），最後接入 S11 指標與告警門檻，跑**開機必跑 + 10–15m 週期**兩條線即可。
