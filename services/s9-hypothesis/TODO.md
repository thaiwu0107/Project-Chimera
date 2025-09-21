# S9 Hypothesis Orchestrator — 補充說明（Engineering Spec v3）

> 本補充文件以既有 S9 說明為基礎，將「假設→回測/實驗→統計檢定→產出決策建議」的工程面細化為可實作的分解任務與契約，供後端、數據科學與前端共同落地。

---

## 1. 角色與目標

* 角色：研究/治理中台。負責將人類或系統產生的「假設」系統化驗證，輸出可被 S10 配置中心採用的證據與建議。
* 目標：在不影響線上交易的前提下，離線批量完成：

  1. 資料抽取與樣本切分，
  2. Walk-Forward / Purged K-Fold 回測，
  3. KPI 計算與統計檢定（含 FDR 控制），
  4. 結果落庫與事件通知，
  5. 與 S10 的「模擬＋敏感度」結果對齊，形成最終 Promote 參考。

---

## 2. 邊界與非目標

* S9 不直接觸發線上策略切換（不修改 `config_active`）；僅寫入 `experiments`、更新 `hypotheses.status`、發出 `ops:events` 通知。
* 模型推論線上服務由 S3/（可選 S9 inference 子模組）承擔；S9 主要進行離線訓練與評估。
* S9 不負責產出最終可上線 Bundle；建議提交至 S10 形成 DRAFT，再走 Lint/Dry-run/模擬＋敏感度與 Promote。

---

## 3. 依賴、資料模型與索引

### 3.1 讀寫關係

* 讀：`signals`、`labels_12h/24h/36h`、`fills`、`funding_records`、`strategy_rules`、`factor_registry`、`config_bundles`（作為參考）
* 寫：`experiments`、更新 `hypotheses(status)`、（可選）`experiments_metrics_ts`（衍生時序）

### 3.2 Collections 與索引（摘錄）

* `hypotheses(hypothesis_id)`

  * 索引：hash(hypothesis\_id)、skiplist(created\_at)、skiplist(status)
* `experiments(exp_id)`

  * 索引：hash(exp\_id)、skiplist(created\_at)、skiplist(hypothesis\_id)、skiplist(result)
* 與回測關聯的讀表（參考）

  * `signals`: hash(signal\_id), skiplist(t0, symbol)
  * `labels_*`: hash(signal\_id), skiplist(computed\_at)
  * `fills`: hash(fill\_id), skiplist(timestamp, order\_id)
  * `funding_records`: skiplist(funding\_time, symbol)

---

## 4. Redis（Cluster）鍵與 Streams

* 任務佇列（研究排程）

  * `research:q:bt`（List / Stream，預設 Stream）：回測任務投遞
  * `research:bt:last_run_ts`（String）：最後一次批次啟動時間
  * `research:bt:progress:{exp_id}`（Hash）：任務進度（% / 當前窗編號等）
* 事件

  * `ops:events`（XADD Stream）：實驗開始/完成/失敗、假設狀態變更
* 線上健康

  * `health:s9:state`（String/Hash）：S9 健康摘要，供 S11 聚合

---

## 5. API 介面（對外/被代理；皆需 `GET /health`）

* POST `/experiments/run`
  Request：`ExperimentRunRequest{hypothesis_id?, window, wf_params?, pkfold_params?, kpi_set, seed}`
  Response：`ExperimentRunResponse{exp_id, status=QUEUED|RUNNING|DONE|FAILED}`
* GET `/experiments/{exp_id}` → 單次詳情（含 KPI、顯著性、FDR 校正後結論、資料範圍）
* GET `/experiments/history?hypothesis_id=...&limit=...`
* POST `/models/train`（可選）→ 觸發離線訓練，返回 `model_id`
* GET `/health` → `HealthResponse{Status, Checks{db,redis,queue,workers}}`

備註：S12 以 API Gateway 代理至 S9；內部型別沿用 `internal/apispec/spec.go`。

---

## 6. 批次/排程（Cron）

* 每日/每週回測批次（讀 `hypotheses.status=PENDING`，或 `AUTO` 類型）
* 每日模型重訓（可選）：條件觸發（AUC/Brier 劣化）
* 月度「線上/離線 TCA 偏差」校準任務：比較 `fills` 估計滑價與回測假定參數，偏差 > 20% 觸發告警

---

## 7. 回測與統計方法（可落地公式）

### 7.1 Walk-Forward（WF）

* 時間切片：`[train_1 -> test_1], [train_2 -> test_2], ...`，訓練窗與前推窗滑動或擴增
* 每窗流程：特徵/標籤抽取 →（可選）模型訓練 → 用 test 窗重放規則/模型 → KPI
* 聚合：對所有 test 窗聯集計算最終 KPI（Sharpe, Sortino, PF, Hit, MaxDD, CAGR 等）

### 7.2 Purged K-Fold（PKF）

* 時序 K 折，移除（purge）鄰近樣本避免資訊洩漏；可設 `embargo_pct` 留白
* 對每折計算 KPI，取均值與置信區間

### 7.3 KPI（核心公式）

* CAGR：`CAGR = (1 + R_tot)^(year/days) - 1`
* Max Drawdown：`MaxDD = max_t (1 - Equity_t / max_{τ≤t} Equity_τ)`
* Calmar：`Calmar = CAGR / |MaxDD|`
* Sharpe（日頻）：`Sharpe = (μ_r / σ_r) * sqrt(365)`（若 4h，年化因子換用 `sqrt(6*365)`）
* AUC（近線上一致性評估）：ROC 面積；Brier：`(1/N) * Σ (p_i - y_i)^2`

### 7.4 統計檢定與 FDR

* 顯著性：Mann-Whitney U-test / bootstrap CI（WinRate、Net ROI 分布）
* FDR（Benjamini-Hochberg）：排序 p 值 `p_(i)`，找最大 `i` 使 `p_(i) <= (i/m) * α`；接受前 i 個假設
* 效果量：Cliff’s delta / Cohen’s d（對稱分布時）

---

## 8. 端到端流程（Hop-by-Hop）

1. 任務入列

* 來源：S12（研究 UI）或排程器
* 動作：XADD `research:q:bt`，寫 `experiments(exp_id, RUNNING)`；發 `ops:events{EXP_STARTED}`

2. 樣本裝載

* 從 Arango 依 `window`, `symbols`, `horizons` 拉 `signals` + `labels_*` + 交易成本（`fills/funding_records`）

3. 策略重放

* 規則：依「指定 bundle / 規則集」重放
* 模型：可指定 `model_id` 或每窗內訓；推論結果映射 `size_mult`、`skip_entry` 等

4. KPI 與檢定

* 對 test 聯集計算 KPI；進行顯著性、FDR 校正；生成總結

5. 結果落地與狀態機

* 寫 `experiments{result=PASS|FAIL|INCONCLUSIVE, metrics{...}}`
* 更新 `hypotheses.status=CONFIRMED|REJECTED`（或 `VALIDATING` → `CONFIRMED/REJECTED`）
* 發 `ops:events{EXP_DONE, status, key_metrics}`

6. 與 S10 對齊（可選）

* 若 `CONFIRMED`：自動起一個 `config_bundles` 的 DRAFT（僅寫 DB，不上線）
* 通知風控/研究在 S12 進行模擬＋敏感度與 Promote

---

## 9. 觀測性與告警

* 指標（輸出至 S11）：

  * `bt_runs_success_total`, `bt_runs_fail_total`, `bt_window_latency_ms_p50/p95/p99`, `samples_per_run`, `auc/brier` 分布
* 告警：回測失敗率過高、資料抽取異常、AUC/Brier 劣化、TCA 偏差超閾值
* 健康：`GET /health` 聚合 Arango/Redis/Workers 狀態；同步寫入 `health:s9:state`

---

## 10. 契約與冪等

* 冪等鍵：`exp_id`（若同一 `hypothesis_id` 與 `window` 重複提交，回覆同 `exp_id`）
* 失敗恢復：`RUNNING` 超時自動標記 `FAILED` 並可重試；進度落 `research:bt:progress:{exp_id}`

---

## 11. 測試矩陣（Contract / E2E）

* Happy path：WF + KPI + 檢定 + FDR → `CONFIRMED`
* Edge 1：樣本不足 → 回應 `INCONCLUSIVE`，`experiments.result=INCONCLUSIVE`
* Edge 2：Arango 讀取中斷 → 任務 `FAILED`，事件 `EXP_FAILED`
* Edge 3：多任務併發 → 每任務不共享狀態，進度獨立
* Edge 4：與 S10 結果對齊 → 指標差異 ≤ 容忍度（回測一致性檢查）

---

## 12. 與其他服務整合

* 來源（讀）：`signals/labels/fills/funding_records`（S1/S2/S4/S7 寫入）
* 去向（寫）：`experiments`、`hypotheses(status)`（供 S10/S12 顯示與審批）
* 事件：`ops:events`（供 S12 通知，S11 審計）
* 一致性：月度與 S10 模擬＋敏感度結果對齊；若偏差>門檻，提示研究校準或資料修復

---

## 13. 交付與 DoD

* API：`POST /experiments/run`, `GET /experiments/{id}`, `GET /experiments/history`, `GET /health`
* 批次：每日/每週回測、月度 TCA 偏差校準
* 指標：Prom/Grafana 可見；告警策略已接入
* 資料：`experiments`/`hypotheses`/Redis 事件與進度可追溯
* 文檔：研究使用手冊（如何撰寫假設、選窗、讀結果）與前端串接說明

---

## 14. 待辦（高→低）

1. WF / PKF 引擎（含 Purge/Embargo、KPI 聚合）
2. 抽樣/載入層（Arango → DataFrame/Arrow，避免記憶體爆量）
3. 檢定 + FDR 模組（U-test、Bootstrap、BH）
4. REST API + 研究佇列（XADD）與 Workers（多併發）
5. 指標/健康/告警接入 S11
6. 與 S10 的結果對齊與差異報表

---

以上補充對 S9 的功能、資料、流程、數學與契約做了「可直接開發」的層面化定義，與你現有的 S9 說明保持一致並進一步工程化落地。
