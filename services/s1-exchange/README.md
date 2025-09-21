# S1 Exchange Connectors ❌ **[未實作]**

Exchange Connectors - Integrate Binance FUT/UM & SPOT REST/WS; Optional MAX USDTTWD as factor; Reconnect/throttle/clock correction

## 📋 實作進度：15% (1/8 功能完成)

### ✅ 已完成功能
- [x] 基礎服務架構
- [x] Health Check API
- [x] WebSocket 連接管理框架
- [x] 市場數據快取結構
- [x] Treasury Transfer API 框架

### ❌ 待實作功能

#### 1. WS 行情/深度/Ticker/Funding 更新
- [ ] **實作 WebSocket 數據處理**
  - [ ] 清洗/時間對齊市場數據
  - [ ] 計算 mid 價格和 spread_bp
  - [ ] 實現最小節流（去抖）機制
- [ ] **Redis Stream 發布**
  - [ ] `mkt:events:{spot}:<SYMBOL>`（現貨）
  - [ ] `mkt:events:{perp}:<SYMBOL>`（永續）
  - [ ] `mkt:events:{funding}:<SYMBOL>`（下一期/實際 funding）
- [ ] **DB 寫入**
  - [ ] `funding_records`（`symbol,funding_time,rate,amount_usdt`）
- [ ] **指標收集**
  - [ ] `metrics:events:s1.ws_rtt`
  - [ ] `s1.mkt_throughput`

#### 2. POST /xchg/treasury/transfer（內部）
- [ ] **冪等性驗證**
  - [ ] Idempotency-Key / `transfer_id` 檢查
  - [ ] 限額/白名單驗證
- [ ] **交易所 API 整合**
  - [ ] 實際呼叫 Binance 劃轉 API
  - [ ] 成功/失敗判定邏輯
- [ ] **DB 寫入**
  - [ ] `treasury_transfers`（狀態流轉）
- [ ] **事件發布**
  - [ ] `ops:events`（審計）
- [ ] **回應格式**
  - [ ] `TransferResponse{TransferID,Result,Message}`

#### 3. 定時任務
- [ ] **每日 exchangeInfo 刷新**
  - [ ] 交易所資訊更新邏輯
- [ ] **每 8h 拉取全量 funding rate 歷史快照補缺**
  - [ ] 資金費率歷史數據補齊

#### 4. 錯誤處理與重連
- [ ] **WebSocket 重連機制**
  - [ ] 自動重連邏輯
  - [ ] 連接狀態監控
- [ ] **錯誤處理**
  - [ ] API 錯誤重試機制
  - [ ] 異常情況處理

#### 5. 核心時序圖相關功能（基於時序圖實作）
- [ ] **FUT 入場流程支持**
  - [ ] NEW_ORDER (LIMIT, postOnly) 下單
  - [ ] CANCEL_ORDER 撤單
  - [ ] STOP_MARKET 止損單下單
  - [ ] 訂單狀態回報 (ACK NEW / FILL / TIMEOUT)
- [ ] **SPOT 入場流程支持**
  - [ ] OCO_ORDER 一鍵雙向單
  - [ ] MARKET BUY/SELL 市價單
  - [ ] PLACE TP / PLACE SL 條件單
  - [ ] OCO 狀態回報 (ACK OCO / REJECTED)
- [ ] **對帳處置支持**
  - [ ] GET openOrders 查詢開放訂單
  - [ ] GET positions 查詢持倉
  - [ ] REST 調用交易所現況
  - [ ] 訂單狀態同步
- [ ] **冪等性支持**
  - [ ] Idempotency-Key 處理
  - [ ] 重複請求檢測和回覆
  - [ ] 訂單 ID 映射管理
- [ ] **事件流支持**
  - [ ] orders:executed Stream 發布
  - [ ] spot:oco:armed Stream 發布
  - [ ] risk:sl_arm Stream 發布
  - [ ] 關鍵狀態變更事件推送

#### 6. 服務與資料流相關功能（基於服務與資料流實作）
- [ ] **行情與交易規則快取**
  - [ ] 中間價計算：`mid_t = (bestBid_t + bestAsk_t)/2`
  - [ ] 價差計算：`spread_bps_t = (bestAsk_t - bestBid_t) / mid_t * 1e4`
  - [ ] Top1 深度計算：`depth_top1_usdt = min(bidTop1Qty, askTop1Qty) * mid_t`
  - [ ] Redis Streams 發布：`mkt:tick:{symbol}`, `mkt:depth:{symbol}`
  - [ ] ArangoDB instrument_registry 更新
- [ ] **定時任務**
  - [ ] 每日 exchangeInfo 刷新（合約規則/tickSize/stepSize/leverageBracket）
  - [ ] 每 8h 拉取全量 funding rate 歷史快照補缺
- [ ] **錢包劃轉支持**
  - [ ] SPOT ↔ FUT 資金劃轉
  - [ ] TransferRequest/Response 事件處理
  - [ ] 劃轉限制和守門檢查

#### 7. 定時任務相關功能（基於定時任務實作）
- [ ] **交易所心跳巡檢（每 30s）**
  - [ ] 呼叫 `GET /fapi/v1/time` 取得伺服器時間
  - [ ] 讀取本地時間並計算時鐘偏差
  - [ ] 時鐘偏差計算：`Δ t = |t_local - t_server|`
  - [ ] 判定：`Δ t ≤ skew_max_ms` → PASS；否則 WARN/ERROR（建議分層：250/500/1000ms）
  - [ ] 度量：成功率 `p_up = ok_calls / total_calls`、RTT 分位 `RTT_p50, RTT_p95`
- [ ] **WS 自動重連（事件驅動 + 每 10s 掃描）**
  - [ ] 為每條 WS 連線維護 `retry_count`
  - [ ] 斷線後按指數退避 + 抖動重連：`wait = min(maxWait, base * 2^retry_count) + U(0,jitter)`
  - [ ] 成功即清零並重新訂閱
  - [ ] 連續失敗超過 N_max → FATAL；降級為「僅管理既有倉位」模式

#### 8. 目標與範圍相關功能（基於目標與範圍實作）
- [ ] **前置依賴實作**
  - [ ] ArangoDB Collections：`funding_records`、`treasury_transfers`
  - [ ] Redis Streams：`mkt:events:{spot}:<SYMBOL>`、`mkt:events:{perp}:<SYMBOL>`、`mkt:events:{funding}:<SYMBOL>`
  - [ ] Redis Keys：`ops:events`（審計）
- [ ] **環境變數配置**
  - [ ] `S1_DB_ARANGO_URI`、`S1_DB_ARANGO_USER/PASS`
  - [ ] `S1_REDIS_ADDRESSES`（逗號分隔，Cluster 模式）
  - [ ] `S1_BINANCE_KEY/SECRET`、`S1_TESTNET=true`
- [ ] **風險與緩解**
  - [ ] 時鐘偏移檢查：下單前先比對 serverTime，偏移>1s 停新倉
  - [ ] 網路波動處理：所有 REST 調用退避重試；WebSocket 自動重連
  - [ ] Redis Cluster slot 移轉：使用官方 cluster client；關鍵操作具重試策略

#### 9. 路過的服務相關功能（基於路過的服務實作）
- [ ] **WS 行情/深度/Ticker/Funding 更新**
  - [ ] 讀：無（直連交易所）
  - [ ] 算：清洗/時間對齊；拼 `mid`、`spread_bp`；可做最小節流（去抖）
  - [ ] 寫 Redis Stream：`mkt:events:{spot}:<SYMBOL>`（現貨）、`mkt:events:{perp}:<SYMBOL>`（永續）、`mkt:events:{funding}:<SYMBOL>`（下一期/實際 funding）
  - [ ] 寫 DB（僅 funding 實收）：`funding_records`（`symbol,funding_time,rate,amount_usdt`）
  - [ ] 指標：`metrics:events:s1.ws_rtt`、`s1.mkt_throughput`
- [ ] **POST /xchg/treasury/transfer（內部）**
  - [ ] 驗：Idempotency-Key / `transfer_id`；限額/白名單
  - [ ] 叫交易所劃轉 API → 判定成功/失敗
  - [ ] 寫 DB：`treasury_transfers`（狀態流轉）
  - [ ] 發事件：`ops:events`（審計）
  - [ ] 回：`TransferResponse{TransferID,Result,Message}`

#### 10. 字段校驗相關功能（基於字段校驗表實作）
- [ ] **TransferRequest 字段校驗**
  - [ ] `transfer_id`：UUID/字串長度 1–128，作為冪等鍵全局唯一
  - [ ] `from_market`/`to_market`：枚舉值 {SPOT, FUT} 驗證
  - [ ] `amount`：數值範圍 > 0，符合最小/最大劃轉限制
  - [ ] `symbol`：正則 `^[A-Z0-9]{3,}$` 驗證
- [ ] **TransferResponse 字段校驗**
  - [ ] `transfer_id`：必填，與請求一致
  - [ ] `result`：枚舉值 {SUCCESS, FAILED, PENDING} 驗證
  - [ ] `message`：可選，錯誤描述長度限制
- [ ] **錯誤處理校驗**
  - [ ] 400 Bad Request：參數格式錯誤、範圍超界
  - [ ] 422 Unprocessable Entity：業務規則違反、數據不完整
  - [ ] 冪等性：相同 `transfer_id` 返回相同結果
- [ ] **契約測試**
  - [ ] TransferRequest 合法參數 → `result`=SUCCESS
  - [ ] TransferRequest 非法參數 → 400/422 錯誤
  - [ ] 冪等性測試：重複請求返回相同結果
  - [ ] 限額檢查：超過限制 → 422 MIN_AMOUNT/MAX_AMOUNT

#### 11. 功能對照補記相關功能（基於功能對照補記實作）
- [ ] **SPOT↔FUT 金庫劃轉（自動/人工）**
  - [ ] 自動劃轉：S6 根據 `min_free_fut` 與 `spot_buffer` 計算 `need` → 產生審批請求 → S12 人工批准 → S1 執行
  - [ ] 冪等性：`transfer_id` 保證一次且僅一次
  - [ ] 劃轉限制：最小/最大劃轉金額檢查
  - [ ] 審計日誌：完整的劃轉操作記錄

#### 12. 全服務一覽相關功能（基於全服務一覽實作）
- [ ] **WS 行情/深度/Ticker/Funding 更新**
  - [ ] 讀：直連交易所（無預讀）
  - [ ] 算：清洗/時間對齊；拼 `mid`、`spread_bp`；必要去抖/節流
  - [ ] 寫 Redis Streams：`mkt:events:{spot}:<SYMBOL>`（現貨）、`mkt:events:{perp}:<SYMBOL>`（永續）、`mkt:events:{funding}:<SYMBOL>`（下一期/實際 funding）
  - [ ] 寫 DB（僅 funding 實收）：`funding_records`（`symbol,funding_time,rate,amount_usdt`）
  - [ ] 指標：`metrics:events:s1.ws_rtt`、`metrics:events:s1.mkt_throughput`
- [ ] **POST /xchg/treasury/transfer（內部）**
  - [ ] 驗：Idempotency-Key / `transfer_id`；限額/白名單
  - [ ] 執行：呼交易所劃轉 API → 判定成功/失敗
  - [ ] 寫 DB：`treasury_transfers`（狀態流轉）
  - [ ] 發事件：`ops:events`（審計）
  - [ ] 回：`TransferResponse{TransferID,Result,Message}`

#### 13. Integration 附錄相關功能（基於 Integration 附錄實作）
- [ ] **事務一致性保證**
  - [ ] 冪等性保證：所有狀態變更操作都使用 `X-Idempotency-Key` 確保重複請求的安全性
  - [ ] 狀態機管理：採用明確的狀態轉換（PENDING_ENTRY → ACTIVE → CLOSED）
  - [ ] 失敗恢復：系統崩潰後能夠通過對帳機制恢復到一致狀態
- [ ] **風險控制機制**
  - [ ] 保守回收策略：無法接管的孤兒訂單/持倉優先採用降風險處理
  - [ ] 多層止損機制：FUT 使用交易所止損，SPOT 使用 OCO 或守護停損
  - [ ] 實時監控：通過 Redis Streams 實現關鍵事件的實時通知
- [ ] **性能優化**
  - [ ] Maker→Taker 回退：優先使用限價單降低交易成本，超時自動回退到市價單
  - [ ] TWAP 執行：大單拆分執行，減少市場衝擊
  - [ ] 並行處理：對帳過程中使用並行查詢提高效率
- [ ] **統一約束**
  - [ ] 冪等性約束：所有會變更系統狀態的請求都必須攜帶 `X-Idempotency-Key`
  - [ ] 時間與數字單位：時間戳統一使用 epoch 毫秒（ms）、費率使用小數表示（0.01 = 1%）、金額統一使用 USDT
  - [ ] Redis Streams 命名規範：信號家族 `signals:new`、執行家族 `orders:executed`、對帳家族 `strategy:reconciled`、告警家族 `alerts`
  - [ ] 事務狀態機：PENDING_ENTRY → ACTIVE → PENDING_CLOSING → CLOSED
  - [ ] 保守回收原則：降風險優先、審計記錄、告警通知、人工確認

#### 14. Hop-by-Hop 執行規格相關功能（基於 Hop-by-Hop 執行規格補遺實作）
- [ ] **WS 行情/深度/Ticker/Funding 更新**
  - [ ] 讀：直連交易所（無預讀）
  - [ ] 算：清洗/時間對齊；拼 `mid`、`spread_bp`；必要去抖/節流
  - [ ] 寫 Redis Streams：
    - [ ] `mkt:events:{spot}:<SYMBOL>`（現貨）
    - [ ] `mkt:events:{perp}:<SYMBOL>`（永續）
    - [ ] `mkt:events:{funding}:<SYMBOL>`（下一期/實際 funding）
  - [ ] 寫 DB（僅 funding 實收）：`funding_records`（`symbol,funding_time,rate,amount_usdt`）
  - [ ] 指標：`metrics:events:s1.ws_rtt`、`metrics:events:s1.mkt_throughput`
- [ ] **POST /xchg/treasury/transfer（內部）**
  - [ ] 驗：Idempotency-Key / `transfer_id`；限額/白名單
  - [ ] 執行：呼交易所劃轉 API → 判定成功/失敗
  - [ ] 寫 DB：`treasury_transfers`（狀態流轉）
  - [ ] 發事件：`ops:events`（審計）
  - [ ] 回：`TransferResponse{TransferID,Result,Message}`

#### 15. 功能規格書相關功能（基於功能規格書實作）
- [ ] **入向（被呼叫）API**
  - [ ] `GET /health`（所有服務）→ `HealthResponse{Status,Checks,...}`
  - [ ] `POST /xchg/treasury/transfer`（S12/S6 內部）→ `TransferResponse{TransferID,Result,Message}`
- [ ] **出向（主以事件）**
  - [ ] 〔Stream〕推送行情/深度/資金費/帳戶事件至 `mkt:*`
  - [ ] （可選）上拋 S11 指標 via 〔Stream: metrics:*〕
- [ ] **金庫資金劃轉（自動/人工）**
  - [ ] 對外：S12 `POST /treasury/transfer`（`TransferRequest`）
  - [ ] 內部：S12 → S1 `POST /xchg/treasury/transfer`（帶 Idempotency-Key）
  - [ ] 成功：寫 strategy_events(kind=TREASURY_TRANSFER)；失敗記 alerts
  - [ ] 鎖：`lock:treasury:<from>:<to>`（Redis）
  - [ ] 失敗補償：重試 N 次；連續失敗升級 FATAL
- [ ] **冪等性與重試**
  - [ ] 資金劃轉：Idempotency-Key（由 S12 產生）→ S1 必須回舊 TransferID
  - [ ] 對 5xx/429 採固定+抖動退避

### 🎯 實作優先順序
1. **高優先級**：WebSocket 數據處理和 Redis Stream 發布
2. **中優先級**：Treasury Transfer 完整實作
3. **低優先級**：定時任務和錯誤處理優化

### 📊 相關資料寫入
- **DB Collections**：`funding_records`、`treasury_transfers`
- **Redis Key/Stream**：`mkt:events:*`、`ops:events`

## 概述

S1 Exchange Connectors 是 Project Chimera 的交易所連接器服務，負責整合 Binance FUT/UM 和 SPOT 的 REST/WebSocket API，提供行情、深度、資金費、帳戶等數據服務。

## 功能特性

### 1. 市場數據服務
- **實時行情**：通過 WebSocket 獲取實時價格和成交量數據
- **訂單簿深度**：提供買賣盤深度信息
- **資金費率**：獲取期貨合約的資金費率信息

### 2. 帳戶管理
- **餘額查詢**：查詢現貨和期貨帳戶餘額
- **持倉信息**：獲取當前持倉詳情
- **PnL 計算**：實時計算未實現盈虧

### 3. 資金劃轉
- **SPOT ↔ FUT 劃轉**：支持現貨和期貨之間的資金劃轉
- **冪等性保證**：使用 Idempotency Key 防止重複操作
- **審計日誌**：完整的劃轉操作記錄

### 4. WebSocket 連接管理
- **自動重連**：連接斷開時自動重連
- **多市場支持**：同時支持 FUT 和 SPOT 市場
- **數據快取**：內存快取最新市場數據

## API 端點

### 健康檢查
- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 市場數據
- `GET /market/data?symbol=BTCUSDT&market=FUT` - 獲取市場數據
- `GET /market/orderbook?symbol=BTCUSDT&market=FUT` - 獲取訂單簿
- `GET /market/funding?symbol=BTCUSDT` - 獲取資金費率

### 帳戶信息
- `GET /account/balance?market=FUT` - 獲取帳戶餘額
- `GET /account/positions?market=FUT` - 獲取持倉信息

### 資金劃轉（內部 API）
- `POST /xchg/treasury/transfer` - 執行資金劃轉

## 配置參數

### 環境變數
- `BINANCE_API_KEY` - Binance API 密鑰
- `BINANCE_SECRET_KEY` - Binance 密鑰
- `BINANCE_SANDBOX` - 是否使用測試網（true/false）

### 資金劃轉配置
- `MaxRetryCount`: 3 - 最大重試次數
- `RetryInterval`: 5s - 重試間隔
- `Timeout`: 30s - 請求超時時間
- `RateLimitPerMin`: 10 - 每分鐘請求限制
- `MinTransferAmount`: 1.0 USDT - 最小劃轉金額
- `MaxTransferAmount`: 10000.0 USDT - 最大劃轉金額

## 定時任務

### 1. Exchange Info 刷新
- **週期**：24 小時
- **功能**：更新合約規則、tickSize、stepSize、leverageBracket

### 2. Funding Rate 補缺
- **週期**：8 小時
- **功能**：拉取全量資金費率歷史快照，補寫 funding_records

## 數據流

### WebSocket 數據流
```
Binance WebSocket → S1 Exchange → Redis Streams
```

- **FUT 市場**：`wss://fstream.binance.com/ws/{symbol}@ticker`
- **SPOT 市場**：`wss://stream.binance.com:9443/ws/{symbol}@ticker`

### Redis Streams 輸出
- `mkt:tick:{symbol}` - 市場行情數據
- `mkt:depth:{symbol}` - 訂單簿深度數據

## 數學計算

### 中間價計算
```
mid_t = (bestBid_t + bestAsk_t) / 2
```

### 價差計算（bps）
```
spread_bps_t = (bestAsk_t - bestBid_t) / mid_t * 1e4
```

### Top1 深度（USDT）
```
depth_top1_usdt = min(bidTop1Qty, askTop1Qty) * mid_t
```

## 錯誤處理

### WebSocket 連接錯誤
- 自動重連機制
- 連接狀態監控
- 錯誤計數和告警

### API 調用錯誤
- 重試機制
- 熔斷保護
- 錯誤日誌記錄

## 監控指標

### 服務健康指標
- WebSocket 連接狀態
- Redis 連接延遲
- ArangoDB 連接延遲

### 業務指標
- 市場數據更新頻率
- 資金劃轉成功率
- API 調用延遲

## 部署說明

### Docker 部署
```bash
docker build -t s1-exchange .
docker run -p 8081:8081 \
  -e BINANCE_API_KEY=your_key \
  -e BINANCE_SECRET_KEY=your_secret \
  s1-exchange
```

### 環境要求
- Go 1.19+
- Redis Cluster
- ArangoDB
- 網路連接（Binance API）

## 開發指南

### 本地開發
```bash
# 安裝依賴
go mod tidy

# 運行服務
go run main.go

# 測試
go test ./...
```

### 添加新的交易所
1. 實現 `ExchangeConnector` 接口
2. 添加相應的 WebSocket 連接邏輯
3. 更新配置和路由

## 故障排除

### 常見問題
1. **WebSocket 連接失敗**
   - 檢查網路連接
   - 確認 Binance API 狀態
   - 查看日誌中的錯誤信息

2. **資金劃轉失敗**
   - 檢查 API 密鑰權限
   - 確認帳戶餘額充足
   - 查看劃轉限制設置

3. **數據更新延遲**
   - 檢查 Redis 連接狀態
   - 確認 WebSocket 連接正常
   - 查看系統資源使用情況

## 版本歷史

### v1.0.0
- 初始版本
- 支持 Binance FUT/SPOT 市場
- 實現基本的市場數據和帳戶功能
- 支持資金劃轉功能