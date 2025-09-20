# Project Chimera — Hop-by-Hop 執行規格補遺（純 MD）

**實作進度：0/12 服務已完成 (0%)**

## 0) 記號 & 通用規則

* **時間**：epoch ms；**金額**：USDT；**比例**小數（0.1=10%）。
* **配置**：所有業務讀 `config_active.rev`；收到 `cfg:events` 後以 RCU 熱載。
* **Redis（Cluster）**：使用**哈希標籤**固定分片，例如：`risk:{budget}:fut_margin:inuse`、`lock:{pos}:<pos_id>`。
* **冪等鍵**：`intent_id`（下單/分片）、`client_order_id`（交易所層）、`signal_id`、`(signal_id,horizon)`（標籤）、`transfer_id`（劃轉）、`trade_id`（復盤）。
* **鎖**：`lock:{pos}:<pos_id>`、`lock:{treasury}:<from>:<to>`、`lock:{router}:param:update` 等（TTL 必設）。

---

## A) Per-Service「到站就做」清單

### S1 — Exchange Connectors（行情/帳戶/劃轉）❌ **[未實作]**

**事件來源 → 到 S1 時要做**

1. **WS 行情/深度/Ticker/Funding 更新**

   * **讀**：直連交易所（無預讀）。
   * **算**：清洗/時間對齊；拼 `mid`、`spread_bp`；必要去抖/節流。
   * **寫 Redis Streams**：

     * `mkt:events:{spot}:<SYMBOL>`（現貨）
     * `mkt:events:{perp}:<SYMBOL>`（永續）
     * `mkt:events:{funding}:<SYMBOL>`（下一期/實際 funding）
   * **寫 DB**（僅 funding 實收）：`funding_records`（`symbol,funding_time,rate,amount_usdt`）
   * **指標**：`metrics:events:s1.ws_rtt`、`metrics:events:s1.mkt_throughput`

2. **POST /xchg/treasury/transfer（內部）**

   * **驗**：Idempotency-Key / `transfer_id`；限額/白名單。
   * **執行**：呼交易所劃轉 API → 判定成功/失敗。
   * **寫 DB**：`treasury_transfers`（狀態流轉）。
   * **發事件**：`ops:events`（審計）。
   * **回**：`TransferResponse{TransferID,Result,Message}`。

---

### S2 — Feature Generator（特徵/Regime）❌ **[未實作]**

**到 S2 時要做**

1. **消費 `mkt:events:*`**

   * **讀**：滑窗快取 `feat:{cache}:<symbol>`、`config_active.rev`。
   * **算**：ATR/RV/ρ/Spread/Depth 等；DQC 標記。
   * **寫 DB**：`signals`（新/補寫 `features`、`t0`、`config_rev`）。
   * **發事件**：`feat:events:<symbol>`（`signal_id,t0,symbol,features`）。

2. **每日 Regime（排程）**

   * **算**：RV 百分位 → Regime（FROZEN/NORMAL/EXTREME）。
   * **寫 Redis KV**：`prod:{regime}:market:state`（帶 `rev` 和過期戳）。
   * **指標**：`metrics:events:s2.regime_latency`。

3. **POST /features/recompute**

   * **讀**：期間 K 線/深度（資料湖/交易所）。
   * **算**：補算特徵。
   * **寫 DB**：回補 `signals.features`；`strategy_events(kind=FEATURE_RECOMPUTE)`。

---

### S3 — Strategy Engine（守門/規則/模型/意圖）❌ **[未實作]**

**到 S3 時要做**

1. **接 `feat:events:<symbol>` 或 `/decide`**

   * **讀**：`prod:{kill_switch}`；`config_active.rev` & bundle；風險配額（Redis）；`funding:{next}:<symbol>`；健康 `prod:{health}:system:state`。
   * **L0 守門**：KillSwitch、交易時窗、保證金/併發（原子配額鍵）。
   * **L1 規則 DSL**：按 `priority` 合成 `skip_entry/size_mult/tp_mult/sl_mult/max_adds_override`。
   * **L2 模型**：推論（超時回退）；映射 `size_mult`。
   * **產決策**：`Decision{action=open|skip, size_mult,…, reason}`。
   * **寫 DB**：`signals.decision`（含 `model_p`、`reason`、`config_rev`）。
   * **發事件**：`sig:events`（決策快照）。
   * **若 open**：組 `OrderIntent{market=FUT|SPOT,…,intent_id}` → 呼 S4 `/orders`。

2. **風險鍵（Redis；原子）**

   * `risk:{budget}:fut_margin:inuse`（USDT 加總）；`risk:{concurrency}:<symbol>`（併發數）。
   * 通過→暫占；失敗→`decision.skip(reason=RISK_BUDGET)`。

---

### S4 — Order Router（路由/TWAP/OCO/撤單）❌ **[未實作]**

**到 S4 時要做**

1. **POST /orders（含 FUT / SPOT / TWAP / OCO / GuardStop）**

   * **驗**：`intent_id` 冪等；KillSwitch（新倉禁）。
   * **讀**：路由參數 `router:{param}:curves`；最新價/深度（Redis 快照）。
   * **決策**：Maker→Taker 或 TWAP；SPOT 是否原生 OCO；需則啟動守護停損監控。
   * **下單**：產 `client_order_id`；REST/WS 下發。
   * **寫 DB**：`orders`（NEW/部分/FILLED）；`fills`（含 `mid_at_send/top3/slippage_bps`）。
   * **寫 Redis**：

     * SPOT 守護：`guard:{stop}:<symbol>:<intent_id>`（armed/armed\_at）。
     * TWAP 佇列：`prod:{exec}:twap:queue`（ZSet；`score=due_ts`）。
     * 成交流：`ord:{results}`（Stream；匯總給 S6/S5）。
   * **回**：`OrderResult{status, order_id, filled_avg, …}`。

2. **POST /cancel**

   * **驗**：冪等；讀現況。
   * **執行**：撤單；必要時升級為市價騰挪。
   * **寫 DB**：`orders(status=CANCELED)`；`strategy_events(kind=CANCEL)`。
   * **發**：`ord:{results}`（撤單回報）。

3. **TWAP tick（排程）**

   * **取** ZSet 到期任務 → 依序切片下單 → 未完再入列。
   * **指標**：`metrics:events:s4.router_p95`、`metrics:events:s4.maker_timeout_count`。

---

### S5 — Reconciler（對帳/孤兒處置）❌ **[未實作]**

**到 S5 時要做**

1. **POST /reconcile（ALL|ORDERS|POSITIONS）**

   * **拉**：交易所 `openOrders/positionRisk`；DB `orders/positions_snapshots`。
   * **算**：集合差異（Jaccard、一致率）。
   * **處置**：孤兒掛單→S4 `/cancel`；數量不符→以交易所為準修 DB 或進保守降風險（小額市價）。
   * **寫 DB**：`strategy_events(kind=RECONCILE_*)`；同步 `orders/positions_snapshots`。
   * **寫 Redis**：`recon:{last_run_ts}`；嚴重時 `alerts(ERROR)`、健康降級信號。

---

### S6 — Position Manager（停損/止盈/加倉/資金）❌ **[未實作]**

**到 S6 時要做**

1. **POST /positions/manage** 或 **管理 tick（排程）**

   * **讀**：最新價、ATR、Regime；`pos` 當前 SL/TP 階；加倉上限；健康度。
   * **算**：ROE、強平距離；是否上升鎖利階；是否命中止盈；是否加倉。
   * **行動**：

     * **鎖利**：`/cancel` 舊 SL → `/orders` 新 SL（reduceOnly）。
     * **止盈**：`/orders` 市價 reduceOnly（分批）。
     * **加倉**：`/orders` 新單；更新 `add_on_count`。
   * **寫 DB**：`positions_snapshots`（新快照）；（`orders/fills` 由 S4 回報）。
   * **寫 Redis**：`pos:{sl}:level:<pos_id>`、`pos:{tp}:ladder:<pos_id>`、`pos:{adds}:<pos_id>`。
   * **釋放配額**：倉位關閉時 `risk:{budget}`/`risk:{concurrency}` 反向調整。

2. **自動資金劃轉（排程）**

   * **讀**：SPOT/FUT 可用餘額。
   * **算**：`need=max(0,min_free_fut-free_fut)`。
   * **流程**：足額→造 `transfer_id`，寫 DB `treasury_transfers(PENDING)` → 請 S12 審批 → S1 執行（若啟用全自動，可直連 S1）。

---

### S7 — Label Backfill（12/24/36h）❌ **[未實作]**

**到 S7 時要做**

1. **POST /labels/backfill?h=…**

   * **查**：`signals` 滿足 `t0+H<=now` & 無 `labels_H`。
   * **聚合**：該交易窗口 `fills`、`funding_records`、費用。
   * **算**：`ROI_net(H)`、`label`。
   * **寫 DB**：`labels_12h/24h/36h`（Upsert）；`strategy_events(kind=LABEL_WRITE)`。
   * **發**：`labels:{ready}`（可選 Stream；觸發 Autopsy）。

---

### S8 — Autopsy Generator（復盤）❌ **[未實作]**

**到 S8 時要做**

1. **POST /autopsy/{trade\_id}`** 或 **監聽 `pos\:events(EXIT|STAGNATED)\`**

   * **拉**：`signals/orders/fills/positions_snapshots/funding_records/labels_*`。
   * **算**：ROE 曲線、TCA/滑價、Peer 分位、反事實、敘事摘要。
   * **寫 DB**：`autopsy_reports{trade_id,...}`；物件存 MinIO `autopsy/<trade_id>.html|pdf`。
   * **發**：`strategy_events(kind=AUTOPSY_DONE)`；指標 `metrics:events:s8.autopsy_latency`。

---

### S9 — Hypothesis Orchestrator（實驗/回測）❌ **[未實作]**

**到 S9 時要做**

1. **POST /experiments/run**

   * **讀**：`hypotheses` 選 PENDING；配置樣本窗口/Walk-Forward。
   * **跑**：回測引擎（可離線批）→ KPI/檢定/FDR。
   * **寫 DB**：`experiments`（結果）、`hypotheses(status=CONFIRMED|REJECTED)`。
   * **發**：`ops:events`（通知/審計）。

---

### S10 — Config Service（Lint/模擬/敏感度/Promote）❌ **[未實作]**

**到 S10 時要做**

1. **POST /bundles** → **Lint** → **Dry-run**（近 N 天 `signals` 重放）。
2. **POST /simulate** → 差異估算 + **敏感度**（±ε 擾動）。
3. **POST /promote** → **寫 DB**：`promotions`、切 `config_active`、**發** `cfg:events`。
4. **GET /active** → 回 `bundle_id,rev`（供各服務啟動/熱載）。

---

### S11 — Metrics & Health（彙整/守門）❌ **[未實作]**

**到 S11 時要做**

1. **收 `metrics:events:*` & 拉關鍵指標**

   * **聚合**：寫 DB `metrics_timeseries/strategy_metrics_daily`。
   * **判等級**：綜合 `maker_fill_ratio / ib_rate / AUC / Brier / router_p95 / stream_lag`。
   * **寫 Redis**：`prod:{health}:system:state=GREEN|YELLOW|ORANGE|RED`。
   * **發告警**：`alerts`（DB + 通知）。

2. **GET /metrics, /alerts** → 提供前端面板。

---

### S12 — Web UI / API GW（代理/RBAC/Kill-switch）❌ **[未實作]**

**到 S12 時要做**

1. **代理後端 API**：驗票/RBAC → 轉發 → 回傳。
2. **POST /kill-switch**：設 `prod:{kill_switch}=ON`（TTL）；發 `ops:events`；各核心服務讀此旗標拒新倉。
3. **POST /treasury/transfer**：建立/審批劃轉請求 → 內呼 S1。

---

## B) 代表性流程：逐站行為 + 入庫/入 Redis 明細

> 表頭：**Step | Service | Input | Validate/Read | Compute | Write DB | Write Redis | Next**

### B1) FUT 入場（含 SL/TP 掛單）❌ **[未實作]**

1. **S3** | `feat:events` or `/decide` | `config_active, kill_switch, risk:*` | L0/L1/L2 → `open` & `OrderIntent(FUT,intent_id)` | `signals.decision` | `sig:events` | → **S4**
2. **S4** | `POST /orders`（市價） | 冪等/路由參數 | 下市價，收 `FILL` 聚合均價 | `orders(FILLED), fills` | `ord:{results}` | → **S6**
3. **S6** | `ord:{results}` or 管理 tick | ATR/Regime/配置 | 算 SL/TP 價 | — | — | 呼 **S4** 掛 `STOP_MARKET/TP`（reduceOnly）
4. **S4** | `POST /orders`（兩腿） | 冪等 | 掛單 | `orders(NEW)` | `ord:{results}`, `strategy_events(TP_SL_PLACED)` | → 完成

### B2) SPOT 入場（OCO 或 GuardStop）❌ **[未實作]**

1. **S3** | 決策 → `OrderIntent(SPOT, exec=OCO|GUARD)` | 同上 | 寫 `signals.decision` | — | `sig:{events}` | → **S4**
2. **S4(OCO)** | `POST /orders` | 檢 OCO 支援 | 原生 OCO 下單 | `orders/`子腿 `fills` | `ord:{results}` | → 完成
3. **S4(Guard)** | `POST /orders` `LIMIT_MAKER` | — | 下限價；監控觸價→先撤再守護/反手 | `orders/`守護狀態 | `guard:{stop}:<symbol>:<intent_id>` | → 完成

### B3) Trailing Stop / 分批止盈 / 加倉 ❌ **[未實作]**

1. **S6** | 管理 tick | 價/ATR/Regime/持倉狀態 | 升階/TP/加倉判斷 | `positions_snapshots` | `pos:{sl}/{tp}/{adds}` | → **S4**
2. **S4** | `/cancel` 舊 SL → `/orders` 新 SL / TP / 加倉 | 冪等 | 執行 | `orders/fills` | `ord:{results}` | → **S6** 更新/重評

### B4) 對帳處置 ❌ **[未實作]**

1. **S12** | `POST /reconcile` | RBAC | — | — | — | — | → **S5**
2. **S5** | 對帳 | 交易所/DB | 集合集合差異 & 策略 | 修 `orders/positions`、`strategy_events` | `alerts`, `recon:{last_run_ts}` | → 完成

### B5) 標籤回填 & 復盤 ❌ **[未實作]**

1. **S7** | `/labels/backfill?h=24` | — | 聚合 PnL/費用/Funding → `ROI_net`/label | `labels_24h` | `labels:{ready}`（可選） | → **S8**
2. **S8** | `/autopsy/{trade_id}` or 監聽 `pos:{events}` | — | TCA/Peer/反事實/敘事 | `autopsy_reports` + MinIO | `strategy_events(AUTOPSY_DONE)` | → 完成

### B6) 配置模擬 + 敏感度 + 推廣 ❌ **[未實作]**

1. **S12→S10** | `/bundles` | Lint/Dry-run | — | `config_bundles` | — | → **S10**
2. **S10** | `/simulate` | — | 重放差異 & ±ε 敏感度 | `simulations` | — | → **S12**
3. **S12→S10** | `/promote` | 守門/Canary/Ramp | 切 `config_active`、記 `promotions` | `config_active/promotions` | `cfg:{events}` | → 各服務熱載

### B7) 風險預算/併發守門 ❌ **[未實作]**

1. **S3** | 進場前 | 原子 `INCR/INCRBY` | 檢剩餘額度 | 若不足：`signals.decision.skip=RISK_BUDGET` | — | → 否決或放行
2. **S6/S5** | 關閉倉位 | — | 釋放額度 `DECR/DECRBY` | — | — | → 完成

### B8) 金庫劃轉（自動/人工）❌ **[未實作]**

1. **S6** | 餘額不足判定 | — | 算 `need` → 造 `transfer_id` | `treasury_transfers(PENDING)` | `lock:{treasury}` | → **S12**
2. **S12→S1** | 審批→執行 | 驗/呼交易所→結果 | 更新 `treasury_transfers` 狀態 | — | `ops:{events}` | → 完成

### B9) 健康降級 → 策略行為調參 ❌ **[未實作]**

1. **S11** | 彙總 | — | 合成健康等級 | `metrics_timeseries` | `prod:{health}:system:state` | → 全服務
2. **S3/S4/S6** | 讀健康等級 | — | 套用矩陣（`size_mult`↓、`maker_wait`↑、市價比例↑） | — | — | → 影響後續決策/執行

### B10) 成本學習 → 路由參數再估 ❌ **[未實作]**

1. **S11** | 月批 | — | 擬合 `slip_bps = g(spread,depth,vol,regime)` | `experiments` 或 `config_bundles` 草稿 | `router:{param}:curves(draft)` | → **S10** 守門
2. **S10** | 模擬/Promote | — | — | `config_active` | `cfg:{events}` | → S4 熱載新參數

---

## C) 到站要寫哪裡（速查表）❌ **[全部未實作]**

| 服務  | **DB Collections（寫入時機）**                                                     | **Redis Key/Stream（寫入時機）**                                                  | 實作狀態 |
| --- | ---------------------------------------------------------------------------- | --------------------------------------------------------------------------- | ---- |
| S1  | `funding_records`（收到實收／結算時）；`treasury_transfers`（執行結果）                       | \`mkt\:events:{spot                                                         | perp | funding}:<sym>`（即時）；`ops\:events\`（審計） | ❌ **[未實作]** |
| S2  | `signals(features,t0,config_rev)`（新/補算）；`strategy_events(FEATURE_RECOMPUTE)` | `feat:events:<sym>`（新特徵）；`prod:{regime}:market:state`（每日）                   | ❌ **[未實作]** |
| S3  | `signals.decision`（每次決策）                                                     | `sig:events`（決策快照）；`risk:{budget}*`、`risk:{concurrency}*`（入場占用/釋放）          | ❌ **[未實作]** |
| S4  | `orders`（所有狀態）；`fills`（每筆成交流）；`strategy_events(TP_SL_PLACED/CANCEL)`         | `ord:{results}`（子/父單結果）；`prod:{exec}:twap:queue`（TWAP）；`guard:{stop}:*`（守護） | ❌ **[未實作]** |
| S5  | `strategy_events(RECONCILE_*)`；修 `orders/positions_snapshots`                | `recon:{last_run_ts}`；`alerts`（錯誤）                                          | ❌ **[未實作]** |
| S6  | `positions_snapshots`（每 tick/事件）                                             | `pos:{sl}:level:*`、`pos:{tp}:ladder:*`、`pos:{adds}:*`；釋放 `risk:{*}`         | ❌ **[未實作]** |
| S7  | `labels_12h/24h/36h`（Upsert）；`strategy_events(LABEL_WRITE)`                  | `labels:{ready}`（可選）；`labels:{last_backfill_ts}`                            | ❌ **[未實作]** |
| S8  | `autopsy_reports`（Upsert）                                                    | —（或 `ops:events` 通知）                                                        | ❌ **[未實作]** |
| S9  | `experiments`；`hypotheses(status)`                                           | `bt:{last_run_ts}`                                                          | ❌ **[未實作]** |
| S10 | `config_bundles`、`simulations`、`promotions`、`config_active`                  | `cfg:{events}`（推廣）                                                          | ❌ **[未實作]** |
| S11 | `metrics_timeseries`、`strategy_metrics_daily`、`alerts`                       | `prod:{health}:system:state`                                                | ❌ **[未實作]** |
| S12 | `treasury_transfers`（審批）                                                     | `prod:{kill_switch}`；`ops:events`                                           | ❌ **[未實作]** |

---

## 📊 實作進度總結

### ❌ 全部未實作 (0%)
- **S1-S12**：所有服務均未實作
- **B1-B10**：所有流程均未實作
- **C 速查表**：所有資料寫入均未實作

### 🎯 建議優先順序
1. **S1 Exchange Connectors** - 交易所通信基礎
2. **S2 Feature Generator** - 特徵計算引擎
3. **S3 Strategy Engine** - 決策邏輯核心
4. **S4 Order Router** - 訂單執行核心
5. **S6 Position Manager** - 持倉治理
6. **S5 Reconciler** - 對帳處置
7. **S12 API Gateway** - 統一入口
8. **S10 Config Service** - 配置管理
9. **S11 Metrics & Health** - 監控系統
10. **S7-S9** - 分析、研究服務

---
