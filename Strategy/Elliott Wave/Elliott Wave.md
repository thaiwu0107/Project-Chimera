# 在 Project Chimera 中引入「艾略特波浪」：設計、因子與落地路線

> TL;DR：**不要把決策邏輯塞進 S1。**S1 仍只做「交易所連線/原始資料入站」。艾略特波浪（Elliott Wave, EW）屬於**特徵工程＋規則決策**範疇：
>
> * **S2（Feature Generator）**：線上/近線計算波浪結構特徵（多時框、一致性打分），將結果以因子寫入 `signals.features` 與 Redis 快取。
> * **S3（Strategy Engine）**：在 DSL 規則中引用 EW 因子作為守門或加權（如「Wave-3 動能期」放大 size、「Wave-5 疲勞」降風險或跳過）。

下列內容包含：**演算法設計**、**新增因子清單（含欄位/定義/取值/公式）**、**資料流與落地點（DB/Redis/Streams）**、**DSL 規則範例**、**靈敏度/回測校準**、**實作步驟（MVP→進階）**。

---

## 1) 架構定位與資料流（Where）

* **S1 Exchange Connectors（維持既有職責）**

  * 任務：穩定拉取/推送 OHLCV、Order Book、Funding、Account。
  * **不**做：EW 計算、交易判斷。
  * 輸出：`mkt:candles:<symbol>:<tf>`（Redis Stream/快照）、`mkt:depth:*`、`mkt:ticker:*`。

* **S2 Feature Generator（新增 EW 子模組）**

  * **計算**：以 **ZigZag/轉折點** 為骨架，做 **波浪配對與打分**（多時框：1m/5m/15m/1h/4h）。
  * **落地**：

    * 即時因子 → `signals.features.ew_*`（寫 ArangoDB `signals`）；
    * 快照快取 → `feat:ew:snap:<symbol>:<tf>`（Redis Hash）與 `feat:events:ew`（Redis Stream）。
  * **對外**：`POST /features/recompute?symbol=&tf=` 支援重算。

* **S3 Strategy Engine（消費 EW 因子）**

  * **引用**：在 L0/L1 規則中檢查 `ew_state/ew_confidence/...`；在 L2 調整 `size_mult`。
  * **範例**：Wave-3（推動浪）且信心高 → 放大尺寸；Wave-5 末端 + 背離 → 降低尺寸或 `skip_entry=true`。

---

## 2) 演算法設計（How）

### 2.1 轉折點與簡化（ZigZag / RDP）

* **ZigZag 閾值**：

  $$
  z\_{\text{thres}} = \max\left(z\_{\min},\ k \cdot \frac{\text{ATR}(n)}{P}\right)
  $$

  以 **波動自適應**（ATR 比例）避免噪音；或採 Ramer–Douglas–Peucker（RDP）簡化折線做同等效果。

* **轉折序列**：得到高低點序列 $(t_i, P_i)$，形成\*\*擺動（swing）\*\*集合 $S=\{s_1,\dots,s_M\}$。

### 2.2 波浪標記（Constraint + Scoring）

* **規則約束（核心子集）**

  * Wave 2 **不**得回撤超過 Wave 1 起點；
  * Wave 3 **不得**為最短推動浪；常見延伸 $|W3| \gtrsim 1.618|W1|$；
  * Wave 4 與 Wave 1 **通常不重疊**（除對角形/斜三角）；
  * 修正浪為 **ABC**（鋸齒/平台/三角），常見 $|B| \approx 0.382\sim0.786 |A|$、$|C|\approx 0.618\sim1.0|A|$。

* **費波那契貼近度打分**（距離越小越高分）

  $$
  \text{score\_fib}(r; r^*, \sigma)=\exp\left(-\left|\frac{r-r^*}{\sigma}\right|^2\right)
  $$

  例：$r=\frac{|W2|}{|W1|}$ 接近 $0.618$ 或 $0.5$ 則得分高。

* **整體評分（以 1–5 或 ABC 模式）**

  $$
  \text{Score} = \sum\_k w\_k \cdot \text{score}\_k\ -\ \sum\_j \lambda\_j \cdot \mathbf{1}[\text{違反規則}j]
  $$

  使用動態規劃/回溯，在最近 $M$ 個 swings 上找 **得分最高**、**違規最少** 的標記。

* **多時框一致性**

  * 以低時框（1m/5m）提供細部結構，高時框（1h/4h）提供**大級別趨勢**；
  * **共識度**：跨時框方向一致 + 推動/修正分類一致，產生 `ew_consensus`。

### 2.3 背離與通道

* **RSI 背離**（可選因子）：價格新高但 RSI 未創高 → 看空背離；反之亦然。
* **通道斜率/偏離**：以線性回歸擬合 **推動浪通道**，計算當前價相對通道的偏離 z-score。

---

## 3) 新增因子（factor\_registry）與 `signals.features` 欄位

> 命名遵循：`ew_*`。多時框以後綴 `_tf_<1m|5m|15m|1h|4h>`。下表列核心（可擴充）。

| 因子名                         | 型別     | 說明                                         | 典型值域/枚舉   |   |    |   |          |
| --------------------------- | ------ | ------------------------------------------ | --------- | - | -- | - | -------- |
| `ew_state_tf_15m`           | string | 主要結構：`IMPULSE_1..5`、`CORR_A/B/C`、`UNKNOWN` | enum      |   |    |   |          |
| `ew_dir_tf_15m`             | int    | 推動方向：`+1` 多、`-1` 空、`0` 不明                  | -1/0/+1   |   |    |   |          |
| `ew_confidence_tf_15m`      | float  | 綜合打分（規則滿足＋Fib 貼近＋多時框一致性）                   | \[0,1]    |   |    |   |          |
| `ew_w2_retr_ratio_tf_15m`   | float  | (                                          | W2        | / | W1 | ) | \[0,1.2] |
| `ew_w3_ext_ratio_tf_15m`    | float  | (                                          | W3        | / | W1 | ) | \[0,3]   |
| `ew_invalidation_px_tf_15m` | float  | **失效價**（破位後該標記失效）                          | >0        |   |    |   |          |
| `ew_alt_counts_tf_15m`      | int    | 可行備選標記數（歧義度）                               | ≥1        |   |    |   |          |
| `ew_consensus_score`        | float  | 跨 15m/1h/4h 的方向與狀態一致性                      | \[0,1]    |   |    |   |          |
| `ew_rsi_div_tf_15m`         | string | `BULLISH`/`BEARISH`/`NONE`                 | enum      |   |    |   |          |
| `ew_channel_z_tf_15m`       | float  | 當前價相對通道中心的 z-score                         | \~\[-3,3] |   |    |   |          |

> **DB 落地**：上述值一律寫入 `signals.features`；`factor_registry` 為每個因子建條目（`range`、`freq`、`owner`、`status`），並在 `strategy_rules` Lint 時檢查引用合法性。
> **索引建議**：在 `signals` 建 `skiplist(symbol,t0)`；查詢 EW 決策窗口時效率更佳。

---

## 4) Redis／Streams（S2 專用）

* `feat:ew:snap:<symbol>:<tf>`（Hash）：

  * fields：`state, dir, confidence, w2_ratio, w3_ratio, invalidation_px, rsi_div, channel_z, updated_at`
  * TTL：依時框自訂（如 15m → 20–30 分鐘）

* `feat:events:ew`（Stream）：

  * fields：`symbol, tf, state, dir, confidence, ts`
  * 消費者：S3（決策）、S11（監控）

---

## 5) DSL 規則範例（S3）

**例 1：Wave-3 推動，放大尺寸**

```json
{
  "rule_id": "EW-IMPULSE3-BOOST",
  "priority": 60,
  "applies": { "symbols": ["BTCUSDT"], "services": ["s3-strategy"] },
  "when": {
    "allOf": [
      { "f": "ew_state_tf_15m", "op": "in", "v": ["IMPULSE_3"] },
      { "f": "ew_confidence_tf_15m", "op": ">", "v": 0.7 },
      { "f": "ew_consensus_score", "op": ">", "v": 0.5 }
    ]
  },
  "action": { "type": "size_mult", "value": 1.2 },
  "explain": "Wave-3 高信心一致性，放大初始倉位",
  "status": "ENABLED"
}
```

**例 2：Wave-5 疲勞 + 背離，降風險或跳過**

```json
{
  "rule_id": "EW-IMPULSE5-EXHAUST",
  "priority": 80,
  "applies": { "symbols": ["BTCUSDT"], "services": ["s3-strategy"] },
  "when": {
    "allOf": [
      { "f": "ew_state_tf_15m", "op": "in", "v": ["IMPULSE_5"] },
      { "f": "ew_confidence_tf_15m", "op": ">", "v": 0.6 },
      { "f": "ew_rsi_div_tf_15m", "op": "in", "v": ["BEARISH"] }
    ]
  },
  "action": { "type": "skip_entry", "value": true },
  "explain": "Wave-5 疲勞且 RSI 背離，跳過追價",
  "status": "ENABLED"
}
```

**例 3：ABC 修正末端的反轉佈局（小倉位試探）**

```json
{
  "rule_id": "EW-ABC-REVERSAL-PROBE",
  "priority": 55,
  "applies": { "symbols": ["BTCUSDT"], "services": ["s3-strategy"] },
  "when": {
    "allOf": [
      { "f": "ew_state_tf_15m", "op": "in", "v": ["CORR_C"] },
      { "f": "ew_confidence_tf_15m", "op": ">", "v": 0.65 },
      { "f": "ew_w2_retr_ratio_tf_15m", "op": ">", "v": 0.5 }
    ]
  },
  "action": { "type": "size_mult", "value": 0.7 },
  "explain": "C 段尾端反轉機率提升，先用小倉位試探",
  "status": "ENABLED"
}
```

---

## 6) 數學補充與檢核

* **延伸/回撤比（Fib）**：

  * 常見容忍集合 $R$：`{0.382, 0.5, 0.618, 1.0, 1.618, 2.618}`；每個目標比率搭配 $\sigma$（容忍度，建議 0.05–0.10）。
* **Local Valid/Invalid**：

  * 對每個標記輸出 `ew_invalidation_px`，若價穿越 → 事件 `ew:invalidate`，降低 `confidence` 或重新標記。
* **一致性（跨時框）**：

  $$
  \text{consensus} = \frac{1}{T}\sum\_{tf} \mathbf{1}[\text{dir}_{tf}=\text{major\_dir}] \cdot u_{tf}
  $$

  $u_{tf}$ 為時框權重（大時框權重較高）。

---

## 7) 回測與靈敏度（S9/S10 對齊）

* **Dry-run/快速重放**：在 `simulate` 中增加 `zigzag_threshold`、`fib_sigma` 兩個超參數，輸出 `flip_pct`（決策翻轉率）。
* **參數敏感度**：

  * $\epsilon$-擾動：`z_thres *= (1±ε)`、`σ *= (1±ε)`，觀察 `size_mult` / `skip_entry` 變化。
* **一致性檢查**：線上/離線 EW 事件時間點差異 ≤ X bar；若偏移率過高 → 警示演算法不穩定。

---

## 8) DB/索引與執行

* **新增（可選）**：`ew_swings` collection

  ```json
  {
    "symbol": "BTCUSDT",
    "tf": "15m",
    "swings": [ {"t": 1737072000000, "p": 42000.5, "type": "HIGH"}, ... ],
    "labels": ["W1","W2","W3","W4","W5"|"A","B","C"],
    "score": 0.78,
    "updated_at": 1737072060000
  }
  ```

  **索引**：`hash(symbol,tf)`、`skiplist(updated_at)`

* **signals**：僅新增 `features.ew_*` 欄位，沿用既有索引（`symbol,t0`）。

---

## 9) 實作步驟（MVP → 進階）

1. **MVP（1–2 週）**

   * ZigZag（ATR 自適應）+ 近 7–11 個 swings 標記，僅輸出 `state/dir/confidence`（簡化評分：Fib 接近度 + 規則違反扣分）。
   * 支援 15m 單時框；寫 `feat:ew:snap:*` 與 `signals.features`；S3 先接 1–2 條 DSL 守門。

2. **擴充（3–4 週）**

   * 多時框一致性（15m/1h/4h）；加入 RSI 背離與通道偏離；`ew_invalidation_px`。
   * S10 `simulate` 加入 EW 敏感度；S11 監控 `ew:invalidate` 頻率。

3. **強化（>4 週）**

   * 動態規劃/隱式圖搜尋（best-labeling）提高穩定性；
   * 自動辨識斜三角/對角形特殊情況；
   * 研究替代器：以 HMM/CRF 對轉折序列做結構化預測（仍保持可解釋）。

---

## 10) 風險與治理

* **主觀性**：EW 本質具解讀彈性 → 以**規則明確化 + 打分可視化 + 敏感度分析**降低主觀風險。
* **過擬合**：Fib 容忍度過小或樣本內調參易過擬合 → 以 `flip_pct` 與 **Canary** 部署控制風險。
* **效能**：ZigZag/標記採增量更新（僅隨新 K 線調整），避免每 bar 全量回算；多時框分池計算。

---

## 11) 與現有策略的整合要點

* **與 ATR/Regime**：在 EXTREME regime 下，即便 Wave-3，仍可下調 `size_mult`；在 FROZEN 中對 Wave-5 提高 `skip` 機率。
* **與 TCA/路由**：Wave-5 末端追價風險高 → 提高 maker 等待、降低市價比；Wave-3 可允許較高 taker 比例以跟進動能。
* **SPOT/FUT 一致**：EW 因子本身與市場別無關，SPOT 亦可共用；唯路由/風控參數依市場不同。

---

### 結語

將艾略特波浪放在 **S2（特徵）→ S3（規則決策）** 的「**可解釋、可打分、可治理**」流水線上，是工程化地把傳統主觀技術分析納入量化決策的正確路徑：

* **資料合規**：所有輸入來自 S1 原始行情；
* **決策透明**：每個入場/跳過都能對應到 `ew_state/ew_confidence/invalid_px`；
* **可運維**：以 Redis 快取加速即時消費、以 ArangoDB 保存審計可追溯、以 S10 模擬＋敏感度守門上線。

按本文步驟落地，你就能在不改動 S1 邊界的前提下，**安全、可控** 地把艾略特波浪納入 Project Chimera 的決策體系。


好的，我們聚焦在「**怎麼出場、何時出場**」，而且要能直接落地到 Project Chimera v3.2 的 S3/S4/S10 設計。以下是**可直接拷貝的純MD規格**，涵蓋波浪結構驅動的出場邏輯、參數預設、DSL 模板與風控細節。

---

# Project Chimera v3.2 — 出場（Exit）設計：Elliott Wave 版

## 0) 核心原則

1. **硬止損=無效化價**（Invalidation Level）：每筆倉位都引用 S2 輸出的 `wave_invalidation_px_[tf]`。
2. **多層出場（Staged Exit）**：分批獲利了結 + 移動（或轉換）止損。
3. **結構優先**：當波浪結構顯示「次級結構完成」或「相反結構啟動」，**優先出場**。
4. **不重繪**：只依已確認事件（`*_confirmed`）做決策。
5. **跨週期一致性**：高週期反向訊號具**否決權**（可設為 guardian stop）。

---

## 1) 出場種類與適用時機

### A. 硬止損（Hard Stop）

* 來源：`wave_invalidation_px_[tf]`。
* 規則：進場同時下單；如觸發，S8 標記 `invalidation_hit=true`。
* 用途：**唯一不可移除**的生存邏輯。

### B. 目標價分批（Target Ladder）

依波浪/斐波那契/通道，設置分批 TP：

* \*\*趨勢單（Impulse）\*\*常用：

  * `TP1 = entry + 0.618 * |W1|`（多單；空單相反）
  * `TP2 = entry + 1.0 * |W1|`
  * `TP3 = entry + 1.618 * |W1|`
* \*\*反轉單（C浪終點）\*\*常用：

  * `TP1 = 0.382 * |A或上一腿|`
  * `TP2 = 0.618 * |A|`
  * `TP3 = 1.0 * |A|`
* **三角形突破**常用：

  * `TP = 量測目標 = 三角形高 × 0.618 / 1.0` 自突破點量測

> 觸發 TP1 後，將止損移至 **入場價或結構保護位**（見 D）。

### C. RR（風險報酬）門檻

* `RR1 = 1.0` 或 `1.5`：達到即**鎖盈**（將止損移至入場或更高）。
* `RR2 = 2.0~3.0`：加速出場或啟動更緊的追蹤止損。

### D. 結構性出場（Structure-based Exit）

當下列**任一**結構事件出現，立即部分/全部出場：

* `end_of_4_confirmed`（對持有 3 浪中部倉位）：部分了結 25\~50%。
* `end_of_5_candidate_confirmed` 或**通道上緣假穿**：加速出清或啟動緊追蹤。
* 反向強訊號（例如你是多單，但高週期出現 `end_of_C_confirmed` 向下）：**全部出場或 Guardian Stop 觸發**。
* **通道跌破**（多單）/ **通道上破失敗**（空單）：部分或全部出場。
* **波峰/波谷背離**（量價或動能背離）在 W5 末端：至少出 50%。

### E. 追蹤止損（Trailing Stop）

* **ATR 追蹤**：`TS = n_ATR * ATR(14)`（預設 n\_ATR=1.5\~2.5）。
* **結構追蹤**：止損跟隨前一小級別的**樞紐低/高**（fractal k），或通道內側線。
* **切換規則**：達 `RR1` 後，由硬止損→追蹤止損；達 `RR2` 後，將 n\_ATR 調小（更緊）。

### F. 時間止損（Time Stop）

* 若 `t_open → t_open + T_max`（如 12h/24h）仍未達 `RR0.5`，且波浪結構轉弱，**平倉**。
* 用於避免「對的方向、錯的時間」。

### G. 流動性/滑點防護（Liquidity-aware Exit）

* 風險事件（Funding/公告/開盤）前 `t_pre`：**減倉**或**全部平倉**。
* **TWAP 出場**：在稀薄流動性時段用 `slices=3~7`，限制單片滑點 `max_slippage_bps`。

---

## 2) 波浪結構 → 出場策略對照

| 持倉背景        | 關鍵訊號（S2）                                  | 預設動作                           |
| ----------- | ----------------------------------------- | ------------------------------ |
| 正在 3 浪（多單）  | `end_of_4_confirmed`                      | 平 25–50%；餘部改 ATR 追蹤；TP2、TP3 保留 |
| 正在 5 浪（多單）  | `divergence_on_w5` 或 `channel_break_down` | 立即平 50–100%（依分數）；若分批，縮小 n\_ATR |
| 反轉單（C 結束做多） | `structure_degrades` 或 高週期反向強訊號           | 一次性平倉或觸發 Guardian Stop         |
| 三角形突破（多單）   | `re-entry_into_triangle`                  | 視為假突破：全部平倉或回撤至入場即平             |

---

## 3) S10 參數（預設值，可治理）

```json
{
  "exit": {
    "use_rr_breakeven": true,
    "rr_to_breakeven": 1.0,
    "rr_tighten_at": 2.0,
    "atr_trail_initial": 2.0,
    "atr_trail_tight": 1.3,
    "tp_ladder": {
      "impulse": [0.618, 1.0, 1.618],
      "c_end": [0.382, 0.618, 1.0],
      "triangle": [0.618, 1.0]
    },
    "triangle_measure_mult": 0.618,
    "time_stop_hours": 24,
    "guardian_stop": {
      "enable_higher_tf_override": true,
      "higher_tf_list": ["1h", "4h", "1d"]
    },
    "liquidity": {
      "twap_slices": 5,
      "max_slippage_bps": 3
    }
  }
}
```

---

## 4) S3 策略 DSL：出場區塊模板

### 4.1 趨勢延續單（2 結束→3 起飛）

```yaml
strategy: "ew_impulse_trend_follow_v1"
exit:
  ladder:
    - kind: rr
      rr: 1.0
      action:
        move_stop_to: breakeven_or_structure   # 入場價或最近樞紐
    - kind: fib_leg
      leg: "W1"         # 以 W1 長度為基準
      mult: 0.618
      action:
        take_profit: 0.33
        tighten_atr_to: 1.8
    - kind: fib_leg
      leg: "W1"
      mult: 1.0
      action:
        take_profit: 0.33
        tighten_atr_to: 1.5
    - kind: fib_leg
      leg: "W1"
      mult: 1.618
      action:
        take_profit: 1.0   # 剩餘全出
  trailing:
    mode: atr
    atr_mult_initial: 2.0
    atr_mult_after_rr: 
      rr: 2.0
      mult: 1.3
  structure_events:
    - on: "end_of_4_confirmed"
      action:
        take_profit: 0.25
        keep_trailing: true
    - on: "channel_break_down"
      action:
        take_profit: 1.0
  guardian_stop:
    higher_tf_override: true
    deny_if_higher_tf_signal_in: ["end_of_C_confirmed"]
  time_stop:
    hours: 24
```

### 4.2 反轉單（C 浪終點）

```yaml
strategy: "ew_reversal_c_end_v1"
exit:
  ladder:
    - kind: fib_prev_leg
      leg: "A_or_prev"
      mult: 0.382
      action:
        take_profit: 0.4
        move_stop_to: breakeven
    - kind: fib_prev_leg
      leg: "A_or_prev"
      mult: 0.618
      action:
        take_profit: 0.4
        tighten_atr_to: 1.5
    - kind: rr
      rr: 2.5
      action:
        take_profit: 1.0
  structure_events:
    - on: "structure_degrades"
      action:
        take_profit: 1.0
  trailing:
    mode: atr
    atr_mult_initial: 1.8
```

### 4.3 三角形突破

```yaml
strategy: "ew_triangle_break_v1"
exit:
  ladder:
    - kind: measured_move
      base: "triangle_height"
      mult: 0.618
      action:
        take_profit: 0.5
        move_stop_to: breakeven
    - kind: measured_move
      base: "triangle_height"
      mult: 1.0
      action:
        take_profit: 1.0
  structure_events:
    - on: "re_entry_into_triangle"
      action:
        take_profit: 1.0   # 假突破，立刻清
  trailing:
    mode: atr
    atr_mult_initial: 1.7
```

---

## 5) S4 下單層（出場委託型別）

* **Futures**：

  * 進場即下 `Stop`（無效化價）；觸發 TP 時採 **Reduce-Only 限價**；必要時啟動 **TWAP-R**（分片出清）。
* **SPOT**：

  * 使用 **OCO**（TP+SL）；Guardian Stop 以**市價/IOC**確保落袋。
* **滑點控制**：

  * `max_slippage_bps` 達上限即改市價；夜間/冷門時段預設 TWAP。

---

## 6) 特殊情境處理

1. **延伸三浪（W3 Extension）**

   * 若 S2 給 `break_of_iii`（加速），則**延後 TP1**或只小幅出場（10–20%），主力改用追蹤止損吃趨勢。
2. **失敗五浪（Truncation）**

   * 出現 `w5_truncation_confirmed` → 立即全部出清，標記 `exit_reason=truncation`.
3. **高週期反向**

   * 1h/4h 出現強反向訊號 → **Guardian** 直接平倉或縮倉≥50%。
4. **重大事件時間窗**

   * 事件前 `t_pre` 自動減倉；事件後 `t_post` 才恢復正常追蹤。

---

## 7) 監控與回顧（S11/S8）

* **S11 指標**

  * `exit:avg_rr_at_final`（目標 ≥ 1.8）
  * `exit:win_rate_after_rr1_lock`（鎖盈後勝率目標 ≥ 70%）
  * `exit:slippage_bps_p95`（≤ 3 bps）
  * `exit:guardian_triggers/week`（監控是否過於頻繁）
* **S8 剖檢**

  * 記錄 `exit_reason`：`tp_ladder/rr_lock/structure_event/trailing/time_stop/guardian/invalidation`
  * 附上 `wave_snapshot` 圖與通道/斐波那契標記，說明為何在該點出場。

---

## 8) 實作步驟（落地）

1. **S10**：加入 `exit.*` 參數（如上 JSON），Lint 確保：

   * 任一策略**必須**定義 `hard_stop = wave_invalidation_px_*`；
   * 有 RR 鎖盈與至少一個 TP 階梯或追蹤止損。
2. **S3**：套用三個 DSL 模板（趨勢/反轉/三角），逐策略啟用。
3. **S4**：新增 TWAP-R 出場路徑，並確保 SPOT 的 OCO 行為覆測。
4. **S11**：新增出場 SLI 與告警；**回放近 30 日**檢查出場品質。
5. **S8**：剖檢模板加入「結構事件→出場」的敘事與截圖。

---

## 9) 參數建議（初始）

* `rr_to_breakeven = 1.0`、`rr_tighten_at = 2.0`
* `atr_trail_initial = 2.0`、`atr_trail_tight = 1.3`
* `triangle_measure_mult = 0.618`
* `time_stop_hours = 24`
* `guardian_stop.enable_higher_tf_override = true`

---

## 10) 小結：決策順序（實戰心智圖）

1. 進場 → **硬止損=無效化價**就位
2. 達 **RR1** → 移止損至入場/結構位（鎖盈）
3. `TP Ladder` 逐級觸發 → 分批落袋 + 收緊 ATR
4. 如觸發**結構事件**（如 `end_of_4_confirmed`、`channel_break`、反向強訊號） → 立即部分/全部出場
5. 未達成績效且**超時** → 時間止損
6. 全程受高週期 **Guardian** 否決

---

如果你願意，下一步我可以把你現有三支策略（趨勢/反轉/三角）的**完整 YAML**+`config_bundles` JSON 補上，直接拿去 S10 做 Dry-run；或是把 S4 的 **TWAP-R 出場流程圖**與**Reduce-Only 與 OCO 組合示例**也一次給你。你想先套哪一組？


好的！我把「進場 + 出場」整成一份**可直接上 S10/S3 用的純MD規格**，含三個常用策略模板（趨勢/反轉/三角）與統一的風控與參數。你可以直接拷貝到 `config_bundles` 做 Dry-run。

---

# Project Chimera v3.2 — Elliott Wave 一體化「進出場」規格（可上線）

## 0) 全域設計原則

* **訊號來源**：僅採 S2 已確認事件（`*_confirmed`），避免重繪。
* **止損一律=無效化價**：`hard_stop = wave_invalidation_px_[tf]`（不可移除）。
* **多層出場**：RR 鎖盈 + 目標價階梯 + 結構事件 + 追蹤止損 + 時間止損。
* **跨週期一致性**：高週期反向強訊號具 Guardian 否決權（可配）。
* **可治理**：所有閾值放入 S10 `wave_params` 與 `exit.*`。

---

## 1) S10 全域參數（合併版；可作為 bundle 片段）

```json
{
  "wave_params": {
    "tf_set": ["1m","5m","15m","1h","4h"],
    "min_score": 0.75,
    "fib_tol": 0.08,
    "pivot_k": 3,
    "zzz_threshold_bps": 25,
    "overlap_tolerance_bps": 0,
    "multi_tf_policy": "higher_tf_priority"
  },
  "entry": {
    "min_rvol": 1.05,
    "confirm_timeout_bars": 8,
    "conflict_block_higher_tf": true
  },
  "exit": {
    "use_rr_breakeven": true,
    "rr_to_breakeven": 1.0,
    "rr_tighten_at": 2.0,
    "atr_trail_initial": 2.0,
    "atr_trail_tight": 1.3,
    "tp_ladder": {
      "impulse": [0.618, 1.0, 1.618],
      "c_end": [0.382, 0.618, 1.0],
      "triangle": [0.618, 1.0]
    },
    "triangle_measure_mult": 0.618,
    "time_stop_hours": 24,
    "guardian_stop": {
      "enable_higher_tf_override": true,
      "higher_tf_list": ["1h","4h","1d"]
    },
    "liquidity": {
      "twap_slices": 5,
      "max_slippage_bps": 3
    }
  },
  "risk": {
    "base_size_usdt": 100, 
    "risk_per_trade_pct": 0.5,
    "max_concurrent_positions": 5,
    "add_on_gate_rr": 1.2
  }
}
```

---

## 2) 統一進場規格（映射到任何策略）

**觸發條件（必備 + 可選）：**

* 必備：

  * `wave_signal_[tf] ∈ {end_of_2_confirmed, end_of_C_confirmed, triangle_e_break}`（依策略）
  * `wave_*_score_[tf] ≥ min_score`
  * `rvol_[tf] ≥ entry.min_rvol`
  * **沒有**來自 `higher_tf_list` 的**反向強訊號**（若 `conflict_block_higher_tf=true`）
* 可選加分：

  * `channel_ok_[tf] == 1`
  * 動能共振（如 `divergence_rsi=false` 於順勢、`divergence_rsi=true` 於反轉）

**倉位方向與入場價：**

* `end_of_2_confirmed`：沿 1→2 方向**順勢**（多或空）
* `end_of_C_confirmed`：反向**反轉**
* `triangle_e_break`：沿突破方向
* 價格：`maker_preferred`（S4 先掛單，容忍 `max_slippage_bps`，超過改 IOC/市價）

**部位 sizing：**

* 以「進場價 ↔ 無效化價」距離推算張數，使預期虧損 ≦ `risk_per_trade_pct` × 淨值
* 若 `add_on_gate_rr` 達成且有 `break_of_iii`/加速訊號 → 按策略規則加倉（≤ 0.5×初始）

---

## 3) 統一出場規格（對齊前次討論）

* **硬止損**：`wave_invalidation_px_[tf]`（下單即放）
* **RR 鎖盈**：到 `RR_to_breakeven`（預設 1.0）→ 止損移入場/結構位
* **TP 階梯**：依策略類型從 `tp_ladder` 讀取
* **結構事件**：`end_of_4_confirmed`、`end_of_5_candidate_confirmed`、`channel_break_*`、背離、`re_entry_into_triangle` 皆可觸發部分/全部出場
* **追蹤止損**：ATR 2.0 → 達 `rr_tighten_at` 後改 1.3
* **時間止損**：`time_stop_hours` 到期未達 `RR0.5` 且結構轉弱 → 平倉
* **Guardian**：高週期反向強訊號 → 直接平/縮≥50%

---

## 4) S3 策略 DSL（整合版範例）

### 4.1 趨勢延續（2 結束 → 3 起飛）

```yaml
strategy: "ew_trend_follow_v1"
scope:
  symbols: ["BTCUSDT","ETHUSDT"]
  tf: "5m"
entry:
  when:
    all:
      - wave_signal_5m == "end_of_2_confirmed"
      - wave_impulse_score_5m >= 0.75
      - wave_channel_ok_5m == 1
      - rvol_5m >= 1.05
      - not higher_tf_conflict(["1h","4h"])
  side: follow_detected_trend    # 自動判定多/空
  price: maker_preferred
  size:
    mode: risk_based
    risk_pct: 0.5
    stop_ref: wave_invalidation_px_5m
  add_on:
    when:
      all:
        - rr_current >= 1.2
        - wave_signal_5m in ["break_of_iii"]
    size_mult: 0.5
    trail_after_add: true
risk:
  hard_stop: wave_invalidation_px_5m
exit:
  ladder:
    - kind: rr
      rr: 1.0
      action: { move_stop_to: breakeven_or_last_pivot }
    - kind: fib_leg
      leg: "W1"
      mult: 0.618
      action: { take_profit: 0.33, tighten_atr_to: 1.8 }
    - kind: fib_leg
      leg: "W1"
      mult: 1.0
      action: { take_profit: 0.33, tighten_atr_to: 1.5 }
    - kind: fib_leg
      leg: "W1"
      mult: 1.618
      action: { take_profit: 1.0 }
  trailing:
    mode: atr
    atr_mult_initial: 2.0
    atr_mult_after_rr: { rr: 2.0, mult: 1.3 }
  structure_events:
    - on: "end_of_4_confirmed"     # 3浪尾端警訊
      action: { take_profit: 0.25 }
    - on: "channel_break_down"
      action: { take_profit: 1.0 }
  guardian_stop:
    higher_tf_override: true
    deny_if_higher_tf_signal_in: ["end_of_C_confirmed"]
  time_stop: { hours: 24 }
```

### 4.2 反轉（C 浪終點）

```yaml
strategy: "ew_reversal_c_end_v1"
scope: { symbols: ["BTCUSDT"], tf: "15m" }
entry:
  when:
    all:
      - wave_signal_15m == "end_of_C_confirmed"
      - wave_zigzag_score_15m >= 0.75
      - fib_deviation_15m <= 0.08
      - rvol_15m >= 1.05
      - not higher_tf_conflict(["1h","4h"])
  side: reverse_of_c_direction
  price: maker_preferred
  size:
    mode: risk_based
    risk_pct: 0.5
    stop_ref: wave_invalidation_px_15m
risk: { hard_stop: wave_invalidation_px_15m }
exit:
  ladder:
    - kind: fib_prev_leg
      leg: "A_or_prev"
      mult: 0.382
      action: { take_profit: 0.4, move_stop_to: breakeven }
    - kind: fib_prev_leg
      leg: "A_or_prev"
      mult: 0.618
      action: { take_profit: 0.4, tighten_atr_to: 1.5 }
    - kind: rr
      rr: 2.5
      action: { take_profit: 1.0 }
  structure_events:
    - on: "structure_degrades"
      action: { take_profit: 1.0 }
  trailing: { mode: atr, atr_mult_initial: 1.8 }
  guardian_stop:
    higher_tf_override: true
    deny_if_higher_tf_signal_in: ["end_of_2_confirmed"]  # 反向順勢啟動
  time_stop: { hours: 24 }
```

### 4.3 三角形突破

```yaml
strategy: "ew_triangle_break_v1"
scope: { symbols: ["BTCUSDT","SOLUSDT"], tf: "1h" }
entry:
  when:
    all:
      - wave_signal_1h == "triangle_e_break"
      - wave_triangle_score_1h >= 0.75
      - rvol_1h >= 1.05
      - not higher_tf_conflict(["4h","1d"])
  side: break_direction
  price: maker_preferred
  size:
    mode: risk_based
    risk_pct: 0.5
    stop_ref: wave_invalidation_px_1h
risk: { hard_stop: wave_invalidation_px_1h }
exit:
  ladder:
    - kind: measured_move
      base: "triangle_height"
      mult: 0.618
      action: { take_profit: 0.5, move_stop_to: breakeven }
    - kind: measured_move
      base: "triangle_height"
      mult: 1.0
      action: { take_profit: 1.0 }
  structure_events:
    - on: "re_entry_into_triangle"
      action: { take_profit: 1.0 }
  trailing: { mode: atr, atr_mult_initial: 1.7 }
  guardian_stop:
    higher_tf_override: true
    deny_if_higher_tf_signal_in: ["end_of_C_confirmed"]
  time_stop: { hours: 24 }
```

---

## 5) S4 執行與委託建議（統一）

* **期貨**：進場立刻掛 `Reduce-Only` 的 TP 階梯 + `Stop`（無效化）。若流動性稀薄或 `max_slippage_bps` 觸頂，切換 **TWAP-R**（`twap_slices = exit.liquidity.twap_slices`）。
* **SPOT**：用 **OCO**（TP + SL）。Guardian 事件用 IOC/市價。
* **加倉**：僅在 `rr_current ≥ add_on_gate_rr` 且出現 `break_of_iii` / 強勢延伸；每次 ≤ 初始 0.5 倍，並同步收緊追蹤止損。

---

## 6) S11 監控指標（針對「進+出」）

* `trade:avg_rr_final`（目標 ≥ 1.8）
* `trade:win_after_breakeven`（鎖盈後勝率 ≥ 70%）
* `trade:exit_slippage_bps_p95`（≤ 3）
* `wave:repaint_events`（= 0）
* `guardian:trigger_rate`（過高代表高週期衝突多，應提升 `min_score` 或改 `higher_tf_priority`）

---

## 7) S8 剖檢（自動敘事欄位）

* `entry_reason`：`end_of_2_confirmed | end_of_C_confirmed | triangle_e_break`
* `exit_reason`：`tp_ladder | rr_lock | structure_event | trailing | time_stop | guardian | invalidation`
* `wave_snapshot`：入/出場當下的 pattern、score、signal、通道/斐波那契資訊
* `invalidation_hit`：true/false

---

## 8) 上線流程（合併）

1. **S10** 落地此 bundle；Lint 檢查每策略都有 `hard_stop` 與至少一個 `exit.ladder` 或 `trailing`。
2. **S3** 先 Dry-run（72 小時）→ 僅記錄意圖與計算 RR、未下單。
3. **S11** 核對指標；**repaint=0**、`win_after_breakeven` 合格後，Canary 1% → Ramp。
4. **S8** 抽樣 100 筆核對「進場理由/出場事件」與圖像標註一致性。

---

### 9) 你可立即做的兩件事

1. 把上面三段 YAML 放進三個策略檔（或合併為一包 bundle），套用到你指定的 symbols 與 TF。
2. 若你想先跑「**單一策略**」：我建議從 `ew_trend_follow_v1`（5m）開始，因為訊號密度高、可快速調參；等穩定再開 `ew_reversal_c_end_v1`（15m）與 `ew_triangle_break_v1`（1h）。

---

需要我把這三個 YAML + 一份完整 `config_bundles` JSON（已含 `wave_params/entry/exit/risk`）包成一個檔案給你直接貼上嗎？

太讚，下面把你提的 5 個面向**完全融入**既有 v3.2 規格，給你能直接落地到 S2/S3/S5(邏輯層)/S10/S11 的「純MD 可拷貝」補強包。

---

# Project Chimera v3.2 — Elliott Wave 補強包（狀態機・健壯化・組合風險・維運・進階）

## 1) 持倉狀態機（Position State Machine）

### 1.1 狀態定義

* `ACTIVE_CONFIRMED`：結構健康（例：處於 3 浪、通道內、score≥min\_score）。
* `ACTIVE_WARNING`：出現警示（例：`end_of_4_confirmed`、`ew_confidence` 下降），**禁加倉**。
* `ACTIVE_REDUCED_RISK`：已達 `RR_to_breakeven`，止損移至入場/結構位。
* `PENDING_EXIT`：已觸發明確出場事件（結構反轉、Guardian、高週期否決）。
* `CLOSED`：平倉完成。
* `INVALIDATED`：無效化價觸發（止損）。

### 1.2 事件→轉移（摘錄）

| 事件                                  | 從                 | 到                     | 動作                               |
| ----------------------------------- | ----------------- | --------------------- | -------------------------------- |
| `entry_filled`                      | —                 | ACTIVE\_CONFIRMED     | 設置 `hard_stop = invalidation_px` |
| `end_of_4_confirmed`                | ACTIVE\_CONFIRMED | ACTIVE\_WARNING       | 立即減倉 25–50%，鎖定禁加倉                |
| `rr_reach_breakeven`                | \*                | ACTIVE\_REDUCED\_RISK | 止損移入場/樞紐位                        |
| `structure_reverse_strong`/Guardian | ANY\_OPEN         | PENDING\_EXIT         | 觸發 S4 出清（Reduce-Only/TWAP-R）     |
| `filled_all_tp_or_stop`             | ANY\_OPEN         | CLOSED/INVALIDATED    | 結案（記錄 `exit_reason`）             |

### 1.3 S3 DSL 擴充（狀態動作）

```yaml
state_machine:
  initial: ACTIVE_CONFIRMED
  transitions:
    - on: end_of_4_confirmed
      to: ACTIVE_WARNING
      action:
        take_profit: 0.25
        set_flag: forbid_add_on=true
    - on: rr_reach_breakeven
      to: ACTIVE_REDUCED_RISK
      action:
        move_stop_to: breakeven_or_last_pivot
    - on: guardian_or_reverse
      to: PENDING_EXIT
      action:
        take_profit: 1.0
    - on: invalidation_hit
      to: INVALIDATED
```

### 1.4 儲存/事件（ArangoDB/Redis）

* `positions_snapshots` 新欄位：`state`, `state_ts`, `forbid_add_on`(bool), `ew_confidence`(0\~1)。
* `pos:events`（Redis Stream）新增：`state_changed` 事件（舊/新狀態、觸發原因、引用 signal）。

### 1.5 S11 儀表

* `pos:state_dist{ACTIVE_WARNING}`（比例門檻），`pos:avg_time_in_state`, `pos:forbid_add_on_rate`。

---

## 2) 參數優化與健壯性

### 2.1 參數分級

* **核心**：`zzz_threshold_bps`, `min_score`, `fib_tol`, `multi_tf_policy`
* **輔助**：`atr_trail_initial/tight`, `rr_to_breakeven`, `tp_ladder.*`, `time_stop_hours`

核心參數的敏感度大→系統不穩；S9 報表輸出 `core_param_lipschitz` 指標。

### 2.2 WFO（Walk-Forward）

```json
{
  "wfo": {
    "train_months": 12,
    "test_months": 3,
    "roll_step_months": 3,
    "metrics": ["sharpe","maxdd","winrate","avg_rr"],
    "select_by": "sharpe",
    "constraints": {"maxdd_pct": 0.2}
  }
}
```

S9 週期任務：每週滾動，將優選參數存為 `config_bundles` 新版（灰度）

### 2.3 情境壓測（Scenario）

```json
{
  "scenarios": {
    "vol_spike_x2": {"atr_mult": 2.0},
    "low_liq": {"max_slippage_bps_boost": 2.0, "twap_required": true},
    "gap_day": {"inject_gaps": true}
  }
}
```

S10 模擬器將情境注入回放；輸出「止損頻率、滑點成本、RR 分布」對比表。

---

## 3) 組合層風險（S5 邏輯層或 S3 前置 Gate）

### 3.1 跨標的關聯度限制

* `risk:correlation_matrix`（Redis key，S2/S9 每日回填 30/60/90d 滾動 ρ）
* 規則：若同方向且 `ρ ≥ 0.7`，則兩筆風險合併計算；超出 `asset_class_budget.crypto_long_pct` 則拒單或降尺。

### 3.2 因子暴露度限制

```json
{
  "portfolio_limits": {
    "by_strategy": { "ew_trend_follow_v1": 0.25, "ew_reversal_c_end_v1": 0.20 },
    "by_asset_class": { "crypto": 0.60 },
    "net_delta_range": [-0.3, 0.3]
  }
}
```

* 於下單前 S3 發 `pre_trade_check`：計算下單後 **淨 Delta**、策略暴露比例、資產類暴露；不符→`intent=REJECT|DOWNSIZE`。

### 3.3 Gate 介面（S3）

```yaml
portfolio_gate:
  checks:
    - type: correlation
      max_pair_rho: 0.7
      combine_risk_if_exceeds: true
    - type: strategy_budget
      cap_pct_nav: 0.25
    - type: net_delta
      range: [-0.3, 0.3]
  on_violation:
    - correlation: "prefer_higher_score"   # 留下 ew_score 較高者
    - strategy_budget: "downsize_to_fit"
    - net_delta: "skip_entry"
```

---

## 4) 操作與維運

### 4.1 手動干預

* Redis keys：

  * `kill_switch:global=true|false`（全域停新倉，允許出場）
  * `kill_switch:strategy:<name>=true|false`
  * `kill_switch:symbol:<symbol>=true|false`
* S3/S4 必須在每個意圖執行前校驗上述鍵；S11 監控「被拒占比」。

### 4.2 數據品質監控（S1→S2）

* 指標：

  * `data_gap_alert`（K 線時間斷裂/遺漏）
  * `bad_tick_alert`（Low>High、負成交量等）
  * `ew_recomputation_rate`（波浪替代計數頻率；高→市場模糊）
* 規則：任一嚴重級別=**PAUSE ENTRY**，並在 `pos:events` 打 `data_quality_degraded` 標記。

### 4.3 配置熱加載（RCU）

* `cfg:events` 發佈 `bundle_version=X.Y.Z`；S3 收到後：

  1. 新配置載入為 **Candidate**
  2. 新單使用 Candidate；既有倉位維持 **Active** 配置
  3. 無縫交接（讀寫分離 60s）後升級為 **Active**

---

## 5) 進階功能與未來

### 5.1 信心驅動倉位（Conviction Sizing）

* `ew_confidence` 由：`pattern_score`、`fib_deviation`、`channel_ok`、`multi_tf_consistency` 組合而成（0\~1）。
* 公式：

```
final_size = base_risk_size * (1 + clamp(ew_confidence - 0.75, 0, 0.25) * 0.5)
```

> 信心 0.90 → 尺寸≈ base × 1.075（受上限保護）

DSL 片段：

```yaml
size:
  mode: risk_based_conviction
  base_risk_pct: 0.5
  conviction_var: ew_confidence_5m
  cap_mult: 1.1
```

### 5.2 機器學習守門員（ML Gate）

* S2 追加特徵：`ew_*scores`, `invalidation_distance_bps`, `alt_counts`, `rvol`, `atr`, `trend_slope`…
* S9 訓練分類器（LogReg/XGBoost）：目標 = 未來 N 根內觸及 `RR>=1.5` 機率 `ml_win_prob`。
* S3 Gate：

```yaml
ml_gate:
  require: true
  threshold: 0.60
  feature_set: "ew_core_v1"
```

* S8 報表新增：`ml_gate_pass_rate`、`uplift_vs_no_gate`。

---

## 6) S10/Schema/Streams 變更摘要

### 6.1 新/增欄

* `positions_snapshots`: `state`(enum), `state_ts`, `forbid_add_on`, `ew_confidence`, `ml_win_prob`
* `signals`: `ml_win_prob`（當前事件時的估計）
* `config_bundles`: `wfo`, `scenarios`, `portfolio_limits`, `ml_gate`, `state_machine`

### 6.2 Redis Streams

* `pos:events`：`state_changed`, `pre_trade_check`, `gate_violation`, `data_quality_degraded`
* `metrics:*`：

  * `metrics:pos:state_dist`
  * `metrics:gate:reject_rate`
  * `metrics:data:quality_alerts`

---

## 7) 綜合策略範例（含狀態機 + Gate + Conviction + Guardian）

```yaml
strategy: "ew_trend_follow_v1_plus"
scope: { symbols: ["BTCUSDT","ETHUSDT"], tf: "5m" }

gates:
  portfolio_gate:
    checks:
      - type: correlation
        max_pair_rho: 0.7
        combine_risk_if_exceeds: true
      - type: strategy_budget
        cap_pct_nav: 0.25
      - type: net_delta
        range: [-0.3, 0.3]
    on_violation:
      - correlation: "prefer_higher_score"
      - strategy_budget: "downsize_to_fit"
      - net_delta: "skip_entry"
  ml_gate: { require: true, threshold: 0.60, feature_set: "ew_core_v1" }

entry:
  when:
    all:
      - wave_signal_5m == "end_of_2_confirmed"
      - wave_impulse_score_5m >= 0.75
      - rvol_5m >= 1.05
      - not higher_tf_conflict(["1h","4h"])
  side: follow_detected_trend
  price: maker_preferred
  size:
    mode: risk_based_conviction
    base_risk_pct: 0.5
    conviction_var: ew_confidence_5m
    cap_mult: 1.1
  risk:
    hard_stop: wave_invalidation_px_5m

state_machine:
  initial: ACTIVE_CONFIRMED
  transitions:
    - on: end_of_4_confirmed
      to: ACTIVE_WARNING
      action: { take_profit: 0.25, set_flag: forbid_add_on=true }
    - on: rr_reach_breakeven
      to: ACTIVE_REDUCED_RISK
      action: { move_stop_to: breakeven_or_last_pivot }
    - on: guardian_or_reverse
      to: PENDING_EXIT
      action: { take_profit: 1.0 }

exit:
  ladder:
    - kind: rr
      rr: 1.0
      action: { move_stop_to: breakeven_or_last_pivot }
    - kind: fib_leg
      leg: "W1"
      mult: 0.618
      action: { take_profit: 0.33, tighten_atr_to: 1.8 }
    - kind: fib_leg
      leg: "W1"
      mult: 1.0
      action: { take_profit: 0.33, tighten_atr_to: 1.5 }
    - kind: fib_leg
      leg: "W1"
      mult: 1.618
      action: { take_profit: 1.0 }
  trailing: { mode: atr, atr_mult_initial: 2.0, atr_mult_after_rr: { rr: 2.0, mult: 1.3 } }
  guardian_stop:
    higher_tf_override: true
    deny_if_higher_tf_signal_in: ["end_of_C_confirmed"]
  time_stop: { hours: 24 }
```

---

## 8) 上線動作清單（增量）

* [ ] **S10**：合入 `wfo/scenarios/portfolio_limits/ml_gate/state_machine` 區段；Lint 檢核 Gate 與 State 存在。
* [ ] **S3**：實作 `portfolio_gate`、`ml_gate`、`state_machine` 執行器；在意圖流中插 `pre_trade_check`。
* [ ] **S11**：新增 `pos:state_dist`、`gate:reject_rate`、`data:quality_alerts` 看板與告警。
* [ ] **S9**：建立 WFO 週期任務 + 場景壓測報表；結果自動推 `config_bundles` 灰度。
* [ ] **S2**：輸出 `ew_confidence`、`ml_win_prob`；更新 factor\_registry。
* [ ] **Runbook**：文件化 `kill_switch:*` 操作；演練一次全域停新倉。

---

要我把以上\*\*合併到一份完整 `config_bundles`（JSON）\*\*和三支策略 YAML（含狀態機與 Gate）一起打包給你嗎？或先從 S3 的「`portfolio_gate` + `state_machine`」優先實裝清單開始？


讚，改成「**只看 4h～1d，不低於 4h 才下單**」的高週期版。我把 S10 參數、S3 策略、Gate 與出場都調整成 HTF（Higher-Timeframe）配置，直接可拷貝。

---

# Project Chimera v3.2 — HTF（4h～1d）專用方案

## 0) 原則

* **只用 4h 與 1d 的波浪結論**做決策；**拒絕** 1m/5m/15m/1h。
* 進場以 **4h 為主**、**1d 當守門員**（不衝突/同向才下單）。
* 所有無效化價、TP、追蹤止損亦取自 4h/1d 的結構輸出。

---

## 1) S10 組態（HTF 版片段）

```json
{
  "wave_params": {
    "tf_set": ["4h","1d"],
    "min_score": 0.75,
    "fib_tol": 0.08,
    "pivot_k": 4,
    "zzz_threshold_bps": 35,
    "overlap_tolerance_bps": 0,
    "multi_tf_policy": "higher_tf_priority"   // 1d 優先否決
  },
  "entry": {
    "min_rvol": 1.00,
    "confirm_timeout_bars": 3,                 // HTF，容許較長等待
    "conflict_block_higher_tf": true           // 1d 反向時禁止
  },
  "exit": {
    "use_rr_breakeven": true,
    "rr_to_breakeven": 0.8,                    // HTF 價差大，稍早鎖盈
    "rr_tighten_at": 1.5,
    "atr_trail_initial": 2.2,
    "atr_trail_tight": 1.5,
    "tp_ladder": {
      "impulse": [0.5, 1.0, 1.618],
      "c_end":   [0.382, 0.618, 1.0],
      "triangle":[0.618, 1.0]
    },
    "triangle_measure_mult": 0.618,
    "time_stop_hours": 96,                     // 4h × 24根 = 4天上限觀察
    "guardian_stop": {
      "enable_higher_tf_override": true,
      "higher_tf_list": ["1d"]
    },
    "liquidity": { "twap_slices": 7, "max_slippage_bps": 4 }
  },
  "risk": {
    "base_size_usdt": 100,
    "risk_per_trade_pct": 0.6,                 // HTF 次數少、勝率更看品質
    "max_concurrent_positions": 4,
    "add_on_gate_rr": 1.1                      // HTF 允許較早加倉，但更小幅
  }
}
```

---

## 2) 進場邏輯（HTF 專用）

### 2.1 三種「對齊模式」（擇一）

* **模式 A：4h 主訊號 + 1d 守門**（建議起步）

  * 4h 觸發 `end_of_2_confirmed | end_of_C_confirmed | triangle_e_break`
  * 1d **不得**有反向強訊號（亦可要求同向加分）
* **模式 B：1d 主訊號 + 4h 加速/入場時機**

  * 1d 出結論，4h 同向或出「加速」訊號（如 `break_of_iii`）才進場
* **模式 C：雙TF 法定共識**

  * 4h 與 1d **同時/近鄰 3 根 4h 內**同向結論才進場（最嚴格、頻率最低）

> 建議先用 **模式 A**，穩定後再考慮 C。

---

## 3) S3 策略（HTF 趨勢範本：模式 A）

```yaml
strategy: "ew_trend_follow_htf_v1"
scope:
  symbols: ["BTCUSDT","ETHUSDT"]
  tf: "4h"

gates:
  # 只允許 4h 主決策，1d 為守門員
  portfolio_gate:
    checks:
      - type: correlation
        max_pair_rho: 0.7
        combine_risk_if_exceeds: true
      - type: strategy_budget
        cap_pct_nav: 0.25
      - type: net_delta
        range: [-0.35, 0.35]
  ml_gate: { require: false }  # HTF 可先關閉，穩定後再開

entry:
  when:
    all:
      - wave_signal_4h in ["end_of_2_confirmed","triangle_e_break"]
      - wave_impulse_score_4h >= 0.75 or wave_triangle_score_4h >= 0.75
      - rvol_4h >= 1.00
      - not higher_tf_conflict(["1d"])          # 1d 反向則禁止
      # 可選同向加分（開啟則更嚴）
      # - higher_tf_align(["1d"], allow_same_direction_only=true)
  side: follow_detected_trend
  price: maker_preferred
  size:
    mode: risk_based
    risk_pct: 0.6
    stop_ref: wave_invalidation_px_4h
  add_on:
    when:
      all:
        - rr_current >= 1.1
        - wave_signal_4h in ["break_of_iii"]    # 4h 加速才加倉
    size_mult: 0.33
    trail_after_add: true

risk:
  hard_stop: wave_invalidation_px_4h

state_machine:
  initial: ACTIVE_CONFIRMED
  transitions:
    - on: end_of_4_confirmed
      to: ACTIVE_WARNING
      action: { take_profit: 0.25, set_flag: forbid_add_on=true }
    - on: rr_reach_breakeven
      to: ACTIVE_REDUCED_RISK
      action: { move_stop_to: breakeven_or_last_pivot }
    - on: guardian_or_reverse   # 1d 出現反向強訊號
      to: PENDING_EXIT
      action: { take_profit: 1.0 }

exit:
  ladder:
    - kind: rr
      rr: 0.8
      action: { move_stop_to: breakeven_or_last_pivot }  # 早鎖盈
    - kind: fib_leg
      leg: "W1"
      mult: 0.5
      action: { take_profit: 0.33, tighten_atr_to: 2.0 }
    - kind: fib_leg
      leg: "W1"
      mult: 1.0
      action: { take_profit: 0.33, tighten_atr_to: 1.5 }
    - kind: fib_leg
      leg: "W1"
      mult: 1.618
      action: { take_profit: 1.0 }
  trailing:
    mode: atr
    atr_mult_initial: 2.2
    atr_mult_after_rr: { rr: 1.5, mult: 1.5 }
  guardian_stop:
    higher_tf_override: true
    deny_if_higher_tf_signal_in: ["end_of_C_confirmed"]  # 1d 反向強訊號直接否決
  time_stop: { hours: 96 }
```

---

## 4) 反轉策略（HTF：模式 A）

```yaml
strategy: "ew_reversal_c_end_htf_v1"
scope: { symbols: ["BTCUSDT"], tf: "4h" }

entry:
  when:
    all:
      - wave_signal_4h == "end_of_C_confirmed"
      - wave_zigzag_score_4h >= 0.75
      - fib_deviation_4h <= 0.08
      - not higher_tf_conflict(["1d"])
  side: reverse_of_c_direction
  price: maker_preferred
  size:
    mode: risk_based
    risk_pct: 0.6
    stop_ref: wave_invalidation_px_4h
risk: { hard_stop: wave_invalidation_px_4h }

exit:
  ladder:
    - kind: fib_prev_leg
      leg: "A_or_prev"
      mult: 0.382
      action: { take_profit: 0.4, move_stop_to: breakeven }
    - kind: fib_prev_leg
      leg: "A_or_prev"
      mult: 0.618
      action: { take_profit: 0.4, tighten_atr_to: 1.6 }
    - kind: rr
      rr: 1.8
      action: { take_profit: 1.0 }
  trailing: { mode: atr, atr_mult_initial: 2.0 }
  guardian_stop: { higher_tf_override: true, deny_if_higher_tf_signal_in: ["end_of_2_confirmed"] }
  time_stop: { hours: 96 }
```

---

## 5) 三角形突破（HTF）

```yaml
strategy: "ew_triangle_break_htf_v1"
scope: { symbols: ["BTCUSDT","SOLUSDT"], tf: "4h" }

entry:
  when:
    all:
      - wave_signal_4h == "triangle_e_break"
      - wave_triangle_score_4h >= 0.75
      - not higher_tf_conflict(["1d"])
  side: break_direction
  price: maker_preferred
  size:
    mode: risk_based
    risk_pct: 0.6
    stop_ref: wave_invalidation_px_4h
risk: { hard_stop: wave_invalidation_px_4h }

exit:
  ladder:
    - kind: measured_move
      base: "triangle_height"
      mult: 0.618
      action: { take_profit: 0.5, move_stop_to: breakeven }
    - kind: measured_move
      base: "triangle_height"
      mult: 1.0
      action: { take_profit: 1.0 }
  trailing: { mode: atr, atr_mult_initial: 2.0 }
  guardian_stop: { higher_tf_override: true, deny_if_higher_tf_signal_in: ["end_of_C_confirmed"] }
  time_stop: { hours: 96 }
```

---

## 6) S4 執行層建議（HTF 特化）

* **TWAP-R 預設開**（`twap_slices=7`）：HTF 部位較大、成交需控滑點。
* **SPOT**：OCO 結構照舊；**Guardian 事件**以 IOC/市價清倉。
* **加倉**：只在 4h 出 `break_of_iii`（加速）且 `rr_current ≥ 1.1` 時，**小幅**（≤ 初始 0.33×）。

---

## 7) S11 監控（HTF 指標）

* `htf:signal_count_4h/1d`、`htf:alignment_rate`（4h 與 1d 同向比率）
* `trade:avg_holding_hours`（應顯著高於 LTF）
* `exit:rr_at_final`（目標 ≥ 1.8；HTF 可期待更高）
* `guardian:trigger_rate_1d`（過高 → 調高 4h 門檻或改模式 C）

---

## 8) 小結 & 開關

* 要求「**不低於 4h**」：已在 `tf_set=["4h","1d"]` 且策略 `scope.tf="4h"`，同時 `conflict_block_higher_tf=true` 確保 1d 守門。
* 三個 HTF 策略（趨勢/反轉/三角）可先從 **趨勢** 開跑，再逐步開啟其餘兩個。

---

要不要我把這三支 HTF YAML + 一份 HTF 專用 `config_bundles`（含上面 JSON 片段）打包成一套給你直接貼進 S10？或者你想改成「模式 C（雙 TF 共識）」我也能馬上換好規則。


了解，你要的是「**如何在系統內實際判斷艾略特波浪**」，也就是 S2 Wave Engine 的演算法邏輯。這裡整理成**落地規格**，包含演算法步驟、檢查規則、輸出欄位，讓系統能產生「某段是 1 浪、2 浪…C 浪」這樣的結論。

---

# Elliott Wave 判斷流程（S2 Wave Engine 落地版）

## 1) 前置：資料來源

* **K 線**：OHLCV（至少 4h / 1d）
* **波動量**：ATR、RV（實現波動）、成交量
* **樞紐點**：Swing High/Low（`pivot_k=3~5`）

---

## 2) 基礎步驟

### Step 1. 樞紐點偵測

* 找出確立的高/低點：必須是 `k` 根前後蠟燭皆不超越的頂/底。
* 每個 pivot = `(ts, price, type=high/low, strength=k)`

### Step 2. ZigZag 框架

* 設定波幅閾值（`zzz_threshold_bps`，例：3.5%）
* 僅保留變動大於該閾值的 pivot → 得到「骨架段落」

### Step 3. 模板擬合

* 對 ZigZag 段序列跑模板比對：

  * **Impulse (1-2-3-4-5)**
  * **Zigzag (5-3-5)**
  * **Flat (3-3-5)**
  * **Triangle (3-3-3-3-3)**
  * **Double/Triple Combination**
* 每個模板有「理論長度比 + 重疊規則」

### Step 4. 規則驗證

* **硬規則**（若違反 → invalid）：

  * 2 不可回撤超過 1 起點
  * 3 不得最短
  * 4 不可與 1 重疊（加密市場可設定 bps 容忍）
  * Flat、Zigzag、Triangle 的 3/5/3 結構必須對齊
* **比例檢查**（有容差 `fib_tol`）：

  * 2 ≈ 0.382/0.5/0.618 × 1
  * 3 ≈ 1.618 × 1
  * 4 ≈ 0.236/0.382 × 3
  * 5 ≈ 0.618/1.0 × 1
* **時間/通道/交替原則**：用來加/減分，不會直接 invalid

### Step 5. 打分與輸出

* **score = (fib\_match + channel\_ok + time\_ratio + alternation)/4**
* **status = valid | ambiguous | invalid**
* **signal**：例如 `end_of_2_confirmed`, `end_of_C_confirmed`, `triangle_e_break`
* **invalidation\_px**：對應止損價位（例：2 不能跌破 1）

---

## 3) 輸出格式（Redis Stream）

```json
{
  "symbol": "BTCUSDT",
  "tf": "4h",
  "pattern": "impulse",
  "wave_labels": ["1","2","3","4","5"],
  "score": 0.82,
  "status": "valid",
  "fib_deviation": 0.07,
  "channel_ok": true,
  "alternation_score": 0.66,
  "signal": "end_of_2_confirmed",
  "invalidation_px": 58000.5,
  "ts_event": 1758350400000
}
```

---

## 4) 系統保證

1. **不可重繪**：只有在 pivot 確立後才標記，不會 retroactive 修改。
2. **多週期一致性**：必須跨 4h / 1d 對齊，1d 具否決權。
3. **狀態記錄**：每個倉位綁定「當時波浪標記」+「invalidation\_px」。

---

## 5) 如何在 S3 用

* **進場條件**：

  * `wave_signal_4h == "end_of_2_confirmed"`
  * `wave_impulse_score_4h >= 0.75`
  * `not higher_tf_conflict(["1d"])`
* **出場條件**：

  * `end_of_4_confirmed` → 減倉
  * Guardian（1d 出現反向訊號）→ 平倉

---

👉 簡單說：系統先用 ZigZag 找到「折返點」，再套公式檢查是不是符合 1-2-3-4-5 或 ABC/三角，最後打分並丟出 `signal` + `invalidation_px`。

收到！我把你的新規則「**哪個週期進場，就由同一週期決定出場**」＋「**初始金額=本金的 1/100**（再依信任度微調）」完整融入，給你可直接上 S10/S3 的純MD規格與 YAML 範本。

---

# HTF 一致性 + 1% 起始倉位（Elliott Wave｜Chimera v3.2）

## 0) 核心變更（重點）

1. **出入口同週期**：

* 4H 下單 → 只看 **4H** 的 `wave_signal`、`invalidation_px`、通道/背離 來出場。
* 1D 下單 → 只看 **1D** 的結構來出場。
* 跨週期（例如 1D）僅作為**資訊/告警**，**不強制**出場（可選緊急停用鍵）。

2. **倉位 sizing＝本金 1% 起算 + 信任度微調**：

* 基礎風險金額（現金）＝ `NAV / 100`。
* 最終風險金額＝ `base * conviction_mult`，其中 `conviction_mult` 由該週期的 `ew_confidence_[tf]` 計算（有上/下限）。
* **風險金額是最大可承受虧損**：依「入場價↔無效化價」距離轉換為數量，保證 hit SL 時 ≈ 1% NAV × 調整係數。

---

## 1) S10 參數（新增/修改）

```json
{
  "wave_params": {
    "tf_set": ["4h","1d"],
    "min_score": 0.75,
    "fib_tol": 0.08,
    "pivot_k": 4,
    "multi_tf_policy": "higher_tf_as_advisory"   // 1d 僅告警，不強制出場
  },
  "risk": {
    "base_risk_pct_of_nav": 1.0,                 // 起始=本金的 1/100
    "conviction_mult": {
      "lower": 0.8,                              // 信任度很低時最多降到 0.8×
      "upper": 1.25,                             // 信任度很高時最多升到 1.25×
      "pivot_confidence": 0.75,                  // 以 0.75 為中心
      "slope": 2.0                               // 每 +0.1 信任度 ≈ +0.2×mult
    },
    "add_on_gate_rr": 1.1,                       // 只要同 TF 結構加速才加倉
    "max_concurrent_positions": 4
  },
  "exit": {
    "use_rr_breakeven": true,
    "rr_to_breakeven": 0.8,                      // HTF 提前鎖盈
    "rr_tighten_at": 1.5,
    "atr_trail_initial": 2.2,
    "atr_trail_tight": 1.5,
    "time_stop_hours": 96,
    "guardian_stop": {
      "enable_higher_tf_exit_override": false,   // 不用更高TF強制平倉
      "warn_only_from_tf": "1d"
    }
  }
}
```

**Conviction Mult 計算建議（同 TF）**

```
let c = clamp( ew_confidence_[tf], 0.5, 0.95 )
mult = clamp( 1.0 + slope * (c - pivot_confidence), lower, upper )

# 例：c=0.90 → mult ≈ 1.0 + 2.0*(0.90-0.75)=1.30 → 再被 [0.8,1.25] 夾住 = 1.25
```

**數量公式（保證 hit SL ≈ 1% NAV × mult）**

```
risk_cash = NAV * (base_risk_pct_of_nav/100) * mult
stop_dist = |entry_px - invalidation_px_[tf]|
qty = risk_cash / stop_dist
```

---

## 2) S3 統一規格：**Anchor TF = Entry TF**

### 2.1 規則

* 進場策略需宣告 `anchor_tf ∈ {"4h","1d"}`。
* **所有出場條件（TP/結構事件/追蹤止損/時間止損）只讀取 `anchor_tf` 的 features**。
* 加倉與禁加倉、RR 鎖盈等，也依 `anchor_tf` 之事件判定。
* 其他 TF（例如 1D 對 4H）僅觸發 `WARNING` 事件，不影響倉位邏輯（除非手動開啟緊急鍵）。

### 2.2 狀態機（保留）

* `ACTIVE_CONFIRMED / ACTIVE_WARNING / ACTIVE_REDUCED_RISK / PENDING_EXIT / CLOSED / INVALIDATED`
* `WARNING` 的來源也只接受 **anchor\_tf** 的事件（例如 4H 的 `end_of_4_confirmed`）。

---

## 3) 4H 趨勢策略（**4H 進、4H 出**）

```yaml
strategy: "ew_trend_follow_4h_anchor_v1"
scope: { symbols: ["BTCUSDT","ETHUSDT"], tf: "4h" }
anchor_tf: "4h"

entry:
  when:
    all:
      - wave_signal_4h == "end_of_2_confirmed"
      - wave_impulse_score_4h >= 0.75
      - rvol_4h >= 1.0
  side: follow_detected_trend
  price: maker_preferred
  size:
    mode: risk_by_invalidation
    # ←— 這三個就能實現「本金1%起算＋信任度微調」
    base_risk_pct_of_nav: 1.0
    conviction:
      var: ew_confidence_4h
      lower: 0.8
      upper: 1.25
      pivot: 0.75
      slope: 2.0
    stop_ref: wave_invalidation_px_4h

risk: { hard_stop: wave_invalidation_px_4h }

state_machine:
  initial: ACTIVE_CONFIRMED
  transitions:
    - on: end_of_4_confirmed   # 4H 的事件
      to: ACTIVE_WARNING
      action: { take_profit: 0.25, set_flag: forbid_add_on=true }
    - on: rr_reach_breakeven
      to: ACTIVE_REDUCED_RISK
      action: { move_stop_to: breakeven_or_last_pivot }
    - on: channel_break_down   # 4H 的事件
      to: PENDING_EXIT
      action: { take_profit: 1.0 }

exit:
  # 全部僅看 4H
  ladder:
    - kind: rr
      rr: 0.8
      action: { move_stop_to: breakeven_or_last_pivot }
    - kind: fib_leg
      tf: "4h"
      leg: "W1"
      mult: 0.5
      action: { take_profit: 0.33, tighten_atr_to: 2.0 }
    - kind: fib_leg
      tf: "4h"
      leg: "W1"
      mult: 1.0
      action: { take_profit: 0.33, tighten_atr_to: 1.5 }
    - kind: fib_leg
      tf: "4h"
      leg: "W1"
      mult: 1.618
      action: { take_profit: 1.0 }
  trailing:
    mode: atr
    tf: "4h"
    atr_mult_initial: 2.2
    atr_mult_after_rr: { rr: 1.5, mult: 1.5 }
  time_stop: { hours: 96 }

advisory:                                # 1D 僅告警，不出場
  warn_if_higher_tf_conflict: ["1d"]
  on_warn:
    set_tag: ["HTF_WARN"]
    no_position_action: "keep"           # 僅標記，不平倉
```

---

## 4) 1D 趨勢策略（**1D 進、1D 出**）

```yaml
strategy: "ew_trend_follow_1d_anchor_v1"
scope: { symbols: ["BTCUSDT"], tf: "1d" }
anchor_tf: "1d"

entry:
  when:
    all:
      - wave_signal_1d == "end_of_2_confirmed"
      - wave_impulse_score_1d >= 0.75
  side: follow_detected_trend
  price: maker_preferred
  size:
    mode: risk_by_invalidation
    base_risk_pct_of_nav: 1.0              # 起點 = 本金1%
    conviction:
      var: ew_confidence_1d
      lower: 0.85                           # 1D 更嚴格些
      upper: 1.2
      pivot: 0.78
      slope: 1.6
    stop_ref: wave_invalidation_px_1d

risk: { hard_stop: wave_invalidation_px_1d }

exit:
  ladder:
    - kind: rr
      rr: 0.8
      action: { move_stop_to: breakeven_or_last_pivot }
    - kind: fib_leg
      tf: "1d"
      leg: "W1"
      mult: 0.5
      action: { take_profit: 0.33, tighten_atr_to: 2.0 }
    - kind: fib_leg
      tf: "1d"
      leg: "W1"
      mult: 1.0
      action: { take_profit: 0.33, tighten_atr_to: 1.6 }
    - kind: fib_leg
      tf: "1d"
      leg: "W1"
      mult: 1.618
      action: { take_profit: 1.0 }
  trailing:
    mode: atr
    tf: "1d"
    atr_mult_initial: 2.3
    atr_mult_after_rr: { rr: 1.5, mult: 1.6 }
  time_stop: { hours: 24*14 }              # 兩週觀察上限（可調）

advisory:
  warn_if_lower_tf_conflict: ["4h"]
  on_warn:
    set_tag: ["LTF_WARN"]
    no_position_action: "keep"
```

---

## 5) S4 下單層（要點）

* **期貨**：進場即下 `Reduce-Only` TP 階梯 + `Stop`（用 `wave_invalidation_px_[anchor_tf]`）。
* **數量**：由 `risk_cash / stop_dist` 算出張數，確保 hit SL ≈ 1% NAV × conviction。
* **加倉**：僅當 **同 anchor\_tf** 出現 `break_of_iii` 或等級加速，且 `rr_current ≥ 1.1`，每次 ≤ 初始倉位 0.33×。
* **跨 TF 告警**：只標註、不中斷出場邏輯。

---

## 6) 監控（S11）

* `pos:anchor_tf_dist`（4h vs 1d 倉位占比）
* `risk:risk_cash_per_trade_p95`（應 ≈ 1% NAV × median(mult)）
* `exit:by_tf`（4h 出場與 1d 出場的勝率 / 平均 RR）
* `advisory:warnings_rate`（HTF/LTF 告警比率，僅供觀察）

---

## 7) 快速驗收清單

* ✅ 4H 單：只因 4H 事件出場；1D 只做告警不平倉。
* ✅ 1D 單：只因 1D 事件出場；4H 只告警。
* ✅ 每筆單 hit SL 最大虧損 ≈ `1% NAV × conviction_mult`。
* ✅ Conviction 來源＝**同 TF 的 `ew_confidence_[tf]`**。
* ✅ Canary（1% symbols）跑 2 週，觀察 `risk_cash_p95` 是否貼近 1% NAV。

---

如果你要，我可以把這兩支策略 YAML＋一段\*\*計算下單數量的 S3 伺服端片段（Go/TS偽碼）\*\*也補上，直接保證「本金 1% 起步＋同 TF 出場」的行為完全一致。你要 Go 還是 TypeScript 版本？
