# S10 Config Service

## 概述

S10 Config Service 是 Project Chimera 交易系統的配置管理服務，負責管理交易策略配置、執行配置推廣、模擬和敏感度分析。

## 功能

- **配置管理**：管理策略配置包（Bundle）
- **配置推廣**：執行配置的階段性推廣
- **模擬分析**：進行配置變更的模擬分析
- **敏感度分析**：分析配置參數的敏感度
- **配置分發**：向其他服務分發配置更新

## API 接口

### 健康檢查

- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 配置管理

- `POST /bundles` - 創建或更新配置包
- `POST /bundles/{id}/stage` - 配置包進場
- `POST /simulate` - 配置模擬分析
- `POST /promote` - 配置推廣
- `GET /active` - 獲取當前活躍配置

#### Upsert Bundle

**請求**：
```json
{
  "bundle_id": "bundle_001",
  "name": "momentum_strategy_v2",
  "description": "Updated momentum strategy",
  "status": "DRAFT",
  "factors": [
    {
      "name": "lookback_period",
      "value": 20,
      "type": "INTEGER",
      "min": 10,
      "max": 50
    }
  ],
  "rules": [
    {
      "name": "entry_threshold",
      "condition": "momentum > threshold",
      "action": "OPEN_LONG",
      "enabled": true
    }
  ]
}
```

**回應**：
```json
{
  "bundle_id": "bundle_001",
  "status": "DRAFT",
  "message": "Bundle created successfully",
  "validation_issues": []
}
```

#### Simulate Bundle

**請求**：
```json
{
  "bundle_id": "bundle_001",
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

**回應**：
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
      },
      "threshold": {
        "impact": 0.08,
        "optimal_value": 0.02,
        "confidence": 0.92
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
    "Consider gradual rollout",
    "Monitor performance closely"
  ]
}
```

#### Promote Bundle

**請求**：
```json
{
  "bundle_id": "bundle_001",
  "promotion_type": "CANARY",
  "target_services": ["s3-strategy", "s6-position"],
  "rollout_percentage": 10,
  "monitoring_duration": 3600
}
```

**回應**：
```json
{
  "promotion_id": "promo_001",
  "status": "IN_PROGRESS",
  "rollout_plan": {
    "phase": "CANARY",
    "target_percentage": 10,
    "duration_seconds": 3600,
    "next_phase": "RAMP"
  },
  "monitoring": {
    "metrics": ["error_rate", "latency", "success_rate"],
    "thresholds": {
      "error_rate": 0.01,
      "latency_p95": 100
    }
  }
}
```

#### Get Active Config

**請求**：`GET /active`

**回應**：
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

## 服務間交互

### 入向（被呼叫）
- **S12 Web UI** → `POST /bundles` - 配置管理
- **S2/S3/S4/S6/S12** → `GET /active` - 獲取當前配置
- **S12 Web UI** → `POST /simulate` - 模擬分析
- **S12 Web UI** → `POST /promote` - 配置推廣

### 出向（主動呼叫）
- **Redis Stream** → 廣播 cfg:events 事件
- **數據庫** → 存儲 promotions/simulations/config_active
- **其他服務** → 通知配置更新

## 配置生命週期

### 1. DRAFT 階段
- 創建配置包
- 參數設定
- 規則配置
- 內部驗證

### 2. STAGED 階段
- 配置進場
- 完整性檢查
- 依賴驗證
- 準備推廣

### 3. 推廣階段
- **CANARY**：小範圍測試（10%）
- **RAMP**：逐步推廣（50%）
- **FULL**：全量推廣（100%）

### 4. 監控階段
- 性能監控
- 錯誤追蹤
- 自動回滾
- 效果評估

## 推廣策略

### Canary 推廣
- 小範圍測試
- 快速發現問題
- 低風險驗證

### Ramp 推廣
- 逐步擴大範圍
- 持續監控
- 風險控制

### Full 推廣
- 全量部署
- 完整監控
- 性能優化

## 失敗處理

### 自動回滾
- 監控指標超標
- 自動觸發回滾
- 恢復穩定狀態

### 手動回滾
- 管理員觸發
- 選擇性回滾
- 影響評估

## 配置

服務使用以下配置：
- Redis：用於事件廣播和緩存
- ArangoDB：用於配置數據存儲
- 推廣配置：監控閾值和策略
- 端口：8090（可通過環境變量 PORT 覆蓋）

## 部署

```bash
# 構建
go build -o s10-config .

# 運行
./s10-config
```

## 監控

服務提供以下監控指標：
- 配置推廣成功率
- 推廣延遲
- 回滾次數
- 配置更新頻率
- 服務同步狀態
