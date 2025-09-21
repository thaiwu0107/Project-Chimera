# S10 Config Service

## 概述

S10 Config Service 負責配置管理，包括因子/規則/標的版本管理、模擬器、敏感度分析、Promote 等功能。

## 功能描述

- **配置管理**：管理因子、規則、標的版本
- **模擬器**：配置變更的模擬和測試
- **敏感度分析**：分析配置參數的敏感度
- **Promote**：配置的推廣和部署
- **Canary**：配置的灰度發布和監控

## 實作進度

**實作進度：基礎架構完成，核心功能未實作 (15%)**

### ✅ 已完成功能
- [x] 基礎服務架構
- [x] 健康檢查端點 (`/health`, `/ready`)
- [x] API 框架基礎

### ⚠️ 待實作功能

#### 1. 配置中心（模擬器＋敏感度＋Promote）
- [ ] **差異估計**
  - [ ] 重放最近 N 天 signals → `Δ(trades, size_mult>1, skip率)`
  - [ ] 配置變更影響分析
  - [ ] 差異計算邏輯
- [ ] **敏感度分析**
  - [ ] 對關鍵特徵施加 ±ε 擾動，量測 flip rate 與 boundary margin
  - [ ] 敏感度計算邏輯
  - [ ] 擾動分析邏輯
- [ ] **守門機制**
  - [ ] MaxDD、AUC/Brier 滾動健康度 ≥ 閾值
  - [ ] 健康度檢查邏輯
  - [ ] 閾值管理

#### 2. 配置管理
- [ ] **版本管理**
  - [ ] 因子版本管理
  - [ ] 規則版本管理
  - [ ] 標的版本管理
- [ ] **配置熱載**
  - [ ] RCU 熱載機制
  - [ ] 配置變更事件廣播
  - [ ] 熱載驗證
- [ ] **定時任務**
  - [ ] 手動發起配置變更
  - [ ] Promote Canary 期間持續監測
  - [ ] 配置保活/熱載驗證

#### 3. API 接口
- [ ] **配置管理 API**
  - [ ] GET /bundles 查詢配置包
  - [ ] POST /bundles/create 創建配置包
  - [ ] PUT /bundles/{id} 更新配置包
- [ ] **模擬器 API**
  - [ ] POST /simulate 執行配置模擬
  - [ ] GET /simulate/{id} 查詢模擬結果
- [ ] **Promote API**
  - [ ] POST /promote 執行配置推廣
  - [ ] GET /promote/{id} 查詢推廣狀態
- [ ] **敏感度分析 API**
  - [ ] POST /sensitivity/analyze 執行敏感度分析
  - [ ] GET /sensitivity/{id} 查詢分析結果

#### 4. 數據處理
- [ ] **數據讀取**
  - [ ] 從 ArangoDB 讀取配置數據
  - [ ] 從 ArangoDB 讀取歷史 signals
  - [ ] 數據驗證和清理
- [ ] **數據寫入**
  - [ ] 配置變更寫入 ArangoDB
  - [ ] 模擬結果存儲
  - [ ] 數據一致性保證
- [ ] **數據分析**
  - [ ] 配置變更影響分析
  - [ ] 敏感度分析
  - [ ] 性能評估

#### 5. 監控和日誌
- [ ] **監控指標**
  - [ ] 配置變更成功率
  - [ ] 模擬執行性能
  - [ ] 敏感度分析準確性
- [ ] **日誌記錄**
  - [ ] 配置變更日誌
  - [ ] 模擬過程日誌
  - [ ] 錯誤日誌

#### 6. 配置管理
- [ ] **配置參數**
  - [ ] 模擬參數配置
  - [ ] 敏感度分析參數配置
  - [ ] Promote 參數配置
- [ ] **定時任務配置**
  - [ ] 監測頻率配置
  - [ ] 觸發條件配置

#### 6. 核心時序圖相關功能（基於時序圖實作）
- [ ] **配置管理觸發**
  - [ ] 監聽 `cfg:events` 事件
  - [ ] 觸發配置驗證和推廣流程
  - [ ] 執行 Lint/Dry-run/Simulate/Promote
- [ ] **配置管理流程**
  - [ ] POST /bundles → Lint → Dry-run（近 N 天 `signals` 重放）
  - [ ] POST /simulate → 差異估算 + 敏感度（±ε 擾動）
  - [ ] POST /promote → 寫 DB：`promotions`、切 `config_active`、發 `cfg:events`
- [ ] **配置查詢**
  - [ ] GET /active → 回 `bundle_id,rev`（供各服務啟動/熱載）
  - [ ] 配置版本管理

#### 7. 定時任務相關功能（基於定時任務實作）
- [ ] **模擬 + 敏感度批次（手動 / 每夜）**
  - [ ] 差異重放：對歷史 `signals` 套用新 bundle 重算決策，產出 `trades_new`、`delta_trades_pct`、規則命中分布、覆蓋度等
  - [ ] 敏感度分析（特徵擾動 ±ε）：決策翻轉率 `flip_pct = #{decision(x) ≠ decision(x+δ)} / N`
  - [ ] Local Lipschitz 近似（連續輸出，如 `size_mult`）：`L ≈ median(|f(x+δ)-f(x)| / ||δ||)`，`||δ|| = ε * ||x||`
  - [ ] 穩健分數：`robustness = 1 - flip_pct`
- [ ] **Active / 守門巡檢（每 1 分鐘）**
  - [ ] 在 Canary / Ramp 階段，滾動監控線上 MaxDD、AUC、Brier 等；越界即自動回滾
  - [ ] AUC（近 100 筆）：`pred_prob` vs `label` 的 ROC AUC
  - [ ] Brier Score：`Brier = (1/N) * Σ(p_i - y_i)²`

#### 8. 目標與範圍相關功能（基於目標與範圍實作）
- [ ] **前置依賴實作**
  - [ ] ArangoDB Collections：`config_bundles`、`simulations`、`promotions`、`config_active`
  - [ ] Redis Keys：`cfg:{events}`（推廣）

#### 9. 路過的服務相關功能（基於路過的服務實作）
- [ ] **POST /bundles** → Lint → Dry-run（近 N 天 `signals` 重放）
- [ ] **POST /simulate** → 差異估算 + 敏感度（±ε 擾動）
- [ ] **POST /promote** → 寫 DB：`promotions`、切 `config_active`、發 `cfg:events`
- [ ] **GET /active** → 回 `bundle_id,rev`（供各服務啟動/熱載）

#### 8. Integration 附錄相關功能（基於 Integration 附錄實作）
- [ ] **配置管理流程**
  - [ ] 配置 Lint：POST /bundles → Lint → Dry-run（近 N 天 `signals` 重放）
  - [ ] 配置模擬：POST /simulate → 差異估算 + 敏感度（±ε 擾動）
  - [ ] 配置推廣：POST /promote → 寫 DB：`promotions`、切 `config_active`、發 `cfg:events`
  - [ ] 配置查詢：GET /active → 回 `bundle_id,rev`（供各服務啟動/熱載）
- [ ] **事務一致性保證**
  - [ ] 冪等性保證：使用 `bundle_id` 作為冪等鍵
  - [ ] 狀態機管理：配置狀態 DRAFT → VALIDATED → PROMOTED → ACTIVE
  - [ ] 失敗恢復：系統崩潰後能夠恢復配置狀態
- [ ] **性能優化**
  - [ ] 並行處理：多個配置驗證並行執行
  - [ ] 批量處理：批量處理配置推廣任務
  - [ ] 緩存機制：配置結果緩存避免重複計算

#### 9. Hop-by-Hop 執行規格相關功能（基於 Hop-by-Hop 執行規格補遺實作）
- [ ] **POST /bundles** → **Lint** → **Dry-run**（近 N 天 `signals` 重放）
- [ ] **POST /simulate** → 差異估算 + **敏感度**（±ε 擾動）
- [ ] **POST /promote** → **寫 DB**：`promotions`、切 `config_active`、**發** `cfg:events`
  - [ ] **GET /active** → 回 `bundle_id,rev`（供各服務啟動/熱載）

#### 10. 功能規格書相關功能（基於功能規格書實作）
- [ ] **入向（被呼叫）API**
  - [ ] `GET /health`（所有服務）→ `HealthResponse{Status,Checks,...}`
  - [ ] `POST /bundles`（S12 研究/風控）→ `BundleUpsertResponse`
  - [ ] `POST /bundles/{id}/stage`（S12）→ `BundleStageResponse`
  - [ ] `POST /simulate`（S12）→ `SimulateResponse`
  - [ ] `POST /promote`（S12）→ `PromoteResponse`
  - [ ] `GET /active`（S2/S3/S4/S6/S12）→ `ActiveConfigResponse`
- [ ] **出向（主以事件）**
  - [ ] 〔Stream: cfg:events〕；寫 promotions/simulations/config_active
- [ ] **配置管理（新 bundle）**
  - [ ] S12（研究/風控）→ S10 `POST /bundles`（`BundleUpsertRequest`）→ `BundleUpsertResponse`
  - [ ] 建 DRAFT/更新
- [ ] **配置進場（Stage）**
  - [ ] S12 → S10 `POST /bundles/{id}/stage` → `BundleStageResponse`
  - [ ] 進入 STAGED
- [ ] **模擬＋敏感度**
  - [ ] S12 → S10 `POST /simulate`（`SimulateRequest`）→ `SimulateResponse`
  - [ ] S10 進行差異估算與穩健性
- [ ] **推廣/回滾**
  - [ ] S12 → S10 `POST /promote`（`PromoteRequest`）→ `PromoteResponse`
  - [ ] 觸發 cfg:events、Canary/Ramp/Full
  - [ ] S10 廣播〔cfg:events〕→ 各服務拉 `GET /active` 熱載
- [ ] **系統開機與配置收斂**
  - [ ] Sx → S10 `GET /active` 取得 rev/bundle_id
  - [ ] 訂閱〔Stream: cfg:events〕，本地 RCU 熱載
  - [ ] `GET /active` 失敗：退避重試（exponential backoff 5→30s）；未就緒前僅 `/health` OK=DEGRADED
- [ ] **配置推廣補償**
  - [ ] 推廣過程失敗 → S10 發 ROLLBACK，並在 promotions 記錄
  - [ ] 模擬或守門不過→回覆詳細原因；推廣過程護欄觸發→自動 ROLLBACK

### 🎯 實作優先順序
1. **高優先級**：配置管理和版本控制
2. **中優先級**：模擬器和敏感度分析
3. **低優先級**：Promote 和 Canary 機制
4. **低優先級**：Integration 附錄相關功能優化

## 技術規格

### 環境要求
- Go 1.19+
- ArangoDB
- Redis Cluster

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
1. **配置變更失敗**
   - 檢查配置格式
   - 確認權限設置
   - 查看日誌中的錯誤信息

2. **模擬結果不準確**
   - 檢查數據質量
   - 確認模擬參數
   - 查看模擬日誌

## 版本歷史

### v1.0.0
- 初始版本