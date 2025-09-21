# S12 Web UI / API GW ❌ **[未實作]**

Web UI / API GW - Web interface and API Gateway (代理/RBAC/Kill-switch)

## 📋 實作進度：25% (2/8 功能完成)

### ✅ 已完成功能
- [x] 基礎服務架構
- [x] Health Check API
- [x] RBAC 中間件框架
- [x] 代理請求框架
- [x] Kill Switch API
- [x] Treasury Transfer API

### ❌ 待實作功能

#### 1. 代理後端 API：驗票/RBAC → 轉發 → 回傳
- [ ] **認證系統**
  - [ ] JWT Token 驗證
  - [ ] 用戶身份驗證
- [ ] **RBAC 系統**
  - [ ] 角色權限檢查
  - [ ] 資源訪問控制
- [ ] **代理轉發**
  - [ ] 請求轉發邏輯
  - [ ] 響應處理邏輯

#### 2. POST /kill-switch：系統控制
- [ ] **Kill Switch 邏輯**
  - [ ] 設 `prod:{kill_switch}=ON`（TTL）
  - [ ] 發 `ops:events`
  - [ ] 各核心服務讀此旗標拒新倉
- [ ] **狀態管理**
  - [ ] Kill Switch 狀態管理
  - [ ] 狀態持久化

#### 3. POST /treasury/transfer：資金劃轉
- [ ] **劃轉審批**
  - [ ] 建立/審批劃轉請求
  - [ ] 內呼 S1
- [ ] **審計記錄**
  - [ ] 劃轉操作審計
  - [ ] 操作人員記錄

#### 4. 前端界面
- [ ] **Web 界面**
  - [ ] 交易控制面板
  - [ ] 系統監控面板
- [ ] **用戶界面**
  - [ ] 用戶登錄界面
  - [ ] 權限管理界面

#### 5. API 網關功能
- [ ] **請求路由**
  - [ ] 動態路由配置
  - [ ] 負載均衡
- [ ] **請求處理**
  - [ ] 請求驗證
  - [ ] 請求轉換

#### 6. 安全機制
- [ ] **認證機制**
  - [ ] 多因子認證
  - [ ] 會話管理
- [ ] **授權機制**
  - [ ] 細粒度權限控制
  - [ ] 資源級權限

#### 7. 監控和日誌
- [ ] **訪問日誌**
  - [ ] API 訪問日誌
  - [ ] 用戶操作日誌
- [ ] **性能監控**
  - [ ] API 性能監控
  - [ ] 系統性能監控

#### 8. 配置管理
- [ ] **動態配置**
  - [ ] 動態路由配置
  - [ ] 動態權限配置
- [ ] **配置熱載**
  - [ ] 配置熱載機制
  - [ ] 配置版本管理

### 🎯 實作優先順序
1. **高優先級**：認證系統和 RBAC 完善
2. **中優先級**：前端界面和 API 網關功能
3. **低優先級**：監控日誌和配置管理
4. **低優先級**：Integration 附錄相關功能優化

### 📊 相關資料寫入
- **DB Collections**：`treasury_transfers`（審批）
- **Redis Key/Stream**：`prod:{kill_switch}`、`ops:events`

## 概述

S12 Web UI / API Gateway 是 Project Chimera 交易系統的 Web 界面和 API 網關，提供統一的用戶界面和系統控制功能，包括資金劃轉、系統控制等關鍵操作。

## 功能

- **Web 界面**：提供交易系統的 Web 用戶界面
- **API 網關**：統一的外部 API 入口
- **系統控制**：Kill Switch 等系統控制功能
- **資金管理**：資金劃轉的對外接口
- **權限管理**：RBAC 權限控制
- **審計日誌**：完整的操作審計記錄

## API 接口

### 健康檢查

- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 系統控制

- `POST /kill-switch` - 系統停機開關
- `POST /treasury/transfer` - 資金劃轉

#### Kill Switch

**請求**：
```json
{
  "enable": true
}
```

**回應**：
```json
{
  "enabled": true
}
```

#### Treasury Transfer

**請求**：
```json
{
  "from": "SPOT",
  "to": "FUT",
  "amount_usdt": 1000.0,
  "reason": "Trading capital allocation"
}
```

**回應**：
```json
{
  "transfer_id": "transfer_1640995200",
  "result": "OK",
  "message": "Transfer completed successfully"
}
```

## 服務間交互

### 入向（被呼叫）
- **前端應用** → `GET /health` - 健康檢查
- **管理員** → `POST /kill-switch` - 系統控制
- **用戶** → `POST /treasury/transfer` - 資金劃轉

### 出向（主動呼叫）
- **S10 Config Service** → 配置管理操作
- **S5 Reconciler** → 對帳操作
- **S6 Position Manager** → 持倉管理
- **S4 Order Router** → 訂單取消
- **S11 Metrics** → 指標和告警查詢
- **S1 Exchange** → 內部資金劃轉

## 系統控制功能

### Kill Switch
- **啟用**：設置全域停機旗標
- **禁用**：解除停機狀態
- **廣播**：向所有服務發送停機事件
- **持久化**：在 Redis/DB 中保存狀態

### 權限控制
- **RBAC**：基於角色的訪問控制
- **操作權限**：細粒度的操作權限
- **審計追蹤**：完整的操作記錄

## 資金劃轉功能

### 對外接口
- **參數驗證**：驗證劃轉參數
- **風控檢查**：執行風險控制檢查
- **冪等性**：生成冪等性鍵值
- **分散鎖**：避免併發衝突
- **審計記錄**：完整的操作日誌

### 內部委派
- **S1 Exchange** → 執行實際劃轉
- **幣安 API** → 調用交易所 API
- **重試機制**：處理失敗重試
- **錯誤處理**：統一的錯誤處理

## 審計與合規

### 操作審計
- **用戶追蹤**：記錄操作用戶
- **IP 追蹤**：記錄操作來源 IP
- **時間戳**：精確的操作時間
- **操作詳情**：完整的操作參數

### 合規要求
- **數據保留**：審計數據保留策略
- **隱私保護**：敏感數據保護
- **訪問控制**：審計數據訪問控制

## 前端集成

### Web 界面
- **儀表板**：系統狀態儀表板
- **交易界面**：交易操作界面
- **配置管理**：策略配置管理
- **監控告警**：系統監控告警

### API 文檔
- **Swagger**：自動生成的 API 文檔
- **交互式**：可交互的 API 測試
- **版本管理**：API 版本管理

## 安全特性

### 認證授權
- **JWT Token**：基於 JWT 的認證
- **會話管理**：安全的會話管理
- **權限驗證**：細粒度的權限驗證

### 安全防護
- **CORS**：跨域請求控制
- **Rate Limiting**：請求頻率限制
- **輸入驗證**：嚴格的輸入驗證
- **SQL 注入防護**：數據庫安全防護

## 配置

服務使用以下配置：
- Redis：用於會話管理和分散鎖
- ArangoDB：用於審計日誌存儲
- JWT 配置：認證和授權配置
- 端口：8092（可通過環境變量 PORT 覆蓋）

## 部署

```bash
# 構建
go build -o s12-ui .

# 運行
./s12-ui
```

## 監控

服務提供以下監控指標：
- API 響應時間
- 請求成功率
- 認證失敗率
- 權限檢查延遲
- 審計日誌寫入成功率

## 詳細實作項目（基於目標與範圍文件）

### Web UI / API Gateway 功能詳細實作
- [ ] **RBAC 權限控制**
  - [ ] 實現角色提取和驗證
  - [ ] 實現權限檢查邏輯
  - [ ] 實現 JWT Token 處理
- [ ] **Kill Switch 功能**
  - [ ] 實現系統開關控制
  - [ ] 實現開關狀態管理
  - [ ] 實現開關影響範圍控制
- [ ] **金庫劃轉**
  - [ ] 實現劃轉請求驗證
  - [ ] 實現冪等性檢查
  - [ ] 實現分散鎖管理
  - [ ] 實現審計日誌記錄
- [ ] **API 代理**
  - [ ] 實現請求路由和代理
  - [ ] 實現響應聚合和轉發
  - [ ] 實現錯誤處理和重試
- [ ] **前端 UI**
  - [ ] 實現交易儀表板
  - [ ] 實現配置管理界面
  - [ ] 實現監控和告警界面

#### 6. 核心時序圖相關功能（基於時序圖實作）
- [ ] **對帳觸發界面**
  - [ ] POST /reconcile {mode=ALL} 對帳觸發
  - [ ] 對帳結果展示界面
  - [ ] ReconcileResponse{summary, fixed, adopted, closed} 顯示
- [ ] **API Gateway 功能**
  - [ ] 對帳請求路由和代理
  - [ ] 對帳結果聚合和轉發
  - [ ] 錯誤處理和重試
- [ ] **管理界面**
  - [ ] 孤兒訂單管理界面
  - [ ] 倉位狀態監控界面
  - [ ] 對帳歷史記錄查詢
- [ ] **事件監控**
  - [ ] alerts{severity=ERROR, msg="orphan closed"} 告警顯示
  - [ ] strategy:reconciled 事件監控
  - [ ] 對帳狀態實時更新

#### 7. 服務與資料流相關功能（基於服務與資料流實作）
- [ ] **Web UI / API 代理**
  - [ ] 表單/儀表板界面
  - [ ] 代理其他服務 API
  - [ ] RBAC 權限控制
  - [ ] OpenAPI 驗證中介層
- [ ] **外部呼叫 API**
  - [ ] S12 → S3 `/decide`：前端試算/下單前決策（dry_run 可）
  - [ ] S12 → S4 `/orders|/cancel`：實際 FUT/SPOT 下單、OCO、撤單
  - [ ] S12 → S6 `/positions/manage`：手動觸發 trailing/partial
  - [ ] S12 → S7 `/labels/backfill`：資料科學重算窗口
  - [ ] S12 → S8 `/autopsy/{trade_id}`：復盤生成/重建
  - [ ] S12 → S9 `/experiments/run`：研究實驗
  - [ ] S12 → S10 `/bundles|/simulate|/promote`：配置治理
  - [ ] S12 → S11 `/metrics|/alerts`：監控頁資料
- [ ] **工程交付要點**
  - [ ] `GET /health`（依賴樹狀回報）
  - [ ] 中介層驗證：S12 對所有 API 做 OpenAPI schema 驗證
  - [ ] 事件冪等：下單/撤單/對帳均以 `intent_id/client_order_id` 控制

#### 8. 定時任務相關功能（基於定時任務實作）
- [ ] **UI 快取 / 清理（每 5 分鐘）**
  - [ ] 清理過期快取；刷新 `kill_switch`、`active_rev` 等全域狀態 TTL
  - [ ] 偵測 TTL 失效風險 → 立即刷新或標記 WARN

#### 9. 目標與範圍相關功能（基於目標與範圍實作）
- [ ] **前置依賴實作**
  - [ ] ArangoDB Collections：`treasury_transfers`（審批）
  - [ ] Redis Keys：`prod:{kill_switch}`、`ops:events`
- [ ] **風險與緩解**
  - [ ] Redis Cluster slot 移轉：使用官方 cluster client；關鍵操作具重試策略

#### 10. 路過的服務相關功能（基於路過的服務實作）
- [ ] **代理後端 API**：驗票/RBAC → 轉發 → 回傳
- [ ] **POST /kill-switch**：設 `prod:{kill_switch}=ON`（TTL）；發 `ops:events`；各核心服務讀此旗標拒新倉
- [ ] **POST /treasury/transfer**：建立/審批劃轉請求 → 內呼 S1

#### 11. Integration 附錄相關功能（基於 Integration 附錄實作）
- [ ] **對帳請求代理**
  - [ ] 對帳請求：代理對帳請求到 S5 Reconciler
  - [ ] 請求驗證：驗證 RBAC 權限和請求格式
  - [ ] 響應處理：處理對帳響應並返回給前端
  - [ ] 事件監控：監控對帳完成事件
- [ ] **Kill-switch 管理**
  - [ ] Kill-switch 設置：POST /kill-switch 設 `prod:{kill_switch}=ON`（TTL）
  - [ ] 事件發布：發 `ops:events`；各核心服務讀此旗標拒新倉
  - [ ] 狀態監控：監控 Kill-switch 狀態變化
- [ ] **金庫劃轉管理**
  - [ ] 劃轉請求：POST /treasury/transfer 建立/審批劃轉請求
  - [ ] 審批流程：人工審批劃轉請求
  - [ ] 執行代理：內呼 S1 執行劃轉
- [ ] **事務一致性保證**
  - [ ] 冪等性保證：使用 `request_id` 作為冪等鍵
  - [ ] 狀態機管理：請求狀態 PENDING → APPROVED → EXECUTED
  - [ ] 失敗恢復：系統崩潰後能夠恢復請求狀態
- [ ] **性能優化**
  - [ ] 並行處理：多個 API 請求並行處理
  - [ ] 批量處理：批量處理審批任務
  - [ ] 緩存機制：API 響應緩存提高性能

#### 12. Hop-by-Hop 執行規格相關功能（基於 Hop-by-Hop 執行規格補遺實作）
- [ ] **代理後端 API**：驗票/RBAC → 轉發 → 回傳
- [ ] **POST /kill-switch**：設 `prod:{kill_switch}=ON`（TTL）；發 `ops:events`；各核心服務讀此旗標拒新倉
- [ ] **POST /treasury/transfer**：建立/審批劃轉請求 → 內呼 S1

#### 13. 功能規格書相關功能（基於功能規格書實作）
- [ ] **入向（被呼叫）API**
  - [ ] `GET /health`（所有服務）→ `HealthResponse{Status,Checks,...}`
  - [ ] `POST /kill-switch`（操控）→ `KillSwitchResponse`
  - [ ] `POST /treasury/transfer`（對外）→ `TransferResponse`
- [ ] **出向（主以事件）**
  - [ ] **配置**：`/bundles` `/simulate` `/promote` `/active` → S10
  - [ ] **操作**：`/reconcile` → S5、`/positions/manage` → S6、`/cancel` → S4
  - [ ] **監控**：`/metrics` `/alerts` → S11
  - [ ] **資金**：`/xchg/treasury/transfer` → S1（私有）
- [ ] **Kill Switch**
  - [ ] S12（操控）`POST /kill-switch`（`KillSwitchRequest`）→ `KillSwitchResponse`
  - [ ] 設置全域停機旗標（Redis/DB）並廣播事件
- [ ] **金庫資金劃轉（對外）**
  - [ ] S12 → S1(私有) `POST /xchg/treasury/transfer`（`TransferRequest`）→ `TransferResponse`
  - [ ] S12 對外 `POST /treasury/transfer` 接入後委派 S1
  - [ ] 成功：寫 strategy_events(kind=TREASURY_TRANSFER)；失敗記 alerts
  - [ ] 鎖：`lock:treasury:<from>:<to>`（Redis）
  - [ ] 失敗補償：重試 N 次；連續失敗升級 FATAL
- [ ] **配置模擬＋敏感度 → 推廣**
  - [ ] 觸發：人員在 S12 提交新 bundle
  - [ ] S12 → S10 `POST /bundles`（DRAFT）
  - [ ] S12 → S10 `POST /simulate`（差異估算＋敏感度）
  - [ ] S12 → S10 `POST /bundles/{id}/stage`（進 STAGED）
  - [ ] S12 → S10 `POST /promote`（CANARY/RAMP/FULL）
  - [ ] S10 廣播〔cfg:events〕→ 各服務拉 `GET /active` 熱載
  - [ ] 失敗補償：模擬或守門不過→回覆詳細原因；推廣過程護欄觸發→自動 ROLLBACK
- [ ] **開發建議**
  - [ ] 契約測試：把呼叫關係「逐一」落入各服務 README 的 "Integration" 小節，並在測試加上契約測試（contract test）
  - [ ] 序列圖：在 S12 做一個 "交易生命周期" 的序列圖頁（mermaid），把核心劇本圖像化，方便新同事理解

### 🎯 實作優先順序
