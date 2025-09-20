# Project Chimera 功能規格書

**實作進度：0/12 服務已完成 (0%)**

## 0) 讀我

- ❌ **時間戳**：一律 epoch ms **[S1-S3未實作]**
- ❌ **金額**：USDT **[S1-S3未實作]**
- ❌ **百分比**：用小數（0.10 = 10%）**[S1-S3未實作]**

所有服務均提供 `GET /health`，回傳 `HealthResponse`（已在 `internal/apispec/spec.go` 定義）。

文內型別皆引用你現有的 `spec.go`：如 `DecideRequest/Response`、`OrderCmdRequest/Result` 等。

事件流採 Redis Streams（命名規則已在白皮書 v3 訂妥），本文聚焦 HTTP 介面的互動；遇到「事件/資料」以 Streams 交換者，以〔Stream: …〕標註。

**實作狀態**：
- ❌ **S1-S12 所有服務**：Exchange Connectors、Feature Generator、Strategy Engine、Order Router、Reconciler、Position Manager 等 **[未實作]**

## 1) 互動矩陣（誰呼叫誰、用哪條 API）

| 行為/情境 | Caller → Callee | Endpoint | Request 型別 | Response 型別 | 備註 | 實作狀態 |
|-----------|-----------------|----------|--------------|---------------|------|----------|
| 系統開機：讀取現行配置 | S2/S3/S4/S6/S12 → S10 | `GET /active` | – | `ActiveConfigResponse` | 取 rev/bundle_id；初始化 Config Watcher | ❌ **[S10未實作]** |
| 配置推廣事件 | S10 → （S2/S3/S4/S6/S12） | 〔Stream: cfg:events〕 | – | – | 客戶端收到後拉 `GET /active` 取新 rev | ❌ **[S10未實作]** |
| 特徵補算 | （維運/批次）→ S2 | `POST /features/recompute` | `RecomputeFeaturesRequest` | `RecomputeFeaturesResponse` | 依需要跑回補（例如資料缺口） | ❌ **[S2未實作]** |
| 產生決策（含 FUT 或 SPOT） | S3（策略引擎）自我觸發 | `POST /decide` | `DecideRequest` | `DecideResponse` | 多數情況 S3 內部事件觸發；拿到 intents 後續呼叫 S4 | ❌ **[S3未實作]** |
| 下單（執行 intents） | S3/S6 → S4 | `POST /orders` | `OrderCmdRequest` | `OrderResult` | 冪等：intent.intent_id；Maker→Taker 回退與 TWAP 由 S4 控 | ❌ **[S4未實作]** |
| 撤單/撤換 | S3/S6/S5/S12 → S4 | `POST /cancel` | `CancelRequest` | `CancelResponse` | 用 order_id 或 client_order_id | ❌ **[S4未實作]** |
| 持倉治理（移動停損/分批止盈/加倉） | S6 → S4 | `POST /orders` | `OrderCmdRequest` | `OrderResult` | S6 計畫（ManagePlan）→ S4 執行 | ❌ **[S6未實作]** |
| 啟動對帳 | S12/排程 → S5 | `POST /reconcile` | `ReconcileRequest` | `ReconcileResponse` | S5 期間可能呼叫 S4 取消殘單、平「孤兒倉」 | ❌ **[S5未實作]** |
| 標籤回填 | 排程/手動 → S7 | `POST /labels/backfill` | `BackfillRequest` | `BackfillResponse` | 依 12/24/36h 寫回 labels | ❌ **[S7未實作]** |
| 復盤報告 | S12/排程 → S8 | `POST /autopsy/{trade_id}` | `AutopsyRequest` | `AutopsyResponse` | 產出 report id / URL（MinIO） | ❌ **[S8未實作]** |
| 假設實驗/回測 | S12/研究 → S9 | `POST /experiments/run` | `ExperimentRunRequest` | `ExperimentRunResponse` | S9 主要讀 DB/檔湖，不需叫 S3 | ❌ **[S9未實作]** |
| 配置管理（新 bundle） | S12（研究/風控）→ S10 | `POST /bundles` | `BundleUpsertRequest` | `BundleUpsertResponse` | 建 DRAFT/更新 | ❌ **[S10未實作]** |
| 配置進場（Stage） | S12 → S10 | `POST /bundles/{id}/stage` | – | `BundleStageResponse` | 進入 STAGED | ❌ **[S10未實作]** |
| 模擬＋敏感度 | S12 → S10 | `POST /simulate` | `SimulateRequest` | `SimulateResponse` | S10 進行差異估算與穩健性 | ❌ **[S10未實作]** |
| 推廣/回滾 | S12 → S10 | `POST /promote` | `PromoteRequest` | `PromoteResponse` | 觸發 cfg:events、Canary/Ramp/Full | ❌ **[S10未實作]** |
| 指標/告警拉取（前端） | S12 → S11 | `GET /metrics` / `GET /alerts` | – | `MetricsResponse` / `AlertsResponse` | 前端面板資料源 | ❌ **[S11未實作]** |
| 金庫資金劃轉（對外） | S12 → S1(私有) | `POST /xchg/treasury/transfer` | `TransferRequest` | `TransferResponse` | S12 對外 `POST /treasury/transfer` 接入後委派 S1 | ❌ **[S1未實作]** |
| 金庫資金劃轉（自動） | S6 → S1(私有) | `POST /xchg/treasury/transfer` | `TransferRequest` | `TransferResponse` | 風險預算/保證金自動補充 | ❌ **[S6未實作]** |
| Kill Switch | S12（操控） | `POST /kill-switch` | `KillSwitchRequest` | `KillSwitchResponse` | 設置全域停機旗標（Redis/DB）並廣播事件 | ❌ **[S12未實作]** |

## 2) 典型情境的「序列化劇本」

以下每個劇本都寫清楚：觸發 → 呼叫鏈 → 載荷 → 冪等/重試 → 失敗補償。

### 2.1 系統開機 & 配置收斂 ❌ **[未實作]**

**觸發**：服務 Pod 啟動

**呼叫鏈**：
- ❌ Sx → S10 `GET /active` 取得 rev/bundle_id **[S10未實作]**
- ❌ 訂閱〔Stream: cfg:events〕，本地 RCU 熱載 **[S10未實作]**

**失敗補償**：
- ❌ `GET /active` 失敗：退避重試（exponential backoff 5→30s）；未就緒前僅 `/health` OK=DEGRADED，拒絕交易行為 **[S10未實作]**

**備註**：所有寫 signals 的服務須把 config_rev 寫入紀錄

### 2.2 產生決策 → 下 FUT 期貨入場 ✅ **[S3已實作] / ❌ [S4/S6未實作]**

**觸發**：S3 收到 S2 特徵與守門通過

**呼叫鏈**：
- ❌ S3 `POST /decide`（自內部流程，實作上可直接呼叫引擎模組）→ `DecideResponse`（含 Intents，market=FUT）**[S3未實作]**
- ❌ S3 → S4 `POST /orders`（`OrderCmdRequest.Intent`）→ `OrderResult` **[S4未實作]**
- ❌ S6 監控到新倉（來自交易所/DB）後，若需掛 STOP_MARKET：S6 → S4 `POST /orders`（SL/TP/ReduceOnly）**[S6未實作]**

**冪等/重試**：
- ❌ intent_id 作為冪等鍵 **[S3未實作]**；❌ S4 對 5xx/429 重試（同一鍵）**[S4未實作]**

**失敗補償**：
- ❌ 下單逾時不確定：重送同 intent_id；若交易所有單→回傳既有 OrderID **[S4未實作]**

### 2.3 產生決策 → 下 SPOT 現貨（含 OCO 或守護停損）✅ **[S3已實作] / ❌ [S4未實作]**

**觸發**：S3 決策 market=SPOT

**呼叫鏈**：
- ❌ S3 → S4 `POST /orders`，`ExecPolicy.OCO` 或 `GuardStopEnable=true` **[S3未實作]**
- ❌ S4 成交回傳 `GuardStopArmed`（如有本地守護）**[S4未實作]**

**冪等/重試**：同上

**失敗補償**：
- ❌ OCO 一腿掛失敗：S4 回 status=PARTIAL 並附訊息；S6 或 S3 依「OCO 補掛策略」再次 `POST /orders` **[S4未實作]**

### 2.4 移動停損 / 分批止盈 / 加倉 ❌ **[S6未實作]**

**觸發**：S6 定時計算持倉健康；ROE/ATR/規則命中

**呼叫鏈**：
- ❌ S6 `POST /positions/manage`（可外露給 S12/維運觸發）→ `ManagePlan` **[S6未實作]**
- ❌ 依計畫逐條 S6 → S4 `POST /orders` **[S6/S4未實作]**

**失敗補償**：
- ❌ 任一子單失敗：S6 記錄部分成功，對失敗單重試或回滾（撤舊掛新）**[S6未實作]**

### 2.5 啟動對帳（含孤兒處理）❌ **[S5未實作]**

**觸發**：手動或排程

**呼叫鏈**：
- ❌ S12/排程 → S5 `POST /reconcile`（Mode=ALL）**[S5/S12未實作]**
- ❌ S5 期間：
  - 發現 API 有單/DB 無單：依策略→S4 `POST /cancel` 或 建立接管紀錄 **[S5/S4未實作]**
  - 發現 DB 有單/API 無單：清理本地訂單狀態 **[S5未實作]**

**失敗補償**：
- ❌ S4 取消失敗：記 alerts(FATAL)，列入下一輪對帳重試 **[S4/S5未實作]**

### 2.6 標籤回填 / 復盤 ❌ **[S7/S8未實作]**

**觸發**：t0 + 12/24/36h

**呼叫鏈**：
- ❌ S7 `POST /labels/backfill` → `BackfillResponse` **[S7未實作]**
- ❌ （選）對符合觸發條件者，S8 `POST /autopsy/{trade_id}` → 報告 URL **[S8未實作]**

**失敗補償**：❌ 失敗者記號重試佇列；連續 3 次失敗→alerts(ERROR) **[S7/S8未實作]**

### 2.7 配置模擬＋敏感度 → 推廣 ❌ **[S10/S12未實作]**

**觸發**：人員在 S12 提交新 bundle

**呼叫鏈**：
- ❌ S12 → S10 `POST /bundles`（DRAFT）**[S10/S12未實作]**
- ❌ S12 → S10 `POST /simulate`（差異估算＋敏感度）**[S10/S12未實作]**
- ❌ S12 → S10 `POST /bundles/{id}/stage`（進 STAGED）**[S10/S12未實作]**
- ❌ S12 → S10 `POST /promote`（CANARY/RAMP/FULL）**[S10/S12未實作]**
- ❌ S10 廣播〔cfg:events〕→ 各服務拉 `GET /active` 熱載 **[S10未實作]**

**失敗補償**：❌ 模擬或守門不過→回覆詳細原因；推廣過程護欄觸發→自動 ROLLBACK **[S10未實作]**

### 2.8 金庫資金劃轉（人工/自動）✅ **[S1已實作] / ❌ [S12/S6未實作]**

**觸發**：S12（人工）或 S6（自動）

**呼叫鏈**：
- ❌ 對外：S12 `POST /treasury/transfer`（`TransferRequest`）**[S12未實作]**
- ❌ 內部：S12 → S1 `POST /xchg/treasury/transfer`（帶 Idempotency-Key）**[S1未實作]**
- ❌ 成功：寫 strategy_events(kind=TREASURY_TRANSFER)；失敗記 alerts **[S12未實作]**

**鎖**：❌ `lock:treasury:<from>:<to>`（Redis）**[S12未實作]**

**失敗補償**：❌ 重試 N 次；連續失敗升級 FATAL **[S12未實作]**

## 3) 每個服務的「出/入向」呼叫清單（速查）

### S1 Exchange Connectors ❌ **[未實作]**

**入向（被呼叫）**：
- ❌ `GET /health`（所有）**[未實作]**
- ❌ `POST /xchg/treasury/transfer`（S12/S6 內部）**[未實作]**

**出向（主以事件）**：
- ❌ 〔Stream〕推送行情/深度/資金費/帳戶事件至 `mkt:*` **[未實作]**
- ❌ （可選）上拋 S11 指標 via 〔Stream: metrics:*〕**[未實作]**

### S2 Feature Generator ❌ **[未實作]**

**入向**：❌ `GET /health`、`POST /features/recompute`（維運/批次）**[未實作]**

**出向**：❌ 寫入 DB signals.features；發 signals:new 事件（Stream）**[未實作]**

### S3 Strategy Engine ❌ **[未實作]**

**入向**：❌ `GET /health`、`POST /decide`（一般由自身流程觸發）**[未實作]**

**出向**：❌ `POST /orders` → S4（執行 intents）**[S4未實作]**；❌ 記 signals、strategy_events **[未實作]**

### S4 Order Router ❌ **[未實作]**

**入向**：❌ `GET /health`、`POST /orders`、`POST /cancel` **[未實作]**

**出向**：❌ 寫 orders/fills；必要時回報 alerts；（內部）呼叫交易所 **[未實作]**

### S5 Reconciler ❌ **[未實作]**

**入向**：❌ `GET /health`、`POST /reconcile` **[未實作]**

**出向**：❌ `POST /cancel` → S4（清理殘單/平倉）**[未實作]**

### S6 Position Manager ❌ **[未實作]**

**入向**：❌ `GET /health`、`POST /positions/manage` **[未實作]**

**出向**：
- ❌ `POST /orders` → S4（移動停損/減倉/加倉）**[未實作]**
- ❌ （如需補保證金）`POST /xchg/treasury/transfer` → S1（內部）**[未實作]**

### S7 Label Backfill ❌ **[未實作]**

**入向**：❌ `GET /health`、`POST /labels/backfill` **[未實作]**

**出向**：❌ 更新 labels_*；（可選）推送 labels:ready（Stream）**[未實作]**

### S8 Autopsy Generator ❌ **[未實作]**

**入向**：❌ `GET /health`、`POST /autopsy/{trade_id}` **[未實作]**

**出向**：❌ 寫 autopsy_reports、MinIO 檔案；回傳 URL **[未實作]**

### S9 Hypothesis Orchestrator ❌ **[未實作]**

**入向**：❌ `GET /health`、`POST /experiments/run` **[未實作]**

**出向**：❌ 寫 experiments 結果；可通知 S12/S10 **[未實作]**

### S10 Config Service ❌ **[未實作]**

**入向**：❌ `GET /health`、`POST /bundles`、`/bundles/{id}/stage`、`/simulate`、`/promote`、`GET /active` **[未實作]**

**出向**：❌ 〔Stream: cfg:events〕；寫 promotions/simulations/config_active **[未實作]**

### S11 Metrics & Health ❌ **[未實作]**

**入向**：❌ `GET /health`、`GET /metrics`、`GET /alerts` **[未實作]**

**出向**：❌ 聚合指標到 metrics_timeseries、告警到 alerts **[未實作]**

### S12 Web UI / API GW ❌ **[未實作]**

**入向**：❌ `GET /health`、`POST /kill-switch`、`POST /treasury/transfer` **[未實作]**

**出向**：
- ❌ **配置**：`/bundles` `/simulate` `/promote` `/active` → S10 **[未實作]**
- ❌ **操作**：`/reconcile` → S5、`/positions/manage` → S6、`/cancel` → S4 **[未實作]**
- ❌ **監控**：`/metrics` `/alerts` → S11 **[未實作]**
- ❌ **資金**：`/xchg/treasury/transfer` → S1（私有）**[未實作]**

## 4) 冪等、重試、補償（跨服務統一約束）

- ❌ **下單/撤單**：`OrderCmdRequest.Intent.IntentID` / `CancelRequest.ClientID` 必填作冪等鍵 **[S3未實作]**；❌ S4 對 5xx/429 採固定+抖動退避 **[S4未實作]**
- ❌ **資金劃轉**：Idempotency-Key（由 S12 產生）→ S1 必須回舊 TransferID **[S1未實作]**
- ❌ **配置推廣**：推廣過程失敗 → S10 發 ROLLBACK，並在 promotions 記錄 **[S10未實作]**
- ❌ **對帳補償**：孤兒處置先走減風險路徑（小額市價平倉/取消掛單），不可反向加倉 **[S5未實作]**

## 5) 錯誤碼與告警標準（摘要）

| 層級 | 範例 | 動作 | 實作狀態 |
|------|------|------|----------|
| INFO | `/simulate` 完成 | 事件記錄 | ❌ **[S10未實作]** |
| WARN | Maker 等待逾時→Taker 回退 | 記錄＋計數 | ❌ **[S4未實作]** |
| ERROR | `/orders` 連續 3 次失敗 | 告警通知、熔斷路由（凍結新倉） | ❌ **[S4未實作]** |
| FATAL | 資金劃轉 3 次失敗；配置收斂失敗 | 觸發 Kill-switch 或回滾 | ❌ **[S12/S10未實作]** |

## 6) 快速對照：請求/回應型別一覽（重點）

- ❌ **策略決策**：`DecideRequest` → `DecideResponse{Decision, []OrderIntent}` **[S3未實作]**
- ❌ **下單**：`OrderCmdRequest{Intent}` → `OrderResult` **[S4未實作]**
- ❌ **撤單**：`CancelRequest` → `CancelResponse` **[S4未實作]**
- ❌ **持倉治理**：`ManagePositionsRequest` → `ManagePositionsResponse{Plan, Orders}` **[S6未實作]**
- ❌ **對帳**：`ReconcileRequest` → `ReconcileResponse` **[S5未實作]**
- ❌ **標籤**：`BackfillRequest` → `BackfillResponse` **[S7未實作]**
- ❌ **復盤**：`AutopsyRequest` → `AutopsyResponse{ReportID, Url}` **[S8未實作]**
- ❌ **實驗**：`ExperimentRunRequest` → `ExperimentRunResponse` **[S9未實作]**
- ❌ **配置**：`BundleUpsertRequest` / `PromoteRequest` / `SimulateRequest` / `ActiveConfigResponse` **[S10未實作]**
- ❌ **資金劃轉**：`TransferRequest{From,To,Amount,Reason}` → `TransferResponse{TransferID,Result,Message}` **[S1未實作]**
- ❌ **健康**：所有服務 `GET /health` → `HealthResponse{Status,Checks,...}` **[S1-S3未實作]**

## 7) 開發建議（落地提示）

1. ❌ **契約測試**：把這份文件內的呼叫關係「逐一」落入各服務 README 的 "Integration" 小節，並在測試加上契約測試（contract test）**[S4-S12未實作]**：
   - 例：S3 對 S4 的 `POST /orders`，用 httptest 或 mock server 固定回應碼與錯誤碼，驗證退避/冪等是否正確

2. ❌ **序列圖**：在 S12 做一個 "交易生命周期" 的序列圖頁（mermaid），把 2.2/2.3/2.4 三個核心劇本圖像化，方便新同事理解 **[S12未實作]**

3. ❌ **健康監控**：在 S11 增加對所有 `GET /health` 的巡檢（例如每 10s 抽查一個服務），把 Status 與關鍵相依（Redis/Arango/WS）滾動寫入 metrics_timeseries，可用於 Readiness Gate **[S11未實作]**

---

## 📊 實作進度總結

### ❌ 全部未實作 (0%)
- **S1-S12**：所有服務均未實作

### ❌ 待實作 (100%)
- **S1-S12**：所有服務均待實作

### 🎯 建議優先順序
1. **S4 Order Router** - 訂單執行核心（下單、撤單、Maker→Taker）
2. **S6 Position Manager** - 持倉治理（移動停損、分批止盈）
3. **S5 Reconciler** - 對帳處置（孤兒訂單處理）
4. **S12 API Gateway** - 統一入口（代理、RBAC、監控）
5. **S10 Config Service** - 配置管理（bundle、推廣、熱載）
6. **S11 Metrics & Health** - 監控系統（指標、告警、健康檢查）