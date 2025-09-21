# S7 Label Backfill

## 概述

S7 Label Backfill 負責計算和回填交易標籤，包括 12/24/36 小時的淨利標籤與回寫功能。

## 功能描述

- **標籤計算**：計算 12/24/36 小時的淨利標籤
- **數據回寫**：將計算結果回寫到相應的數據庫
- **定時任務**：每小時掃描到期樣本並計算標籤

## 實作進度

**實作進度：基礎架構完成，核心功能未實作 (15%)**

### ✅ 已完成功能
- [x] 基礎服務架構
- [x] 健康檢查端點 (`/health`, `/ready`)
- [x] API 框架基礎

### ⚠️ 待實作功能

#### 1. 標籤回填（12/24/36h 淨利）
- [ ] **實現/未實現 PnL 計算**
  - [ ] 用 fills（含 fee）與 positions_snapshots 計算
  - [ ] 實現 PnL 計算邏輯
  - [ ] 未實現 PnL 計算邏輯
- [ ] **Funding（FUT）計算**
  - [ ] 累加結算期間的 amount_usdt
  - [ ] Funding 費用計算
- [ ] **淨利計算**
  - [ ] 淨利公式：`net_pnl = realized + unrealized - fees - funding`
  - [ ] ROI 計算：`net_roi = net_pnl / (Σ margin or Σ spot_cost)`
- [ ] **標籤生成**
  - [ ] 標籤：pos/neg/neutral by ROI 閾值（例：≥+0.5% / ≤-0.5%）
  - [ ] 標籤分類邏輯
- [ ] **定時任務**
  - [ ] 每小時掃描 `t0+{12,24,36}h` 到期樣本
  - [ ] 到期樣本檢測和處理

#### 2. 數據處理
- [ ] **數據讀取**
  - [ ] 從 ArangoDB 讀取 fills 數據
  - [ ] 從 ArangoDB 讀取 positions_snapshots 數據
  - [ ] 數據驗證和清理
- [ ] **數據寫入**
  - [ ] 標籤結果寫入 ArangoDB
  - [ ] 數據一致性保證
- [ ] **錯誤處理**
  - [ ] 數據缺失處理
  - [ ] 計算錯誤處理
  - [ ] 重試機制

#### 3. API 接口
- [ ] **標籤查詢 API**
  - [ ] GET /labels/{trade_id} 查詢特定交易標籤
  - [ ] GET /labels/history 查詢標籤歷史
- [ ] **手動觸發 API**
  - [ ] POST /labels/backfill 手動觸發標籤回填
  - [ ] POST /labels/recalculate 重新計算標籤

#### 4. 監控和日誌
- [ ] **監控指標**
  - [ ] 標籤計算成功率
  - [ ] 計算延遲監控
  - [ ] 數據質量監控
- [ ] **日誌記錄**
  - [ ] 計算過程日誌
  - [ ] 錯誤日誌
  - [ ] 審計日誌

#### 5. 配置管理
- [ ] **標籤參數配置**
  - [ ] ROI 閾值配置
  - [ ] 時間窗口配置
  - [ ] 計算參數配置
- [ ] **定時任務配置**
  - [ ] 掃描頻率配置
  - [ ] 批次大小配置

#### 6. 核心時序圖相關功能（基於時序圖實作）
- [ ] **標籤回填觸發**
  - [ ] 監聽 `pos:events(EXIT)` 事件
  - [ ] 觸發標籤回填流程
  - [ ] 計算 12/24/36h 標籤
- [ ] **標籤計算流程**
  - [ ] 聚合交易窗口內的 fills、funding_records、費用
  - [ ] 計算 ROI_net(H) 和對應標籤
  - [ ] 寫入 labels_12h/24h/36h（Upsert）
- [ ] **事件發布**
  - [ ] 發布 `labels:{ready}` 事件（可選 Stream；觸發 Autopsy）
  - [ ] 標籤計算完成通知

#### 7. 定時任務相關功能（基於定時任務實作）
- [ ] **標籤回填（每 15 分鐘）**
  - [ ] 查詢 `signals`：滿足 `t_0 + H ≤ now` 且尚未標籤
  - [ ] 聚合 `fills`（至 `t_0+H`）、`funding_records` 與 `fees`
  - [ ] 計算 `ROI_net`、`label`（pos/neg/neutral）並寫回
  - [ ] 公式（USDT 永續）：`ROI_net = (PnL_realized - Σ Fees - Σ Funding) / Margin_total`
  - [ ] 標籤規則（示例）：`ROI_net ≥ +0.005` → pos；`≤ -0.005` → neg；其餘 → neutral

#### 7. 目標與範圍相關功能（基於目標與範圍實作）
- [ ] **前置依賴實作**
  - [ ] ArangoDB Collections：`labels_12h/24h/36h`（Upsert）、`strategy_events(LABEL_WRITE)`
  - [ ] Redis Keys：`labels:{ready}`（可選）、`labels:{last_backfill_ts}`

#### 8. 路過的服務相關功能（基於路過的服務實作）
- [ ] **POST /labels/backfill?h=…**
  - [ ] 查：`signals` 滿足 `t_0+H<=now` & 無 `labels_H`
  - [ ] 聚合：該交易窗口內 `fills`、`funding_records`、費用
  - [ ] 算：`ROI_net(H)`、`label`
  - [ ] 寫 DB：`labels_12h/24h/36h`（Upsert）；`strategy_events(kind=LABEL_WRITE)`
  - [ ] 發：`labels:{ready}`（可選 Stream；觸發 Autopsy）

#### 9. Integration 附錄相關功能（基於 Integration 附錄實作）
- [ ] **標籤回填流程**
  - [ ] 數據聚合：聚合該交易窗口 `fills`、`funding_records`、費用
  - [ ] ROI 計算：計算 `ROI_net(H)` 和對應標籤
  - [ ] 數據寫入：寫入 `labels_12h/24h/36h`（Upsert）
  - [ ] 事件發布：發布 `labels:{ready}` 事件（可選 Stream；觸發 Autopsy）
- [ ] **事務一致性保證**
  - [ ] 冪等性保證：使用 `(signal_id,horizon)` 作為冪等鍵
  - [ ] 狀態機管理：標籤計算狀態 PENDING → COMPUTED → PUBLISHED
  - [ ] 失敗恢復：系統崩潰後能夠重新計算標籤
- [ ] **性能優化**
  - [ ] 並行處理：多個標籤計算並行執行
  - [ ] 批量處理：批量處理標籤計算任務
  - [ ] 緩存機制：標籤結果緩存避免重複計算

#### 10. Hop-by-Hop 執行規格相關功能（基於 Hop-by-Hop 執行規格補遺實作）
- [ ] **POST /labels/backfill?h=…**
  - [ ] 查：`signals` 滿足 `t0+H<=now` & 無 `labels_H`
  - [ ] 聚合：該交易窗口 `fills`、`funding_records`、費用
  - [ ] 算：`ROI_net(H)`、`label`
  - [ ] 寫 DB：`labels_12h/24h/36h`（Upsert）；`strategy_events(kind=LABEL_WRITE)`
  - [ ] 發：`labels:{ready}`（可選 Stream；觸發 Autopsy）

#### 11. 功能規格書相關功能（基於功能規格書實作）
- [ ] **入向（被呼叫）API**
  - [ ] `GET /health`（所有服務）→ `HealthResponse{Status,Checks,...}`
  - [ ] `POST /labels/backfill`（排程/手動）→ `BackfillResponse`
- [ ] **出向（主以事件）**
  - [ ] 更新 labels_*；（可選）推送 labels:ready（Stream）
- [ ] **標籤回填 / 復盤**
  - [ ] 觸發：t0 + 12/24/36h
  - [ ] S7 `POST /labels/backfill` → `BackfillResponse`
  - [ ] （選）對符合觸發條件者，S8 `POST /autopsy/{trade_id}` → 報告 URL
  - [ ] 失敗補償：失敗者記號重試佇列；連續 3 次失敗→alerts(ERROR)

### 🎯 實作優先順序
1. **高優先級**：標籤計算邏輯和數據處理
2. **中優先級**：API 接口和監控
3. **低優先級**：配置管理和優化
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
1. **標籤計算失敗**
   - 檢查數據完整性
   - 確認計算參數正確
   - 查看日誌中的錯誤信息

2. **數據不一致**
   - 檢查數據源
   - 確認計算邏輯
   - 查看數據驗證結果

## 版本歷史

### v1.0.0
- 初始版本