# S12 Web UI / API Gateway - 完整 API 文檔

## 概述

S12 Web UI / API Gateway 是 Project Chimera 交易系統的統一入口，提供完整的代理功能、RBAC 權限控制、統一錯誤處理和請求追蹤。

## 功能特性

### ✅ **已實現功能**
- **統一代理**：代理所有 S2-S11 服務的 API
- **RBAC 權限控制**：基於角色的訪問控制
- **統一錯誤處理**：標準化的錯誤響應格式
- **請求追蹤**：X-Request-Id 和 X-Forwarded-* headers
- **冪等性支持**：X-Idempotency-Key 傳遞
- **健康檢查**：包含依賴服務探針

### 🔄 **待實現功能**
- **速率限制**：基於用戶/角色的請求頻率限制
- **Circuit Breaker**：上游服務故障時的快速失敗
- **Schema 驗證**：JSON Schema 請求驗證
- **SSE 事件流**：實時事件推送

## API 接口

### 認證與授權

所有 API（除健康檢查外）都需要 JWT 認證：

```http
Authorization: Bearer <JWT_TOKEN>
```

### RBAC 角色層級

```
admin > risk_officer > researcher > trader > viewer
```

| 角色 | 權限 | 說明 |
|------|------|------|
| **viewer** | 查看權限 | 只能查看指標和告警 |
| **trader** | 交易權限 | 可以執行交易相關操作 |
| **researcher** | 研究權限 | 可以進行策略研究和實驗 |
| **risk_officer** | 風控權限 | 可以進行配置推廣 |
| **admin** | 管理權限 | 擁有所有權限 |

### 健康檢查

#### GET /health
**權限**：無需認證

**響應**：
```json
{
  "service": "s12-ui",
  "version": "v1.0.0",
  "status": "OK",
  "ts": 1640995200000,
  "uptime_ms": 3600000,
  "checks": [
    {
      "name": "redis",
      "status": "OK",
      "latency_ms": 5
    },
    {
      "name": "arangodb", 
      "status": "OK",
      "latency_ms": 10
    }
  ],
  "notes": "Service running normally"
}
```

#### GET /ready
**權限**：無需認證

**響應**：
```json
{
  "service": "s12-ui",
  "version": "v1.0.0",
  "status": "OK",
  "ts": 1640995200000,
  "uptime_ms": 3600000,
  "notes": "Service ready to accept requests"
}
```

### 系統控制

#### POST /kill-switch
**權限**：admin

**請求**：
```json
{
  "enable": true
}
```

**響應**：
```json
{
  "enabled": true
}
```

#### POST /treasury/transfer
**權限**：trader

**請求**：
```json
{
  "from": "SPOT",
  "to": "FUT", 
  "amount_usdt": 1000.0,
  "reason": "Trading capital allocation"
}
```

**響應**：
```json
{
  "transfer_id": "transfer_1640995200",
  "result": "OK",
  "message": "Transfer completed successfully"
}
```

### 代理 API

#### S2 Feature Generator

##### POST /features/recompute
**權限**：researcher

**請求**：
```json
{
  "symbols": ["BTCUSDT", "ETHUSDT"],
  "windows": ["4h", "1d"],
  "force": true
}
```

**響應**：
```json
{
  "job_id": "feat-20250920-001",
  "accepted": true
}
```

#### S3 Strategy Engine

##### POST /decide
**權限**：trader

**請求**：
```json
{
  "signal_id": "auto-or-manual",
  "symbol": "BTCUSDT",
  "config_rev": "CURRENT",
  "dry_run": true
}
```

**響應**：
```json
{
  "decision": {
    "action": "open",
    "size_mult": 1.0,
    "reason": "R-023, AUC=0.68"
  },
  "intent": {
    "market": "FUT",
    "side": "BUY",
    "qty": 0.0012,
    "exec_policy": "MakerThenTaker"
  }
}
```

#### S4 Order Router

##### POST /orders
**權限**：trader

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

**響應**：
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

##### POST /cancel
**權限**：trader

**請求**：
```json
{
  "order_id": "123",
  "reason": "USER_CANCEL",
  "cascade_oco": true
}
```

**響應**：
```json
{
  "result": "CANCELLED",
  "order_id": "123",
  "message": ""
}
```

#### S5 Reconciler

##### POST /reconcile
**權限**：admin

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

**響應**：
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
    }
  ]
}
```

#### S6 Position Manager

##### POST /positions/manage
**權限**：trader

**請求**：
```json
{
  "symbols": ["BTCUSDT"],
  "actions": ["TRAIL_SL", "PARTIAL_TP"],
  "dry_run": false
}
```

**響應**：
```json
{
  "managed": [
    {
      "symbol": "BTCUSDT",
      "actions": ["MOVE_SL", "TP_25%"]
    }
  ],
  "errors": []
}
```

#### S7 Label Backfill

##### POST /labels/backfill
**權限**：researcher

**請求**：
```json
{
  "symbol": "BTCUSDT",
  "market": "FUT",
  "from_time": 1640995200000,
  "to_time": 1641081600000,
  "horizon_hours": 24,
  "label_rules": [
    {
      "rule_id": "rule_001",
      "name": "profit_threshold",
      "threshold": 0.05,
      "enabled": true
    }
  ]
}
```

**響應**：
```json
{
  "updated": 100,
  "message": "Labels backfilled successfully"
}
```

#### S8 Autopsy Generator

##### POST /autopsy/{trade_id}
**權限**：researcher

**請求**：
```json
{
  "trade_id": "trade_001",
  "analysis_type": "FULL",
  "include_charts": true,
  "include_counterfactual": true,
  "peer_comparison": true
}
```

**響應**：
```json
{
  "report_id": "report_001",
  "trade_id": "trade_001",
  "status": "COMPLETED",
  "url": "https://minio.example.com/reports/report_001.pdf",
  "summary": {
    "pnl": 1250.75,
    "pnl_pct": 0.125,
    "max_drawdown": 0.05,
    "sharpe_ratio": 1.85,
    "win_rate": 0.68
  }
}
```

#### S9 Hypothesis Orchestrator

##### POST /experiments/run
**權限**：researcher

**請求**：
```json
{
  "experiment_id": "exp_001",
  "hypothesis": {
    "name": "momentum_strategy",
    "description": "Test momentum strategy effectiveness",
    "parameters": {
      "lookback_period": 20,
      "threshold": 0.02,
      "position_size": 0.1
    }
  },
  "data_range": {
    "from_time": 1640995200000,
    "to_time": 1641081600000,
    "symbols": ["BTCUSDT", "ETHUSDT"]
  },
  "validation_method": "WALK_FORWARD",
  "metrics": ["sharpe_ratio", "max_drawdown", "win_rate"]
}
```

**響應**：
```json
{
  "experiment_id": "exp_001",
  "status": "COMPLETED",
  "results": {
    "overall_performance": {
      "total_return": 0.15,
      "sharpe_ratio": 1.85,
      "max_drawdown": 0.08,
      "win_rate": 0.68,
      "profit_factor": 2.1
    },
    "statistical_significance": 0.95,
    "confidence_interval": [0.12, 0.18]
  },
  "recommendations": [
    "Strategy shows consistent positive performance",
    "Consider reducing position size during high volatility periods"
  ]
}
```

#### S10 Config Service

##### POST /bundles
**權限**：researcher

**請求**：
```json
{
  "bundle_id": "B-2025-09-20-001",
  "rev": 130,
  "factors": ["rv_pctile_30d", "rho_usdttwd_14"],
  "rules": ["R-023", "R-045"],
  "instruments": ["BTCUSDT"],
  "flags": {
    "spot_enabled": true
  },
  "status": "DRAFT"
}
```

**響應**：
```json
{
  "bundle_id": "B-2025-09-20-001",
  "rev": 130,
  "status": "DRAFT",
  "lint": {
    "passed": true
  }
}
```

##### POST /bundles/{id}/stage
**權限**：researcher

**請求**：無請求體

**響應**：
```json
{
  "bundle_id": "B-2025-09-20-001",
  "status": "STAGED",
  "message": "Bundle staged successfully"
}
```

##### POST /simulate
**權限**：researcher

**請求**：
```json
{
  "bundle_id": "B-2025-09-20-001",
  "simulation_type": "SENSITIVITY",
  "parameters": {
    "lookback_period": [15, 20, 25],
    "threshold": [0.01, 0.02, 0.03]
  },
  "data_range": {
    "from_time": 1640995200000,
    "to_time": 1641081600000
  }
}
```

**響應**：
```json
{
  "simulation_id": "sim_001",
  "status": "COMPLETED",
  "results": {
    "parameter_sensitivity": {
      "lookback_period": {
        "impact": 0.15,
        "optimal_value": 20,
        "confidence": 0.85
      }
    },
    "performance_impact": {
      "expected_return": 0.12,
      "risk_increase": 0.05,
      "stability_score": 0.88
    }
  },
  "recommendations": [
    "Parameter changes show positive impact",
    "Consider gradual rollout"
  ]
}
```

##### POST /promote
**權限**：risk_officer

**請求**：
```json
{
  "bundle_id": "B-2025-09-20-001",
  "to_rev": 130,
  "mode": "CANARY",
  "traffic_pct": 10,
  "duration_h": 168
}
```

**響應**：
```json
{
  "promotion_id": "prom-abc",
  "status": "PENDING",
  "guardrail": {
    "max_dd_pct": 0.18
  }
}
```

##### GET /active
**權限**：viewer

**響應**：
```json
{
  "config_rev": 123,
  "bundle_id": "bundle_001",
  "active_since": 1640995200000,
  "services": {
    "s3-strategy": {
      "config_rev": 123,
      "last_updated": 1640995200000,
      "status": "ACTIVE"
    },
    "s6-position": {
      "config_rev": 123,
      "last_updated": 1640995200000,
      "status": "ACTIVE"
    }
  }
}
```

#### S11 Metrics & Health

##### GET /metrics
**權限**：viewer

**查詢參數**：
- `metric`: 指標名稱（如 `pnl.daily`）
- `from_ts`: 開始時間戳
- `to_ts`: 結束時間戳
- `symbol`: 交易對（如 `BTCUSDT`）

**響應**：
```json
{
  "series": [
    {
      "ts": 1758300000000,
      "value": 12.4,
      "labels": {
        "symbol": "BTCUSDT"
      }
    }
  ]
}
```

##### GET /alerts
**權限**：viewer

**查詢參數**：
- `severity`: 告警級別（INFO/WARN/ERROR/FATAL）
- `source`: 告警來源
- `limit`: 返回數量限制

**響應**：
```json
{
  "items": [
    {
      "alert_id": "alert_001",
      "severity": "ERROR",
      "source": "s1-exchange",
      "message": "WebSocket connection lost",
      "ts": 1640995200000
    }
  ]
}
```

## 錯誤處理

### 統一錯誤響應格式

```json
{
  "error": "ERROR_CODE",
  "message": "Human readable error message",
  "request_id": "req_1640995200000"
}
```

### 錯誤碼對應

| HTTP 狀態碼 | 錯誤碼 | 說明 |
|-------------|--------|------|
| 400 | BAD_REQUEST | 請求格式錯誤 |
| 401 | UNAUTHORIZED | 未認證 |
| 403 | FORBIDDEN | 權限不足 |
| 404 | NOT_FOUND | 資源不存在 |
| 409 | CONFLICT | 冪等性衝突 |
| 422 | UNPROCESSABLE_ENTITY | 業務規則拒絕 |
| 429 | RATE_LIMITED | 請求頻率超限 |
| 502 | UPSTREAM_TIMEOUT | 上游服務超時 |
| 503 | UPSTREAM_UNAVAILABLE | 上游服務不可用 |
| 504 | UPSTREAM_ERROR | 上游服務錯誤 |

## 請求 Headers

### 必需 Headers
- `Authorization: Bearer <JWT_TOKEN>` - JWT 認證令牌

### 可選 Headers
- `X-Request-Id: <uuid>` - 請求追蹤 ID
- `X-Idempotency-Key: <key>` - 冪等性鍵值
- `Content-Type: application/json` - 請求內容類型

### 響應 Headers
- `X-Request-Id: <uuid>` - 請求追蹤 ID
- `Content-Type: application/json` - 響應內容類型

## 代理功能

### 服務映射

| S12 路徑 | 上游服務 | 上游路徑 | 權限 |
|----------|----------|----------|------|
| `/features/recompute` | S2 | `/features/recompute` | researcher |
| `/decide` | S3 | `/decide` | trader |
| `/orders` | S4 | `/orders` | trader |
| `/cancel` | S4 | `/cancel` | trader |
| `/reconcile` | S5 | `/reconcile` | admin |
| `/positions/manage` | S6 | `/positions/manage` | trader |
| `/labels/backfill` | S7 | `/labels/backfill` | researcher |
| `/autopsy/{trade_id}` | S8 | `/autopsy/{trade_id}` | researcher |
| `/experiments/run` | S9 | `/experiments/run` | researcher |
| `/bundles` | S10 | `/bundles` | researcher |
| `/bundles/{id}/stage` | S10 | `/bundles/{id}/stage` | researcher |
| `/simulate` | S10 | `/simulate` | researcher |
| `/promote` | S10 | `/promote` | risk_officer |
| `/active` | S10 | `/active` | viewer |
| `/metrics` | S11 | `/metrics` | viewer |
| `/alerts` | S11 | `/alerts` | viewer |

### 代理特性

1. **Header 傳遞**：自動傳遞 `Authorization`、`X-Request-Id`、`X-Idempotency-Key`
2. **代理 Headers**：添加 `X-Forwarded-For`、`X-Forwarded-Host`
3. **超時處理**：5 秒超時，自動返回 `UPSTREAM_TIMEOUT`
4. **錯誤映射**：上游錯誤自動映射為統一錯誤格式
5. **請求追蹤**：自動生成和傳遞 `X-Request-Id`

## 部署配置

### 環境變量
- `PORT`: 服務端口（默認 8092）
- `SERVICE_URLS`: 上游服務 URL 映射（JSON 格式）

### 服務發現
默認服務 URL 映射：
```json
{
  "s1": "http://localhost:8081",
  "s2": "http://localhost:8082", 
  "s3": "http://localhost:8083",
  "s4": "http://localhost:8084",
  "s5": "http://localhost:8085",
  "s6": "http://localhost:8086",
  "s7": "http://localhost:8087",
  "s8": "http://localhost:8088",
  "s9": "http://localhost:8089",
  "s10": "http://localhost:8090",
  "s11": "http://localhost:8091"
}
```

## 監控指標

### 網關指標
- `gateway_requests_total{route,code}` - 請求總數
- `upstream_latency_ms{service}` - 上游延遲
- `upstream_errors_total{service}` - 上游錯誤數
- `rbac_auth_failures_total{role}` - 認證失敗數

### 健康檢查
- 自檢：服務狀態、版本、運行時間
- 依賴探針：Redis、ArangoDB 連接狀態
- 上游探活：可選的上游服務健康檢查

## 安全特性

### JWT 認證
- 基於 JWT 的無狀態認證
- 角色信息嵌入在 JWT payload 中
- Token 過期時間管理

### RBAC 權限控制
- 基於角色的細粒度權限控制
- 角色層級繼承（高級角色自動擁有低級權限）
- 動態權限檢查

### 請求驗證
- 統一的請求格式驗證
- 惡意請求過濾
- 輸入參數白名單檢查

### 審計日誌
- 完整的請求/響應日誌
- 用戶操作追蹤
- 安全事件記錄
