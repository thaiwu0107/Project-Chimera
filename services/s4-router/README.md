# S4 Order Router ❌ **[未實作]**

Order Router - Route orders to exchanges with TWAP granularity, Maker/Taker strategies, OCO/Guard Stop

## 📋 實作進度：15% (1/7 功能完成)

### ✅ 已完成功能
- [x] 基礎服務架構
- [x] Health Check API
- [x] 訂單驗證邏輯
- [x] 基本訂單創建和取消 API

### ❌ 待實作功能

#### 1. POST /orders（含 FUT/SPOT/TWAP/OCO/GuardStop）
- [ ] **冪等性驗證**
  - [ ] `intent_id` 冪等檢查
  - [ ] KillSwitch（新倉禁）檢查
- [ ] **路由參數讀取**
  - [ ] `router:{param}:curves` 讀取
  - [ ] 最新價/深度（Redis 快照）讀取
- [ ] **執行策略決策**
  - [ ] Maker→Taker 或 TWAP 決策
  - [ ] SPOT 是否原生 OCO 判斷
  - [ ] 守護停損需要時啟動監控
- [ ] **訂單下發**
  - [ ] 產 `client_order_id`
  - [ ] REST/WS 下發到交易所
- [ ] **DB 寫入**
  - [ ] `orders`（NEW/部分/FILLED）
  - [ ] `fills`（含 `mid_at_send/top3/slippage_bps`）
- [ ] **Redis 寫入**
  - [ ] SPOT 守護：`guard:{stop}:<symbol>:<intent_id>`（armed/armed_at）
  - [ ] TWAP 佇列：`prod:{exec}:twap:queue`（ZSet；`score=due_ts`）
  - [ ] 成交流：`ord:{results}`（Stream；匯總給 S6/S5）
- [ ] **回應格式**
  - [ ] `OrderResult{status, order_id, filled_avg, …}`

#### 2. POST /cancel
- [ ] **冪等性驗證**
  - [ ] 冪等檢查
  - [ ] 現況讀取
- [ ] **撤單執行**
  - [ ] 撤單邏輯
  - [ ] 必要時升級為市價騰挪
- [ ] **DB 寫入**
  - [ ] `orders(status=CANCELED)`
  - [ ] 事件 `strategy_events(kind=CANCEL)`
- [ ] **事件發布**
  - [ ] `ord:{results}`（撤單回報）

#### 3. TWAP tick（排程）
- [ ] **TWAP 任務處理**
  - [ ] 取 ZSet 到期任務
  - [ ] 依序切片下單
  - [ ] 未完再入列
- [ ] **指標收集**
  - [ ] `router_p95`
  - [ ] `maker_timeout_count`

#### 4. 交易所 API 整合
- [ ] **Binance API 整合**
  - [ ] REST API 調用
  - [ ] WebSocket 訂單狀態更新
- [ ] **錯誤處理**
  - [ ] API 錯誤重試
  - [ ] 網路異常處理

#### 5. 執行策略優化
- [ ] **Maker/Taker 策略**
  - [ ] 動態策略選擇
  - [ ] 滑價控制
- [ ] **TWAP 優化**
  - [ ] 切片大小動態調整
  - [ ] 時間窗口優化

#### 6. 守護停損
- [ ] **Guard Stop 機制**
  - [ ] 價格監控
  - [ ] 自動觸發邏輯
- [ ] **OCO 訂單**
  - [ ] 原生 OCO 支持
  - [ ] 模擬 OCO 實現

#### 7. 監控和指標
- [ ] **執行指標**
  - [ ] 延遲監控
  - [ ] 成功率統計
- [ ] **業務指標**
  - [ ] 滑價分析
  - [ ] 執行成本統計

#### 8. 詳細實作項目（基於目標與範圍文件）
- [ ] **API 與健康檢查**
  - [ ] **GET /health**：連線（Exch、Redis、Arango）、路由表 rev、CB 狀態
  - [ ] **POST /orders**
    - **入（FUT）**：`{intent_id,uuid,market:FUT,symbol:BTCUSDT,side:BUY,qty:0.002,exec_policy:{maker_wait_ms:2000,twap:{slices:2,gap_ms:600}},sl:{type:STOP_MARKET,price:64800},tp:{type:TAKE_PROFIT_MARKET,target:net_profit_usdt,value:2.0},client_order_id:s3-btc-...,reduce_only:false}`
    - **入（SPOT-OCO）**：`{intent_id,uuid,market:SPOT,symbol:BTCUSDT,side:BUY,qty:0.003,exec_policy:{type:OCO,tp_price:67000,sl_price:64500},client_order_id:s3-btc-...}`
    - **出**：`{ result: OK|FAIL, order_ids[], message? }`
  - [ ] **POST /cancel**：按 order_id 或 client_order_id 撤單
- [ ] **路由策略（Maker→Taker / TWAP / 流動性探測）**
  - [ ] **Maker 等待** `wait_ms = f(notional)`（路由表）
  - [ ] 不足量或超時 → cancel → 市價（或限價追 1 檔）
  - [ ] **TWAP**：按 slices/gap_ms 切片，支援 ± 抖動
  - [ ] **流動性探測**：下單前查 spread_bps、top1_depth；過閾值延時或改拆單
- [ ] **交易規則與 rounding**
  - [ ] 檢查 minNotional、tickSize/stepSize；round_to_tick/step
  - [ ] **FUT**：設 marginType=ISOLATED、leverage=20（若尚未設）
  - [ ] **FUT**：市價開倉成交後，立即下 STOP_MARKET（workingType=MARK_PRICE、reduceOnly）
  - [ ] **TP**：TAKE_PROFIT_MARKET 或由 S6 管理（本波可先由 S4 挂單）
  - [ ] **SPOT**：交易所 OCO 支援則直用；否則 客戶端守護（WS/輪巡）確保一腿成交另一腿撤銷
- [ ] **冪等／熔斷／錯誤重試**
  - [ ] `idem:order:{client_order_id}` 保證一次性
  - [ ] **熔斷**：近 60s 內 5xx/429 超閾值 → CB=OPEN（暫停新開，允許平倉與風險操作）
  - [ ] **重試**：網路超時 N 次內退避重試；重試仍失敗 → 發 alerts
- [ ] **事件與持久化**
  - [ ] 寫 orders/fills；Stream `orders:executed`、`router:events`
  - [ ] fills 記錄 mid_at_send、book_top3、slippage_bps
  - [ ] **OCO 守護**：維持內部狀態機；任何腿成交→另一腿撤銷；寫 strategy_events
- [ ] **指標與告警**
  - [ ] `router.submit.latency_ms`（P50/P95）、`maker.fill_ratio`、`twap.slice_fill_ratio`
  - [ ] `taker.rate`、`slippage.bps.p50/p95`、`fee.usdt.total`
  - [ ] **告警**：CB 開啟、maker_fill_ratio 長期過低、insufficient_balance_rate 過高
- [ ] **測試與驗收**
  - [ ] **單元**：四捨五入、最小單位、OCO 守護狀態機、Maker→Taker 回退
  - [ ] **契約**：/orders 入出 schema；/cancel 多場景
  - [ ] **整合**：串接 Sandbox 或 Stub，驗證 SL/TP/ReduceOnly 行為
  - [ ] **驗收（Ready）**
    - 1000 筆連續意圖：成功下單 ≥ 99%，平均延遲 P95 ≤ 500ms
    - 市場極端（大 spread/薄深度）能自動延時/拆分/降級
    - OCO 守護 100% 一腿成交另一腿撤銷；無重覆成交

#### 9. 核心時序圖相關功能（基於時序圖實作）
- [ ] **FUT 入場流程**
  - [ ] POST /orders (intent_id 冪等, policy=MakerThenTaker, market=FUT)
  - [ ] 下單(限價, passive) + Idempotency-Key
  - [ ] Maker 等待視窗內成交處理
  - [ ] 等待逾時或流動性不足 → CANCEL 限價 → MARKET 回退
  - [ ] TWAP 切片執行
- [ ] **SPOT 入場流程**
  - [ ] POST /orders (intent_id, market=SPOT, execPolicy=OCO, tp_px/sl_px)
  - [ ] CREATE_OCO (limit + stopLoss leg)
  - [ ] OCO 支援檢查和回退處理
  - [ ] 一腿失敗 → MARKET BUY/SELL → 分別掛 TP/SL
  - [ ] 守護停損 fallback 機制
- [ ] **訂單狀態管理**
  - [ ] UPSERT orders, INSERT fills
  - [ ] 成交均價/量記錄
  - [ ] slippage_bps 計算和記錄
  - [ ] XADD orders:executed {order_id,filled,…}
- [ ] **冪等性處理**
  - [ ] intent_id 作為冪等鍵
  - [ ] 重送同鍵回同結果
  - [ ] 失敗/超時重試機制
- [ ] **事件流發布**
  - [ ] orders:executed Stream 發布
  - [ ] spot:oco:armed Stream 發布
  - [ ] risk:sl_arm Stream 發布
  - [ ] OrderResult{FILLED/ACCEPTED} 回報

#### 10. 服務與資料流相關功能（基於服務與資料流實作）
- [ ] **智能執行（Maker→Taker、TWAP、流動性探測）**
  - [ ] Maker 等待：`wait_ms = f(notional)`（路由策略表）；若 `fill_ratio < θ` → 撤單改 Taker
  - [ ] TWAP：N 片，間隔 Δt，每片 `qty_i = qty_total/N`；可加入抖動 `U(-ε, ε)`
  - [ ] 滑價估計：`slip_bps = (VWAP - mid_at_send)/mid_at_send * 1e4`
  - [ ] 流動性探測：下單前讀 depth、spread_bps；若 `spread_bps > 2×1h_mean` → 延時 5s 或拆單
- [ ] **定時任務**
  - [ ] 路由策略表每日滾動更新（基於 TCA 統計）
- [ ] **錢包劃轉（SPOT ↔ FUT）**
  - [ ] 觸發：`insufficient_balance` 或 `risk.budget` 需要
  - [ ] 守門：上限/最小留存額
  - [ ] 記帳：TransferRequest/Response 事件寫入 strategy_events

#### 11. 定時任務相關功能（基於定時任務實作）
- [ ] **路由器殘單清理（每 1–5 分鐘）**
  - [ ] 以交易所 `openOrders` 對比 DB 中 `orders(status∈{NEW,PARTIALLY_FILLED})`
  - [ ] 孤兒單 → 撤單；過期單 → 撤/改價/升級市價（依路由策略）
  - [ ] 一致性度量（Jaccard）：`J = |O_ex ∩ O_db| / |O_ex ∪ O_db|`；低於門檻觸發對帳 / 告警
- [ ] **TWAP / 批次執行 tick（每 1–3 秒）**
  - [ ] 目標名目 N（USDT），切片大小 s → 切片數 n = ⌈N/s⌉
  - [ ] 時距 Δt（秒），起點 t_0
  - [ ] 調度：`t_i = t_0 + i * Δt`，`q_i = Q_total / n`
  - [ ] 可選曲線與修正：時間平方根曲線 `q_i ∝ sqrt(i)`（正規化到 Q_total）
  - [ ] 流動性修正：spread / top1 depth 異常 → 推遲或縮片；估計滑價超閾值 → 改限價或更細分片

#### 12. 目標與範圍相關功能（基於目標與範圍實作）
- [ ] **API 與健康檢查**
  - [ ] GET /health：連線（Exch、Redis、Arango）、路由表 rev、CB 狀態
  - [ ] POST /orders：入（FUT/SPOT-OCO），出 `{ result: OK|FAIL, order_ids[], message? }`
  - [ ] POST /cancel：按 order_id 或 client_order_id 撤單
- [ ] **路由策略（Maker→Taker / TWAP / 流動性探測）**
  - [ ] Maker 等待 `wait_ms = f(notional)`（路由表）
  - [ ] 不足量或超時 → cancel → 市價（或限價追 1 檔）
  - [ ] TWAP：按 slices/gap_ms 切片，支援 ± 抖動
  - [ ] 流動性探測：下單前查 spread_bps、top1_depth；過閾值延時或改拆單
- [ ] **交易規則與 rounding**
  - [ ] 檢查 minNotional、tickSize/stepSize；round_to_tick/step
  - [ ] FUT：設 marginType=ISOLATED、leverage=20（若尚未設）
  - [ ] FUT：市價開倉成交後，立即下 STOP_MARKET（workingType=MARK_PRICE、reduceOnly）
  - [ ] TP：TAKE_PROFIT_MARKET 或由 S6 管理（本波可先由 S4 挂單）
  - [ ] SPOT：交易所 OCO 支援則直用；否則 客戶端守護（WS/輪巡）確保一腿成交另一腿撤銷
- [ ] **冪等／熔斷／錯誤重試**
  - [ ] `idem:order:{client_order_id}` 保證一次性
  - [ ] 熔斷：近 60s 內 5xx/429 超閾值 → CB=OPEN（暫停新開，允許平倉與風險操作）
  - [ ] 重試：網路超時 N 次內退避重試；重試仍失敗 → 發 alerts
- [ ] **事件與持久化**
  - [ ] 寫 orders/fills；Stream `orders:executed`、`router:events`
  - [ ] fills 記錄 mid_at_send、book_top3、slippage_bps
  - [ ] OCO 守護：維持內部狀態機；任何腿成交→另一腿撤銷；寫 strategy_events
- [ ] **指標與告警**
  - [ ] `router.submit.latency_ms`（P50/P95）、`maker.fill_ratio`、`twap.slice_fill_ratio`
  - [ ] `taker.rate`、`slippage.bps.p50/p95`、`fee.usdt.total`
  - [ ] 告警：CB 開啟、maker_fill_ratio 長期過低、insufficient_balance_rate 過高
- [ ] **環境變數配置**
  - [ ] `S4_EXCHANGE=binance`、`S4_BINANCE_KEY/SECRET`、`S4_TESTNET=true`
  - [ ] `S4_REDIS_ADDRESSES`、`S4_DB_ARANGO_*`
  - [ ] `S4_ROUTE_TABLE_PATH=/etc/chimera/route.json`
  - [ ] `S4_CB_ERROR_RATE_WINDOW=60s`、`S4_CB_ERROR_RATE_THRESH=0.2`
  - [ ] `S4_MAKER_MAX_WAIT_MS=3000`、`S4_TWAP_MAX_SLICES=4`
  - [ ] `S4_OCO_GUARDIAN=true`
  - [ ] `S4_WORKING_TYPE=MARK_PRICE`

#### 13. 路過的服務相關功能（基於路過的服務實作）
- [ ] **POST /orders（含 FUT/ SPOT / TWAP / OCO / GuardStop）**
  - [ ] 驗：`intent_id` 冪等；KillSwitch（新倉禁）
  - [ ] 讀：路由參數 `router:{param}:curves`；最新價/深度（Redis 快照）
  - [ ] 決策：Maker→Taker 或 TWAP；SPOT 是否原生 OCO；守護停損需要時啟動監控
  - [ ] 下單：產 `client_order_id`；REST/WS 下發
  - [ ] 寫 DB：`orders`（NEW/部分/FILLED）；`fills`（含 `mid_at_send/top3/slippage_bps`）
  - [ ] 寫 Redis：SPOT 守護：`guard:{stop}:<symbol>:<intent_id>`（armed/armed_at）；TWAP 佇列：`prod:{exec}:twap:queue`（ZSet；`score=due_ts`）；成交流：`ord:{results}`（Stream；匯總給 S6/S5）
  - [ ] 回：`OrderResult{status, order_id, filled_avg, …}`
- [ ] **POST /cancel**
  - [ ] 驗：冪等；讀現況
  - [ ] 執行：撤單；必要時升級為市價騰挪
  - [ ] 寫 DB：`orders(status=CANCELED)`；事件 `strategy_events(kind=CANCEL)`
  - [ ] 發：`ord:{results}`（撤單回報）
- [ ] **TWAP tick（排程）**
  - [ ] 取 ZSet 到期任務 → 依序切片下單 → 未完再入列
  - [ ] 指標：`router_p95`、`maker_timeout_count`

#### 14. 字段校驗相關功能（基於字段校驗表實作）
- [ ] **OrderResult 字段校驗**
  - [ ] `status`：必填，枚舉 {NEW, FILLED, PARTIALLY_FILLED, ACCEPTED, CANCELED, REJECTED}
  - [ ] `order_id`：可選，OCO 可能回 group/legs
  - [ ] `avg_price`：可選，> 0，FILLED 才有
  - [ ] `filled_qty`：可選，>= 0
  - [ ] `fills`：可選，明細；含 price/qty/fee/slippage_bps
  - [ ] `slippage_bps`：可選，>= 0
  - [ ] `legs`：可選，OCO 雙腿回傳
- [ ] **CancelRequest 字段校驗**
  - [ ] `order_id`：XOR 必填，與 client_order_id 擇一
  - [ ] `client_order_id`：XOR 必填
  - [ ] `cascade_oco`：可選，預設 true
  - [ ] `reason`：可選，枚舉字串，寫入審計
- [ ] **錯誤處理校驗**
  - [ ] 400 Bad Request：參數格式錯誤、範圍超界
  - [ ] 404 Not Found：不存在的單
  - [ ] 409 Conflict：已成交
  - [ ] 422 Unprocessable Entity：業務規則違反、數據不完整
  - [ ] 冪等性：相同 `intent_id` 返回相同結果
- [ ] **契約測試**
  - [ ] FUT MakerThenTaker：限價 3s 內成交 → status=FILLED、reduce_only=false、fills/avg_price 正確
  - [ ] FUT 止損：STOP_MARKET + working_type=MARK_PRICE + reduce_only=true → status=ACCEPTED
  - [ ] SPOT OCO：雙腿掛單成功 → status=ACCEPTED、legs 各有 order_id
  - [ ] Idempotency：相同 intent_id 重送 → 應返回相同結果
  - [ ] 量價校驗：qty < stepSize、或 price 非 tickSize 整數倍 → 400
  - [ ] minNotional 未達 → 422 MIN_NOTIONAL
  - [ ] OCO 一腿失敗 → fallback 生效
  - [ ] FUT 止損無 stop_price → 400
  - [ ] FUT reduce_only=true + 新倉 → 422
  - [ ] 市場流動性不足 → Maker 超時回退 Taker；slippage_bps 記錄>0

#### 15. 功能對照補記相關功能（基於功能對照補記實作）
- [ ] **FUT 入場與 SL/TP 掛單**
  - [ ] S4 市價入場 → 等 `FILL`（聚合均價 $\bar P$）
  - [ ] 由 S6 計算 $SL, TP$（`reduceOnly=true`）→ 立即掛 `STOP_MARKET` 與 `TAKE_PROFIT_MARKET`
  - [ ] 若掛單失敗 → 重試/降級為守護停損（客戶端監控）
  - [ ] TP（多）：$TP=\bar P + m \cdot ATR$ 或目標 ROE 反解價格：$TP=\bar P + \frac{(ROE^* \cdot \text{Margin} + \sum \text{Fees})}{Q}$
- [ ] **SPOT 入場與 OCO/守護停損**
  - [ ] 支援 OCO：下 `LIMIT_MAKER` + `STOP_LOSS_LIMIT`；撮合引擎保障互斥
  - [ ] 不支援：守護序列
    - a) 先 `LIMIT_MAKER`；b) 若觸發停損價先到 → 先撤入場 → 視規則可反手；c) 入場成交後 → 立即掛 `STOP_MARKET`
  - [ ] 風險守門：若預估滑價 > $\text{slip\_bp\_max}$ 或 spread > $\text{spread\_max}$ → 改限價 / 降片
- [ ] **智能執行（Maker→Taker、TWAP）**
  - [ ] Maker→Taker：等候 $T_{\text{wait}}=f(\text{spread}, \text{depth})$；部分成交比率 $\phi$；剩餘 $(1-\phi)Q$ 以市價完成
  - [ ] TWAP：切片 $n=\lceil Q/s \rceil$；時間序列 $t_i=t_0+i \cdot \Delta t$；每片 $q_i=Q/n$ 或曲線加權 $q_i \propto \sqrt{i}$

#### 16. 全服務一覽相關功能（基於全服務一覽實作）
- [ ] **POST /orders（含 FUT / SPOT / TWAP / OCO / GuardStop）**
  - [ ] 驗：`intent_id` 冪等；KillSwitch（新倉禁）
  - [ ] 讀：路由參數 `router:{param}:curves`；最新價/深度（Redis 快照）
  - [ ] 決策：Maker→Taker 或 TWAP；SPOT 是否原生 OCO；需則啟動守護停損監控
  - [ ] 下單：產 `client_order_id`；REST/WS 下發
  - [ ] 寫 DB：`orders`（NEW/部分/FILLED）；`fills`（含 `mid_at_send/top3/slippage_bps`）
  - [ ] 寫 Redis：SPOT 守護：`guard:{stop}:<symbol>:<intent_id>`（armed/armed_at）；TWAP 佇列：`prod:{exec}:twap:queue`（ZSet；`score=due_ts`）；成交流：`ord:{results}`（Stream；匯總給 S6/S5）
  - [ ] 回：`OrderResult{status, order_id, filled_avg, …}`
- [ ] **POST /cancel**
  - [ ] 驗：冪等；讀現況
  - [ ] 執行：撤單；必要時升級為市價騰挪
  - [ ] 寫 DB：`orders(status=CANCELED)`；`strategy_events(kind=CANCEL)`
  - [ ] 發：`ord:{results}`（撤單回報）
- [ ] **TWAP tick（排程）**
  - [ ] 取 ZSet 到期任務 → 依序切片下單 → 未完再入列
  - [ ] 指標：`metrics:events:s4.router_p95`、`metrics:events:s4.maker_timeout_count`

#### 17. Integration 附錄相關功能（基於 Integration 附錄實作）
- [ ] **FUT 入場執行（Maker→Taker 回退）**
  - [ ] 限價單嘗試：首先嘗試掛限價單（Maker 策略）
  - [ ] 等待成交：在指定時間窗口內等待成交（`post_only_wait_ms: 3000`）
  - [ ] 回退機制：如果超時或流動性不足，自動取消限價單並改為市價單（Taker 策略）
  - [ ] TWAP 執行：大單可選擇 TWAP 方式拆分執行
  - [ ] 滑點記錄：記錄實際成交價格與預期價格的差異（`slippage_bps`）
- [ ] **SPOT 入場執行（OCO / 守護停損 fallback）**
  - [ ] OCO 嘗試：嘗試創建 OCO 訂單，包含限價單和止損單
  - [ ] 雙腿掛單：交易所同時掛上止盈腿和止損腿
  - [ ] OCO 失敗回退：檢測到 OCO 不支援或掛單失敗，改為市價單入場
  - [ ] 守護停損啟動：條件單不支援時啟動本地守護停損機制
  - [ ] 價格監控：通過 WebSocket 監控中間價格，觸及止損線時自動平倉
- [ ] **訂單執行事件發布**
  - [ ] 訂單執行事件：發布 `orders:executed` 事件到 Redis Stream
  - [ ] OCO 武裝事件：發布 `spot:oco:armed` 事件
  - [ ] 守護停損武裝：發布 `guard:spot:arm` 事件
  - [ ] 風險止損武裝：發布 `risk:sl_arm` 事件
- [ ] **事務一致性保證**
  - [ ] 冪等性保證：使用 `intent_id` 作為冪等鍵確保重複請求的安全性
  - [ ] 狀態機管理：訂單狀態 NEW → PARTIALLY_FILLED → FILLED → CLOSED
  - [ ] 失敗恢復：系統崩潰後能夠通過 `intent_id` 查詢訂單狀態
- [ ] **錯誤處理與重試**
  - [ ] 超時處理：上游服務超時時自動重試，最多重試 3 次
  - [ ] 部分成交：記錄部分成交情況，剩餘部分繼續執行
  - [ ] 交易所錯誤：根據錯誤碼決定是否重試或放棄

#### 18. Hop-by-Hop 執行規格相關功能（基於 Hop-by-Hop 執行規格補遺實作）
- [ ] **POST /orders（含 FUT / SPOT / TWAP / OCO / GuardStop）**
  - [ ] 驗：`intent_id` 冪等；KillSwitch（新倉禁）
  - [ ] 讀：路由參數 `router:{param}:curves`；最新價/深度（Redis 快照）
  - [ ] 決策：Maker→Taker 或 TWAP；SPOT 是否原生 OCO；需則啟動守護停損監控
  - [ ] 下單：產 `client_order_id`；REST/WS 下發
  - [ ] 寫 DB：`orders`（NEW/部分/FILLED）；`fills`（含 `mid_at_send/top3/slippage_bps`）
  - [ ] 寫 Redis：
    - [ ] SPOT 守護：`guard:{stop}:<symbol>:<intent_id>`（armed/armed_at）
    - [ ] TWAP 佇列：`prod:{exec}:twap:queue`（ZSet；`score=due_ts`）
    - [ ] 成交流：`ord:{results}`（Stream；匯總給 S6/S5）
  - [ ] 回：`OrderResult{status, order_id, filled_avg, …}`
- [ ] **POST /cancel**
  - [ ] 驗：冪等；讀現況
  - [ ] 執行：撤單；必要時升級為市價騰挪
  - [ ] 寫 DB：`orders(status=CANCELED)`；`strategy_events(kind=CANCEL)`
  - [ ] 發：`ord:{results}`（撤單回報）
- [ ] **TWAP tick（排程）**
  - [ ] 取 ZSet 到期任務 → 依序切片下單 → 未完再入列
  - [ ] 指標：`metrics:events:s4.router_p95`、`metrics:events:s4.maker_timeout_count`

#### 19. 功能規格書相關功能（基於功能規格書實作）
- [ ] **入向（被呼叫）API**
  - [ ] `GET /health`（所有服務）→ `HealthResponse{Status,Checks,...}`
  - [ ] `POST /orders`（S3/S6）→ `OrderResult`
  - [ ] `POST /cancel`（S3/S6/S5/S12）→ `CancelResponse`
- [ ] **出向（主以事件）**
  - [ ] 寫 orders/fills；必要時回報 alerts；（內部）呼叫交易所
- [ ] **下單（執行 intents）**
  - [ ] S3/S6 → S4 `POST /orders`（`OrderCmdRequest`）→ `OrderResult`
  - [ ] 冪等：intent.intent_id；Maker→Taker 回退與 TWAP 由 S4 控
- [ ] **撤單/撤換**
  - [ ] S3/S6/S5/S12 → S4 `POST /cancel`（`CancelRequest`）→ `CancelResponse`
  - [ ] 用 order_id 或 client_order_id
- [ ] **錯誤處理與告警**
  - [ ] WARN：Maker 等待逾時→Taker 回退（記錄＋計數）
  - [ ] ERROR：`/orders` 連續 3 次失敗（告警通知、熔斷路由（凍結新倉））
  - [ ] S4 取消失敗：記 alerts(FATAL)，列入下一輪對帳重試
- [ ] **冪等性與重試**
  - [ ] 下單/撤單：`OrderCmdRequest.Intent.IntentID` / `CancelRequest.ClientID` 必填作冪等鍵
  - [ ] S4 對 5xx/429 採固定+抖動退避

### 🎯 實作優先順序
1. **高優先級**：基本訂單執行和交易所 API 整合
2. **中優先級**：TWAP 和 OCO 功能
3. **低優先級**：守護停損和優化

### 📊 相關資料寫入
- **DB Collections**：`orders`、`fills`、`strategy_events(TP_SL_PLACED/CANCEL)`
- **Redis Key/Stream**：`ord:{results}`、`prod:{exec}:twap:queue`、`guard:{stop}:*`

## 概述

S4 Order Router 是 Project Chimera 交易系統的訂單路由引擎，負責執行交易訂單、管理訂單生命周期，並與交易所進行交互。

## 功能

- **訂單執行**：執行來自策略引擎的訂單意圖
- **訂單管理**：管理訂單狀態和生命周期
- **撤單處理**：處理訂單取消和修改
- **交易所交互**：與幣安等交易所進行 API 交互
- **訂單路由**：支援 Maker/Taker 回退和 TWAP 執行

## API 接口

### 健康檢查

- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 訂單管理

- `POST /orders` - 創建訂單
- `POST /cancel` - 取消訂單

#### Create Order

**請求**：
```json
{
  "intent": {
    "intent_id": "intent_001",
    "kind": "ENTRY",
    "side": "BUY",
    "market": "FUT",
    "symbol": "BTCUSDT",
    "size": 0.1,
    "exec_policy": {
      "order_type": "MARKET",
      "time_in_force": "IOC"
    }
  }
}
```

**回應**：
```json
{
  "order_id": "order_001",
  "client_order_id": "client_001",
  "status": "FILLED",
  "fills": [
    {
      "fill_id": "fill_001",
      "price": 45000.0,
      "size": 0.1,
      "timestamp": 1640995200000
    }
  ]
}
```

#### Cancel Order

**請求**：
```json
{
  "order_id": "order_001",
  "client_order_id": "client_001",
  "reason": "Risk management"
}
```

**回應**：
```json
{
  "order_id": "order_001",
  "status": "CANCELLED",
  "message": "Order cancelled successfully"
}
```

## 服務間交互

### 入向（被呼叫）
- **S3 Strategy Engine** → `POST /orders` - 執行訂單意圖
- **S6 Position Manager** → `POST /orders` - 持倉治理訂單
- **S5 Reconciler** → `POST /cancel` - 清理殘單
- **S12 Web UI** → `POST /cancel` - 手動撤單

### 出向（主動呼叫）
- **交易所 API** → 執行實際交易
- **數據庫** → 記錄 orders/fills
- **告警系統** → 回報訂單異常

## 訂單執行策略

### Maker/Taker 回退
1. 首先嘗試 Maker 訂單（限價單）
2. 如果等待超時，自動轉為 Taker 訂單（市價單）

### TWAP 執行
- 支援時間加權平均價格執行
- 將大單拆分為多個小單
- 在指定時間內均勻執行

## 冪等性處理

- 使用 `intent_id` 作為冪等鍵
- 對 5xx/429 錯誤進行重試
- 避免重複下單

## 配置

服務使用以下配置：
- Redis：用於訂單狀態緩存
- ArangoDB：用於訂單歷史存儲
- 交易所 API：幣安等交易所連接
- 端口：8084（可通過環境變量 PORT 覆蓋）

## 部署

```bash
# 構建
go build -o s4-router .

# 運行
./s4-router
```

## 監控

服務提供以下監控指標：
- 訂單執行延遲
- 訂單成功率
- 撤單成功率
- 交易所連接狀態
- TWAP 執行效率
