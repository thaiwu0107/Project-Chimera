# S9 Hypothesis Orchestrator

## 概述

S9 Hypothesis Orchestrator 是 Project Chimera 交易系統的假設測試編排器，負責執行交易策略的假設測試、回測和實驗，驗證策略的有效性。

## 功能

- **假設測試**：執行交易策略假設測試
- **回測分析**：進行歷史數據回測
- **實驗管理**：管理實驗配置和執行
- **結果分析**：分析實驗結果和統計
- **模型驗證**：驗證機器學習模型

## API 接口

### 健康檢查

- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 實驗管理

- `POST /experiments/run` - 執行實驗

#### Run Experiment

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

**回應**：
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
    "fold_results": [
      {
        "fold": 1,
        "train_period": "2022-01-01 to 2022-01-15",
        "test_period": "2022-01-16 to 2022-01-31",
        "performance": {
          "return": 0.05,
          "sharpe_ratio": 1.2,
          "max_drawdown": 0.03
        }
      }
    ],
    "statistical_significance": 0.95,
    "confidence_interval": [0.12, 0.18]
  },
  "recommendations": [
    "Strategy shows consistent positive performance",
    "Consider reducing position size during high volatility periods",
    "Monitor correlation with market indices"
  ]
}
```

## 服務間交互

### 入向（被呼叫）
- **S12 Web UI** → `POST /experiments/run` - 手動執行實驗
- **研究團隊** → `POST /experiments/run` - 策略研究實驗

### 出向（主動呼叫）
- **數據庫** → 讀取歷史數據和標籤
- **數據庫** → 存儲實驗結果
- **S12 Web UI** → 通知實驗完成

## 實驗類型

### 回測實驗
- 歷史數據回測
- 策略參數優化
- 風險評估

### 假設測試
- 統計假設檢驗
- A/B 測試
- 對照組比較

### 交叉驗證
- K-Fold 交叉驗證
- Walk-Forward 驗證
- 時間序列驗證

## 驗證方法

### Walk-Forward 驗證
- 滾動窗口訓練
- 前向測試
- 時間序列特性保持

### K-Fold 交叉驗證
- 數據分割驗證
- 統計穩健性
- 過擬合檢測

### 時間序列驗證
- 時間依賴性保持
- 未來數據洩露防護
- 真實交易模擬

## 性能指標

### 收益指標
- 總收益率
- 年化收益率
- 風險調整收益

### 風險指標
- 最大回撤
- 波動率
- VaR 和 CVaR

### 統計指標
- Sharpe 比率
- Sortino 比率
- Calmar 比率

### 交易指標
- 勝率
- 盈虧比
- 交易頻率

## 實驗管理

### 實驗配置
- 參數範圍設定
- 數據範圍選擇
- 指標配置

### 執行控制
- 並行執行
- 資源限制
- 優先級管理

### 結果存儲
- 結構化存儲
- 版本控制
- 結果比較

## 配置

服務使用以下配置：
- Redis：用於任務佇列和緩存
- ArangoDB：用於實驗數據存儲
- 計算資源配置
- 端口：8089（可通過環境變量 PORT 覆蓋）

## 部署

```bash
# 構建
go build -o s9-hypothesis .

# 運行
./s9-hypothesis
```

## 監控

服務提供以下監控指標：
- 實驗執行時間
- 實驗成功率
- 計算資源使用率
- 結果準確性
- 統計顯著性
