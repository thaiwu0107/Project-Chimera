# S3 Strategy Engine

## 概述

S3 Strategy Engine 是 Project Chimera 的策略引擎服務，負責執行交易策略的核心邏輯，包括 L0 守門、L1 規則 DSL、L2 置信度模型，最終產生下單意圖。

## 功能特性

### 1. L0 守門（Gate Keeper）
- **資金費檢查**：檢查資金費率是否超過上限
- **流動性檢查**：檢查價差和深度是否滿足交易條件
- **風險預算檢查**：檢查現貨名義金額和期貨保證金限制
- **併發控制**：限制同一市場的同時入場數量

### 2. L1 規則引擎（Rule Engine）
- **DSL 規則解析**：支持複雜的條件組合和動作定義
- **規則優先級**：支持規則優先級和衝突解決
- **動態規則加載**：支持熱更新規則配置
- **規則命中追蹤**：記錄觸發的規則和相應動作

### 3. L2 機器學習模型（ML Model）
- **置信度評分**：基於特徵計算交易置信度
- **倉位倍率調整**：根據 ML 分數動態調整倉位大小
- **模型版本管理**：支持多版本模型並行運行
- **特徵重要性分析**：分析各特徵對決策的影響

### 4. 配置管理（Config Manager）
- **RCU 熱載**：讀取複製更新模式的配置熱載
- **版本一致性**：確保配置版本的一致性
- **配置快取**：內存快取提高配置訪問效率
- **配置監聽**：實時監聽配置變更事件

## API 端點

### 健康檢查
- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 策略決策
- `POST /decide` - 執行策略決策，生成交易意圖

## 決策流程

### 1. 請求驗證
- 驗證請求格式和必填字段
- 檢查特徵數據完整性
- 驗證配置版本有效性

### 2. L0 守門檢查
```
資金費檢查: |funding_next| <= max_funding_abs
流動性檢查: spread_bps <= spread_bp_limit && depth_top1_usdt >= min
風險預算檢查: Σ spot_notional <= spot_quote_usdt_max
```

### 3. L1 規則評估
- 遍歷所有啟用的規則
- 評估規則條件是否滿足
- 累積規則動作（size_mult, tp_mult, sl_mult）
- 應用白名單限制

### 4. L2 ML 評分
- 基於特徵計算 ML 分數
- 計算置信度
- 確定倉位倍率調整

### 5. 結果合併
- 合併規則和 ML 結果
- 生成最終決策
- 創建訂單意圖（如需要）

## 數學計算

### 倉位計算（FUT）
```
margin_base = 20 USDT
margin = margin_base × size_mult
notional = margin × leverage
qty = round_to_step(notional / price, stepSize)
```

### 停損距離計算
```
d_atr = ATR_mult × ATR
d_losscap = max_loss_usdt / qty
d = min(d_atr, d_losscap)
```

### SL/TP 價格計算
```
多單: SL = entry - d, TP = entry + d × tp_mult
空單: SL = entry + d, TP = entry - d × tp_mult
```

### ML 分數到倉位倍率映射
```
score > 0.85 → size_mult = 1.2
score 0.6-0.85 → size_mult = 1.0
score 0.4-0.6 → size_mult = 0.5
score < 0.4 → skip
```

## 規則 DSL 語法

### 條件語法
```json
{
  "allOf": [
    {"f": "rv_pctile_30d", "op": "<", "v": 0.25},
    {"f": "rho_usdttwd_14", "op": "<", "v": -0.3}
  ]
}
```

### 動作語法
```json
{
  "size_mult": 1.2,
  "tp_mult": 2.0,
  "sl_mult": 0.5
}
```

### 支持的運算符
- `<`, `>`, `<=`, `>=`, `==`: 數值比較
- `allOf`: 所有條件必須滿足
- `anyOf`: 任一條件滿足即可
- `not`: 條件取反

## 配置參數

### 守門參數
- `maxFundingAbs`: 0.0005 - 最大資金費絕對值
- `spreadBpLimit`: 3.0 - 價差限制（bps）
- `depthTop1UsdtMin`: 200.0 - 最小深度（USDT）
- `spotQuoteUsdtMax`: 10000.0 - 現貨最大名義金額
- `futMarginUsdtMax`: 5000.0 - 期貨最大保證金

### ML 模型參數
- `modelName`: "default_model" - 模型名稱
- `version`: "v1.0" - 模型版本
- `confidenceThreshold`: 0.6 - 置信度閾值

## 數據流

### 輸入數據
- **特徵數據**: 來自 S2 Feature Generator
- **市場數據**: 來自 S1 Exchange Connectors
- **配置數據**: 來自 S10 Config Service

### 輸出數據
- **交易信號**: 保存到 ArangoDB signals collection
- **訂單意圖**: 發送到 S4 Order Router
- **Redis Streams**: `orders:intent` 訂單意圖流

## 性能特性

### 決策延遲
- **P50**: ≤ 200ms
- **P95**: ≤ 500ms
- **目標**: 1000 筆模擬特徵決策

### 並發處理
- **信號處理**: 支持高並發信號處理
- **規則評估**: 並行規則條件評估
- **ML 推論**: 異步 ML 模型推論

## 錯誤處理

### 守門失敗
- 記錄失敗原因
- 返回 skip 決策
- 不生成訂單意圖

### 規則評估錯誤
- 跳過錯誤規則
- 繼續評估其他規則
- 記錄錯誤日誌

### ML 模型錯誤
- 使用默認分數
- 降級到規則引擎
- 發送告警通知

## 監控指標

### 服務健康指標
- Redis 連接延遲
- ArangoDB 連接延遲
- 配置加載狀態

### 業務指標
- 決策延遲（P50/P95）
- 規則命中率
- ML 模型準確率
- 守門通過率

## 部署說明

### Docker 部署
```bash
docker build -t s3-strategy .
docker run -p 8083:8083 s3-strategy
```

### 環境要求
- Go 1.19+
- Redis Cluster
- ArangoDB
- 足夠的 CPU（ML 推論）

## 開發指南

### 添加新規則
1. 定義規則條件和動作
2. 在 `loadStrategyRules` 中註冊
3. 測試規則邏輯

### 添加新特徵
1. 更新 `DecideRequest` 結構
2. 修改守門和規則邏輯
3. 更新 ML 模型輸入

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
1. **決策延遲過高**
   - 檢查 Redis 連接狀態
   - 確認規則數量是否過多
   - 檢查 ML 模型性能

2. **規則不生效**
   - 檢查規則是否啟用
   - 確認條件語法正確
   - 查看規則命中日誌

3. **ML 模型錯誤**
   - 檢查模型文件是否存在
   - 確認特徵數據格式
   - 查看模型推論日誌

## 版本歷史

### v1.0.0
- 初始版本
- 實現 L0 守門、L1 規則引擎、L2 ML 模型
- 支持 FUT/SPOT 市場決策
- 實現配置熱載和規則管理