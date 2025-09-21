# S8 Autopsy — 交易復盤服務補充說明（Project Chimera）

> 角色：**交易「驗屍官」**。對每筆已結束或被標記關注的交易，產出可追溯、可解釋、可比對的復盤報告，輸出為結構化 JSON 與可讀報表（HTML/PDF）。

實作狀態：**未實作**（僅基礎骨架）。本文件補齊：資料來源/去向、演算法公式、API、Redis Streams 與 Keys、排程、冪等/重試、觀測性與測試要點。

---

## 1. 職責與邊界

* **輸入**：已結束（或停滯）的交易 `trade_id` / `position_id`；其關聯的 signals、orders、fills、positions、funding、labels\_\*、metrics。
* **處理**：

  * **TCA**：滑價/費用/資金費分解；名目與相對（bps）雙尺度。
  * **Peer 對比**：同 cohort（regime/方向/名目桶/市場）分位與異常度。
  * **反事實**：SL/TP/size/時間偏移（±Δ）快速重放，估計 ΔROI。
  * **敘事摘要**：將規則命中與（可選）SHAP 轉為自然語言 TL;DR。
* **輸出**：`autopsy_reports`（ArangoDB）、報表物件（MinIO `autopsy/<trade_id>.html|pdf`）、事件 `strategy_events(AUTOPSY_DONE)` 與核心 SLI。

**非職責**：不直接改變持倉或觸發下單（僅寫報告與指標）。

---

## 2. 上/下游與資料流（何時入 DB / Redis）

### 2.1 消費（Redis Streams / DB 查詢）

* 〔Stream, 必讀〕`pos:events`

  * **事件**：`EXIT`, `STAGNATED`（停滯交易），`FORCE_CLOSED`
  * **動作**：入佇列 `autopsy:queue` 以 `trade_id` 去重，觸發生成。
* 〔DB, 查詢〕`signals`（含 `features`、`decision`、`config_rev`）
* 〔DB, 查詢〕`orders`, `fills`（含 `mid_at_send`, `book_top3`, `slippage_bps`）
* 〔DB, 查詢〕`positions_snapshots`（ROE 時序/加倉次數/均價）
* 〔DB, 查詢〕`funding_records`（FUT 資金費）
* 〔DB, 查詢〕`labels_12h/24h/36h`（已回填標籤）
* 〔DB, 查詢〕`strategy_events`（過程事件與版本切換）

### 2.2 產生（DB / MinIO / Redis）

* 〔DB, Upsert〕`autopsy_reports`

  * `trade_id`, `tl_dr`, `entry_snapshot`, `trajectory`, `exit_analysis`, `peer_comparison`, `tca`, `counterfactual`, `generated_at`
* 〔Obj, Put〕MinIO：`autopsy/<trade_id>.html|pdf`
* 〔Stream, 發佈〕`strat:events` with `kind=AUTOPSY_DONE`, `trade_id`
* 〔Metrics, 追加〕`metrics:ts:s8.autopsy_latency_ms`, `s8.autopsy_errors_total`（S11 聚合）
* 〔Alerts, 可選〕`alerts:stream`（嚴重異常或資料缺口）

> **寫入時機**：
>
> * TCA/Peer/反事實計算**全部完成** → 單次 Upsert `autopsy_reports`；失敗重試採冪等（同 `trade_id` 覆蓋）。
> * 報表產出後再發 `AUTOPSY_DONE`，避免消費端讀到半成品。

---

## 3. Redis Keys / Streams（命名與存取語義）

* `autopsy:queue`（**XADD**）

  * 欄位：`trade_id`, `reason` ∈ `{EXIT,STAGNATED,MANUAL}`，`ts`（ms）
  * **用途**：待處理佇列；後台 worker 消費（XREADGROUP）
* `autopsy:dedup:<trade_id>`（**SETNX**，TTL=24h）

  * **用途**：去重；並發觸發只算一次
* `metrics:ts:s8.autopsy_latency_ms`（**HINCRBY / XADD**）

  * 欄位：`trade_id`, `ms`；由 S11 彙整
* `alerts:stream`（**XADD**）

  * 欄位：`severity`, `source=s8`, `message`, `trade_id?`
* `cfg:active:rev`（**GET**）

  * 讀當下策略版本，供敘事摘要引用

---

## 4. API（對外/對內）

* `GET /health`：標準健康檢查（含依賴 Redis/Arango/MinIO 狀態）
* `POST /autopsy/{trade_id}`（同步排程）

  * **Request**：`{ "reason": "EXIT|STAGNATED|MANUAL", "force_rebuild": false }`
  * **Effect**：寫 `autopsy:queue`；若 `force_rebuild` 跳過去重
* `GET /autopsy/{trade_id}`（查詢）

  * **Response**：`autopsy_reports` 文件＋物件 URL（若存在）
* `POST /autopsy/rebuild-batch`（批次重建）

  * **Request**：`{ "from":"iso", "to":"iso", "reasons":["EXIT","STAGNATED"] }` → 災後重建

---

## 5. 計算方法（公式與細節）

### 5.1 TCA（交易成本分析）

* **單筆滑價（bps）**

  $$
  \text{slip\_bps} = dir \cdot \frac{P_{\text{fill}}-P_{\text{mid@send}}}{P_{\text{mid@send}}}\times 10^4
  $$
* **交易成本占比**

  $$
  \text{cost\_share}=\frac{\sum \text{Fees}+\sum \text{Funding}+\sum|\text{Slippage}|}{|\text{PnL}_{\text{gross}}|+\varepsilon}
  $$
* **分解**：Maker/Taker 成交占比、OCO/守護停損觸發率、TWAP 片數與片均滑價

### 5.2 Peer 對比（同類分位）

* **分組**：`regime`×`dir`×名目桶（e.g. 0–200、200–1k、1k+ USDT）
* **指標**：`winrate`, `ROI_net`, `holding_duration`, `slip_bps`, `fees_share`
* **百分位名次（ties）**

  $$
  \text{PctRank}(X)=\frac{C_L + 0.5\cdot C_E}{N}
  $$

### 5.3 反事實分析（快速重放）

* **TP/SL/Size 擾動**：對向量 $\theta=\{tp,sl,size\}$ 施加 $\pm \Delta$
* **ΔROI 估計**：以 fills 序列與 `positions_snapshots` 重建路徑；對守護停損/OCO 以觸發條件回放

  $$
  \Delta ROI = ROI(\theta+\Delta) - ROI(\theta)
  $$

### 5.4 敘事生成（模板 + 規則/SHAP）

* **摘要示例**：
  `本次入場置信度 0.78（主要來自「低波動」+0.12、「強相關」+0.05；資金費偏高 -0.08）。Maker 等待 450ms 後 70% 成交，剩餘以市價完成。最終 ROI_net 1.2%，成本占比 19%。屬 NORMAL/多倉/名目 500–1k 分位第 62 百分位。`

---

## 6. 任務與排程

| 任務         | 觸發                 | 輸入                           | 輸出         | 冪等/重試                                            |                                                        |
| ---------- | ------------------ | ---------------------------- | ---------- | ------------------------------------------------ | ------------------------------------------------------ |
| Autopsy 生成 | \`pos\:events(EXIT | STAGNATED)`、`POST /autopsy\` | `trade_id` | Upsert `autopsy_reports`；MinIO 報表；`AUTOPSY_DONE` | `SETNX autopsy:dedup:<id>`；失敗寫 `alerts` 並重試（退避+jitter） |
| 缺圖/缺段修復    | 每小時                | DB/物件清單                      | 重建缺失       | 以 `trade_id` 為鍵逐一重建                              |                                                        |
| 批次重建       | 手動/排程              | 時窗                           | 重算         | 控制併發，阻擋重覆任務                                      |                                                        |

---

## 7. 冪等、重試、補償

* **冪等**：`trade_id` 為唯一鍵；`autopsy:dedup:<trade_id>` 作為 24h 窗口內的重入保護。
* **重試**：內部步驟（DB/MinIO）各自最多 3 次；全流程最多 2 次；退避 `min(max, base*2^k)+jitter`。
* **補償**：只要核心 JSON 已 Upsert 成功但物件生成失敗 → 允許補償性重建報表，不覆蓋 JSON 核心。

---

## 8. 設定（由 S10 管理，熱載）

* `autopsy.cohort.buckets`：名目分桶邊界
* `autopsy.counterfactual.grid`：Δ 取值集合與上限
* `autopsy.narrative.templates`：TL;DR 模板（i18n）
* `autopsy.guardrails`：執行時長上限、最大反事實組合數、資料缺口容忍度

> 所有設定**只讀**於 runtime，透過 `cfg:events` 熱載，並記錄 `config_rev` 進報告 metadata。

---

## 9. 觀測性（SLI/SLO）與日誌

* **SLI**：

  * `autopsy_latency_ms`（P50/P95）
  * `autopsy_error_rate`
  * `report_size_bytes`
  * `peer_cohort_coverage`（當期 cohort 有效樣本占比）
* **SLO（建議）**：

  * `P95 lat ≤ 5s`（單筆）
  * `error_rate ≤ 0.5%/day`
* **日誌**：每步驟帶 `trace_id`、`trade_id`；輸出關鍵參數（tp/sl/size/Δ）與分支（OCO/守護）

---

## 10. 測試與驗證

* **單元測試**：

  * TCA：滑價/成本占比公式邊界（零費用、極端滑價）
  * Peer：分位計算含 ties、空 cohort 的降級策略
  * 反事實：TWAP/OCO/守護觸發的路徑回放一致性
* **契約測試**：

  * `POST /autopsy/{trade_id}` → 佇列寫入/去重
  * `GET /autopsy/{trade_id}` → 與 DB 一致、URL 存在
* **整合測試**：

  * 以錄製的 `signals/orders/fills` 範例重演，產出穩定 **Golden Report**（Hash 校驗）

---

## 11. 故障/邊界處理清單

* **資料缺口**：

  * 缺 `mid_at_send` → 用鄰近 tick 補插（記 `data_imputed=true`）；超過窗口 → 降級僅 JSON 無報表
* **OCO 一腿失敗**：按 `strategy_events` 重建決策分支，標註「守護升級」
* **SPOT WAC**：以 `holdings_spot_snapshots` 重建成本，已/未實現分開統計
* **多幣種（僅 USDT 現階段）**：估值統一 USDT；匯率場景保留擴充掛鉤點

---

## 12. 序列化（核心步驟）

1. **觸發**：`EXIT|STAGNATED|MANUAL` → `autopsy:queue`；`SETNX autopsy:dedup:<id>`
2. **讀**：拉取 `signals/orders/fills/positions_snapshots/funding_records/labels_*`
3. **算**：

   * TCA：滑價/費用/資金費；Maker/Taker 占比
   * Peer：cohort 分位
   * 反事實：Δ grid 快速重放（限制組合上限）
   * 敘事：TL;DR
4. **寫**：Upsert `autopsy_reports`；生成 HTML/PDF → MinIO
5. **發**：`strategy_events(AUTOPSY_DONE)`；寫指標
6. **監控**：失敗 → `alerts:stream`

---

## 13. 依賴與資源

* **必需**：ArangoDB、Redis Cluster、MinIO、S10（配置）、S11（彙整）
* **建議**：Grafana 版面：`S8 Autopsy`（延遲/錯誤率/cohort 覆蓋）

---

## 14. DoD（完成定義）

* `POST /autopsy/{trade_id}` 可觸發；事件觸發亦可
* 10+ 筆樣本可生成報告；`autopsy_reports` 與 MinIO 物件一致
* 指標與告警上線；Golden Report 穩定再現
* 在 Canary（10% 交易）一週內錯誤率 < 0.5%，P95 < 5s

---

*附註：本文聚焦 S8 的落地實務，與既有白皮書 v3 的資料模型與 Redis 命名規範一致；細節以 `config_bundles` 的 `autopsy.*` 旗標為準，並沿用全域 `config_rev` 追蹤來源版本。*
