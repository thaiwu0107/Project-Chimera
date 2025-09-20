# S4 Order Router

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
