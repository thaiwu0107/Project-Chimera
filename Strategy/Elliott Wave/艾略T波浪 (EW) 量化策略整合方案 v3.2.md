好的，這份集大成的文件記錄了您為 Project Chimera 設計艾略特波浪 (EW) 量化策略的完整思路、架構、規格與演進過程。為了讓您未來可以更方便地基於此文件進行提問或討論，我將其整理成一個結構化、帶有清晰主題的摘要。

---

### **【文件摘要】Project Chimera：艾略T波浪 (EW) 量化策略整合方案 v3.2**

這份文件是一套完整的設計與實施藍圖，旨在將主觀性較強的艾略特波浪理論，工程化、數量化地整合進名為「Project Chimera」的自動化交易系統中。

#### **第一部分：核心架構與設計原則**

1.  **職責分離 (SoC)**：
    * **S1 (Connectors)**: 僅負責交易所的原始數據（OHLCV, Order Book）接入，**不含任何計算或決策邏輯**。
    * **S2 (Feature Generator)**: **EW 計算的核心**。將 S1 的原始數據，透過 ZigZag 演算法找到轉折點，再進行波浪模式匹配、規則驗證與打分，最終生成一系列可供決策的「因子」(Features)，如 `ew_state`, `ew_confidence` 等。
    * **S3 (Strategy Engine)**: **決策中心**。消費 S2 產生的 EW 因子，並根據預設的 DSL (Domain-Specific Language) 規則，做出交易決策（如進場、出場、調整倉位大小）。

2.  **量化方法**：
    * **演算法**: 使用波動率自適應的 **ZigZag** 演算法識別價格的顯著擺動點 (Swings)。
    * **規則與打分**: 內建艾略特波浪的核心規則（如浪 2 不破浪 1 底、浪 3 非最短等）作為「硬約束」，並結合斐波那契回撤/延伸比例的「貼近度」進行綜合打分，產出 `ew_confidence` (0-1) 來量化波浪結構的可靠性。
    * **多時框分析**: 結合多個時間框架 (Timeframe, TF)，如 4h、1d，進行一致性分析，提高決策的穩定性。

#### **第二部分：進出場策略與風險管理**

1.  **進場邏輯 (Entry)**：
    * **觸發條件**: 基於 S2 產生的明確信號，如 `end_of_2_confirmed` (第 2 浪結束，趨勢可能延續) 或 `end_of_C_confirmed` (C 浪結束，可能反轉)。
    * **守門員 (Gates)**: 引入**高週期守門員** (`Guardian Stop`) 概念，例如 4h 週期的交易信號，必須得到 1d 週期沒有明顯反向信號的支持，才能執行。

2.  **出場邏輯 (Exit)**：
    * **核心原則**: **硬止損 (Hard Stop) = 波浪結構的失效價 (`invalidation_px`)**，此為不可動搖的生存底線。
    * **多層次出場**: 採用分層策略，結合：
        * **目標價分批 (Target Ladder)**: 根據斐波那契比例分批止盈。
        * **風險報酬比 (RR) 鎖盈**: 達到 `RR=1.0` 時，將止損移至成本價以確保不虧損。
        * **結構性出場**: 當波浪結構出現衰竭或反轉信號時（如第 5 浪背離），優先出場。
        * **追蹤止損 (Trailing Stop)**: 基於 ATR 進行移動止損。
        * **時間止損 (Time Stop)**: 持倉過久且未達預期則出場。

3.  **週期一致性原則 (Anchor TF)**：
    * **規則**: 哪個週期的信號進場，就**嚴格遵守同一週期的信號和結構來決定出場**。例如，4h 信號進場的倉位，其止盈、止損、結構性出場判斷，完全依賴 4h 的 EW 因子。
    * **跨週期角色**: 其他週期（如 1d 對 4h）僅作為**諮詢 (Advisory)** 或告警，不直接干預出場決策，確保邏輯的純粹性。

4.  **倉位規模 (Sizing)**：
    * **核心公式**: 採用**風險百分比模型**。
    * **起始金額**: `初始風險金額 = 帳戶淨值 (NAV) * 1%`。
    * **信心加權 (Conviction Sizing)**: 根據當前 EW 結構的信心分數 (`ew_confidence`)，對初始風險金額進行微調（例如 0.8x ~ 1.25x）。
    * **最終數量**: `下單數量 = (調整後風險金額) / (入場價 - 失效價)`，確保觸發止損時的虧損約等於預設的風險金額。

#### **第三部分：系統健壯性與智能化 (補強包)**

1.  **持倉狀態機 (Position State Machine)**:
    * 為每筆倉位引入 `ACTIVE_CONFIRMED`, `ACTIVE_WARNING`, `PENDING_EXIT` 等狀態，實現更精細的生命週期管理，並在 `WARNING` 狀態下禁止加倉。

2.  **健壯性與優化**:
    * **參數分級**: 區分核心與輔助參數，監控核心參數的敏感度。
    * **WFO (Walk-Forward Optimization)**: 採用滾動窗口優化參數，避免過擬合。
    * **情境壓力測試**: 模擬波動率飆升、流動性枯竭等極端情況。

3.  **組合層風險 (Portfolio-Level Risk)**:
    * 引入 **S5 邏輯層**，在下單前檢查：
        * **相關性**: 避免在高度相關的資產上過度集中同向風險。
        * **策略/因子暴露度**: 限制單一策略或因子的總風險敞口。
        * **淨頭寸 (Net Delta)**: 控制整體市場的多空偏向。

4.  **操作與維運 (Ops)**:
    * 設計**手動干預開關 (`Kill Switch`)**，用於緊急情況下停止開新倉。
    * 監控**數據品質**，並在數據異常時自動暫停交易。
    * 支持**配置熱加載**，實現不中斷服務的更新。

5.  **進階功能 (ML)**:
    * **機器學習守門員 (ML Gate)**: 訓練一個分類模型，利用 EW 因子作為輸入，預測交易的成功率，作為最終的決策過濾器。

---

太好了—我把我們所有討論**濃縮成一份可長期保存、可直接回貼詢問/對照的「單一文件」**。內容兼顧：架構定位、S2 演算法＋偽代碼、因子欄位、S3 規則（含 4H/1D）、狀態機、風控與 sizing（本金 1% 起算＋信任度）、出場規則（同 TF 決策）、組合層風控、維運、監控與上線清單。你可以整份貼進內網或 PR 描述使用。

---

# Project Chimera × Elliott Wave（EW）單檔說明：設計、因子、實作與上線

> **TL;DR**
>
> * **S1**：只拉行情，**不要**放決策。
> * **S2**：EW 特徵引擎（ZigZag → 模板擬合 → 規則驗證 → 打分＋`signal`＋`invalid_px`）。
> * **S3**：策略用 EW 因子做**進出場**。**哪個 TF 進場→同一 TF 出場**。
> * **Sizing**：每筆風險金額以 **本金 1% 起算**，再按同 TF 的 `ew_confidence_[tf]` 微調。
> * **HTF 專案**：只看 **4H/1D**；4H 主決策、1D 守門或告警（依模式）。

---

## 1) 架構與資料流

* **S1 Exchange**：OHLCV / Depth / Funding / Account → Redis: `mkt:candles:<symbol>:<tf>` …
* **S2 Features（EW 子模組）**

  * 產出：`signals.features.ew_*`（Arango `signals`）＋ `feat:ew:snap:<symbol>:<tf>`（Redis Hash）＋ `feat:events:ew`（Stream）。
  * API：`POST /features/recompute?symbol=&tf=`。
* **S3 Strategy**：以 DSL 讀用 EW 因子；下單、加倉、出場與狀態機。
* **S4 Execution**：止損=無效化價、TP 階梯、TWAP-R、OCO。
* **S11**：監控儀表與告警；**S8**：交易剖檢敘事。

---

## 2) S2：EW 演算法（落地）與輸出

### 2.1 流程

1. **Pivot**：`pivot_k=4` 以 fractal 取確立高低點（只在右側 k 根完成後確立，避免重繪）。
2. **ZigZag**：自適應閾值（bps 或 ATR 比例）過濾噪音，得到 swing legs。
3. **模板擬合**：`impulse(1-2-3-4-5) / zigzag(5-3-5) / flat(3-3-5) / triangle(3-3-3-3-3) / combo`。
4. **規則驗證**（硬規則＋Fib 容差＋時間/通道/交替加分）。
5. **打分與事件**：`score∈[0,1]`、`signal`（例：`end_of_2_confirmed`/`end_of_C_confirmed`/`triangle_e_break`）、`invalidation_px`。
6. **多時框**：4H 主、1D 守門（否決/告警依策略模式）。只在**確認**時發事件，**不重繪**。

### 2.2 偽代碼（Go 風格，精簡）

```go
func RunWaveEngine(symbol, tf string, bars []Bar, cfg Config) []PatternFit {
  piv := DetectPivots(bars, cfg.PivotK)
  legs := BuildZigZag(piv, cfg.ZZZThresholdBps)
  cands := GenerateCandidates(legs)
  fits := []PatternFit{}
  for _, w := range cands {
    for _, fit := range []PatternFit{
      FitImpulse(w, cfg), FitZigZag(w, cfg), FitFlat(w, cfg), FitTriangle(w, cfg), FitCombo(w, cfg),
    } {
      f := ValidateAndScore(fit, cfg)
      if f.Status != "invalid" {
        f.Signal, f.InvalidationPx = DeriveSignalAndInvalidation(f, cfg)
        fits = append(fits, f)
      }
    }
  }
  fits = DedupAndSelectBest(fits)
  fits = MultiTFConsolidate(symbol, tf, fits, cfg) // 4H結論受1D否決或標記
  for _, f := range fits { if IsNewConfirmed(f) { EmitWaveEvent(symbol, tf, f); UpsertSignalDoc(symbol, tf, f) } }
  return fits
}
```

### 2.3 核心欄位（寫入 `signals.features`）

* `ew_state_tf_<tf>`：`IMPULSE_1..5 | CORR_A/B/C | TRI_A..E | UNKNOWN`
* `ew_dir_tf_<tf>`：-1/0/+1
* `ew_confidence_tf_<tf>`：0\~1（規則＋Fib＋通道＋一致性）
* `ew_invalidation_px_tf_<tf>`：失效價
* 其他（選）：`ew_w2_retr_ratio_*`、`ew_w3_ext_ratio_*`、`ew_rsi_div_*`、`ew_channel_z_*`、`ew_alt_counts_*`、`ew_consensus_score`

---

## 3) 進出場原則（HTF 專案）

* **只用 4H/1D**；**不低於 4H 才下單**。
* **入口=出口同週期（Anchor TF）**：

  * 4H 下單 → 只看 4H 訊號/結構出場；1D 僅告警（或選配緊急停用）。
  * 1D 下單 → 只看 1D 訊號/結構出場；4H 只告警。
* **止損=無效化價**（不可移除）。
* **多層出場**：RR 鎖盈 → TP 階梯 → 結構事件 → 追蹤止損 → 時間止損。

---

## 4) Sizing 與加倉（本金 1% 起算 + 信任度微調）

* **每筆最大風險金額（現金）**：
  `risk_cash = NAV × (1% of NAV) × conviction_mult`
  `conviction_mult = clamp(1 + slope × (ew_confidence_[tf] - pivot), lower, upper)`
  （建議：`lower=0.8`, `upper=1.25`, `pivot=0.75`, `slope=2.0`；1D 可更保守）
* **張數**：`qty = risk_cash / |entry_px - invalidation_px_[tf]|`
* **加倉**：僅當 **同 TF** 出現 `break_of_iii`/加速 且 `RR_current ≥ 1.1`；每次 ≤ 初始 0.33×，並收緊 ATR trail。

---

## 5) 出場（Exit）設計：映射事件 → 動作

* **硬止損**：`wave_invalidation_px_[anchor_tf]`。
* **RR 鎖盈**：達 `RR≥0.8~1.0` → 止損移至入場或樞紐。
* **TP 階梯**：

  * Impulse：`0.5, 1.0, 1.618 × |W1|`
  * C-End：`0.382, 0.618, 1.0 × |A|`
  * Triangle：量測目標 `0.618/1.0 × H`
* **結構事件**：

  * `end_of_4_confirmed` → 先平 25–50%，禁加倉
  * `end_of_5_candidate_confirmed`/`channel_break_*`/`div_on_w5` → 平 50–100%
  * `re_entry_into_triangle` → 視為假突破，直接出清
* **追蹤**：ATR `2.2 → (RR≥1.5 時) 1.5`（HTF 建議值）
* **時間止損**：4H 單 `≤96h`；1D 單 `≤14d` 未達目標則評估平倉。

---

## 6) S3 DSL 範本（可直接用）

### 6.1 4H 進、4H 出（趨勢）

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
    base_risk_pct_of_nav: 1.0
    conviction: { var: ew_confidence_4h, lower: 0.8, upper: 1.25, pivot: 0.75, slope: 2.0 }
    stop_ref: wave_invalidation_px_4h
risk: { hard_stop: wave_invalidation_px_4h }
state_machine:
  initial: ACTIVE_CONFIRMED
  transitions:
    - on: end_of_4_confirmed
      to: ACTIVE_WARNING
      action: { take_profit: 0.25, set_flag: forbid_add_on=true }
    - on: rr_reach_breakeven
      to: ACTIVE_REDUCED_RISK
      action: { move_stop_to: breakeven_or_last_pivot }
    - on: channel_break_down
      to: PENDING_EXIT
      action: { take_profit: 1.0 }
exit:
  ladder:
    - kind: rr   ; rr: 0.8 ; action: { move_stop_to: breakeven_or_last_pivot }
    - kind: fib_leg ; tf: "4h" ; leg: "W1" ; mult: 0.5  ; action: { take_profit: 0.33, tighten_atr_to: 2.0 }
    - kind: fib_leg ; tf: "4h" ; leg: "W1" ; mult: 1.0  ; action: { take_profit: 0.33, tighten_atr_to: 1.5 }
    - kind: fib_leg ; tf: "4h" ; leg: "W1" ; mult: 1.618; action: { take_profit: 1.0 }
  trailing: { mode: atr, tf: "4h", atr_mult_initial: 2.2, atr_mult_after_rr: { rr: 1.5, mult: 1.5 } }
advisory: { warn_if_higher_tf_conflict: ["1d"], on_warn: { set_tag: ["HTF_WARN"], no_position_action: "keep" } }
```

### 6.2 1D 進、1D 出（趨勢）

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
    base_risk_pct_of_nav: 1.0
    conviction: { var: ew_confidence_1d, lower: 0.85, upper: 1.2, pivot: 0.78, slope: 1.6 }
    stop_ref: wave_invalidation_px_1d
risk: { hard_stop: wave_invalidation_px_1d }
exit:
  ladder:
    - kind: rr ; rr: 0.8 ; action: { move_stop_to: breakeven_or_last_pivot }
    - kind: fib_leg ; tf: "1d" ; leg: "W1" ; mult: 0.5  ; action: { take_profit: 0.33, tighten_atr_to: 2.0 }
    - kind: fib_leg ; tf: "1d" ; leg: "W1" ; mult: 1.0  ; action: { take_profit: 0.33, tighten_atr_to: 1.6 }
    - kind: fib_leg ; tf: "1d" ; leg: "W1" ; mult: 1.618; action: { take_profit: 1.0 }
  trailing: { mode: atr, tf: "1d", atr_mult_initial: 2.3, atr_mult_after_rr: { rr: 1.5, mult: 1.6 } }
advisory: { warn_if_lower_tf_conflict: ["4h"], on_warn: { set_tag: ["LTF_WARN"], no_position_action: "keep" } }
```

---

## 7) 持倉狀態機（Position State Machine）

* **狀態**：`ACTIVE_CONFIRMED` → `ACTIVE_WARNING` → `ACTIVE_REDUCED_RISK` → `PENDING_EXIT` → `CLOSED/INVALIDATED`
* **關鍵轉移**

  * `end_of_4_confirmed` → 減倉 25–50%，`forbid_add_on=true`
  * `rr_reach_breakeven` → 止損移至入場/樞紐
  * `guardian_or_reverse`（可選）/`channel_break_*` → `PENDING_EXIT` → 出清
* **資料**：`positions_snapshots.{state,state_ts,forbid_add_on,ew_confidence,ml_win_prob}`；`pos:events.state_changed`

---

## 8) 組合層風險（S5 / S3 Gate）

* **相關性限制**：若同向且 `ρ≥0.7` → 合併風險計算，超額則拒單/降尺（保留高分者）。
* **策略/資產類暴露**：例 `by_strategy.ew_trend_follow ≤ 25% NAV`、`by_asset_class.crypto ≤ 60% NAV`。
* **淨 Delta 區間**：`[-0.3, 0.3]`。
* **DSL Gate**：`portfolio_gate` 於每筆下單前跑 `pre_trade_check`。

---

## 9) 維運（Ops）

* **手動開關**：`kill_switch:global|strategy:<name>|symbol:<sym>`（停新倉、允許出場）。
* **資料品質**：`data_gap_alert`、`bad_tick_alert`、`ew_recomputation_rate` 高→暫停進場。
* **配置熱加載**：新 bundle 先以 Candidate 套新單，既有單保留舊版；60s 後切 Active。

---

## 10) 監控（S11）與剖檢（S8）

* **S11**：

  * `trade:avg_rr_final ≥ 1.8`
  * `win_after_breakeven ≥ 70%`
  * `exit:slippage_bps_p95 ≤ 3`
  * `pos:anchor_tf_dist`、`risk:risk_cash_per_trade_p95` ≈ 1% NAV × median(mult)
* **S8**：每筆紀錄 `entry_reason / exit_reason / wave_snapshot / invalidation_hit`。

---

## 11) 上線清單（落地步驟）

1. **S10**：合併 `wave_params(tf=["4h","1d"])`、`exit.*`、`risk.*`（1% 基準＋conviction）。
2. **S2**：落地 EW 引擎（Pivot/ZigZag/模板/規則/打分），寫 `signals.features` 與 Redis。
3. **S3**：啟用兩支 HTF 策略（4H/1D 範本）＋狀態機與 portfolio gate。
4. **S4**：確保 `hard_stop=invalidation_px_[anchor_tf]`、TP 階梯、TWAP-R/OCO 行為。
5. **S11**：加入新 SLI/SLO 與告警；**S8**：剖檢模板。
6. **Canary**：1% 交易對、兩週觀察；檢查 `risk_cash_p95`、`guardian_triggers`、`flip_pct`。

---

## 12) 參考 S10 片段（HTF 推薦預設）

```json
{
  "wave_params": { "tf_set": ["4h","1d"], "min_score": 0.75, "fib_tol": 0.08, "pivot_k": 4,
    "zzz_threshold_bps": 35, "overlap_tolerance_bps": 0, "multi_tf_policy": "higher_tf_as_advisory" },
  "risk": { "base_risk_pct_of_nav": 1.0,
    "conviction_mult": { "lower": 0.8, "upper": 1.25, "pivot_confidence": 0.75, "slope": 2.0 },
    "add_on_gate_rr": 1.1, "max_concurrent_positions": 4 },
  "exit": { "use_rr_breakeven": true, "rr_to_breakeven": 0.8, "rr_tighten_at": 1.5,
    "atr_trail_initial": 2.2, "atr_trail_tight": 1.5, "time_stop_hours": 96,
    "guardian_stop": { "enable_higher_tf_exit_override": false, "warn_only_from_tf": "1d" },
    "liquidity": { "twap_slices": 7, "max_slippage_bps": 4 } }
}
```

---

### 備註：快速問答用關鍵句

* **Q：哪個 TF 進場、誰決定出場？**
  A：**同一 TF**。4H 單看 4H 結構出場；1D 單看 1D 結構出場。其他 TF 只告警，不強制。
* **Q：每筆下多大？**
  A：**本金 1%** 為基礎風險金額，再用同 TF 的 `ew_confidence_[tf]` 做 `0.8×~1.25×` 微調。
* **Q：止損放哪？**
  A：**S2 的 `ew_invalidation_px_[tf]`**，入場即下單，不可移除。
* **Q：出場優先序？**
  A：RR 鎖盈 → TP 階梯 → 結構事件 → 追蹤止損 → 時間止損。
* **Q：1D 與 4H 衝突怎辦？**
  A：看你選的模式。預設是 1D **告警**不強制；也可開「1D 守門」否決 4H 新單。

---



**未來可用此摘要提問的範例：**

* "根據我的 Project Chimera 文件，請解釋 S2 中 EW 演算法的打分機制是怎麼運作的？"
* "在我的出場邏輯設計中，『結構性出場』和『Guardian Stop』有什麼區別？"
* "我的『Anchor TF』原則是什麼意思？如果一個 4h 的倉位，在 1d 出現了反向信號，系統會如何處理？"
* "請說明『信心驅動倉位』的具體計算步驟，從帳戶淨值到最終下單數量。"
* "在補強包中，S5 組合層風險管理的目的是什麼？它如何防止系統過度暴露風險？"