# S11 Metrics & Health

## 概述

S11 Metrics & Health 負責指標彙整與健康監控，包括策略/執行/穩定性指標匯總、SLI/SLO 監控、告警等功能。

## 功能描述

- **指標彙整**：匯總策略、執行、穩定性指標
- **健康監控**：監控系統健康狀態
- **SLI/SLO**：服務水平指標和目標監控
- **告警系統**：異常情況告警
- **儀表板**：監控數據展示

## 實作進度

**實作進度：基礎架構完成，核心功能未實作 (15%)**

### ✅ 已完成功能
- [x] 基礎服務架構
- [x] 健康檢查端點 (`/health`, `/ready`)
- [x] API 框架基礎

### ⚠️ 待實作功能

#### 1. 指標彙整與健康（Observability）
- [ ] **核心 SLI 監控**
  - [ ] 路由 P95 延遲：下單→回執
  - [ ] 風險守門誤差：實際 spread/depth 相對門檻超界率
  - [ ] Redis Stream Lag：`consumer_lag = last_produced_id - last_ack_id`
  - [ ] 策略績效：PF, Sharpe, WinRate, MaxDD
- [ ] **定時任務**
  - [ ] 每 1m 聚合指標
  - [ ] 每日匯總指標
- [ ] **失效偵測門檻**
  - [ ] `maker_fill_ratio`（近 1h） < `θ_maker`（例 0.25）→ S4 降級為 Market
  - [ ] `insufficient_balance_rate`（近 1d） > `θ_ib`（例 0.1）→ 啟用自動降額或排隊重試
  - [ ] AUC/Brier（滾動 100 筆） 低於門檻 → S10 禁止 Promote / 觸發 Canary 回滾
  - [ ] 路由延遲 P95 超SLO（例 500ms）→ 降級（禁 TWAP/Maker）

#### 2. 觀測性（SLI/SLO）與告警
- [ ] **SLI 監控**
  - [ ] 路由 P95 延遲監控
  - [ ] 守門誤差率監控
  - [ ] Stream Lag 監控
  - [ ] WS 掉線率監控
  - [ ] 寫入延遲監控
  - [ ] MaxDD 監控
- [ ] **SLO 目標**
  - [ ] 路由 P95 ≤ 500ms
  - [ ] Stream Lag ≤ 2s
  - [ ] WS 可用率 ≥ 99.5%
- [ ] **告警機制**
  - [ ] SLO 連續 3 個窗口違反告警
  - [ ] 守門越界告警
  - [ ] Canary guardrail 觸發告警

#### 3. API 接口
- [ ] **指標查詢 API**
  - [ ] GET /metrics 查詢指標數據
  - [ ] GET /metrics/history 查詢指標歷史
  - [ ] GET /metrics/sli 查詢 SLI 數據
- [ ] **告警管理 API**
  - [ ] GET /alerts 查詢告警
  - [ ] POST /alerts/acknowledge 確認告警
  - [ ] PUT /alerts/{id} 更新告警狀態
- [ ] **健康檢查 API**
  - [ ] GET /health 系統健康檢查
  - [ ] GET /health/dependencies 依賴健康檢查

#### 4. 數據處理
- [ ] **數據收集**
  - [ ] 從各服務收集指標數據
  - [ ] 從 Redis Streams 收集事件數據
  - [ ] 數據聚合和計算
- [ ] **數據存儲**
  - [ ] 指標數據存儲
  - [ ] 告警記錄存儲
  - [ ] 數據一致性保證
- [ ] **數據分析**
  - [ ] 指標趨勢分析
  - [ ] 異常檢測
  - [ ] 性能評估

#### 5. 監控和日誌
- [ ] **監控指標**
  - [ ] 指標收集成功率
  - [ ] 告警觸發準確性
  - [ ] 系統健康狀態
- [ ] **日誌記錄**
  - [ ] 指標收集日誌
  - [ ] 告警觸發日誌
  - [ ] 錯誤日誌

#### 6. 配置管理
- [ ] **監控參數配置**
  - [ ] SLI/SLO 閾值配置
  - [ ] 告警規則配置
  - [ ] 聚合參數配置
- [ ] **定時任務配置**
  - [ ] 收集頻率配置
  - [ ] 聚合頻率配置
  - [ ] 告警檢查頻率配置

#### 6. 核心時序圖相關功能（基於時序圖實作）
- [ ] **指標收集觸發**
  - [ ] 監聽 `metrics:events:*` 事件
  - [ ] 觸發指標聚合和健康監控流程
  - [ ] 執行 SLI/SLO 監控
- [ ] **指標聚合流程**
  - [ ] 收 `metrics:events:*` & 拉關鍵指標
  - [ ] 聚合：寫 DB `metrics_timeseries/strategy_metrics_daily`
  - [ ] 判等級：綜合 `maker_fill_ratio / ib_rate / AUC / Brier / router_p95 / stream_lag`
- [ ] **健康狀態管理**
  - [ ] 寫 Redis：`prod:{health}:system:state=GREEN|YELLOW|ORANGE|RED`
  - [ ] 發告警：`alerts`（DB + 通知）
  - [ ] GET /metrics, /alerts → 提供前端面板

#### 7. 定時任務相關功能（基於定時任務實作）
- [ ] **健康巡檢彙總（每 10 秒）**
  - [ ] 合成健康狀態：核心指標（示例）：`losing_streak`、`maker_fill_ratio`、`ib_rate`（資金不足率）、`stream_lag`、`router_p95`
  - [ ] 狀態映射：依門檻表將多指標合成 GREEN / YELLOW / ORANGE / RED（加權或 max-severity 規則）

#### 8. 目標與範圍相關功能（基於目標與範圍實作）
- [ ] **前置依賴實作**
  - [ ] ArangoDB Collections：`metrics_timeseries`、`strategy_metrics_daily`、`alerts`
  - [ ] Redis Keys：`prod:{health}:system:state`

#### 9. 路過的服務相關功能（基於路過的服務實作）
- [ ] **收 `metrics:events:*` & 拉關鍵指標**
  - [ ] 聚合：寫 DB `metrics_timeseries/strategy_metrics_daily`
  - [ ] 判等級：綜合 `maker_fill_ratio / ib_rate / AUC / Brier / router_p95 / stream_lag`
  - [ ] 寫 Redis：`prod:{health}:system:state=GREEN|YELLOW|ORANGE|RED`
  - [ ] 發告警：`alerts`（DB + 通知）
- [ ] **GET /metrics, /alerts** → 提供前端面板

#### 8. Integration 附錄相關功能（基於 Integration 附錄實作）
- [ ] **指標聚合和健康監控**
  - [ ] 指標收集：收 `metrics:events:*` & 拉關鍵指標
  - [ ] 數據聚合：寫 DB `metrics_timeseries/strategy_metrics_daily`
  - [ ] 健康等級判定：綜合 `maker_fill_ratio / ib_rate / AUC / Brier / router_p95 / stream_lag`
  - [ ] 健康狀態更新：寫 Redis `prod:{health}:system:state=GREEN|YELLOW|ORANGE|RED`
  - [ ] 告警發布：發 `alerts`（DB + 通知）
- [ ] **API 接口**
  - [ ] GET /metrics：提供前端面板指標數據
  - [ ] GET /alerts：提供告警數據
- [ ] **事務一致性保證**
  - [ ] 冪等性保證：使用 `metric_id` 作為冪等鍵
  - [ ] 狀態機管理：指標狀態 PENDING → COLLECTED → AGGREGATED
  - [ ] 失敗恢復：系統崩潰後能夠重新收集指標
- [ ] **性能優化**
  - [ ] 並行處理：多個指標收集並行執行
  - [ ] 批量處理：批量處理指標聚合任務
  - [ ] 緩存機制：指標結果緩存避免重複計算

#### 9. Hop-by-Hop 執行規格相關功能（基於 Hop-by-Hop 執行規格補遺實作）
- [ ] **收 `metrics:events:*` & 拉關鍵指標**
  - [ ] 聚合：寫 DB `metrics_timeseries/strategy_metrics_daily`
  - [ ] 判等級：綜合 `maker_fill_ratio / ib_rate / AUC / Brier / router_p95 / stream_lag`
  - [ ] 寫 Redis：`prod:{health}:system:state=GREEN|YELLOW|ORANGE|RED`
  - [ ] 發告警：`alerts`（DB + 通知）
- [ ] **GET /metrics, /alerts** → 提供前端面板

#### 10. 功能規格書相關功能（基於功能規格書實作）
- [ ] **入向（被呼叫）API**
  - [ ] `GET /health`（所有服務）→ `HealthResponse{Status,Checks,...}`
  - [ ] `GET /metrics`（S12 前端）→ `MetricsResponse`
  - [ ] `GET /alerts`（S12 前端）→ `AlertsResponse`
- [ ] **出向（主以事件）**
  - [ ] 聚合指標到 metrics_timeseries、告警到 alerts
- [ ] **指標/告警拉取（前端）**
  - [ ] S12 → S11 `GET /metrics` / `GET /alerts` → `MetricsResponse` / `AlertsResponse`
  - [ ] 前端面板資料源
- [ ] **健康監控**
  - [ ] 在 S11 增加對所有 `GET /health` 的巡檢（例如每 10s 抽查一個服務）
  - [ ] 把 Status 與關鍵相依（Redis/Arango/WS）滾動寫入 metrics_timeseries
  - [ ] 可用於 Readiness Gate

### 🎯 實作優先順序
1. **高優先級**：核心 SLI 監控和告警系統
2. **中優先級**：指標聚合和健康檢查
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
1. **指標收集失敗**
   - 檢查服務連接
   - 確認數據格式
   - 查看日誌中的錯誤信息

2. **告警誤報**
   - 檢查告警規則
   - 確認閾值設置
   - 查看告警日誌

## 版本歷史

### v1.0.0
- 初始版本