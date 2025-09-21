# Project Chimera — S1 Exchange Connectors 施工清單（合併版 v1）

> 本清單把你「原始 TODO」與我先前整理的 S1 製作清單**合併為單一文件**。
> 每個項目以標籤標示來源：**\[原TODO]**＝你的原始列點；**\[新增]**＝我補充的工程化細節；**\[強化]**＝在原條目基礎上加內容。
> 目標是：**一份可直接導引開發的清單**，不遺漏你既有的任何要求。

---

## 0) 共通規範

* [ ] **時間戳**：epoch ms（整數） **\[強化]**
* [ ] **金額**：USDT，小數以 `float64` 儲存 **\[強化]**
* [ ] **百分比/比率**：小數（0.1 = 10%） **\[強化]**
* [ ] **冪等鍵**：HTTP 要求頭 `Idempotency-Key` 或 body `transfer_id`/`client_order_id` **\[強化]**
* [ ] **Redis Cluster**：官方 cluster client，所有關鍵寫入具備重試與 `MOVED/ASK` 跟隨 **\[新增]**
* [ ] **/health /ready**：Liveness/Readiness 皆回傳 JSON，內含依賴（Redis/Arango/WS）檢測 **\[新增]**

---

## 1) 市場資料（WS/Ticker/Depth/Funding）

### 1.1 WebSocket 連線與資料處理

* [ ] 建立 Binance FUT/SPOT WS（心跳、重連、訂閱管理） **\[原TODO]**
* [ ] 清洗/時間對齊：將交易所 `eventTime` 轉換為本地 epoch ms，校正 `Δt` **\[原TODO]**
* [ ] 計算中間價與價差：

  * **中間價**：`mid_t = (bestBid_t + bestAsk_t) / 2` **\[原TODO]**
  * **價差（bps）**：`spread_bps_t = (bestAsk_t - bestBid_t) / mid_t * 1e4` **\[原TODO]**
* [ ] Top1 深度估算（名目）：`depth_top1_usdt = min(bidTop1Qty, askTop1Qty) * mid_t` **\[原TODO]**
* [ ] 最小節流（去抖）：合併同秒多次更新，保留最後一筆 **\[原TODO]**
* [ ] 監控 throughput 指標 `s1.mkt_throughput`（每秒訊息數） **\[原TODO]**

### 1.2 Funding（預估/實收）與持久化

* [ ] 解析 funding tick（下一期預估/實際扣收） **\[原TODO]**
* [ ] 寫入 Redis Stream：`mkt:events:funding:<SYMBOL>` **\[原TODO]**
* [ ] 持久化實收紀錄：`funding_records(symbol,funding_time,rate,amount_usdt)`（Arango） **\[原TODO]**
* [ ] 每 8 小時全量補缺（歷史 funding） **\[原TODO]**

### 1.3 Redis Streams 發布（事件總表）

* [ ] `mkt:events:spot:<SYMBOL>`（現貨 Ticker/Depth）**\[原TODO]**
* [ ] `mkt:events:perp:<SYMBOL>`（永續 Ticker/Depth）**\[原TODO]**
* [ ] `mkt:events:funding:<SYMBOL>`（Funding 預估/實收）**\[原TODO]**
* [ ] 事件欄位固定：`ts, symbol, side?, bid1, ask1, mid, spread_bps, top1_qty, top1_usdt, src` **\[強化]**

---

## 2) 交易所資訊與註冊表（exchangeInfo → instrument\_registry）

* [ ] **每日**同步 `exchangeInfo`：tickSize/stepSize/leverageBracket/minNotional **\[原TODO]**
* [ ] 更新 `instrument_registry`（Arango），並以 `updated_at` 打時間戳 **\[原TODO]**
* [ ] 風險門檻（如 `spread_bp_limit, depth_top1_usdt_min`）同時刷新 **\[新增]**

---

## 3) 金庫劃轉 API（內部私有）

### 3.1 端點與冪等

* [ ] `POST /xchg/treasury/transfer`（From=SPOT/FUT, To=FUT/SPOT, amount\_usdt, reason）**\[原TODO]**
* [ ] 接受 `Idempotency-Key` 或 `transfer_id`，重複請求回相同結果 **\[原TODO]**
* [ ] 限額與白名單檢查（最小/最大、當日上限）**\[原TODO]**

### 3.2 交易所整合與持久化

* [ ] 直呼 Binance 劃轉 API（SPOT↔FUT）**\[原TODO]**
* [ ] 狀態機：`PENDING → DONE/FAILED`，寫 `treasury_transfers`（Arango）**\[原TODO]**
* [ ] 審計事件 `ops:events`（Redis Stream）**\[原TODO]**
* [ ] 回傳：`TransferResponse{transfer_id, result(OK|FAIL|PENDING), message?}` **\[原TODO]**

### 3.3 字段校驗與契約測試

* [ ] `transfer_id`：UUID 或 1–128 長度字串 **\[原TODO]**
* [ ] `from/to`：枚舉 {SPOT,FUT} **\[原TODO]**
* [ ] `amount_usdt`：>0 且在限制內 **\[原TODO]**
* [ ] 400/422 錯誤分類；冪等重放一致 **\[原TODO]**
* [ ] 合法/非法/冪等/超額四類契約測試 **\[原TODO]**

---

## 4) 錯誤處理與重連

### 4.1 WS 重連（指數退避＋抖動）

* [ ] 維護 `retry_count`；斷線時重連，成功清零 **\[原TODO]**
* [ ] 等候時間：`wait = min(maxWait, base * 2^retry_count) + U(0, jitter)` **\[原TODO]**
* [ ] 超過 `N_max` 次 → 觸發 FATAL，降級為「僅管理既有倉位」 **\[原TODO]**

### 4.2 REST 退避/重試與錯誤分類

* [ ] 對 5xx/429 採退避重試；4xx 快速失敗 **\[原TODO]**
* [ ] Redis/Arango 臨時錯誤重試與死信記錄 **\[新增]**

---

## 5) 核心時序圖支援（與 S4/S5 對齊）

* [ ] **FUT 入場**：NEW\_ORDER(LIMIT postOnly) → CANCEL\_ORDER → STOP\_MARKET → 狀態回報（ACK/FILL/TIMEOUT）**\[原TODO]**
* [ ] **SPOT 入場**：OCO\_ORDER（或守護）→ MARKET BUY/SELL → TP/SL 條件單 → OCO 狀態回報 **\[原TODO]**
* [ ] **對帳處置**：openOrders/positions 拉取、狀態同步 **\[原TODO]**
* [ ] **冪等**：`client_order_id` 映射；重送不重下 **\[原TODO]**
* [ ] **事件流**：`orders:executed`、`spot:oco:armed`、`risk:sl_arm` 等發佈 **\[原TODO]**

> 說明：雖然大多由 S4 下單，但 S1 需保證**行情/規格**與**錢包/劃轉**資料面完整，支撐上述流程。**\[新增]**

---

## 6) 服務與資料流（「路過的服務」對齊）

* [ ] 讀：交易所（WS/REST）**\[原TODO]**
* [ ] 算：`mid/spread_bps/top1_usdt` 等即時計算 **\[原TODO]**
* [ ] 寫 Redis Streams：`mkt:events:spot|perp|funding` **\[原TODO]**
* [ ] 寫 DB：`funding_records`；更新 `instrument_registry` **\[原TODO]**
* [ ] 事件：審計 `ops:events`、延遲 `metrics:events:s1.ws_rtt` **\[原TODO]**

---

## 7) 定時任務（排程）

* [ ] **交易所心跳巡檢（每 30s）**：`GET /fapi/v1/time` → 取 `t_server`；讀本地 `t_local`；計算時鐘偏差

  * 公式：`Δt = |t_local - t_server|`；`Δt ≤ skew_max_ms` 視為 PASS（建議 250/500/1000ms 分層） **\[原TODO]**
  * 度量：`p_up = ok_calls / total_calls`、`RTT_p50/p95` **\[原TODO]**
* [ ] **WS 自動重連掃描（每 10s）**：見 §4.1 公式 **\[原TODO]**
* [ ] **每日 exchangeInfo 刷新**：見 §2 **\[原TODO]**
* [ ] **每 8h Funding 補缺**：見 §1.2 **\[原TODO]**

---

## 8) 監控與可觀測性

* [ ] 延遲：`metrics:events:s1.ws_rtt`（RTT/處理延遲） **\[原TODO]**
* [ ] 吞吐：`s1.mkt_throughput`（每秒訊息量） **\[原TODO]**
* [ ] 錯誤：重連次數、REST 退避計數、Redis `MOVED/ASK` 次數 **\[新增]**
* [ ] 健康：/health 聚合 Redis/Arango/WS 狀態（GREEN/YELLOW/ORANGE/RED）**\[新增]**
* [ ] Grafana：提供 S1 儀表板 JSON（面板：WS RTT、Throughput、Δt、Funding 寫入速率）**\[新增]**

---

## 9) 組態與 Kill-switch

* [ ] 讀取 S10 `/active`（僅需規格/風險門檻部分）**\[新增]**
* [ ] 訂閱 `cfg:events`（遇到 rev 變更即熱載）**\[新增]**
* [ ] `kill_switch`（Redis Key）：ON 時停止推送新行情/或降頻（可配置）**\[新增]**

---

## 10) 安全與密鑰

* [ ] `S1_BINANCE_KEY/SECRET`（K8s Secret）**\[原TODO]**
* [ ] `S1_DB_ARANGO_URI/USER/PASS`、`S1_REDIS_ADDRESSES`（Cluster，多節點逗號分隔）**\[原TODO]**
* [ ] 出站 IP 白名單（若雲廠商提供），REST 透過 egress gateway **\[新增]**

---

## 11) 本地開發與測試

* [ ] 本地回放：支援從檔案回放 sample WS 訊息到 `mkt:*` Streams（不連外）**\[新增]**
* [ ] 契約測試：`/xchg/treasury/transfer` 四類用例（合法/非法/冪等/超額）**\[原TODO]**
* [ ] 負載測試：WS→Stream→消費鏈路在極端行情（高頻）下的 backpressure 行為 **\[新增]**

---

## 12) 驗收（DoD）

* [ ] **市場資料**：`mkt:events:*` 三族群穩定產出（帶 `mid/spread_bps/top1_usdt`）**\[強化]**
* [ ] **Funding**：`funding_records` 連續完整、每 8h 補缺無缺口 **\[原TODO]**
* [ ] **exchangeInfo**：`instrument_registry` 每日刷新且 hash 無異常 **\[原TODO]**
* [ ] **劃轉**：冪等一致（重送 `transfer_id` 回相同結果）**\[原TODO]**
* [ ] **監控**：Grafana 可視；/health 反映依賴狀態 **\[新增]**
* [ ] **壓力**：在高波動下 WS 重連/退避正常、Stream 無阻塞（有背壓保護）**\[新增]**

---

## 附錄 A）Redis Streams 與 Keys（S1 產出/讀取）

### Streams（XADD）

* `mkt:events:spot:<SYMBOL>`：`{ts,symbol,bid1,ask1,mid,spread_bps,top1_qty,top1_usdt,src}`
* `mkt:events:perp:<SYMBOL>`：同上
* `mkt:events:funding:<SYMBOL>`：`{ts,symbol,rate_est,rate_real?,amount_usdt?}`
* `ops:events`：`{ts,source="s1",kind,detail_json}`

### Keys

* `metrics:s1:ws_rtt`（HLL/TS 或作為指標輸出）
* `s1.mkt_throughput`（Counter）
* `cfg:active_rev`（只讀，來自 S10）
* `kill_switch`（只讀，ON 則降頻/停推）

---

## 附錄 B）ArangoDB Collections（S1 涉及）

* `funding_records`（寫入）
* `treasury_transfers`（寫入）
* `instrument_registry`（每日更新）
* （指標類集合依整體白皮書 v3/v2 規範）

---

## 附錄 C）數學公式彙總（S1 範圍）

* **時鐘偏差**：`Δt = |t_local - t_server|`
* **WS 重連退避**：`wait = min(maxWait, base * 2^retry_count) + U(0, jitter)`
* **中間價**：`mid_t = (bestBid_t + bestAsk_t)/2`
* **價差（bps）**：`spread_bps_t = (bestAsk_t - bestBid_t)/mid_t * 1e4`
* **Top1 深度（USDT）**：`depth_top1_usdt = min(bidTop1Qty, ask1Qty) * mid_t`

---

## 對位說明（來源標籤）

* **\[原TODO]**：完全承襲你原本的待辦事項（未刪除）。
* **\[新增]**：補齊工程化落地細節（監控、降級、熱載、測試、DoD）。
* **\[強化]**：在原條目上加上具體輸入/輸出與約束（欄位、頻率、公式）。

> 若需要，我可以把此清單拆成「GitHub Issues 匯入用 CSV/JSON」，自動建立看板與標籤。
