# S2 Feature Generator ❌ **[未實作]**

Feature Generator - Generate features from market data and signals

## 📋 實作進度：20% (1/5 功能完成)

### ✅ 已完成功能
- [x] 基礎服務架構
- [x] Health Check API
- [x] 特徵計算器框架（ATR、RV、相關性、深度）
- [x] 特徵快取機制
- [x] 任務管理系統

### ❌ 待實作功能

#### 1. 消費 `mkt:events:*`
- [ ] **市場數據消費**
  - [ ] 從 Redis Stream 消費市場事件
  - [ ] 滑窗快取 `feat:{cache}:<symbol>`
  - [ ] 讀取 `config_active.rev`
- [ ] **特徵計算**
  - [ ] ATR/RV/ρ/Spread/Depth 等特徵計算
  - [ ] DQC 標記（數據質量檢查）
- [ ] **DB 寫入**
  - [ ] `signals`（新/補寫 `features`、`t0`、`config_rev`）
- [ ] **事件發布**
  - [ ] `feat:events:<symbol>`（含 `signal_id,t0,symbol,features`）

#### 2. 每日 Regime（排程）
- [ ] **Regime 計算**
  - [ ] RV 百分位計算
  - [ ] 標籤 FROZEN/NORMAL/EXTREME
- [ ] **Redis KV 寫入**
  - [ ] `prod:{regime}:market:state`（帶 `rev` 與過期時間戳）
- [ ] **指標收集**
  - [ ] `metrics:events:s2.regime_latency`

#### 3. POST /features/recompute
- [ ] **期間數據讀取**
  - [ ] 期間 K 線/深度（資料湖/交易所）
- [ ] **特徵補算**
  - [ ] 補算特徵邏輯
- [ ] **DB 寫入**
  - [ ] 回補 `signals.features`
  - [ ] 寫 `strategy_events(kind=FEATURE_RECOMPUTE)`

#### 4. Redis Stream 整合
- [ ] **Redis Stream 發布**
  - [ ] 實現實際的 Redis Stream 發布
  - [ ] 特徵數據序列化
- [ ] **Redis 消費**
  - [ ] 從 `mkt:events:*` 消費數據

#### 5. 配置管理
- [ ] **配置熱載**
  - [ ] 監聽 `cfg:events`
  - [ ] RCU 熱載機制
- [ ] **配置快取**
  - [ ] `config_active.rev` 讀取和快取

#### 6. 核心時序圖相關功能（基於時序圖實作）
- [ ] **信號觸發機制**
  - [ ] features.ready 事件生成
  - [ ] Stream signals:new 發布
  - [ ] 特徵計算完成通知
- [ ] **特徵數據準備**
  - [ ] 特徵數據格式化和驗證
  - [ ] config_rev 版本標記
  - [ ] t0 時間戳標記
- [ ] **事件流整合**
  - [ ] feat:events:{INSTR} Stream 發布
  - [ ] 特徵數據序列化
  - [ ] 事件格式標準化
- [ ] **數據質量保證**
  - [ ] 特徵數據完整性檢查
  - [ ] 不允許未來資料驗證
  - [ ] 特徵計算錯誤處理

#### 7. 服務與資料流相關功能（基於服務與資料流實作）
- [ ] **特徵工程（ATR / RV / 相關性 / 深度）**
  - [ ] True Range 計算：`TR_t = max(H_t - L_t, |H_t - C_{t-1}|, |L_t - C_{t-1}|)`
  - [ ] ATR(n) Wilder 計算：`ATR_t = ATR_{t-1} + (TR_t - ATR_{t-1})/n`
  - [ ] log return 計算：`r_t = ln(P_t / P_{t-1})`
  - [ ] 實現波動率計算：`rv_m = sqrt(252) * std(r_{t-m+1..t})`
  - [ ] rv 分位計算：`rv_pctile_30d = pctile(rv_1d, lookback=365d)`
  - [ ] USDT/TWD×BTC 相關性：`rho_usdttwd_k = corr(r_btc, r_fx, window=k)`
  - [ ] 深度因子：`liq_score = min(depth_top1_usdt / threshold, 1.0)`
- [ ] **定時任務**
  - [ ] 每 1m/5m/1h/4h/1d 滾動計算
  - [ ] 掉線補算任務（補 K 線缺口）
- [ ] **數據輸出**
  - [ ] signals.features（ArangoDB）寫入
  - [ ] Redis feat:last:{symbol} 更新

#### 8. 定時任務相關功能（基於定時任務實作）
- [ ] **特徵回補（每小時 / 手動）**
  - [ ] 掃描 `signals`，篩出特徵缺失或過期記錄
  - [ ] 取得歷史 K 線 / 深度 / 資金費等原始數據
  - [ ] 重新計算特徵寫回；標記 DQC 欄位
  - [ ] True Range：`TR_t = max{H_t-L_t, |H_t-C_{t-1}|, |L_t-C_{t-1}|}`
  - [ ] ATR（EMA, 參數 n）：`ATR_t = α * TR_t + (1-α) * ATR_{t-1}`，`α = 2/(n+1)`
  - [ ] 對數報酬：`r_t = ln(P_t / P_{t-1})`
  - [ ] 已實現波動率（N 期）：`RV_N = sqrt(K) * σ(r_{t-N+1..t})`（年化因子 K：日線 sqrt(365)；4h 線 ≈ sqrt(6×365)）
  - [ ] 皮爾森相關（滑窗 M）：`ρ_XY = Σ(x-x̄)(y-ȳ) / sqrt(Σ(x-x̄)²) * sqrt(Σ(y-ȳ)²)`
- [ ] **市場 Regime 更新（每日 00:05）**
  - [ ] 取近 N 日 RV 序列
  - [ ] 計算百分位名次與 Regime，寫入 `prod:regime:market:state`
  - [ ] 百分位名次（含 ties）：`PctRank(X) = (C_L + 0.5 * C_E) / N`（C_L：小於 X；C_E：等於 X）
  - [ ] 分群建議：`pct < 0.10` → FROZEN；`0.10 ≤ pct ≤ 0.90` → NORMAL；`pct > 0.90` → EXTREME

#### 9. 目標與範圍相關功能（基於目標與範圍實作）
- [ ] **前置依賴實作**
  - [ ] ArangoDB Collections：`signals`、`strategy_events`
  - [ ] Redis Streams：`feat:events:<symbol>`
  - [ ] Redis Keys：`prod:{regime}:market:state`（每日）、`feat:{cache}:<symbol>`（滑窗快取）
- [ ] **環境變數配置**
  - [ ] `S2_DB_ARANGO_URI`、`S2_DB_ARANGO_USER/PASS`
  - [ ] `S2_REDIS_ADDRESSES`（逗號分隔，Cluster 模式）
  - [ ] `S2_SYMBOLS`（預設：BTCUSDT，可多）
- [ ] **風險與緩解**
  - [ ] Redis Cluster slot 移轉：使用官方 cluster client；關鍵操作具重試策略

#### 10. 路過的服務相關功能（基於路過的服務實作）
- [ ] **消費 `mkt:events:*`**
  - [ ] 讀：滑窗快取 `feat:{cache}:<symbol>`、`config_active.rev`
  - [ ] 算：ATR/RV/ρ/Spread/Depth 等；DQC 標記
  - [ ] 寫 DB：`signals`（新/補寫 `features`、`t0`、`config_rev`）
  - [ ] 發事件：`feat:events:<symbol>`（含 `signal_id,t0,symbol,features`）
- [ ] **每日 Regime（排程）**
  - [ ] 算：RV 百分位；標籤 FROZEN/NORMAL/EXTREME
  - [ ] 寫 Redis KV：`prod:{regime}:market:state`（帶 `rev` 與過期時間戳）
  - [ ] 指標：`metrics:events:s2.regime_latency`
- [ ] **POST /features/recompute**
  - [ ] 讀：期間 K 線/深度（資料湖/交易所）
  - [ ] 算：補算特徵
  - [ ] 寫 DB：回補 `signals.features`；寫 `strategy_events(kind=FEATURE_RECOMPUTE)`

#### 11. 字段校驗相關功能（基於字段校驗表實作）
- [ ] **POST /features/recompute 字段校驗**
  - [ ] `symbols[]`：必填，≥1 個符號，每個符號正則 `^[A-Z0-9]{3,}$` 驗證
  - [ ] `windows[]`：必填，值域受限 {1m, 5m, 1h, 4h, 1d} 枚舉驗證
  - [ ] `force`：可選布爾值，預設 false
  - [ ] `from_ts`/`to_ts`：可選時間戳，範圍 1–90 天驗證
- [ ] **FeatureRequest 字段校驗**
  - [ ] `symbol`：必填，正則 `^[A-Z0-9]{3,}$` 驗證
  - [ ] `config_rev`：可選，CURRENT 或整數驗證
  - [ ] `dry_run`：可選布爾值，預設 false
- [ ] **錯誤處理校驗**
  - [ ] 400 Bad Request：未知 window、symbols 為空
  - [ ] 422 Unprocessable Entity：特徵缺失、數據不完整
  - [ ] 冪等性：相同參數返回相同結果
- [ ] **契約測試**
  - [ ] 合法 symbol/window → `accepted`=true
  - [ ] 未知 window → 400 錯誤
  - [ ] symbols 為空 → 400 錯誤
  - [ ] 特徵缺失 → 422 錯誤

#### 12. 功能對照補記相關功能（基於功能對照補記實作）
- [ ] **USDT/TWD × BTCUSDT 規則信號**
  - [ ] 計算兩資產日對數報酬：$r^{\text{usdttwd}}_t, r^{\text{btcusdt}}_t$
  - [ ] 以兩日同向 + 首日反向構造方向：
    - 做多：$r^{\text{usdttwd}}_{t-1}<0 \land r^{\text{usdttwd}}_{t-2}<0 \land r^{\text{btcusdt}}_{t-1}<0$
    - 做空：$r^{\text{usdttwd}}_{t-1}>0 \land r^{\text{usdttwd}}_{t-2}>0 \land r^{\text{btcusdt}}_{t-1}>0$
  - [ ] 穩健條件：$|\rho_{14}(\text{usdttwd},\text{btcusdt})|>\rho_{\min}$
- [ ] **ATR 停損（Regime 倍數 + 風險上限）**
  - [ ] 產出 $ATR_t$ 與 `Regime` ∈ {FROZEN, NORMAL, EXTREME}
  - [ ] 對應倍數 $k_{\text{regime}}$；初始停損（多）：$SL_0=P_{entry}-k \cdot ATR$（空相反）
  - [ ] 風險上限：不允許首筆損失超過 $\text{RiskCapPct}$ 的保證金
  - [ ] 公式：$SL = P_{entry} - dir \cdot \min\!\left(\frac{Loss_{cap}}{Q},\ k_{\text{regime}} \cdot ATR\right)$

#### 13. 全服務一覽相關功能（基於全服務一覽實作）
- [ ] **消費 `mkt:events:*`**
  - [ ] 讀：滑窗快取 `feat:{cache}:<symbol>`、`config_active.rev`
  - [ ] 算：ATR/RV/ρ/Spread/Depth 等；DQC 標記
  - [ ] 寫 DB：`signals`（新/補寫 `features`、`t0`、`config_rev`）
  - [ ] 發事件：`feat:events:<symbol>`（`signal_id,t0,symbol,features`）
- [ ] **每日 Regime（排程）**
  - [ ] 算：RV 百分位 → Regime（FROZEN/NORMAL/EXTREME）
  - [ ] 寫 Redis KV：`prod:{regime}:market:state`（帶 `rev` 和過期戳）
  - [ ] 指標：`metrics:events:s2.regime_latency`
- [ ] **POST /features/recompute**
  - [ ] 讀：期間 K 線/深度（資料湖/交易所）
  - [ ] 算：補算特徵
  - [ ] 寫 DB：回補 `signals.features`；`strategy_events(kind=FEATURE_RECOMPUTE)`

#### 14. Integration 附錄相關功能（基於 Integration 附錄實作）
- [ ] **信號事件生成（S2 → S3）**
  - [ ] 信號事件格式：`signal_id`、`symbol`、`t0`、`features`、`config_rev`
  - [ ] 特徵數據：`atr_14`、`volatility_30d`、`correlation_spx`、`rsi_14`、`macd_signal`
  - [ ] Redis Stream 發布：`signals:new` 事件到 Redis Stream
  - [ ] 時間戳統一：使用 epoch 毫秒（ms）格式
- [ ] **事務一致性保證**
  - [ ] 冪等性保證：特徵計算使用 `signal_id` 作為冪等鍵
  - [ ] 狀態機管理：特徵計算狀態 PENDING → COMPUTED → PUBLISHED
  - [ ] 失敗恢復：系統崩潰後能夠重新計算特徵
- [ ] **性能優化**
  - [ ] 並行處理：多個符號的特徵計算並行執行
  - [ ] 緩存機制：特徵結果緩存避免重複計算
  - [ ] 批量處理：批量發布特徵事件提高效率

#### 15. Hop-by-Hop 執行規格相關功能（基於 Hop-by-Hop 執行規格補遺實作）
- [ ] **消費 `mkt:events:*`**
  - [ ] 讀：滑窗快取 `feat:{cache}:<symbol>`、`config_active.rev`
  - [ ] 算：ATR/RV/ρ/Spread/Depth 等；DQC 標記
  - [ ] 寫 DB：`signals`（新/補寫 `features`、`t0`、`config_rev`）
  - [ ] 發事件：`feat:events:<symbol>`（`signal_id,t0,symbol,features`）
- [ ] **每日 Regime（排程）**
  - [ ] 算：RV 百分位 → Regime（FROZEN/NORMAL/EXTREME）
  - [ ] 寫 Redis KV：`prod:{regime}:market:state`（帶 `rev` 和過期戳）
  - [ ] 指標：`metrics:events:s2.regime_latency`
- [ ] **POST /features/recompute**
  - [ ] 讀：期間 K 線/深度（資料湖/交易所）
  - [ ] 算：補算特徵
  - [ ] 寫 DB：回補 `signals.features`；`strategy_events(kind=FEATURE_RECOMPUTE)`

#### 16. 功能規格書相關功能（基於功能規格書實作）
- [ ] **入向（被呼叫）API**
  - [ ] `GET /health`（所有服務）→ `HealthResponse{Status,Checks,...}`
  - [ ] `POST /features/recompute`（維運/批次）→ `RecomputeFeaturesResponse`
- [ ] **出向（主以事件）**
  - [ ] 寫入 DB signals.features；發 signals:new 事件（Stream）
- [ ] **特徵補算流程**
  - [ ] 觸發：維運/批次 → S2 `POST /features/recompute`（`RecomputeFeaturesRequest`）
  - [ ] 處理：依需要跑回補（例如資料缺口）
  - [ ] 失敗補償：失敗者記號重試佇列；連續 3 次失敗→alerts(ERROR)
- [ ] **系統開機與配置收斂**
  - [ ] S2 → S10 `GET /active` 取得 rev/bundle_id
  - [ ] 訂閱〔Stream: cfg:events〕，本地 RCU 熱載
  - [ ] `GET /active` 失敗：退避重試（exponential backoff 5→30s）；未就緒前僅 `/health` OK=DEGRADED
  - [ ] 所有寫 signals 的服務須把 config_rev 寫入紀錄

### 🎯 實作優先順序
1. **高優先級**：Redis Stream 消費和特徵計算
2. **中優先級**：每日 Regime 計算
3. **低優先級**：配置管理和優化

### 📊 相關資料寫入
- **DB Collections**：`signals(features,t0,config_rev)`、`strategy_events(FEATURE_RECOMPUTE)`
- **Redis Key/Stream**：`feat:events:<sym>`、`prod:{regime}:market:state`

## 概述

S2 Feature Generator 是 Project Chimera 的特徵工程服務，負責從市場數據和信號中計算各種技術特徵，包括 ATR、已實現波動率、相關性、深度等特徵，為策略引擎提供決策依據。

## 功能特性

### 1. 特徵計算
- **ATR (Average True Range)**：計算平均真實範圍，用於衡量價格波動性
- **已實現波動率 (Realized Volatility)**：基於歷史價格計算的波動率指標
- **相關性分析**：計算不同標的之間的價格相關性
- **深度特徵**：分析訂單簿深度和價差特徵

### 2. 多時間窗口支持
- **1分鐘**：短期特徵計算
- **5分鐘**：中短期特徵計算
- **1小時**：中期特徵計算
- **4小時**：中長期特徵計算
- **1天**：長期特徵計算

### 3. 實時計算與快取
- **內存快取**：最新特徵數據的內存快取
- **Redis 發布**：將特徵數據發布到 Redis Streams
- **定時補算**：每 5 分鐘自動補算特徵

### 4. 任務管理
- **異步計算**：支持異步特徵計算任務
- **進度追蹤**：實時追蹤計算任務進度
- **錯誤處理**：完善的錯誤處理和重試機制

## API 端點

### 健康檢查
- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 特徵管理
- `POST /features/recompute` - 重新計算特徵
- `GET /features?symbol=BTCUSDT&feature_type=ATR` - 獲取特徵數據
- `GET /features/computation?task_id=xxx` - 獲取計算任務狀態

## 數學計算

### ATR (Average True Range)
```
TR_t = max(H_t - L_t, |H_t - C_{t-1}|, |L_t - C_{t-1}|)
ATR_t = ATR_{t-1} + (TR_t - ATR_{t-1}) / n
ATR_pct = (ATR / current_price) * 100
```

### 已實現波動率 (Realized Volatility)
```
r_t = ln(P_t / P_{t-1})
rv_m = sqrt(252) * std(r_{t-m+1..t})
```

### 相關性計算
```
rho = corr(Δln P_btc, Δln FX, window=k)
```

### 深度因子
```
liq_score = min(depth_top1_usdt / threshold, 1.0)
spread_bps = (bestAsk - bestBid) / mid * 1e4
```

## 特徵類型

### ATR 特徵
- `atr`: ATR 絕對值
- `atr_pct`: ATR 百分比
- `period`: 計算週期

### RV 特徵
- `rv`: 已實現波動率
- `rv_pct`: 波動率百分比
- `period`: 計算週期

### 相關性特徵
- `correlation`: 相關性係數
- `symbol1`: 第一個標的
- `symbol2`: 第二個標的
- `period`: 計算週期

### 深度特徵
- `bid_depth`: 買盤深度
- `ask_depth`: 賣盤深度
- `bid_ask_ratio`: 買賣比例
- `spread`: 價差
- `spread_pct`: 價差百分比

## 配置參數

### 計算器配置
- **ATR 週期**: 14（可調整）
- **RV 週期**: 20（可調整）
- **相關性週期**: 14（可調整）

### 定時任務配置
- **補算間隔**: 5 分鐘
- **支援標的**: BTCUSDT, ETHUSDT, ADAUSDT
- **支援窗口**: 1m, 5m, 1h, 4h, 1d

## 數據流

### 輸入數據
- **市場數據**: 來自 S1 Exchange Connectors
- **K 線數據**: OHLCV 格式的價格數據
- **深度數據**: 訂單簿買賣盤數據

### 輸出數據
- **特徵快照**: 內存快取的特徵數據
- **Redis Streams**: `feat:last:{symbol}` 最新特徵
- **ArangoDB**: `signals.features` 持久化存儲

## 性能特性

### 計算效率
- **並行計算**: 多個特徵並行計算
- **快取機制**: 避免重複計算
- **增量更新**: 只計算新增數據

### 內存管理
- **LRU 快取**: 最近使用的特徵優先保留
- **定期清理**: 清理過期的計算任務
- **內存監控**: 實時監控內存使用情況

## 錯誤處理

### 數據不足
- 檢查數據點數量是否足夠
- 提供詳細的錯誤信息
- 跳過無法計算的特徵

### 計算錯誤
- 記錄計算失敗的詳細信息
- 繼續計算其他特徵
- 返回部分成功的結果

## 監控指標

### 服務健康指標
- Redis 連接延遲
- ArangoDB 連接延遲
- 特徵計算延遲

### 業務指標
- 特徵計算成功率
- 快取命中率
- 任務完成時間

## 部署說明

### Docker 部署
```bash
docker build -t s2-feature .
docker run -p 8082:8082 s2-feature
```


## 技術規格

### 環境要求
- Go 1.19+
- Redis Cluster
- ArangoDB
- 足夠的內存（建議 2GB+）

## 開發指南
### API 端點
- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查
- `POST /features/recompute` - 重新計算特徵
- `GET /features?symbol=BTCUSDT&feature_type=ATR` - 獲取特徵數據


### 添加新特徵
1. 實現 `FeatureCalculator` 接口
2. 在 `initializeFeatureCalculators` 中註冊
3. 添加相應的數據模型

### 本地開發
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
1. **特徵計算失敗**
   - 檢查輸入數據是否完整
   - 確認計算參數是否正確
   - 查看日誌中的錯誤信息

2. **快取數據過期**
   - 檢查定時任務是否正常運行
   - 確認 Redis 連接狀態
   - 手動觸發特徵重算

3. **內存使用過高**
   - 檢查快取大小設置
   - 確認是否有內存洩漏
   - 調整快取清理策略

## 版本歷史

### v1.0.0
- 初始版本
- 支持 ATR、RV、相關性、深度特徵計算
- 實現多時間窗口支持
- 添加任務管理和進度追蹤

### 數學計算
- **ATR**: `TR_t = max(H_t - L_t, |H_t - C_{t-1}|, |L_t - C_{t-1}|)`, `ATR_t = ATR_{t-1} + (TR_t - ATR_{t-1}) / n`
- **RV**: `r_t = ln(P_t / P_{t-1})`, `rv_m = sqrt(252) * std(r_{t-m+1..t})`
- **相關性**: `rho = corr(Δln P_btc, Δln FX, window=k)`
- **深度因子**: `liq_score = min(depth_top1_usdt / threshold, 1.0)`