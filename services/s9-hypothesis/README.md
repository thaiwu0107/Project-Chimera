# S9 Hypothesis Orchestrator

## 概述

S9 Hypothesis Orchestrator 負責假設與實驗管理，包括假設生成、回測/實驗執行（Walk-Forward、Purged K-Fold）等功能。

## 功能描述

- **假設管理**：管理和驗證交易假設
- **回測執行**：執行 Walk-Forward 和 Purged K-Fold 回測
- **實驗管理**：管理實驗流程和結果
- **模型訓練**：離線訓練和驗證模型
- **統計檢定**：執行統計檢定和 FDR 控制

## 實作進度

**實作進度：基礎架構完成，核心功能未實作 (15%)**

### ✅ 已完成功能
- [x] 基礎服務架構
- [x] 健康檢查端點 (`/health`, `/ready`)
- [x] API 框架基礎

### ⚠️ 待實作功能

#### 1. 假設與實驗（研究）
- [ ] **Walk-Forward 回測**
  - [ ] 滾動窗訓練→前推驗證
  - [ ] 滾動窗口管理
  - [ ] 前推驗證邏輯
- [ ] **Purged K-Fold 回測**
  - [ ] 避免資訊洩漏
  - [ ] K-Fold 分割邏輯
  - [ ] 資訊洩漏檢測
- [ ] **統計檢定**
  - [ ] p-value 計算
  - [ ] FDR 控制
  - [ ] 統計顯著性檢定

#### 2. 模型管理
- [ ] **離線訓練**
  - [ ] 監督式模型訓練（Logistic/XGBoost）
  - [ ] 模型驗證和選擇
  - [ ] 模型版本管理
- [ ] **線上推論**
  - [ ] 模型熱載
  - [ ] 推論服務
  - [ ] 模型性能監控
- [ ] **定時任務**
  - [ ] 每週/每月批次訓練
  - [ ] 模型重訓觸發
  - [ ] AUC/Brier 劣化檢測

#### 3. API 接口
- [ ] **實驗管理 API**
  - [ ] POST /experiments/run 執行實驗
  - [ ] GET /experiments/{id} 查詢實驗結果
  - [ ] GET /experiments/history 查詢實驗歷史
- [ ] **假設管理 API**
  - [ ] POST /hypotheses/create 創建假設
  - [ ] GET /hypotheses/{id} 查詢假設
  - [ ] PUT /hypotheses/{id} 更新假設
- [ ] **模型管理 API**
  - [ ] POST /models/train 訓練模型
  - [ ] GET /models/{id} 查詢模型
  - [ ] POST /models/deploy 部署模型

#### 4. 數據處理
- [ ] **數據讀取**
  - [ ] 從 ArangoDB 讀取歷史數據
  - [ ] 數據預處理和清洗
  - [ ] 特徵工程
- [ ] **數據寫入**
  - [ ] 實驗結果寫入 ArangoDB
  - [ ] 模型元數據存儲
  - [ ] 數據一致性保證
- [ ] **數據分析**
  - [ ] 回測結果分析
  - [ ] 統計分析
  - [ ] 性能評估

#### 5. 監控和日誌
- [ ] **監控指標**
  - [ ] 實驗執行成功率
  - [ ] 模型訓練性能
  - [ ] 回測執行時間
- [ ] **日誌記錄**
  - [ ] 實驗過程日誌
  - [ ] 模型訓練日誌
  - [ ] 錯誤日誌

#### 6. 配置管理
- [ ] **實驗參數配置**
  - [ ] 回測參數配置
  - [ ] 統計檢定參數配置
  - [ ] 模型參數配置
- [ ] **定時任務配置**
  - [ ] 訓練頻率配置
  - [ ] 實驗觸發條件配置

#### 6. 核心時序圖相關功能（基於時序圖實作）
- [ ] **實驗觸發**
  - [ ] 監聽 `ops:events` 事件
  - [ ] 觸發實驗執行流程
  - [ ] 執行 Walk-Forward 和 Purged K-Fold 回測
- [ ] **實驗執行流程**
  - [ ] 讀取 `hypotheses` 選 PENDING 狀態
  - [ ] 跑回測引擎（可離線批處理）→ KPI/檢定/FDR
  - [ ] 寫入 `experiments`（結果）、`hypotheses(status=CONFIRMED|REJECTED)`
- [ ] **事件發布**
  - [ ] 發布 `ops:events`（通知/審計）
  - [ ] 實驗完成通知

#### 7. 定時任務相關功能（基於定時任務實作）
- [ ] **回測 / 實驗批次（每日 / 每週）**
  - [ ] CAGR：`CAGR = (1 + R_tot)^(year/days) - 1`
  - [ ] 最大回撤：`MaxDD = max_t(1 - Equity_t / max_{τ≤t} Equity_τ)`
  - [ ] Calmar：`Calmar = CAGR / |MaxDD|`
  - [ ] 其他：Sharpe、Sortino、Profit Factor、Hit Ratio、平均持倉時間等
  - [ ] 統計檢定：差異顯著性：U-test／bootstrap CI；以 FDR 控制 α

#### 8. 目標與範圍相關功能（基於目標與範圍實作）
- [ ] **前置依賴實作**
  - [ ] ArangoDB Collections：`experiments`、`hypotheses(status)`
  - [ ] Redis Keys：`bt:{last_run_ts}`

#### 9. 路過的服務相關功能（基於路過的服務實作）
- [ ] **POST /experiments/run**
  - [ ] 讀：`hypotheses` 選 PENDING；配置樣本窗口/Walk-Forward
  - [ ] 跑：回測引擎（可離線批）→ KPI/檢定/FDR
  - [ ] 寫 DB：`experiments`（結果）、`hypotheses(status=CONFIRMED|REJECTED)`
  - [ ] 發：`ops:events` 通知/審計

#### 8. Integration 附錄相關功能（基於 Integration 附錄實作）
- [ ] **實驗執行流程**
  - [ ] 實驗選擇：讀取 `hypotheses` 選 PENDING 狀態的假設
  - [ ] 回測執行：跑回測引擎（可離線批處理）→ KPI/檢定/FDR
  - [ ] 結果記錄：寫入 `experiments`（結果）、`hypotheses(status=CONFIRMED|REJECTED)`
  - [ ] 事件發布：發布 `ops:events`（通知/審計）
- [ ] **事務一致性保證**
  - [ ] 冪等性保證：使用 `experiment_id` 作為冪等鍵
  - [ ] 狀態機管理：實驗狀態 PENDING → RUNNING → COMPLETED
  - [ ] 失敗恢復：系統崩潰後能夠重新執行實驗
- [ ] **性能優化**
  - [ ] 並行處理：多個實驗並行執行
  - [ ] 批量處理：批量處理實驗任務
  - [ ] 緩存機制：實驗結果緩存避免重複計算

#### 9. Hop-by-Hop 執行規格相關功能（基於 Hop-by-Hop 執行規格補遺實作）
- [ ] **POST /experiments/run**
  - [ ] 讀：`hypotheses` 選 PENDING；配置樣本窗口/Walk-Forward
  - [ ] 跑：回測引擎（可離線批）→ KPI/檢定/FDR
  - [ ] 寫 DB：`experiments`（結果）、`hypotheses(status=CONFIRMED|REJECTED)`
  - [ ] 發：`ops:events`（通知/審計）

#### 10. 功能規格書相關功能（基於功能規格書實作）
- [ ] **入向（被呼叫）API**
  - [ ] `GET /health`（所有服務）→ `HealthResponse{Status,Checks,...}`
  - [ ] `POST /experiments/run`（S12/研究）→ `ExperimentRunResponse`
- [ ] **出向（主以事件）**
  - [ ] 寫 experiments 結果；可通知 S12/S10
- [ ] **假設實驗/回測**
  - [ ] S12/研究 → S9 `POST /experiments/run`（`ExperimentRunRequest`）→ `ExperimentRunResponse`
  - [ ] S9 主要讀 DB/檔湖，不需叫 S3
  - [ ] 失敗補償：失敗者記號重試佇列；連續 3 次失敗→alerts(ERROR)

### 🎯 實作優先順序
1. **高優先級**：Walk-Forward 和 Purged K-Fold 回測
2. **中優先級**：模型訓練和統計檢定
3. **低優先級**：配置管理和優化
4. **低優先級**：Integration 附錄相關功能優化

## 技術規格

### 環境要求
- Go 1.19+
- ArangoDB
- Redis Cluster
- Python (用於 ML 模型)

### 開發指南

#### 本地開發
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
1. **實驗執行失敗**
   - 檢查數據完整性
   - 確認實驗參數正確
   - 查看日誌中的錯誤信息

2. **模型訓練失敗**
   - 檢查數據質量
   - 確認模型參數
   - 查看訓練日誌

## 版本歷史

### v1.0.0
- 初始版本