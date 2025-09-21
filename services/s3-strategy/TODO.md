# Project Chimera — S3 Strategy Engine 補充說明（Spec Addendum v3） ❌ *未實作*

> 本補充文件整合並具體化你先前的 S3 README 草案，擴充「輸入/輸出契約、Redis Stream 與 Key、資料落地、時序與錯誤處理、監控與契約測試」等細節，供工程直接落地。

---

## 0) 服務定位與邊界

**職責**：消費特徵→經 L0/L1/L2 決策→產生 *FUT / SPOT* 訂單意圖→（雙軌）呼叫 S4 或寫入執行指令流；並回寫 `signals.decision` 與決策事件。

**不負責**：實際下單/撤單（S4）、倉位治理（S6）、對帳（S5）、配置治理（S10）。

---

## 1) 互動矩陣（HTTP × Streams × DB）

### 1.1 輸入

-〔Stream〕`feat:events:{SYMBOL}`：特徵事件（S2 產生）
-〔KV〕`prod:kill_switch`：全域停機旗標（S12 設）
-〔KV〕`prod:health:system:state`：GREEN/YELLOW/ORANGE/RED（S11 計）
-〔KV〕`risk:budget:fut_margin:inuse`、`risk:budget:spot_quote:inuse`：已用額度
-〔KV〕`risk:concurrency:{SYMBOL}`：該標的併發入場計數
-〔Hash/Doc〕`config_active` + `config_bundles/{bundle_id}`：規則/旗標/參數（S10）
-〔KV〕`funding:next:{SYMBOL}`：下一期資金費估計（S1/S2）

### 1.2 輸出

-〔HTTP〕`POST S4 /orders`：提交 `OrderCmdRequest`
-〔Stream（可選冗餘）〕`ord:cmd:{SYMBOL}`：下單指令流（若採雙軌）
-〔Stream〕`sig:events`：決策快照事件
-〔DB〕`signals`（含 `decision` 子文件）、`strategy_events`（DECIDE/ENTRY\_\*）

---

## 2) 決策流水線（L0→L1→L2）

1. **L0 硬性守門**

   * Kill Switch / 交易時窗
   * 市場品質：`spread_bps ≤ spread_bp_limit`、`depth_top1_usdt ≥ min`
   * 資金費門檻：`|funding_next| ≤ max_funding_abs`
   * 風險預算：`fut_margin_usdt_max`、`spot_quote_usdt_max`、`concurrent_entries_per_market`（Redis 原子暫占）

2. **L1 規則 DSL（白名單）**

   * 命中多條：`size_mult/tp_mult/sl_mult` 相乘→**clamp**
   * `skip_entry=true` → 短路否決

3. **L2 置信度模型**

   * 推論得機率 `p`；分段映射 `size_mult_ml`
   * 合併：`size_mult = clamp(size_mult_rules × size_mult_ml)`

4. **Sizing 與 風控**

   * FUT：`margin = 20 * size_mult`、`notional = margin * leverage`、`qty = round(notional/price, step)`
   * SL：`d = min(ATR_mult×ATR, max_loss_usdt/qty)`；多單 `SL = entry − d`；TP 以淨利≥10%反解
   * SPOT：`qty = floor(quote_budget/price, step)`；OCO（或守護停損）

5. **輸出意圖** → S4（REST 為主，Stream 為備）

---

## 3) API（入向）

### `GET /health`

* **200**：`{"status":"OK","redis": "...","arango":"...","config_rev":123,"rules_loaded":N}`
* **依賴**：Redis Cluster、ArangoDB、Config RCU、Streams 可寫

### `POST /decide`

**Request**

```json
{
  "symbol": "BTCUSDT",
  "sideHint": "BUY",
  "dry_run": false,
  "context": { "trigger": "manual|stream", "t0": 1730000000000 }
}
```

**Response**

```json
{
  "decision": "open|skip",
  "reason": "GATE|RULE|MODEL|RISK_BUDGET|OK",
  "config_rev": 129,
  "rules_fired": ["R-023","R-045"],
  "model_p": 0.67,
  "intent": {
    "intent_id": "s3-btc-1730000000000-x",
    "market": "FUT",
    "symbol": "BTCUSDT",
    "side": "BUY",
    "qty": 0.005,
    "exec_policy": "MakerThenTaker",
    "tp_px": 63100.5,
    "sl_px": 59800.0
  }
}
```

**錯誤**

* 400：參數錯誤 / 缺特徵
* 422：規則或風險違反
* 503：依賴不可用（降級）

---

## 4) Redis 契約

### 4.1 Keys（KV/ZSet/Hash）

| Key                            | 型別           | 說明                |         |        |       |
| ------------------------------ | ------------ | ----------------- | ------- | ------ | ----- |
| `prod:kill_switch`             | String       | \`"ON"            | "OFF"\` |        |       |
| `prod:health:system:state`     | String       | \`GREEN           | YELLOW  | ORANGE | RED\` |
| `risk:budget:fut_margin:inuse` | Float        | 期貨保證金已用           |         |        |       |
| `risk:budget:spot_quote:inuse` | Float        | 現貨名目已用            |         |        |       |
| `risk:concurrency:{SYMBOL}`    | Int          | 標的併發入場計數          |         |        |       |
| `feat:last:{SYMBOL}`           | Hash         | 最新特徵快照（S2 填）      |         |        |       |
| `funding:next:{SYMBOL}`        | Float        | 下一期資金費估計          |         |        |       |
| `cfg:rcu:rev`                  | Int          | 本地快取的 config\_rev |         |        |       |
| `idem:order:{INTENT_ID}`       | String (TTL) | 冪等鎖（S3→S4 期間）     |         |        |       |

### 4.2 Streams

| Stream                    | 欄位（示意）                                                        |
| ------------------------- | ------------------------------------------------------------- |
| `feat:events:{SYMBOL}`    | `{ts, mid, spread_bps, atr, depth_top1_usdt, ...}`            |
| `sig:events`              | `{ts, symbol, decision, rules_fired[], model_p, config_rev}`  |
| `ord:cmd:{SYMBOL}` *(可選)* | `{intent_id, market, side, qty, px?, sl_px?, tp_px?, policy}` |
| `cfg:events`              | `{rev, bundle_id, reason}`                                    |

---

## 5) 資料落地（ArangoDB）

### 5.1 `signals`（片段）

```json
{
  "signal_id": "SIG-1730000000000-BTC",
  "t0": 1730000000000,
  "symbol": "BTCUSDT",
  "features": { "atr":"...", "spread_bps": 3.1, "...": "..." },
  "decision": {
    "action": "open",
    "size_mult": 1.0,
    "tp_mult": 1.0,
    "sl_mult": 1.2,
    "reason": "OK",
    "model_p": 0.67
  },
  "config_rev": 129,
  "created_at": 1730000000001
}
```

### 5.2 `strategy_events`

```json
{
  "event_id": "EVT-1730000000100",
  "kind": "DECIDE",
  "symbol": "BTCUSDT",
  "detail": { "intent_id":"...", "decision":"open", "rules":["R-023"] },
  "ts": 1730000000100
}
```

**索引建議**

* `signals`: Hash(`signal_id`), Skiplist(`t0`,`symbol`)
* `strategy_events`: Skiplist(`ts`), Hash(`event_id`)

---

## 6) 決策演算法細節（摘錄）

* **ATR(EMA,n)**：`ATR_t = α·TR_t + (1−α)·ATR_{t−1}, α=2/(n+1)`
* **市價滑價守門**：若估計 `|ΔP|/P > slip_bp_max` → 改限價或細分片
* **Maker 等待**：`T_wait = a + b/ spread + c·log(depth_top1)`（由 TCA 回歸）
* **置信度映射**：`p>0.85→×1.2；0.6–0.85→×1.0；0.4–0.6→×0.5；<0.4→skip`

---

## 7) 定時任務（S3）

* **決策心跳（每 10s）**：若 `feat:events:*` T 秒內無新訊 → 主動拉 `feat:last:{SYMBOL}` 跑 L0 風險檢查（含強平緩衝）
* **配額自癒（每 1m）**：掃描滯留的暫占額度（崩潰後未釋放）→ 與 DB/交易所狀態對齊修正

---

## 8) 錯誤處理與補償

* **冪等**：`intent_id` 唯一；`idem:order:{id}` 以 `SETNX + TTL` 保證單次提交
* **暫占釋放**：S4 回傳失敗 / 超時 → 立即釋放風險配額鍵
* **雙軌容錯**：REST 失敗→可落備援 `ord:cmd:{SYMBOL}` 由路由器守護消費

---

## 9) 監控與 SLO

* **SLI**：`strategy.decide.latency_ms`、`gate.skip.count{reason}`、`rules.fired.count`、`intent.emit.ok/fail`
* **SLO**：P95 `POST /decide` ≤ 500ms；決策成功率 ≥ 99.5%；配置收斂滯後 ≤ 5s
* **健康降級響應**：`health: RED` → `size_mult` 下調、`market_ratio` ↑、優先 Taker

---

## 10) 契約測試（精要）

* **/decide（happy）**：特徵齊全→`open` 並產生合法 `intent`（價階/步長正確）
* **/decide（gate）**：`funding_next` 越界/`spread` 過寬 → `skip(reason=GATE)`
* **/decide（ml timeout）**：模型超時→降級 `size_mult_ml=1.0`
* **冪等**：同一 `intent_id` 重送→同結果、且不重複占用配額
* **RCU 熱載**：`cfg:events` 後次一筆 `signals` 的 `config_rev` 必為新版本

---

## 11) 環境變數（建議）

```
S3_DB_ARANGO_URI, S3_DB_ARANGO_USER, S3_DB_ARANGO_PASS
S3_REDIS_ADDRESSES=host1:6379,host2:6379  # Redis Cluster
S3_SYMBOLS=BTCUSDT,ETHUSDT
S3_MAX_FUNDING_ABS=0.0005
S3_SPREAD_BP_LIMIT_DEFAULT=3.0
S3_DEPTH_TOP1_USDT_MIN_DEFAULT=200
S3_LEVERAGE_DEFAULT=20
S3_MARGIN_BASE_USDT=20
S3_MAX_LOSS_USDT=1.2
S3_ATR_MULT=1.5
S3_CONCURRENT_ENTRIES_PER_MARKET=1
S3_CONFIG_POLL_SEC=5
```

---

## 12) 交付清單（DoD）

* [ ] `GET /health`、`POST /decide` 可用
* [ ] 消費 `feat:events:*`，產出 `sig:events` 與 `signals.decision`
* [ ] 風險鍵原子暫占/釋放完整；冪等鍵生效
* [ ] RCU 熱載（`cfg:events` + 本地校驗）
* [ ] 指標上拋至 S11；Grafana 面板可見延遲/命中/跳過
* [ ] 契約測試全綠；混沌（WS 中斷/Redis 抖動）不破壞冪等與配額一致性

---

> 備註：本補充仍維持 **未實作** 狀態標示，對應你的全系統規劃；可直接據此拆分 Issue/Task 與撰寫單元/契約/整合測試。
