# S6 Position Manager ❌ **[未實作]**

Position Manager - Manage positions and risk across all exchanges

## 📋 實作進度：20% (2/10 功能完成)

### ✅ 已完成功能
- [x] 基礎服務架構
- [x] Health Check API
- [x] 基本持倉管理 API
- [x] 自動劃轉觸發框架

### ❌ 待實作功能

#### 1. POST /positions/manage 或 管理 tick（排程）
- [ ] **數據讀取**
  - [ ] 最新價、ATR、Regime 讀取
  - [ ] `pos` 當前 SL/TP 階讀取
  - [ ] 加倉上限讀取
  - [ ] 健康度讀取
- [ ] **計算邏輯**
  - [ ] ROE 計算
  - [ ] 強平距離計算
  - [ ] 是否上升鎖利階判斷
  - [ ] 是否命中止盈判斷
  - [ ] 是否加倉判斷
- [ ] **行動執行**
  - [ ] 鎖利：`/cancel` 舊 SL → `/orders` 新 SL（reduceOnly）
  - [ ] 止盈：`/orders` 市價 reduceOnly（分批）
  - [ ] 加倉：`/orders` 新單；更新 `add_on_count`
- [ ] **DB 寫入**
  - [ ] `positions_snapshots`（新快照）
  - [ ] `orders/fills`（由 S4 回報）
- [ ] **Redis 寫入**
  - [ ] `pos:{sl}:level:<pos_id>`
  - [ ] `pos:{tp}:ladder:<pos_id>`
  - [ ] `pos:{adds}:<pos_id>`
- [ ] **配額釋放**
  - [ ] 倉位關閉時 `risk:{budget}`/`risk:{concurrency}` 反向調整

#### 2. 自動資金劃轉（排程）
- [ ] **餘額檢查**
  - [ ] SPOT/FUT 可用餘額讀取
- [ ] **需求計算**
  - [ ] `need=max(0,min_free_fut-free_fut)` 計算
- [ ] **劃轉執行**
  - [ ] 若足→造 `transfer_id`
  - [ ] 寫 DB `treasury_transfers(PENDING)`
  - [ ] 請 S12 審批 → S1 執行
  - [ ] （若啟用全自動，可直連 S1）

#### 3. 持倉監控
- [ ] **實時監控**
  - [ ] 持倉狀態監控
  - [ ] PnL 實時計算
- [ ] **風險監控**
  - [ ] 強平風險監控
  - [ ] 保證金率監控

#### 4. 止損止盈管理
- [ ] **動態止損**
  - [ ] Trailing Stop 實現
  - [ ] 鎖利階梯管理
- [ ] **分批止盈**
  - [ ] 階梯止盈實現
  - [ ] 部分平倉邏輯

#### 5. 加倉策略
- [ ] **加倉條件**
  - [ ] 加倉觸發條件
  - [ ] 加倉數量計算
- [ ] **加倉限制**
  - [ ] 最大加倉次數
  - [ ] 加倉間隔控制

#### 6. 風險管理
- [ ] **風險預算**
  - [ ] 持倉風險預算管理
  - [ ] 動態風險調整
- [ ] **併發控制**
  - [ ] 持倉併發限制
  - [ ] 風險分散控制

#### 7. 事件處理
- [ ] **訂單事件**
  - [ ] `ord:{results}` 事件處理
  - [ ] 持倉更新邏輯
- [ ] **市場事件**
  - [ ] 價格變動響應
  - [ ] 風險事件處理

#### 8. 定時任務
- [ ] **管理 tick**
  - [ ] 定期持倉檢查
  - [ ] 自動管理邏輯
- [ ] **劃轉 tick**
  - [ ] 定期餘額檢查
  - [ ] 自動劃轉邏輯

#### 9. 配置管理
- [ ] **持倉配置**
  - [ ] 止損止盈參數
  - [ ] 加倉策略參數
- [ ] **風險配置**
  - [ ] 風險預算配置
  - [ ] 併發限制配置

#### 10. 監控指標
- [ ] **持倉指標**
  - [ ] 持倉數量統計
  - [ ] PnL 統計
- [ ] **風險指標**
  - [ ] 風險預算使用率
  - [ ] 強平風險指標

### 🎯 實作優先順序
1. **高優先級**：基本持倉管理和止損止盈
2. **中優先級**：自動劃轉和加倉策略
3. **低優先級**：風險管理和監控優化

### 📊 相關資料寫入
- **DB Collections**：`positions_snapshots`
- **Redis Key/Stream**：`pos:{sl}:level:*`、`pos:{tp}:ladder:*`、`pos:{adds}:*`、釋放 `risk:{*}`

## 概述

S6 Position Manager 是 Project Chimera 交易系統的持倉管理引擎，負責監控和管理交易持倉，執行移動停損、分批止盈、加倉等持倉治理策略。

## 功能

- **持倉監控**：實時監控持倉狀態和風險
- **移動停損**：動態調整停損價格
- **分批止盈**：執行分批獲利了結
- **加倉管理**：根據策略進行加倉
- **風險控制**：執行持倉風險限制
- **自動劃轉**：觸發保證金自動劃轉

## API 接口

### 健康檢查

- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 持倉管理

- `POST /positions/manage` - 持倉治理計劃
- `POST /auto-transfer/trigger` - 自動劃轉觸發

#### Manage Positions

**請求**：
```json
{
  "symbol": "BTCUSDT",
  "market": "FUT",
  "position_id": "pos_001",
  "action": "STOP_MOVE",
  "new_stop_price": 44000.0,
  "reason": "Trailing stop moved up"
}
```

**回應**：
```json
{
  "plan": {
    "plan_id": "plan_001",
    "position_id": "pos_001",
    "actions": [
      {
        "action": "STOP_MOVE",
        "old_value": 43000.0,
        "new_value": 44000.0,
        "reason": "Trailing stop moved up"
      }
    ],
    "reduce": [],
    "adds": []
  },
  "orders": []
}
```

#### Auto Transfer Trigger

**請求**：
```json
{
  "trigger_id": "margin_call_001",
  "symbol": "BTCUSDT",
  "market": "FUT",
  "trigger_type": "MARGIN_CALL",
  "threshold": 0.8,
  "transfer_from": "SPOT",
  "transfer_to": "FUT",
  "transfer_amount": 500.0,
  "enabled": true
}
```

**回應**：
```json
{
  "log_id": "log_001",
  "trigger_id": "margin_call_001",
  "symbol": "BTCUSDT",
  "market": "FUT",
  "trigger_type": "MARGIN_CALL",
  "from": "SPOT",
  "to": "FUT",
  "amount_usdt": 500.0,
  "reason": "Auto transfer triggered by MARGIN_CALL",
  "status": "SUCCESS",
  "transfer_id": "transfer_001"
}
```

## 服務間交互

### 入向（被呼叫）
- **S12 Web UI** → `POST /positions/manage` - 手動持倉治理
- **內部觸發** → `POST /auto-transfer/trigger` - 自動劃轉觸發

### 出向（主動呼叫）
- **S4 Order Router** → `POST /orders` - 執行持倉治理訂單
- **S12 Web UI** → `POST /treasury/transfer` - 自動資金劃轉
- **數據庫** → 更新持倉狀態和歷史

## 持倉治理策略

### 移動停損
- 根據價格變動動態調整停損
- 支援固定停損和追蹤停損
- 保護獲利和限制損失

### 分批止盈
- 將大額獲利分批了結
- 降低市場衝擊
- 提高執行效率

### 加倉管理
- 根據策略信號進行加倉
- 風險預算控制
- 槓桿管理

## 自動劃轉功能

### 觸發條件
- **MARGIN_CALL**：保證金追繳
- **RISK_LIMIT**：風險限制觸發
- **PROFIT_TAKING**：獲利了結

### 劃轉流程
1. 檢查觸發條件
2. 驗證劃轉配置
3. 呼叫 S12 UI API
4. 記錄劃轉日誌

## 風險控制

### 持倉限制
- 最大持倉大小
- 最大槓桿倍數
- 最大日損失限制

### 保證金管理
- 實時保證金監控
- 自動保證金補充
- 風險預警機制

## 配置

服務使用以下配置：
- Redis：用於持倉狀態緩存
- ArangoDB：用於持倉歷史存儲
- 自動劃轉配置：閾值和限制
- 端口：8086（可通過環境變量 PORT 覆蓋）

## 部署

```bash
# 構建
go build -o s6-position .

# 運行
./s6-position
```

## 監控

服務提供以下監控指標：
- 持倉數量
- 持倉風險度
- 停損觸發次數
- 自動劃轉次數
- 風險預警次數

## 詳細實作項目（基於目標與範圍文件）

### 持倉管理功能詳細實作
- [ ] **持倉管理**
  - [ ] 實現持倉狀態監控和更新
  - [ ] 實現停損/止盈觸發邏輯
  - [ ] 實現持倉加倉/減倉邏輯
  - [ ] 實現持倉風險評估
- [ ] **自動劃轉觸發**
  - [ ] 實現配置檢查和間隔控制
  - [ ] 實現觸發條件檢查（margin call、risk limit、profit taking）
  - [ ] 實現自動劃轉執行邏輯
  - [ ] 實現劃轉日誌記錄和狀態追蹤
- [ ] **風險控制**
  - [ ] 實現保證金追繳檢查
  - [ ] 實現風險限額檢查
  - [ ] 實現獲利了結檢查
  - [ ] 實現風險預警機制
- [ ] **API 整合**
  - [ ] 實現與 S12 UI API 的整合
  - [ ] 實現劃轉請求發送和響應處理
  - [ ] 實現錯誤處理和重試機制
- [ ] **配置管理**
  - [ ] 實現自動劃轉配置讀取和更新
  - [ ] 實現配置變更監聽
  - [ ] 實現配置驗證和默認值設置

#### 6. 核心時序圖相關功能（基於時序圖實作）
- [ ] **新倉成立後止損掛單**
  - [ ] 新倉成立 → 掛止損(workingType=MARK_PRICE, reduceOnly)
  - [ ] POST /orders (STOP_MARKET, reduceOnly=true)
  - [ ] UPSERT orders(stop) 記錄
  - [ ] XADD risk:sl_arm {symbol,sl_px} 事件發布
- [ ] **守護停損機制**
  - [ ] 監控 WS 價格觸發
  - [ ] 觸發時 POST /orders (MARKET reduce-only 模擬平倉)
  - [ ] XADD guard:spot:arm {symbol,sl_px} 狀態管理
  - [ ] 本地監控 mid-price 觸發邏輯
- [ ] **倉位狀態管理**
  - [ ] 持倉狀態監控和更新
  - [ ] 倉位變化事件處理
  - [ ] 倉位風險評估
- [ ] **事件流整合**
  - [ ] pos:events:{INSTR} Stream 發布
  - [ ] 倉位狀態變更事件
  - [ ] 止損觸發事件記錄

#### 7. 服務與資料流相關功能（基於服務與資料流實作）
- [ ] **倉位管理（移動停損、分批止盈、加倉）**
  - [ ] ROE 計算：`roe = unRealizedProfit / isolatedWallet`
  - [ ] 加倉規則：`add_on_count < 2` 且 `roe ≥ {5%,10%}` 分階；加倉後重算 avg_entry
  - [ ] 移動停損：達 10% → `SL=breakeven`；達 20% → `SL=price_for_net(10%)`
  - [ ] 分批止盈：`roe ≥ 12%` → 平掉 `entry_quantities[0]`
- [ ] **定時任務**
  - [ ] 輪巡 5–10s 倉位狀態
  - [ ] 事件驅動（行情觸發）優先處理
- [ ] **錢包劃轉觸發**
  - [ ] 觸發：`insufficient_balance` 或 `risk.budget` 需要
  - [ ] 守門：上限/最小留存額
  - [ ] 記帳：TransferRequest/Response 事件寫入 strategy_events

#### 8. 定時任務相關功能（基於定時任務實作）
- [ ] **持倉管理 tick（每 10–30 秒）**
  - [ ] 取得 `P_mark, P_entry_avg, Q, isolated_margin`
  - [ ] 計算 ROE / ATR / Regime；檢查加倉、分批、移動停損規則
  - [ ] 以「撤舊掛新」或減倉/加倉指令送交 S4
  - [ ] 移動停損（ATR & 鎖利）：ATR 型（多）`SL = P_entry - k * ATR`
  - [ ] 鎖利 ROE 型（多）：給定鎖利門檻 ρ*，`P_lock = P_entry + (ρ* * isolated_margin) / Q`（空倉對稱換號）
  - [ ] 分批止盈（首腿）：若 `ROE ≥ ρ_1` → 平掉 `β_1%` 的 Q；更新均價/保證金後再評估下一腿
- [ ] **自動資金劃轉（每 5 分鐘）**
  - [ ] 合約自由額度不足：`need = max(0, min_free_fut - free_fut)`
  - [ ] 若 `free_spot ≥ need + spot_buffer` → 觸發 SPOT→FUT 劃轉（冪等 ID）

#### 9. 目標與範圍相關功能（基於目標與範圍實作）
- [ ] **前置依賴實作**
  - [ ] ArangoDB Collections：`positions_snapshots`、`orders/fills`（由 S4 回報）
  - [ ] Redis Keys：`pos:{sl}:level:*`、`pos:{tp}:ladder:*`、`pos:{adds}:*`；釋放 `risk:{*}`
- [ ] **風險與緩解**
  - [ ] Redis Cluster slot 移轉：使用官方 cluster client；關鍵操作具重試策略

#### 10. 路過的服務相關功能（基於路過的服務實作）
- [ ] **POST /positions/manage 或 管理 tick（排程）**
  - [ ] 讀：最新價、ATR、Regime；`pos` 當前 SL/TP 階；加倉上限；健康度
  - [ ] 算：ROE、強平距離；是否上升鎖利階；是否命中止盈；是否加倉
  - [ ] 行動：鎖利：`/cancel` 舊 SL → `/orders` 新 SL（reduceOnly）；止盈：`/orders` 市價 reduceOnly（分批）；加倉：`/orders` 新單；更新 `add_on_count`
  - [ ] 寫 DB：`positions_snapshots`（新快照）；`orders/fills`（由 S4 回報）
  - [ ] 寫 Redis：`pos:{sl}:level:<pos_id>`、`pos:{tp}:ladder:<pos_id>`、`pos:{adds}:<pos_id>`
  - [ ] 釋放配額：倉位關閉時 `risk:{budget}`/`risk:{concurrency}` 反向調整
- [ ] **自動資金劃轉（排程）**
  - [ ] 讀：SPOT/FUT 可用餘額
  - [ ] 算：`need=max(0,min_free_fut-free_fut)`
  - [ ] 若足→造 `transfer_id`，寫 DB `treasury_transfers(PENDING)` → 請 S12 審批 → S1 執行（若啟用全自動，可直連 S1）

#### 11. 功能對照補記相關功能（基於功能對照補記實作）
- [ ] **Trailing Stop（撤舊掛新）**
  - [ ] ROE 進入更高鎖利階：$\rho_1,\rho_2,\dots$；每達一階：撤舊 SL → 新 SL 更靠近當前價
  - [ ] 公式（多倉示例）：階段 $k$：$SL_k = \max(SL_{k-1}, P_{entry} + \theta_k \cdot \frac{\text{isolated\_margin}}{Q})$，其中 $\theta_k$ 為第 $k$ 階的 ROE 鎖利門檻
- [ ] **加倉（ROE門檻、上限）**
  - [ ] 觸發：同向新信號 + 當前 $\text{ROE} \ge$ 門檻 + `add_on_count` < cap
  - [ ] 加倉量：幾何遞減 $Q_{add}(j)=Q_0 \cdot \gamma^j$（$0<\gamma<1$）
- [ ] **分批止盈**
  - [ ] 若 $\text{ROE} \ge \rho_1$ → 平 $\beta_1 Q$；達 $\rho_2$ → 再平 $\beta_2 Q_{\text{remaining}}$ …
  - [ ] 每次平倉後重算均價與保證金
- [ ] **SPOT 會計（WAC）、庫存/對沖**
  - [ ] WAC（加權平均成本）：$WAC'=\frac{Q \cdot WAC + Q_{buy} \cdot P_{buy}}{Q+Q_{buy}}$
  - [ ] 賣出已實現損益：$\text{PnL}_{real}= (P_{sell}-WAC) \cdot Q_{sell}$
  - [ ] 未實現損益：$(P_{mark}-WAC) \cdot Q_{\text{remain}}$
  - [ ] 對沖：目標淨曝險（名目）：$E^*$；以 FUT 調整 $Q_{fut}$ 使 $Q_{spot} \cdot P + Q_{fut} \cdot P \approx E^*$
- [ ] **停滯交易觸發（Autopsy/處置）**
  - [ ] 若 $\text{duration} > T_{\text{stagnation}} \land |\text{ROE}|<\rho_{\min}$ → 觸發 `STAGNATED` 事件
  - [ ] S8 自動產生復盤，量化機會成本：期間若資金用於 cohort 中位策略的期望收益 $E[\Delta ROI]$

#### 12. 全服務一覽相關功能（基於全服務一覽實作）
- [ ] **POST /positions/manage** 或 **管理 tick（排程）**
  - [ ] 讀：最新價、ATR、Regime；`pos` 當前 SL/TP 階；加倉上限；健康度
  - [ ] 算：ROE、強平距離；是否上升鎖利階；是否命中止盈；是否加倉
  - [ ] 行動：鎖利：`/cancel` 舊 SL → `/orders` 新 SL（reduceOnly）；止盈：`/orders` 市價 reduceOnly（分批）；加倉：`/orders` 新單；更新 `add_on_count`
  - [ ] 寫 DB：`positions_snapshots`（新快照）；（`orders/fills` 由 S4 回報）
  - [ ] 寫 Redis：`pos:{sl}:level:<pos_id>`、`pos:{tp}:ladder:<pos_id>`、`pos:{adds}:<pos_id>`
  - [ ] 釋放配額：倉位關閉時 `risk:{budget}`/`risk:{concurrency}` 反向調整
- [ ] **自動資金劃轉（排程）**
  - [ ] 讀：SPOT/FUT 可用餘額
  - [ ] 算：`need=max(0,min_free_fut-free_fut)`
  - [ ] 流程：足額→造 `transfer_id`，寫 DB `treasury_transfers(PENDING)` → 請 S12 審批 → S1 執行（若啟用全自動，可直連 S1）

#### 13. Integration 附錄相關功能（基於 Integration 附錄實作）
- [ ] **FUT 止損保護機制**
  - [ ] 新倉監控：監控到新倉建立，立即觸發止損保護流程
  - [ ] 止損掛單：立即掛上 STOP_MARKET 止損單，使用 `MARK_PRICE` 工作類型
  - [ ] 減倉標記：設置 `reduce_only: true` 確保只能減倉不能加倉
  - [ ] 價格設定：基於 ATR 或其他技術指標動態計算止損價格
  - [ ] 風險通知：通過 Redis Stream 發送止損掛單通知
- [ ] **SPOT 守護停損機制**
  - [ ] 守護停損武裝：當 OCO 失敗時啟動守護停損機制
  - [ ] 價格監控：實時監控 WebSocket 中間價格
  - [ ] 觸發條件：價格觸及止損線時立即觸發
  - [ ] 執行方式：使用市價單快速平倉
  - [ ] 對沖標記：雖然不是 `reduce_only`，但通過等量對沖實現減倉效果
- [ ] **守護停損觸發流程**
  - [ ] 觸發檢測：檢測到價格觸及止損線
  - [ ] 平倉執行：生成守護停損觸發的平倉訂單
  - [ ] 事件發布：發布守護停損觸發事件到 Redis Stream
  - [ ] 狀態更新：更新持倉狀態為 PENDING_CLOSING
- [ ] **事務一致性保證**
  - [ ] 冪等性保證：使用 `intent_id` 作為冪等鍵確保重複請求的安全性
  - [ ] 狀態機管理：持倉狀態 PENDING_ENTRY → ACTIVE → PENDING_CLOSING → CLOSED
  - [ ] 失敗恢復：系統崩潰後能夠通過 `intent_id` 查詢持倉狀態

#### 14. Hop-by-Hop 執行規格相關功能（基於 Hop-by-Hop 執行規格補遺實作）
- [ ] **POST /positions/manage 或 管理 tick（排程）**
  - [ ] 讀：最新價、ATR、Regime；`pos` 當前 SL/TP 階；加倉上限；健康度
  - [ ] 算：ROE、強平距離；是否上升鎖利階；是否命中止盈；是否加倉
  - [ ] 行動：
    - [ ] 鎖利：`/cancel` 舊 SL → `/orders` 新 SL（reduceOnly）
    - [ ] 止盈：`/orders` 市價 reduceOnly（分批）
    - [ ] 加倉：`/orders` 新單；更新 `add_on_count`
  - [ ] 寫 DB：`positions_snapshots`（新快照）；（`orders/fills` 由 S4 回報）
  - [ ] 寫 Redis：`pos:{sl}:level:<pos_id>`、`pos:{tp}:ladder:<pos_id>`、`pos:{adds}:<pos_id>`
  - [ ] 釋放配額：倉位關閉時 `risk:{budget}`/`risk:{concurrency}` 反向調整
- [ ] **自動資金劃轉（排程）**
  - [ ] 讀：SPOT/FUT 可用餘額
  - [ ] 算：`need=max(0,min_free_fut-free_fut)`
  - [ ] 流程：足額→造 `transfer_id`，寫 DB `treasury_transfers(PENDING)` → 請 S12 審批 → S1 執行（若啟用全自動，可直連 S1）

#### 15. 功能規格書相關功能（基於功能規格書實作）
- [ ] **入向（被呼叫）API**
  - [ ] `GET /health`（所有服務）→ `HealthResponse{Status,Checks,...}`
  - [ ] `POST /positions/manage`（S12/維運觸發）→ `ManagePositionsResponse{Plan, Orders}`
- [ ] **出向（主以事件）**
  - [ ] `POST /orders` → S4（移動停損/減倉/加倉）
  - [ ] （如需補保證金）`POST /xchg/treasury/transfer` → S1（內部）
- [ ] **移動停損 / 分批止盈 / 加倉**
  - [ ] 觸發：S6 定時計算持倉健康；ROE/ATR/規則命中
  - [ ] S6 `POST /positions/manage`（可外露給 S12/維運觸發）→ `ManagePlan`
  - [ ] 依計畫逐條 S6 → S4 `POST /orders`
  - [ ] 失敗補償：任一子單失敗：S6 記錄部分成功，對失敗單重試或回滾（撤舊掛新）
- [ ] **金庫資金劃轉（自動）**
  - [ ] S6 → S1(私有) `POST /xchg/treasury/transfer`（`TransferRequest`）→ `TransferResponse`
  - [ ] 風險預算/保證金自動補充

### 🎯 實作優先順序
1. **高優先級**：基本持倉管理和止損保護
2. **中優先級**：Trailing Stop 和分批止盈
3. **低優先級**：加倉和自動資金劃轉

### 📊 相關資料寫入
- **DB Collections**：`positions_snapshots`、`treasury_transfers(PENDING)`
- **Redis Key/Stream**：`pos:{sl}:level:*`、`pos:{tp}:ladder:*`、`pos:{adds}:*`
