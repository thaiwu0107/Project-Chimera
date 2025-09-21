# S5 Reconciler ❌ **[未實作]**

Reconciler - Reconcile orders and positions across exchanges

## 📋 實作進度：10% (1/10 功能完成)

### ✅ 已完成功能
- [x] 基礎服務架構
- [x] Health Check API
- [x] 基本對帳 API 框架

### ❌ 待實作功能

#### 1. POST /reconcile（ALL|ORDERS|POSITIONS）
- [ ] **數據拉取**
  - [ ] 拉交易所 `openOrders/positionRisk`
  - [ ] 拉 DB `orders/positions_snapshots`
- [ ] **差異計算**
  - [ ] 集合差異（Jaccard、一致率）計算
- [ ] **處置策略**
  - [ ] 孤兒掛單→S4 `/cancel`
  - [ ] 數量不符→以交易所為準修 DB 或進保守降風險（小額市價）
- [ ] **DB 寫入**
  - [ ] `strategy_events(kind=RECONCILE_*)`
  - [ ] 同步 `orders/positions_snapshots`
- [ ] **Redis 寫入**
  - [ ] `recon:{last_run_ts}`
  - [ ] 嚴重時 `alerts(ERROR)`、健康降級信號

#### 2. 交易所數據同步
- [ ] **Binance API 整合**
  - [ ] `GET /fapi/v2/openOrders`
  - [ ] `GET /fapi/v2/positionRisk`
  - [ ] `GET /api/v3/openOrders`
- [ ] **數據標準化**
  - [ ] 統一數據格式
  - [ ] 時間戳對齊

#### 3. 差異檢測算法
- [ ] **Jaccard 相似度**
  - [ ] 訂單集合比較
  - [ ] 持倉集合比較
- [ ] **一致率計算**
  - [ ] 數量一致性檢查
  - [ ] 狀態一致性檢查

#### 4. 孤兒訂單處理
- [ ] **孤兒檢測**
  - [ ] 交易所有但 DB 沒有的訂單
  - [ ] DB 有但交易所沒有的訂單
- [ ] **處置策略**
  - [ ] 安全撤單
  - [ ] 保守平倉

#### 5. 持倉差異處理
- [ ] **持倉比較**
  - [ ] 數量差異檢測
  - [ ] 價格差異檢測
- [ ] **修正策略**
  - [ ] 以交易所為準更新 DB
  - [ ] 必要時保守平倉

#### 6. 定時對帳
- [ ] **排程任務**
  - [ ] 定期自動對帳
  - [ ] 可配置對帳頻率
- [ ] **觸發條件**
  - [ ] 手動觸發
  - [ ] 異常觸發

#### 7. 告警機制
- [ ] **告警規則**
  - [ ] 差異閾值設定
  - [ ] 嚴重程度分級
- [ ] **告警發送**
  - [ ] 即時告警
  - [ ] 告警聚合

#### 8. 健康度管理
- [ ] **健康度評估**
  - [ ] 對帳成功率
  - [ ] 差異嚴重程度
- [ ] **降級信號**
  - [ ] 系統健康度更新
  - [ ] 風險控制信號

#### 9. 審計日誌
- [ ] **操作記錄**
  - [ ] 對帳操作記錄
  - [ ] 修正操作記錄
- [ ] **審計追蹤**
  - [ ] 操作人員記錄
  - [ ] 操作時間記錄

#### 10. 性能優化
- [ ] **並行處理**
  - [ ] 多交易所並行對帳
  - [ ] 異步處理機制
- [ ] **快取機制**
  - [ ] 對帳結果快取
  - [ ] 歷史對帳記錄

### 🎯 實作優先順序
1. **高優先級**：基本對帳邏輯和交易所 API 整合
2. **中優先級**：差異檢測和處置策略
3. **低優先級**：告警機制和性能優化

### 📊 相關資料寫入
- **DB Collections**：`strategy_events(RECONCILE_*)`、修 `orders/positions_snapshots`
- **Redis Key/Stream**：`recon:{last_run_ts}`、`alerts`（錯誤）

## 概述

S5 Reconciler 是 Project Chimera 交易系統的對帳引擎，負責對比交易所數據與本地數據庫，確保數據一致性，並處理孤兒訂單和持倉。

## 功能

- **數據對帳**：對比交易所與本地數據庫
- **孤兒處理**：處理孤兒訂單和持倉
- **數據修復**：修復不一致的數據
- **風險控制**：確保交易數據的準確性

## API 接口

### 健康檢查

- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 對帳管理

- `POST /reconcile` - 啟動對帳流程

#### Reconcile

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

**回應**：
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
    },
    {
      "type": "CLOSE_POSITION",
      "position_id": "orphan_pos_001",
      "reason": "Position exists in local DB but not in exchange"
    }
  ]
}
```

## 服務間交互

### 入向（被呼叫）
- **S12 Web UI** → `POST /reconcile` - 手動啟動對帳
- **排程系統** → `POST /reconcile` - 定期對帳

### 出向（主動呼叫）
- **S4 Order Router** → `POST /cancel` - 取消孤兒訂單
- **數據庫** → 修復數據不一致
- **告警系統** → 回報嚴重不一致

## 對帳模式

### ALL 模式
- 對比所有訂單、持倉和資金
- 最全面的對帳檢查

### ORDERS 模式
- 僅對比訂單數據
- 用於訂單狀態同步

### POSITIONS 模式
- 僅對比持倉數據
- 用於持倉狀態同步

### HOLDINGS 模式
- 僅對比資金數據
- 用於資金餘額同步

## 孤兒處理策略

### 孤兒訂單
1. **API 有單/DB 無單**：取消交易所訂單
2. **DB 有單/API 無單**：清理本地訂單狀態

### 孤兒持倉
1. **API 有倉/DB 無倉**：建立接管記錄
2. **DB 有倉/API 無倉**：平倉處理

### 風險控制
- 優先採用減風險路徑
- 小額市價平倉
- 禁止反向加倉

## 失敗補償

- S4 取消失敗：記錄 FATAL 告警
- 列入下一輪對帳重試
- 連續失敗升級處理

## 配置

服務使用以下配置：
- Redis：用於對帳狀態緩存
- ArangoDB：用於對帳歷史存儲
- 交易所 API：獲取交易所數據
- 端口：8085（可通過環境變量 PORT 覆蓋）

## 部署

```bash
# 構建
go build -o s5-reconciler .

# 運行
./s5-reconciler
```

## 監控

服務提供以下監控指標：
- 對帳執行時間
- 數據一致性率
- 孤兒處理成功率
- 數據修復次數
- 告警觸發頻率

## 詳細實作項目（基於目標與範圍文件）

### 對帳功能詳細實作
- [ ] **對帳模式支持**
  - [ ] **ALL**：全面對帳（訂單+持倉+餘額）
  - [ ] **ORDERS**：僅對帳訂單
  - [ ] **POSITIONS**：僅對帳持倉
  - [ ] **HOLDINGS**：僅對帳餘額
- [ ] **數據拉取和比較**
  - [ ] 拉取交易所 `openOrders/positionRisk` 數據
  - [ ] 拉取 DB `orders/positions_snapshots` 數據
  - [ ] 實現集合差異（Jaccard、一致率）計算
  - [ ] 實現數據格式標準化和對比
- [ ] **差異處置策略**
  - [ ] 孤兒掛單→S4 `/cancel` 撤單
  - [ ] 數量不符→以交易所為準修 DB 或進保守降風險（小額市價）
  - [ ] 餘額不符→記錄差異並告警
  - [ ] 持倉不符→同步持倉數據
- [ ] **事件和日誌**
  - [ ] `strategy_events(kind=RECONCILE_*)` 記錄
  - [ ] 同步 `orders/positions_snapshots`
  - [ ] `recon:{last_run_ts}` Redis 記錄
  - [ ] 嚴重時 `alerts(ERROR)`、健康降級信號
- [ ] **定時對帳**
  - [ ] 實現定時對帳任務調度
  - [ ] 實現對帳結果統計和報告
  - [ ] 實現對帳失敗重試機制
- [ ] **告警機制**
  - [ ] 對帳失敗告警
  - [ ] 數據不一致告警
  - [ ] 孤兒訂單告警
  - [ ] 系統健康降級告警

#### 7. 核心時序圖相關功能（基於時序圖實作）
- [ ] **對帳觸發機制**
  - [ ] POST /reconcile {mode=ALL} API
  - [ ] 對帳請求處理和驗證
  - [ ] ReconcileResponse{summary, fixed, adopted, closed} 回報
- [ ] **真相數據拉取**
  - [ ] GET openOrders, positions (FUT/Spot) 並行拉取
  - [ ] REST 調用交易所現況
  - [ ] 查 orders/fills/positions_snapshots (近N天)
- [ ] **差異比對邏輯**
  - [ ] order對單差異檢測
  - [ ] 倉位聚合差異檢測
  - [ ] API 有單 / DB 無單（孤兒）處理
  - [ ] DB 有單 / API 無單（應清理）處理
- [ ] **孤兒處置策略**
  - [ ] 判別是否「PENDING_ENTRY 崩潰遺留」可接管
  - [ ] 可接管 → 補寫 orders/position 與 strategy_events{ACTIVE}
  - [ ] 不可接管 → POST /cancel 或反向平倉（保守回收）
  - [ ] 清理殘留狀態（標記CLOSED/RECONCILED）
- [ ] **狀態機修復**
  - [ ] strategy_events{ROLLBACK or ORPHAN_CLOSED} 記錄
  - [ ] XADD alerts{severity=ERROR, msg="orphan closed"}
  - [ ] XADD strategy:reconciled {...}
  - [ ] 記錄處置結果到 DB
- [ ] **保守回收原則**
  - [ ] 遇到不可識別或無法接管的殘留/孤兒
  - [ ] 優先降低風險（取消/平倉）
  - [ ] 寫審計記錄

#### 8. 服務與資料流相關功能（基於服務與資料流實作）
- [ ] **對帳與事務狀態機**
  - [ ] 狀態管理：`PENDING_ENTRY → ACTIVE → PENDING_CLOSING → CLOSED`
  - [ ] 啟動或定時比對：`positionRisk + openOrders` vs DB
  - [ ] 孤兒單處理：API 有、DB 無 → 依 `orphan_policy`：`RECLAIM_IF_SAFE`（接管）或 `CONSERVATIVE`（平倉退出）
- [ ] **定時任務**
  - [ ] 啟動必跑對帳
  - [ ] 每 5 分鐘自動對帳
  - [ ] 異常（告警）即時觸發對帳
- [ ] **錢包劃轉對帳**
  - [ ] TransferRequest/Response 事件對帳
  - [ ] SPOT ↔ FUT 資金劃轉記錄驗證
  - [ ] 劃轉限制和守門檢查對帳

#### 9. 定時任務相關功能（基於定時任務實作）
- [ ] **對帳（每 10–15 分鐘）**
  - [ ] 拉取 `positionRisk`、`openOrders` 與 DB 快照
  - [ ] 差異分類：API 有 / DB 無；DB 有 / API 無；數量或方向不一致
  - [ ] 策略：以 API 為準修 DB；必要時降風險至 FLAT
  - [ ] 集合差異：`D = (S_API ∪ S_DB) \ (S_API ∩ S_DB)`

#### 10. 目標與範圍相關功能（基於目標與範圍實作）
- [ ] **前置依賴實作**
  - [ ] ArangoDB Collections：`strategy_events`、修 `orders/positions_snapshots`
  - [ ] Redis Keys：`recon:{last_run_ts}`、`alerts`（錯誤）
- [ ] **風險與緩解**
  - [ ] Redis Cluster slot 移轉：使用官方 cluster client；關鍵操作具重試策略

#### 11. 路過的服務相關功能（基於路過的服務實作）
- [ ] **POST /reconcile（ALL|ORDERS|POSITIONS）**
  - [ ] 拉：交易所 `openOrders/positionRisk`；DB `orders/positions_snapshots`
  - [ ] 算：集合差異（Jaccard、一致率）
  - [ ] 處置：孤兒掛單→S4 `/cancel`；數量不符→以交易所為準修 DB 或進保守降風險（小額市價）
  - [ ] 寫 DB：`strategy_events(kind=RECONCILE_*)`；同步 `orders/positions_snapshots`
  - [ ] 寫 Redis：`recon:{last_run_ts}`；嚴重時 `alerts(ERROR)`、健康降級信號

#### 12. 字段校驗相關功能（基於字段校驗表實作）
- [ ] **ReconcileRequest 字段校驗**
  - [ ] `mode`：必填，枚舉 {ALL, ORDERS, POSITIONS}，預設 ALL
  - [ ] `dry_run`：可選布爾值，預設 false
  - [ ] `orphan_policy`：可選，枚舉 {RECLAIM_IF_SAFE, CONSERVATIVE}，預設 CONSERVATIVE
  - [ ] `time_window_h`：可選整數，1–168，預設 72
- [ ] **錯誤處理校驗**
  - [ ] 400 Bad Request：參數格式錯誤、範圍超界
  - [ ] 422 Unprocessable Entity：業務規則違反、數據不完整
  - [ ] 冪等性：相同參數返回相同結果
- [ ] **契約測試**
  - [ ] dry_run=true：僅回報差異，不做修改
  - [ ] 孤兒可接管 → action=ADOPTED 並補寫 DB
  - [ ] 孤兒不可接管 → action=CLOSED（取消/反向平倉）並寫 alerts
  - [ ] mode=ORDERS 僅核對訂單

#### 13. 功能對照補記相關功能（基於功能對照補記實作）
- [ ] **對帳/事務狀態機**
  - [ ] 狀態：`PENDING_ENTRY` → `ACTIVE` → `PENDING_CLOSING` → `CLOSED`
  - [ ] 啟動對帳：以 API 為準修 DB；孤兒單 → 撤單或接管；倉位不一致 → 降風險至 FLAT
  - [ ] 度量：一致率 Jaccard（訂單集/倉位集）

#### 14. 全服務一覽相關功能（基於全服務一覽實作）
- [ ] **POST /reconcile（ALL|ORDERS|POSITIONS）**
  - [ ] 拉：交易所 `openOrders/positionRisk`；DB `orders/positions_snapshots`
  - [ ] 算：集合差異（Jaccard、一致率）
  - [ ] 處置：孤兒掛單→S4 `/cancel`；數量不符→以交易所為準修 DB 或進保守降風險（小額市價）
  - [ ] 寫 DB：`strategy_events(kind=RECONCILE_*)`；同步 `orders/positions_snapshots`
  - [ ] 寫 Redis：`recon:{last_run_ts}`；嚴重時 `alerts(ERROR)`、健康降級信號

#### 15. Integration 附錄相關功能（基於 Integration 附錄實作）
- [ ] **對帳處置流程（孤兒訂單/倉位、狀態機修復、保守回收）**
  - [ ] 數據收集：並行查詢交易所 API 和本地數據庫，查詢最近 72 小時的數據
  - [ ] 差異分析：比較交易所訂單與本地訂單、比較交易所持倉與本地持倉、檢查訂單和持倉的狀態一致性
  - [ ] 處置策略：孤兒訂單（API 有單但 DB 無單）、孤兒持倉（API 有倉但 DB 無倉）、殘留數據（DB 有數據但 API 無對應）
- [ ] **保守回收策略**
  - [ ] 風險優先：優先考慮風險控制而非利潤最大化
  - [ ] 接管條件：只有在能夠安全接管的情況下才接管孤兒訂單
  - [ ] 平倉處理：無法接管的訂單/持倉優先平倉處理
  - [ ] 審計記錄：所有處置操作都記錄詳細的審計日誌
- [ ] **狀態機修復**
  - [ ] PENDING_ENTRY：修復崩潰時遺留的待入場狀態
  - [ ] ACTIVE：恢復活躍持倉的正確狀態
  - [ ] CLOSED：清理已關閉的訂單和持倉
  - [ ] RECONCILED：標記已對帳的數據
- [ ] **並行處理**
  - [ ] API 查詢：並行查詢 FUT 和 SPOT 的訂單和持倉
  - [ ] DB 查詢：並行查詢不同類型的本地數據
  - [ ] 處置操作：並行執行多個處置操作以提高效率
- [ ] **對帳完成事件發布**
  - [ ] 對帳完成事件：發布 `strategy:reconciled` 事件到 Redis Stream
  - [ ] 處置摘要：生成包含 `checked_orders`、`checked_positions`、`adopted`、`closed`、`fixed` 的摘要
  - [ ] 處置項目：記錄每個處置項目的 `type`、`action`、`reason`

#### 16. Hop-by-Hop 執行規格相關功能（基於 Hop-by-Hop 執行規格補遺實作）
- [ ] **POST /reconcile（ALL|ORDERS|POSITIONS）**
  - [ ] 拉：交易所 `openOrders/positionRisk`；DB `orders/positions_snapshots`
  - [ ] 算：集合差異（Jaccard、一致率）
  - [ ] 處置：孤兒掛單→S4 `/cancel`；數量不符→以交易所為準修 DB 或進保守降風險（小額市價）
  - [ ] 寫 DB：`strategy_events(kind=RECONCILE_*)`；同步 `orders/positions_snapshots`
  - [ ] 寫 Redis：`recon:{last_run_ts}`；嚴重時 `alerts(ERROR)`、健康降級信號

#### 17. 功能規格書相關功能（基於功能規格書實作）
- [ ] **入向（被呼叫）API**
  - [ ] `GET /health`（所有服務）→ `HealthResponse{Status,Checks,...}`
  - [ ] `POST /reconcile`（S12/排程）→ `ReconcileResponse`
- [ ] **出向（主以事件）**
  - [ ] `POST /cancel` → S4（清理殘單/平倉）
- [ ] **啟動對帳（含孤兒處理）**
  - [ ] 觸發：手動或排程
  - [ ] S12/排程 → S5 `POST /reconcile`（Mode=ALL）
  - [ ] S5 期間：
    - [ ] 發現 API 有單/DB 無單：依策略→S4 `POST /cancel` 或 建立接管紀錄
    - [ ] 發現 DB 有單/API 無單：清理本地訂單狀態
  - [ ] 失敗補償：S4 取消失敗：記 alerts(FATAL)，列入下一輪對帳重試
- [ ] **對帳補償**
  - [ ] 孤兒處置先走減風險路徑（小額市價平倉/取消掛單），不可反向加倉

### 🎯 實作優先順序
1. **高優先級**：基本對帳邏輯和孤兒處置
2. **中優先級**：保守回收策略和狀態修復
3. **低優先級**：並行處理和優化

### 📊 相關資料寫入
- **DB Collections**：`strategy_events(RECONCILE_*)`、修 `orders/positions_snapshots`
- **Redis Key/Stream**：`recon:{last_run_ts}`、`alerts`
