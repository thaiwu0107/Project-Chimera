# S8 Autopsy

## 概述

S8 Autopsy 負責交易復盤分析，包括 TL;DR、TCA（交易成本分析）、Peer 對比、反事實分析等功能。

## 功能描述

- **交易復盤**：分析已完成交易的詳細情況
- **TCA 分析**：交易成本分析，包括滑價和費用分析
- **Peer 對比**：與同類交易進行對比分析
- **反事實分析**：分析不同參數下的可能結果
- **敘事生成**：生成交易分析的自然語言描述

## 實作進度

**實作進度：基礎架構完成，核心功能未實作 (15%)**

### ✅ 已完成功能
- [x] 基礎服務架構
- [x] 健康檢查端點 (`/health`, `/ready`)
- [x] API 框架基礎

### ⚠️ 待實作功能

#### 1. 復盤（Autopsy：TCA/Peer/反事實/敘事）
- [ ] **TCA 滑價分析**
  - [ ] 滑價計算：同 1.8；費用佔比 `cost_share = (fees+funding)/gross_pnl`
  - [ ] 滑價分析邏輯
  - [ ] 費用分析邏輯
- [ ] **Peer 對比分析**
  - [ ] 分組（方向/Regime/size bucket）
  - [ ] 計算百分位（winrate/ROI/持倉時間）
  - [ ] 對比分析邏輯
- [ ] **反事實分析**
  - [ ] 將 tp/sl/size 替換為 ±Δ，重放路徑估計 ΔROI
  - [ ] 反事實計算邏輯
  - [ ] 路徑重放邏輯
- [ ] **敘事生成**
  - [ ] 由規則命中 + SHAP（若有 ML）轉自然語句
  - [ ] 自然語言生成邏輯
  - [ ] 敘事模板管理

#### 2. 定時任務
- [ ] **復盤觸發**
  - [ ] 交易關閉即觸發復盤
  - [ ] 手動重建復盤
- [ ] **數據重建**
  - [ ] 缺圖/缺段修復
  - [ ] 數據完整性檢查

#### 3. API 接口
- [ ] **復盤查詢 API**
  - [ ] GET /autopsy/{trade_id} 查詢特定交易復盤
  - [ ] GET /autopsy/history 查詢復盤歷史
- [ ] **復盤生成 API**
  - [ ] POST /autopsy/generate 生成復盤
  - [ ] POST /autopsy/rebuild 重建復盤

#### 4. 數據處理
- [ ] **數據讀取**
  - [ ] 從 ArangoDB 讀取交易數據
  - [ ] 從 ArangoDB 讀取市場數據
  - [ ] 數據驗證和清理
- [ ] **數據寫入**
  - [ ] 復盤結果寫入 ArangoDB
  - [ ] 數據一致性保證
- [ ] **數據分析**
  - [ ] 統計分析邏輯
  - [ ] 對比分析邏輯
  - [ ] 趨勢分析邏輯

#### 5. 監控和日誌
- [ ] **監控指標**
  - [ ] 復盤生成成功率
  - [ ] 分析延遲監控
  - [ ] 數據質量監控
- [ ] **日誌記錄**
  - [ ] 分析過程日誌
  - [ ] 錯誤日誌
  - [ ] 審計日誌

#### 6. 配置管理
- [ ] **分析參數配置**
  - [ ] TCA 參數配置
  - [ ] Peer 對比參數配置
  - [ ] 反事實分析參數配置
- [ ] **敘事模板配置**
  - [ ] 敘事模板管理
  - [ ] 模板參數配置

#### 6. 核心時序圖相關功能（基於時序圖實作）
- [ ] **復盤觸發**
  - [ ] 監聽 `pos:events(EXIT|STAGNATED)` 事件
  - [ ] 觸發復盤生成流程
  - [ ] 計算 TCA/Peer/反事實分析
- [ ] **復盤生成流程**
  - [ ] 拉取 `signals/orders/fills/positions_snapshots/funding_records/labels_*`
  - [ ] 計算 ROE 曲線、TCA/滑價、Peer 分位、反事實、敘事摘要
  - [ ] 寫入 `autopsy_reports{trade_id,...}`；物件存 MinIO
- [ ] **事件發布**
  - [ ] 發布 `strategy_events(kind=AUTOPSY_DONE)` 事件
  - [ ] 指標 `metrics:events:s8.autopsy_latency`

#### 7. 定時任務相關功能（基於定時任務實作）
- [ ] **復盤生成（每小時 / 事件驅動）**
  - [ ] 滑價（bps）：`slip_bps = sign(side) * (P_fill - P_mid_at_send) / P_mid_at_send * 10^4`
  - [ ] 成本占比：`cost_share = (Σ Fees + Σ Funding + Σ |Slippage|) / (|PnL_gross| + ε)`
  - [ ] Peer 對比：同 cohort（同 Regime、同方向）計算分位；輸出 TL;DR 與圖表

#### 8. 目標與範圍相關功能（基於目標與範圍實作）
- [ ] **前置依賴實作**
  - [ ] ArangoDB Collections：`autopsy_reports`（Upsert）
  - [ ] MinIO：物件存 `autopsy/<trade_id>.html|pdf`

#### 9. 路過的服務相關功能（基於路過的服務實作）
- [ ] **POST /autopsy/{trade_id} 或 監聽 `pos:events(EXIT|STAGNATED)`**
  - [ ] 拉：`signals/orders/fills/positions_snapshots/funding_records/labels_*`
  - [ ] 算：ROE 曲線、TCA/滑價、Peer 分位、反事實、敘事摘要
  - [ ] 寫 DB：`autopsy_reports{trade_id,...}`；物件存 MinIO `autopsy/<trade_id>.html|pdf`
  - [ ] 發：`strategy_events(kind=AUTOPSY_DONE)`；指標 `s8.autopsy_latency`

#### 8. Integration 附錄相關功能（基於 Integration 附錄實作）
- [ ] **復盤生成流程**
  - [ ] 數據拉取：拉取 `signals/orders/fills/positions_snapshots/funding_records/labels_*`
  - [ ] 分析計算：計算 ROE 曲線、TCA/滑價、Peer 分位、反事實、敘事摘要
  - [ ] 報告生成：生成復盤報告並存儲到 MinIO
  - [ ] 事件發布：發布 `strategy_events(kind=AUTOPSY_DONE)` 事件
- [ ] **事務一致性保證**
  - [ ] 冪等性保證：使用 `trade_id` 作為冪等鍵
  - [ ] 狀態機管理：復盤狀態 PENDING → ANALYZING → COMPLETED
  - [ ] 失敗恢復：系統崩潰後能夠重新生成復盤
- [ ] **性能優化**
  - [ ] 並行處理：多個復盤分析並行執行
  - [ ] 批量處理：批量處理復盤任務
  - [ ] 緩存機制：復盤結果緩存避免重複計算

#### 9. Hop-by-Hop 執行規格相關功能（基於 Hop-by-Hop 執行規格補遺實作）
- [ ] **POST /autopsy/{trade_id} 或 監聽 `pos:events(EXIT|STAGNATED)`**
  - [ ] 拉：`signals/orders/fills/positions_snapshots/funding_records/labels_*`
  - [ ] 算：ROE 曲線、TCA/滑價、Peer 分位、反事實、敘事摘要
  - [ ] 寫 DB：`autopsy_reports{trade_id,...}`；物件存 MinIO `autopsy/<trade_id>.html|pdf`
  - [ ] 發：`strategy_events(kind=AUTOPSY_DONE)`；指標 `metrics:events:s8.autopsy_latency`

#### 10. 功能規格書相關功能（基於功能規格書實作）
- [ ] **入向（被呼叫）API**
  - [ ] `GET /health`（所有服務）→ `HealthResponse{Status,Checks,...}`
  - [ ] `POST /autopsy/{trade_id}`（S12/排程）→ `AutopsyResponse{ReportID, Url}`
- [ ] **出向（主以事件）**
  - [ ] 寫 autopsy_reports、MinIO 檔案；回傳 URL
- [ ] **復盤報告**
  - [ ] S12/排程 → S8 `POST /autopsy/{trade_id}`（`AutopsyRequest`）→ `AutopsyResponse`
  - [ ] 產出 report id / URL（MinIO）
  - [ ] 失敗補償：失敗者記號重試佇列；連續 3 次失敗→alerts(ERROR)

### 🎯 實作優先順序
1. **高優先級**：TCA 分析和 Peer 對比
2. **中優先級**：反事實分析和敘事生成
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
1. **復盤生成失敗**
   - 檢查數據完整性
   - 確認分析參數正確
   - 查看日誌中的錯誤信息

2. **分析結果不準確**
   - 檢查數據質量
   - 確認分析邏輯
   - 查看對比基準

## 版本歷史

### v1.0.0
- 初始版本