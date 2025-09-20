# S2 Feature Generator

## 概述

S2 Feature Generator 是 Project Chimera 的特徵工程服務，負責從市場數據和信號中計算各種技術特徵，包括 ATR、已實現波動率、相關性、深度等特徵，為策略引擎提供決策依據。

## 功能特性

### 1. 特徵計算
- **ATR (Average True Range)**：計算平均真實範圍，用於衡量價格波動性
- **已實現波動率 (Realized Volatility)**：基於歷史價格計算的波動率指標
- **相關性分析**：計算不同標的之間的價格相關性
- **深度特徵**：分析訂單簿深度和價差特徵

### 2. 多時間窗口支持
- **1分鐘**：短期特徵計算
- **5分鐘**：中短期特徵計算
- **1小時**：中期特徵計算
- **4小時**：中長期特徵計算
- **1天**：長期特徵計算

### 3. 實時計算與快取
- **內存快取**：最新特徵數據的內存快取
- **Redis 發布**：將特徵數據發布到 Redis Streams
- **定時補算**：每 5 分鐘自動補算特徵

### 4. 任務管理
- **異步計算**：支持異步特徵計算任務
- **進度追蹤**：實時追蹤計算任務進度
- **錯誤處理**：完善的錯誤處理和重試機制

## API 端點

### 健康檢查
- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 特徵管理
- `POST /features/recompute` - 重新計算特徵
- `GET /features?symbol=BTCUSDT&feature_type=ATR` - 獲取特徵數據
- `GET /features/computation?task_id=xxx` - 獲取計算任務狀態

## 數學計算

### ATR (Average True Range)
```
TR_t = max(H_t - L_t, |H_t - C_{t-1}|, |L_t - C_{t-1}|)
ATR_t = ATR_{t-1} + (TR_t - ATR_{t-1}) / n
ATR_pct = (ATR / current_price) * 100
```

### 已實現波動率 (Realized Volatility)
```
r_t = ln(P_t / P_{t-1})
rv_m = sqrt(252) * std(r_{t-m+1..t})
```

### 相關性計算
```
rho = corr(Δln P_btc, Δln FX, window=k)
```

### 深度因子
```
liq_score = min(depth_top1_usdt / threshold, 1.0)
spread_bps = (bestAsk - bestBid) / mid * 1e4
```

## 特徵類型

### ATR 特徵
- `atr`: ATR 絕對值
- `atr_pct`: ATR 百分比
- `period`: 計算週期

### RV 特徵
- `rv`: 已實現波動率
- `rv_pct`: 波動率百分比
- `period`: 計算週期

### 相關性特徵
- `correlation`: 相關性係數
- `symbol1`: 第一個標的
- `symbol2`: 第二個標的
- `period`: 計算週期

### 深度特徵
- `bid_depth`: 買盤深度
- `ask_depth`: 賣盤深度
- `bid_ask_ratio`: 買賣比例
- `spread`: 價差
- `spread_pct`: 價差百分比

## 配置參數

### 計算器配置
- **ATR 週期**: 14（可調整）
- **RV 週期**: 20（可調整）
- **相關性週期**: 14（可調整）

### 定時任務配置
- **補算間隔**: 5 分鐘
- **支援標的**: BTCUSDT, ETHUSDT, ADAUSDT
- **支援窗口**: 1m, 5m, 1h, 4h, 1d

## 數據流

### 輸入數據
- **市場數據**: 來自 S1 Exchange Connectors
- **K 線數據**: OHLCV 格式的價格數據
- **深度數據**: 訂單簿買賣盤數據

### 輸出數據
- **特徵快照**: 內存快取的特徵數據
- **Redis Streams**: `feat:last:{symbol}` 最新特徵
- **ArangoDB**: `signals.features` 持久化存儲

## 性能特性

### 計算效率
- **並行計算**: 多個特徵並行計算
- **快取機制**: 避免重複計算
- **增量更新**: 只計算新增數據

### 內存管理
- **LRU 快取**: 最近使用的特徵優先保留
- **定期清理**: 清理過期的計算任務
- **內存監控**: 實時監控內存使用情況

## 錯誤處理

### 數據不足
- 檢查數據點數量是否足夠
- 提供詳細的錯誤信息
- 跳過無法計算的特徵

### 計算錯誤
- 記錄計算失敗的詳細信息
- 繼續計算其他特徵
- 返回部分成功的結果

## 監控指標

### 服務健康指標
- Redis 連接延遲
- ArangoDB 連接延遲
- 特徵計算延遲

### 業務指標
- 特徵計算成功率
- 快取命中率
- 任務完成時間

## 部署說明

### Docker 部署
```bash
docker build -t s2-feature .
docker run -p 8082:8082 s2-feature
```

### 環境要求
- Go 1.19+
- Redis Cluster
- ArangoDB
- 足夠的內存（建議 2GB+）

## 開發指南

### 添加新特徵
1. 實現 `FeatureCalculator` 接口
2. 在 `initializeFeatureCalculators` 中註冊
3. 添加相應的數據模型

### 本地開發
```bash
# 安裝依賴
go mod tidy

# 運行服務
go run main.go

# 測試
go test ./...
```

## 故障排除

### 常見問題
1. **特徵計算失敗**
   - 檢查輸入數據是否完整
   - 確認計算參數是否正確
   - 查看日誌中的錯誤信息

2. **快取數據過期**
   - 檢查定時任務是否正常運行
   - 確認 Redis 連接狀態
   - 手動觸發特徵重算

3. **內存使用過高**
   - 檢查快取大小設置
   - 確認是否有內存洩漏
   - 調整快取清理策略

## 版本歷史

### v1.0.0
- 初始版本
- 支持 ATR、RV、相關性、深度特徵計算
- 實現多時間窗口支持
- 添加任務管理和進度追蹤