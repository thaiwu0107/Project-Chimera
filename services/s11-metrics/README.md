# S11 Metrics & Health

## 概述

S11 Metrics & Health 是 Project Chimera 交易系統的指標彙整和健康監控服務，負責收集、聚合和提供系統指標數據，以及管理告警系統。

## 功能

- **指標收集**：收集各服務的指標數據
- **指標聚合**：聚合和計算系統指標
- **告警管理**：管理系統告警和通知
- **健康監控**：監控服務健康狀態
- **數據可視化**：為前端提供指標數據

## API 接口

### 健康檢查

- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 指標管理

- `GET /metrics` - 獲取系統指標
- `GET /alerts` - 獲取系統告警
- `GET /treasury/metrics` - 獲取資金劃轉指標
- `GET /treasury/alerts` - 獲取資金劃轉告警

#### Get Metrics

**請求**：`GET /metrics?metric=router_p95_ms&tags=service:s4-router&from=1640995200000&to=1672531200000`

**回應**：
```json
{
  "points": [
    {
      "metric": "router_p95_ms",
      "value": 45.2,
      "ts": 1640995200000,
      "tags": {
        "service": "s4-router",
        "symbol": "BTCUSDT"
      }
    },
    {
      "metric": "strategy_pnl_usdt",
      "value": 1250.75,
      "ts": 1640995200000,
      "tags": {
        "service": "s3-strategy",
        "symbol": "BTCUSDT"
      }
    }
  ]
}
```

#### Get Alerts

**請求**：`GET /alerts?severity=ERROR&source=s1-exchange&limit=10`

**回應**：
```json
{
  "items": [
    {
      "alert_id": "alert_001",
      "severity": "ERROR",
      "source": "s1-exchange",
      "message": "WebSocket connection lost",
      "ts": 1640995200000
    },
    {
      "alert_id": "alert_002",
      "severity": "WARN",
      "source": "s4-router",
      "message": "High latency detected",
      "ts": 1640995200000
    }
  ]
}
```

#### Get Treasury Metrics

**請求**：`GET /treasury/metrics?metric=treasury_transfer_p95_ms&from=SPOT&to=FUT`

**回應**：
```json
{
  "sli_id": "sli_1640995200",
  "metric_name": "treasury_transfer_p95_ms",
  "value": 45.2,
  "unit": "ms",
  "window": {
    "from": 1640995200000,
    "to": 1641081600000
  },
  "p95_latency_ms": 45,
  "p99_latency_ms": 78,
  "success_rate": 0.998,
  "failure_rate": 0.002,
  "idempotency_hits": 12,
  "total_requests": 1250,
  "timestamp": 1640995200000
}
```

## 服務間交互

### 入向（被呼叫）
- **S12 Web UI** → `GET /metrics` - 前端指標數據
- **S12 Web UI** → `GET /alerts` - 前端告警數據
- **其他服務** → 推送指標數據（通過 Redis Stream）

### 出向（主動呼叫）
- **數據庫** → 存儲 metrics_timeseries
- **告警系統** → 存儲 alerts
- **通知系統** → 發送告警通知

## 指標類型

### 服務指標
- **延遲指標**：P95/P99 延遲
- **吞吐量指標**：每秒請求數
- **錯誤率指標**：錯誤百分比
- **資源指標**：CPU、記憶體使用率

### 交易指標
- **PnL 指標**：損益數據
- **成交量指標**：交易量統計
- **訂單指標**：訂單成功率
- **滑點指標**：執行滑點

### 系統指標
- **連接指標**：數據庫連接狀態
- **緩存指標**：Redis 命中率
- **存儲指標**：磁盤使用率

## 告警等級

### INFO
- 一般信息事件
- 系統狀態變更
- 操作記錄

### WARN
- 警告級別事件
- 性能下降
- 需要關注

### ERROR
- 錯誤級別事件
- 功能異常
- 需要處理

### FATAL
- 嚴重級別事件
- 系統故障
- 緊急處理

## SLI 監控

### 服務水平指標
- **可用性**：服務可用時間百分比
- **延遲**：P95/P99 響應時間
- **錯誤率**：錯誤請求百分比
- **吞吐量**：每秒處理請求數

### Treasury SLI
- **劃轉延遲**：資金劃轉 P95 延遲
- **成功率**：劃轉成功率
- **冪等命中**：冪等性命中率
- **失敗率**：劃轉失敗率

## 數據聚合

### 時間聚合
- 1分鐘聚合
- 5分鐘聚合
- 1小時聚合
- 1天聚合

### 維度聚合
- 按服務聚合
- 按交易對聚合
- 按市場聚合
- 按策略聚合

## 配置

服務使用以下配置：
- Redis：用於指標緩存和事件流
- ArangoDB：用於指標歷史存儲
- 告警規則配置
- 端口：8091（可通過環境變量 PORT 覆蓋）

## 部署

```bash
# 構建
go build -o s11-metrics .

# 運行
./s11-metrics
```

## 監控

服務提供以下監控指標：
- 指標收集延遲
- 聚合處理時間
- 告警觸發頻率
- 數據存儲成功率
- 查詢響應時間
