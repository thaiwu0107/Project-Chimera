# S5 Reconciler

## 概述

S5 Reconciler 是 Project Chimera 交易系統的對帳引擎，負責對比交易所數據與本地數據庫，確保數據一致性，並處理孤兒訂單和持倉。

## 功能

- **數據對帳**：對比交易所與本地數據庫
- **孤兒處理**：處理孤兒訂單和持倉
- **數據修復**：修復不一致的數據
- **風險控制**：確保交易數據的準確性

## API 接口

### 健康檢查

- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 對帳管理

- `POST /reconcile` - 啟動對帳流程

#### Reconcile

**請求**：
```json
{
  "mode": "ALL",
  "symbols": ["BTCUSDT", "ETHUSDT"],
  "markets": ["FUT", "SPOT"],
  "from_time": 1640995200000,
  "to_time": 1641081600000
}
```

**回應**：
```json
{
  "reconcile_id": "reconcile_001",
  "status": "COMPLETED",
  "summary": {
    "orders_matched": 150,
    "orders_orphaned": 2,
    "positions_matched": 10,
    "positions_orphaned": 1,
    "discrepancies": 3
  },
  "actions_taken": [
    {
      "type": "CANCEL_ORDER",
      "order_id": "orphan_001",
      "reason": "Order exists in exchange but not in local DB"
    },
    {
      "type": "CLOSE_POSITION",
      "position_id": "orphan_pos_001",
      "reason": "Position exists in local DB but not in exchange"
    }
  ]
}
```

## 服務間交互

### 入向（被呼叫）
- **S12 Web UI** → `POST /reconcile` - 手動啟動對帳
- **排程系統** → `POST /reconcile` - 定期對帳

### 出向（主動呼叫）
- **S4 Order Router** → `POST /cancel` - 取消孤兒訂單
- **數據庫** → 修復數據不一致
- **告警系統** → 回報嚴重不一致

## 對帳模式

### ALL 模式
- 對比所有訂單、持倉和資金
- 最全面的對帳檢查

### ORDERS 模式
- 僅對比訂單數據
- 用於訂單狀態同步

### POSITIONS 模式
- 僅對比持倉數據
- 用於持倉狀態同步

### HOLDINGS 模式
- 僅對比資金數據
- 用於資金餘額同步

## 孤兒處理策略

### 孤兒訂單
1. **API 有單/DB 無單**：取消交易所訂單
2. **DB 有單/API 無單**：清理本地訂單狀態

### 孤兒持倉
1. **API 有倉/DB 無倉**：建立接管記錄
2. **DB 有倉/API 無倉**：平倉處理

### 風險控制
- 優先採用減風險路徑
- 小額市價平倉
- 禁止反向加倉

## 失敗補償

- S4 取消失敗：記錄 FATAL 告警
- 列入下一輪對帳重試
- 連續失敗升級處理

## 配置

服務使用以下配置：
- Redis：用於對帳狀態緩存
- ArangoDB：用於對帳歷史存儲
- 交易所 API：獲取交易所數據
- 端口：8085（可通過環境變量 PORT 覆蓋）

## 部署

```bash
# 構建
go build -o s5-reconciler .

# 運行
./s5-reconciler
```

## 監控

服務提供以下監控指標：
- 對帳執行時間
- 數據一致性率
- 孤兒處理成功率
- 數據修復次數
- 告警觸發頻率
