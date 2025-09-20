# S1 Exchange Connectors

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