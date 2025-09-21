# S3 Strategy Engine ❌ **[未實作]**

Strategy Engine - Execute trading strategies based on features and rules

## 📋 實作進度：25% (2/8 功能完成)

### ✅ 已完成功能
- [x] 基礎服務架構
- [x] Health Check API
- [x] L0 守門器（GateKeeper）
- [x] L1 規則引擎（RuleEngine）
- [x] L2 機器學習模型（MLModel）
- [x] 配置管理器（ConfigManager）
- [x] 決策生成邏輯
- [x] 訂單意圖生成

### ❌ 待實作功能

#### 1. 接 `feat:events:<symbol>` 或 `/decide`
- [ ] **數據讀取**
  - [ ] `prod:{kill_switch}` 檢查
  - [ ] `config_active.rev` & bundle 讀取
  - [ ] 風險配額（Redis）讀取
  - [ ] `funding:{next}:<symbol>` 讀取
  - [ ] 健康 `prod:{health}:system:state` 讀取
- [ ] **守門 L0**
  - [ ] KillSwitch 檢查
  - [ ] 交易時窗檢查
  - [ ] 保證金/併發檢查（見 S3 風險鍵）
- [ ] **L1 規則 DSL**
  - [ ] 按 `priority` 合成 `skip_entry/size_mult/tp_mult/sl_mult/max_adds_override`
- [ ] **L2 模型**
  - [ ] 超時回退機制
  - [ ] 映射 `size_mult`
- [ ] **產決策**
  - [ ] `Decision{action=open|skip, size_mult,…, reason}`
- [ ] **DB 寫入**
  - [ ] `signals.decision`（含 `model_p`、`reason`、`config_rev`）
- [ ] **事件發布**
  - [ ] `sig:events`（決策快照）
- [ ] **訂單意圖**
  - [ ] 若 `open`：組 `OrderIntent{market=FUT|SPOT,…,intent_id}` → 呼 S4 `/orders`

#### 2. 風險鍵（Redis；原子）
- [ ] **風險預算管理**
  - [ ] `risk:{budget}:fut_margin:inuse`（USDT 加總）
  - [ ] `risk:{concurrency}:<symbol>`（併發數）
- [ ] **原子操作**
  - [ ] 通過→暫占；失敗→`decision.skip(reason=RISK_BUDGET)`

#### 3. Redis Stream 整合
- [ ] **事件消費**
  - [ ] 從 `feat:events:<symbol>` 消費特徵事件
- [ ] **事件發布**
  - [ ] 發布 `sig:events` 決策事件

#### 4. 配置熱載
- [ ] **配置監聽**
  - [ ] 監聽 `cfg:events`
  - [ ] RCU 熱載機制

#### 5. 風險管理優化
- [ ] **風險預算動態調整**
- [ ] **併發控制優化**
- [ ] **健康度響應機制**

#### 6. 詳細實作項目（基於目標與範圍文件）
- [ ] **API 與健康檢查**
  - [ ] **GET /health**：回傳依賴（Redis、Arango、Config rev、Rules loaded N）
  - [ ] **POST /decide**
    - **入**：`{ symbol, sideHint?, dry_run?, context? }`
    - **出**：`{ decision: open|skip, intent?, reason, config_rev, rules_fired[] }`
  - [ ] **（可選）POST /decide/batch**
- [ ] **Config Watcher（RCU 熱載）**
  - [ ] **來源**：`config_active.rev`（Arango/Redis）；事件：`cfg:events`
  - [ ] **流程**：取得 `bundle_id, rev` → 拉取 `strategy_rules/flags` → 本地 Lint 再替換
  - [ ] **版本一致性**：進行中判斷使用舊；下一筆切新版；`signals.config_rev` 落地
- [ ] **決策管線（演算法）**
  - [ ] **取特徵**：從 `signals.features` 或即時計算快取（`feat:last:{symbol}`）
  - [ ] **L0 守門**：
    - `funding_next_abs ≤ max_funding_abs`
    - `spread_bps ≤ spread_bp_limit`、`depth_top1_usdt ≥ min`
    - 風險預算：`risk.budget.*`、`concurrent_entries_per_market`
  - [ ] **L1 規則 DSL**（白名單、clamp、短路 skip_entry）
  - [ ] **L2 ML 分數**（暫以 mock：`p=0.65 → size_mult_ml=1.0`）
  - [ ] **Sizing（FUT）**：`margin=20*size_mult`；`notional=margin*leverage`；`qty=round(notional/price, stepSize)`
  - [ ] **SL/TP**：
    - `d_atr = ATR_mult*ATR`；`d_cap = max_loss_usdt/qty`；`d=min(...)`
    - 多單：`SL=entry-d`；TP 以「淨利≥10%」反推
  - [ ] **SPOT**：`qty=floor(quote_budget/price, stepSize)`；OCO tp/sl 由策略或固定距離
  - [ ] **意圖輸出**：FUT 或 SPOT；附 `exec_policy`（Maker→Taker/TWAP/OCO）與 `client_order_id`
- [ ] **持久化與事件**
  - [ ] `signals` 寫入：`{signal_id,t0,symbol,features,decision,config_rev}`
  - [ ] 發布至 `orders:intent`（Stream）與 REST 直呼 S4（兩條路皆可，建議雙軌先保險）
- [ ] **冪等 & 鎖**
  - [ ] `client_order_id` 生成規則：`{svc}-btc-TS-rand`；Redis `SETNX idem:order:{id}=1 ttl=1h`
  - [ ] 同 symbol 單次決策：`lock:pos:{symbol}`（最大 3s），避免重覆 open
- [ ] **指標與告警**
  - [ ] `strategy.decide.latency_ms`、`rules.fired.count`、`gate.skip.count`
  - [ ] **告警**：無規則可用、配置與本地 rev 不一致、決策產生但發布失敗
- [ ] **測試與驗收**
  - [ ] **單元**：L0/L1/L2、sizing、SL/TP 反解、clamp、skip 短路
  - [ ] **契約**：POST /decide schema 正確；dry_run 不落地 intent
  - [ ] **整合**：與 S4 stub；亂流（spread/depth 異常）能正確 skip
  - [ ] **驗收（Ready）**
    - 1000 筆模擬特徵決策 ≤ 200ms/P50、≤ 500ms/P95
    - 風險守門全覆蓋；決策落 signals 且 config_rev 正確
    - 意圖可同時走 REST 與 Stream；冪等生效

#### 7. 核心時序圖相關功能（基於時序圖實作）
- [ ] **信號處理流程**
  - [ ] 接收 features.ready 事件
  - [ ] INSERT signals{features, config_rev, t0} 到 DB
  - [ ] 規則 DSL + 置信度 → 決策 & intents(FUT/SPOT)
- [ ] **事務狀態機**
  - [ ] INSERT strategy_events{ENTRY,PENDING_ENTRY}
  - [ ] UPDATE strategy_events{ACTIVE} 狀態轉換
  - [ ] PENDING_ENTRY → ACTIVE → (PENDING_CLOSING) → CLOSED
- [ ] **訂單意圖生成**
  - [ ] POST /orders (intent_id 冪等, policy=MakerThenTaker, market=FUT)
  - [ ] POST /orders (intent_id, market=SPOT, execPolicy=OCO, tp_px/sl_px)
  - [ ] OrderResult{FILLED/ACCEPTED} 處理
- [ ] **決策邏輯整合**
  - [ ] 規則 DSL 條件判斷
  - [ ] 置信度計算和映射
  - [ ] 市場類型選擇 (FUT/SPOT)
  - [ ] 執行策略選擇 (MakerThenTaker/OCO)
- [ ] **事件流發布**
  - [ ] sig:events:{INSTR} Stream 發布
  - [ ] ord:cmd:{INSTR} Stream 發布
  - [ ] strategy_events 事件記錄

#### 8. 服務與資料流相關功能（基於服務與資料流實作）
- [ ] **入場守門（L0 Gate）**
  - [ ] 資金費上限檢查：`|funding_next| <= max_funding_abs`
  - [ ] 流動性下限檢查：`spread_bps <= spread_bp_limit` 且 `depth_top1_usdt >= min`
  - [ ] 風險預算檢查：`Σ spot_notional <= risk.budget.spot_quote_usdt_max`
  - [ ] 風險預算檢查：`Σ fut_margin <= risk.budget.fut_margin_usdt_max`
  - [ ] 並發入場限制：`concurrent_entries_per_market` 不超
  - [ ] 不過門 → `decision.action = "skip"`
- [ ] **規則 DSL（L1 Rules）**
  - [ ] 規則解譯和執行
  - [ ] 命中多條規則 → `size_mult/tp_mult/sl_mult` 相乘後 clamp 至白名單
  - [ ] `skip_entry=true` → 短路處理
- [ ] **置信度模型（L2 ML Score）**
  - [ ] 監督式模型推論（Logistic/XGBoost）：輸入特徵 X，輸出 `p = P(win | X)`
  - [ ] 倉位倍率計算：`size_mult_ml = piecewise(p)`（>0.85 → ×1.2；0.6–0.85 → ×1.0；0.4–0.6 → ×0.5；<0.4 → skip）
  - [ ] 最終倍率：`size_mult = size_mult_rules × size_mult_ml`（clamp）
- [ ] **FUT 下單意圖與倉位 sizing**
  - [ ] 保證金計算：`margin_base = 20 USDT`；`margin = margin_base × size_mult`
  - [ ] 名義倉位：`notional = margin × leverage`
  - [ ] 數量計算：`qty = round_to_step( notional / price, stepSize )`
  - [ ] 停損距離：`d_atr = ATR_mult × ATR`；`d_losscap = (max_loss_usdt) / qty`；`d = min(d_atr, d_losscap)`
  - [ ] SL 價計算：多單 `SL = entry - d`；空單 `SL = entry + d`
  - [ ] TP 計算：`target_pnl = 0.10 × (Σ margins)` ⇒ 對應 target_price 反解
- [ ] **SPOT 入場 + OCO（TP/SL）**
  - [ ] 數量計算：`qty = floor_to_step( quote_budget / price, stepSize )`
  - [ ] TP/SL 設置：相對平均成本 avg_cost 設置 `tp_price > avg_cost`、`sl_price < avg_cost`（BUY，SELL 反向）
  - [ ] OCO 邏輯：一腿成交即自動取消另一腿；交易所不支援則由 S4 客戶端守護

#### 9. 定時任務相關功能（基於定時任務實作）
- [ ] **決策心跳（冗餘，毎 10s）**
  - [ ] 若 `feat:events:*` 在最近 T 秒無更新，主動拉快照並執行一次硬性守門
  - [ ] 強平距離檢查：`LB = |P_mark - P_liq| / P_mark`；若 `LB < lb_min` → 觸發降風險（減倉/收緊 SL）
  - [ ] ROE 計算（USDT 永續，長倉）：`PnL = (P - P_entry) * Q`；`ROE = PnL / isolated_margin`（空倉對稱換號）

#### 10. 目標與範圍相關功能（基於目標與範圍實作）
- [ ] **API 與健康檢查**
  - [ ] GET /health：回傳依賴（Redis、Arango、Config rev、Rules loaded N）
  - [ ] POST /decide：入 `{ symbol, sideHint?, dry_run?, context? }`，出 `{ decision: open|skip, intent?, reason, config_rev, rules_fired[] }`
  - [ ] （可選）POST /decide/batch
- [ ] **Config Watcher（RCU 熱載）**
  - [ ] 來源：`config_active.rev`（Arango/Redis）；事件：`cfg:events`
  - [ ] 流程：取得 `bundle_id, rev` → 拉取 `strategy_rules/flags` → 本地 Lint 再替換
  - [ ] 版本一致性：進行中判斷使用舊；下一筆切新版；`signals.config_rev` 落地
- [ ] **決策管線（演算法）**
  - [ ] 取特徵：從 `signals.features` 或即時計算快取（`feat:last:{symbol}`）
  - [ ] L0 守門：`funding_next_abs ≤ max_funding_abs`、`spread_bps ≤ spread_bp_limit`、`depth_top1_usdt ≥ min`、風險預算：`risk.budget.*`、`concurrent_entries_per_market`
  - [ ] L1 規則 DSL（白名單、clamp、短路 skip_entry）
  - [ ] L2 ML 分數（暫以 mock：`p=0.65 → size_mult_ml=1.0`）
  - [ ] Sizing（FUT）：`margin=20*size_mult`；`notional=margin*leverage`；`qty=round(notional/price, stepSize)`
  - [ ] SL/TP：`d_atr = ATR_mult*ATR`；`d_cap = max_loss_usdt/qty`；`d=min(...)`；多單：`SL=entry-d`；TP 以「淨利≥10%」反推
  - [ ] SPOT：`qty=floor(quote_budget/price, stepSize)`；OCO tp/sl 由策略或固定距離
  - [ ] 意圖輸出：FUT 或 SPOT；附 `exec_policy`（Maker→Taker/TWAP/OCO）與 `client_order_id`
- [ ] **持久化與事件**
  - [ ] `signals` 寫入：`{signal_id,t0,symbol,features,decision,config_rev}`
  - [ ] 發布至 `orders:intent`（Stream）與 REST 直呼 S4（兩條路皆可，建議雙軌先保險）
- [ ] **冪等 & 鎖**
  - [ ] `client_order_id` 生成規則：`{svc}-btc-TS-rand`；Redis `SETNX idem:order:{id}=1 ttl=1h`
  - [ ] 同 symbol 單次決策：`lock:pos:{symbol}`（最大 3s），避免重覆 open
- [ ] **指標與告警**
  - [ ] `strategy.decide.latency_ms`、`rules.fired.count`、`gate.skip.count`
  - [ ] 告警：無規則可用、配置與本地 rev 不一致、決策產生但發布失敗
- [ ] **環境變數配置**
  - [ ] `S3_DB_ARANGO_URI`、`S3_DB_ARANGO_USER/PASS`
  - [ ] `S3_REDIS_ADDRESSES`（逗號分隔，Cluster 模式）
  - [ ] `S3_SYMBOLS`（預設：BTCUSDT，可多）
  - [ ] `S3_MAX_FUNDING_ABS`（例 0.0005）
  - [ ] `S3_SPREAD_BP_LIMIT_DEFAULT`、`S3_DEPTH_TOP1_USDT_MIN_DEFAULT`
  - [ ] `S3_LEVERAGE_DEFAULT=20`、`S3_MARGIN_BASE_USDT=20`
  - [ ] `S3_MAX_LOSS_USDT=1.2`、`S3_ATR_MULT=1.5`
  - [ ] `S3_CONCURRENT_ENTRIES_PER_MARKET=1`
  - [ ] `S3_CONFIG_POLL_SEC=5`

#### 11. 路過的服務相關功能（基於路過的服務實作）
- [ ] **接 `feat:events:<symbol>` 或 `/decide`**
  - [ ] 讀：`prod:{kill_switch}`；`config_active.rev` & bundle；風險配額（Redis）；`funding:{next}:<symbol>`；健康 `prod:{health}:system:state`
  - [ ] 守門 L0：KillSwitch、交易時窗、保證金/併發（見 S3 風險鍵）
  - [ ] L1 規則 DSL：按 `priority` 合成 `skip_entry/size_mult/tp_mult/sl_mult/max_adds_override`
  - [ ] L2 模型：超時回退；映射 `size_mult`
  - [ ] 產決策：`Decision{action=open|skip, size_mult,…, reason}`
  - [ ] 寫 DB：`signals.decision`（含 `model_p`、`reason`、`config_rev`）
  - [ ] 發事件：`sig:events`（決策快照）
  - [ ] 若 `open`：組 `OrderIntent{market=FUT|SPOT,…,intent_id}` → 呼 S4 `/orders`
- [ ] **風險鍵（Redis；原子）**
  - [ ] `risk:{budget}:fut_margin:inuse`（USDT 加總）；`risk:{concurrency}:<symbol>`（併發數）
  - [ ] 通過→暫占；失敗→`decision.skip(reason=RISK_BUDGET)`

#### 12. 字段校驗相關功能（基於字段校驗表實作）
- [ ] **Intent（下單意圖）字段校驗**
  - [ ] `intent_id`：必填，UUID/字串長度 1–128，作為冪等鍵全局唯一
  - [ ] `market`：必填，枚舉 {FUT, SPOT}，決定可用欄位
  - [ ] `symbol`：必填，正則 `^[A-Z0-9]{3,}$`，必須存在於 instrument_registry 且 ENABLED
  - [ ] `side`：必填，枚舉 {BUY, SELL}
  - [ ] `qty`：必填，> 0，步長 = instrument.stepSize，FUT 口數換算，SPOT 金額 ≥ minNotional
  - [ ] `type`：可選，枚舉 {MARKET, LIMIT, STOP_MARKET}，STOP_* 需搭配 stop_price
  - [ ] `price`：可選，> 0，tick = instrument.tickSize，僅 LIMIT/TP leg
  - [ ] `stop_price`：可選，> 0，僅 STOP_MARKET/SL leg
  - [ ] `working_type`：可選，枚舉 {MARK_PRICE, CONTRACT_PRICE}，預設 MARK_PRICE
  - [ ] `reduce_only`：可選，預設 false，FUT 平倉/SL/TP 務必 true
  - [ ] `leverage`：條件必填（FUT），1–125，預設 20
  - [ ] `isolated`：條件必填（FUT），預設 true
  - [ ] `exec_policy`：可選，枚舉 {MakerThenTaker, Market, OCO, LimitOnly}
  - [ ] `post_only_wait_ms`：可選，0–10000，預設 3000
  - [ ] `twap.enabled`：可選，預設 false
  - [ ] `twap.slices`：條件必填（twap），1–10，預設 3
  - [ ] `twap.interval_ms`：條件必填（twap），200–5000，預設 800
  - [ ] `oco.tp_price`：條件必填（exec=OCO），> 0，需與 side 合理
  - [ ] `oco.sl_price`：條件必填（exec=OCO），> 0，BUY → sl < entry；SELL 反向
  - [ ] `oco.leg_time_in_force`：可選，枚舉 {GTC, IOC}，預設 GTC
  - [ ] `client_tags`：可選，每個長度 ≤ 32，最多 10
- [ ] **POST /decide 字段校驗**
  - [ ] `symbol`：必填，正則 `^[A-Z0-9]{3,}$` 驗證
  - [ ] `config_rev`：可選，CURRENT 或整數驗證
  - [ ] `dry_run`：可選布爾值，預設 false
  - [ ] `sideHint`：可選，枚舉 {BUY, SELL}
  - [ ] `context`：可選，額外上下文信息
- [ ] **錯誤處理校驗**
  - [ ] 400 Bad Request：參數格式錯誤、範圍超界
  - [ ] 422 Unprocessable Entity：業務規則違反、數據不完整
  - [ ] 冪等性：相同 `intent_id` 返回相同結果
- [ ] **契約測試**
  - [ ] dry_run=true：返回 decision + intent，不落單
  - [ ] config_rev=CURRENT 與顯式 rev 結果一致
  - [ ] 特徵缺失 → 422 錯誤
  - [ ] 價格關係：BUY 時 tp > entry > sl；SELL 時反向
  - [ ] 訂單類型：STOP_MARKET 需要 stop_price

#### 13. 功能對照補記相關功能（基於功能對照補記實作）
- [ ] **20× 逐倉、保證金 20 USDT、淨利 ≥10% 離場**
  - [ ] 開倉：逐倉，`initial_margin`=20；可用槓桿 $L=20$
  - [ ] S6 監控 $ROE_{net} \ge 0.10$ 觸發平倉（`reduceOnly` 市價）
  - [ ] 公式：$ROE_{net}=\frac{(P_{\text{exit}}-P_{\text{entry}}) \cdot Q \cdot dir - \sum \text{Fees} - \sum \text{Funding}}{\text{InitialMargin}} \ge 0.10$
- [ ] **規則 DSL／Lint／Dry-run／熱載**
  - [ ] Lint：欄位、白名單、值域；依賴因子存在性；合規邊界
  - [ ] Dry-run：對近 $N$ 天 `signals` 重放，產出 `skip_rate`/`size_mult>1` 比率/policy shift Jaccard 等
  - [ ] Promote：守門閾值通過才切 `config_active.rev`；Redis 廣播，S3 熱載（RCU）
  - [ ] 策略位移 Jaccard：$J=\frac{|D_{\text{new}}\cap D_{\text{ref}}|}{|D_{\text{new}}\cup D_{\text{ref}}|}$
- [ ] **置信度模型（推論 + 回退）**
  - [ ] S3 對 L1 合格樣本送模型，得 $p=\Pr(\text{success}|X)$
  - [ ] 對應 `size` 多段映射：$p>0.85 \Rightarrow \times1.2$、$0.6-0.85 \Rightarrow \times1.0$、$0.4-0.6 \Rightarrow \times0.5$、$<0.4 \Rightarrow \text{skip}$
  - [ ] 超時/失敗：降級採預設 $\times1.0$（或 DSL 指定）
  - [ ] Logit 範例：$p=\sigma(\beta_0+\sum \beta_i X_i),\ \sigma(z)=\frac{1}{1+e^{-z}}$
- [ ] **資金費率守門 + 記帳**
  - [ ] S3：$|\hat f_{next}| > f_{\max} \Rightarrow \text{skip}$
  - [ ] S7：對每次 funding 記錄 $\text{funding} = \text{notional} \cdot f$（方向與交易所規格一致），累加入 $ROI_{net}$
- [ ] **風險預算/併發守門**
  - [ ] 進場前檢查：總保證金占用 $\le$ `risk.budget.fut_margin_usdt_max`、同市場併發 $\le$ `concurrent_entries_per_market`
  - [ ] 透過 Redis 原子遞增/遞減，關閉時釋放額度

#### 14. 全服務一覽相關功能（基於全服務一覽實作）
- [ ] **接 `feat:events:<symbol>` 或 `/decide`**
  - [ ] 讀：`prod:{kill_switch}`；`config_active.rev` & bundle；風險配額（Redis）；`funding:{next}:<symbol>`；健康 `prod:{health}:system:state`
  - [ ] L0 守門：KillSwitch、交易時窗、保證金/併發（原子配額鍵）
  - [ ] L1 規則 DSL：按 `priority` 合成 `skip_entry/size_mult/tp_mult/sl_mult/max_adds_override`
  - [ ] L2 模型：推論（超時回退）；映射 `size_mult`
  - [ ] 產決策：`Decision{action=open|skip, size_mult,…, reason}`
  - [ ] 寫 DB：`signals.decision`（含 `model_p`、`reason`、`config_rev`）
  - [ ] 發事件：`sig:events`（決策快照）
  - [ ] 若 open：組 `OrderIntent{market=FUT|SPOT,…,intent_id}` → 呼 S4 `/orders`
- [ ] **風險鍵（Redis；原子）**
  - [ ] `risk:{budget}:fut_margin:inuse`（USDT 加總）；`risk:{concurrency}:<symbol>`（併發數）
  - [ ] 通過→暫占；失敗→`decision.skip(reason=RISK_BUDGET)`

#### 15. Integration 附錄相關功能（基於 Integration 附錄實作）
- [ ] **FUT 入場決策流程**
  - [ ] 信號觸發與決策：接收 S2 的 `signals:new` 事件，基於規則 DSL 和置信度生成交易決策
  - [ ] 狀態記錄：將決策結果記錄到 `strategy_events` 表，狀態設為 `PENDING_ENTRY`
  - [ ] 下單請求生成：生成包含 `intent_id`、`market`、`symbol`、`side`、`qty`、`exec_policy` 的完整下單請求
  - [ ] TWAP 配置：支援 `twap.enabled`、`twap.slices`、`twap.interval_ms` 配置
  - [ ] 槓桿配置：支援 `leverage`、`isolated` 配置
- [ ] **SPOT 入場決策流程**
  - [ ] OCO 訂單配置：生成包含 `exec_policy: "OCO"`、`oco.tp_price`、`oco.sl_price`、`oco.limit_price` 的 OCO 訂單
  - [ ] OCO 失敗回退：檢測 OCO 不支援或掛單失敗，自動回退到守護停損機制
  - [ ] 守護停損配置：為守護停損生成相應的配置參數
- [ ] **事務一致性保證**
  - [ ] 冪等性保證：使用 `intent_id` 作為冪等鍵確保重複請求的安全性
  - [ ] 狀態機管理：決策狀態 PENDING → DECIDED → SENT → CONFIRMED
  - [ ] 失敗恢復：系統崩潰後能夠通過 `intent_id` 查詢決策狀態
- [ ] **風險控制機制**
  - [ ] KillSwitch 檢查：決策前檢查 `prod:{kill_switch}` 狀態
  - [ ] 風險預算檢查：檢查總保證金占用和併發限制
  - [ ] 資金費率守門：檢查 `funding:{next}:<symbol>` 費率是否超過限制

#### 16. Hop-by-Hop 執行規格相關功能（基於 Hop-by-Hop 執行規格補遺實作）
- [ ] **接 `feat:events:<symbol>` 或 `/decide`**
  - [ ] 讀：`prod:{kill_switch}`；`config_active.rev` & bundle；風險配額（Redis）；`funding:{next}:<symbol>`；健康 `prod:{health}:system:state`
  - [ ] L0 守門：KillSwitch、交易時窗、保證金/併發（原子配額鍵）
  - [ ] L1 規則 DSL：按 `priority` 合成 `skip_entry/size_mult/tp_mult/sl_mult/max_adds_override`
  - [ ] L2 模型：推論（超時回退）；映射 `size_mult`
  - [ ] 產決策：`Decision{action=open|skip, size_mult,…, reason}`
  - [ ] 寫 DB：`signals.decision`（含 `model_p`、`reason`、`config_rev`）
  - [ ] 發事件：`sig:events`（決策快照）
  - [ ] 若 open：組 `OrderIntent{market=FUT|SPOT,…,intent_id}` → 呼 S4 `/orders`
- [ ] **風險鍵（Redis；原子）**
  - [ ] `risk:{budget}:fut_margin:inuse`（USDT 加總）；`risk:{concurrency}:<symbol>`（併發數）
  - [ ] 通過→暫占；失敗→`decision.skip(reason=RISK_BUDGET)`

#### 17. 功能規格書相關功能（基於功能規格書實作）
- [ ] **入向（被呼叫）API**
  - [ ] `GET /health`（所有服務）→ `HealthResponse{Status,Checks,...}`
  - [ ] `POST /decide`（一般由自身流程觸發）→ `DecideResponse{Decision, []OrderIntent}`
- [ ] **出向（主以事件）**
  - [ ] `POST /orders` → S4（執行 intents）
  - [ ] 記 signals、strategy_events
- [ ] **產生決策 → 下 FUT 期貨入場**
  - [ ] 觸發：S3 收到 S2 特徵與守門通過
  - [ ] S3 `POST /decide`（自內部流程，實作上可直接呼叫引擎模組）→ `DecideResponse`（含 Intents，market=FUT）
  - [ ] S3 → S4 `POST /orders`（`OrderCmdRequest.Intent`）→ `OrderResult`
  - [ ] S6 監控到新倉（來自交易所/DB）後，若需掛 STOP_MARKET：S6 → S4 `POST /orders`（SL/TP/ReduceOnly）
  - [ ] 冪等：intent_id 作為冪等鍵；S4 對 5xx/429 重試（同一鍵）
  - [ ] 失敗補償：下單逾時不確定：重送同 intent_id；若交易所有單→回傳既有 OrderID
- [ ] **產生決策 → 下 SPOT 現貨（含 OCO 或守護停損）**
  - [ ] 觸發：S3 決策 market=SPOT
  - [ ] S3 → S4 `POST /orders`，`ExecPolicy.OCO` 或 `GuardStopEnable=true`
  - [ ] S4 成交回傳 `GuardStopArmed`（如有本地守護）
  - [ ] 失敗補償：OCO 一腿掛失敗：S4 回 status=PARTIAL 並附訊息；S6 或 S3 依「OCO 補掛策略」再次 `POST /orders`
- [ ] **冪等性與重試**
  - [ ] 下單/撤單：`OrderCmdRequest.Intent.IntentID` / `CancelRequest.ClientID` 必填作冪等鍵
  - [ ] S4 對 5xx/429 採固定+抖動退避

### 🎯 實作優先順序
1. **高優先級**：Redis Stream 整合和風險鍵管理
2. **中優先級**：配置熱載和優化
3. **低優先級**：風險管理優化

### 📊 相關資料寫入
- **DB Collections**：`signals.decision`
- **Redis Key/Stream**：`sig:events`、`risk:{budget}*`、`risk:{concurrency}*`

## 概述

S3 Strategy Engine 是 Project Chimera 的策略引擎服務，負責執行交易策略的核心邏輯，包括 L0 守門、L1 規則 DSL、L2 置信度模型，最終產生下單意圖。

## 功能特性

### 1. L0 守門（Gate Keeper）
- **資金費檢查**：檢查資金費率是否超過上限
- **流動性檢查**：檢查價差和深度是否滿足交易條件
- **風險預算檢查**：檢查現貨名義金額和期貨保證金限制
- **併發控制**：限制同一市場的同時入場數量

### 2. L1 規則引擎（Rule Engine）
- **DSL 規則解析**：支持複雜的條件組合和動作定義
- **規則優先級**：支持規則優先級和衝突解決
- **動態規則加載**：支持熱更新規則配置
- **規則命中追蹤**：記錄觸發的規則和相應動作

### 3. L2 機器學習模型（ML Model）
- **置信度評分**：基於特徵計算交易置信度
- **倉位倍率調整**：根據 ML 分數動態調整倉位大小
- **模型版本管理**：支持多版本模型並行運行
- **特徵重要性分析**：分析各特徵對決策的影響

### 4. 配置管理（Config Manager）
- **RCU 熱載**：讀取複製更新模式的配置熱載
- **版本一致性**：確保配置版本的一致性
- **配置快取**：內存快取提高配置訪問效率
- **配置監聽**：實時監聽配置變更事件

## API 端點

### 健康檢查
- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 策略決策
- `POST /decide` - 執行策略決策，生成交易意圖

## 決策流程

### 1. 請求驗證
- 驗證請求格式和必填字段
- 檢查特徵數據完整性
- 驗證配置版本有效性

### 2. L0 守門檢查
```
資金費檢查: |funding_next| <= max_funding_abs
流動性檢查: spread_bps <= spread_bp_limit && depth_top1_usdt >= min
風險預算檢查: Σ spot_notional <= spot_quote_usdt_max
```

### 3. L1 規則評估
- 遍歷所有啟用的規則
- 評估規則條件是否滿足
- 累積規則動作（size_mult, tp_mult, sl_mult）
- 應用白名單限制

### 4. L2 ML 評分
- 基於特徵計算 ML 分數
- 計算置信度
- 確定倉位倍率調整

### 5. 結果合併
- 合併規則和 ML 結果
- 生成最終決策
- 創建訂單意圖（如需要）

## 數學計算

### 倉位計算（FUT）
```
margin_base = 20 USDT
margin = margin_base × size_mult
notional = margin × leverage
qty = round_to_step(notional / price, stepSize)
```

### 停損距離計算
```
d_atr = ATR_mult × ATR
d_losscap = max_loss_usdt / qty
d = min(d_atr, d_losscap)
```

### SL/TP 價格計算
```
多單: SL = entry - d, TP = entry + d × tp_mult
空單: SL = entry + d, TP = entry - d × tp_mult
```

### ML 分數到倉位倍率映射
```
score > 0.85 → size_mult = 1.2
score 0.6-0.85 → size_mult = 1.0
score 0.4-0.6 → size_mult = 0.5
score < 0.4 → skip
```

## 規則 DSL 語法

### 條件語法
```json
{
  "allOf": [
    {"f": "rv_pctile_30d", "op": "<", "v": 0.25},
    {"f": "rho_usdttwd_14", "op": "<", "v": -0.3}
  ]
}
```

### 動作語法
```json
{
  "size_mult": 1.2,
  "tp_mult": 2.0,
  "sl_mult": 0.5
}
```

### 支持的運算符
- `<`, `>`, `<=`, `>=`, `==`: 數值比較
- `allOf`: 所有條件必須滿足
- `anyOf`: 任一條件滿足即可
- `not`: 條件取反

## 配置參數

### 守門參數
- `maxFundingAbs`: 0.0005 - 最大資金費絕對值
- `spreadBpLimit`: 3.0 - 價差限制（bps）
- `depthTop1UsdtMin`: 200.0 - 最小深度（USDT）
- `spotQuoteUsdtMax`: 10000.0 - 現貨最大名義金額
- `futMarginUsdtMax`: 5000.0 - 期貨最大保證金

### ML 模型參數
- `modelName`: "default_model" - 模型名稱
- `version`: "v1.0" - 模型版本
- `confidenceThreshold`: 0.6 - 置信度閾值

## 數據流

### 輸入數據
- **特徵數據**: 來自 S2 Feature Generator
- **市場數據**: 來自 S1 Exchange Connectors
- **配置數據**: 來自 S10 Config Service

### 輸出數據
- **交易信號**: 保存到 ArangoDB signals collection
- **訂單意圖**: 發送到 S4 Order Router
- **Redis Streams**: `orders:intent` 訂單意圖流

## 性能特性

### 決策延遲
- **P50**: ≤ 200ms
- **P95**: ≤ 500ms
- **目標**: 1000 筆模擬特徵決策

### 並發處理
- **信號處理**: 支持高並發信號處理
- **規則評估**: 並行規則條件評估
- **ML 推論**: 異步 ML 模型推論

## 錯誤處理

### 守門失敗
- 記錄失敗原因
- 返回 skip 決策
- 不生成訂單意圖

### 規則評估錯誤
- 跳過錯誤規則
- 繼續評估其他規則
- 記錄錯誤日誌

### ML 模型錯誤
- 使用默認分數
- 降級到規則引擎
- 發送告警通知

## 監控指標

### 服務健康指標
- Redis 連接延遲
- ArangoDB 連接延遲
- 配置加載狀態

### 業務指標
- 決策延遲（P50/P95）
- 規則命中率
- ML 模型準確率
- 守門通過率

## 部署說明

### Docker 部署
```bash
docker build -t s3-strategy .
docker run -p 8083:8083 s3-strategy
```

### 環境要求
- Go 1.19+
- Redis Cluster
- ArangoDB
- 足夠的 CPU（ML 推論）

## 開發指南

### 添加新規則
1. 定義規則條件和動作
2. 在 `loadStrategyRules` 中註冊
3. 測試規則邏輯

### 添加新特徵
1. 更新 `DecideRequest` 結構
2. 修改守門和規則邏輯
3. 更新 ML 模型輸入

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
1. **決策延遲過高**
   - 檢查 Redis 連接狀態
   - 確認規則數量是否過多
   - 檢查 ML 模型性能

2. **規則不生效**
   - 檢查規則是否啟用
   - 確認條件語法正確
   - 查看規則命中日誌

3. **ML 模型錯誤**
   - 檢查模型文件是否存在
   - 確認特徵數據格式
   - 查看模型推論日誌

## 版本歷史

### v1.0.0
- 初始版本
- 實現 L0 守門、L1 規則引擎、L2 ML 模型
- 支持 FUT/SPOT 市場決策
- 實現配置熱載和規則管理