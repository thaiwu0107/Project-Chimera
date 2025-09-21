# S10 Config Service — 補充說明（工程就緒版）

> 本節把 **Project Chimera** 內 S10 的職責、資料模型、API、排程、守門與熱載機制一次補齊，供前後端與 S2/S3/S4/S6/S12 對齊實作。

---

## 1. 核心職責（What & Why）

* **配置生命週期管理**：因子 / 規則 / 標的以 **Bundle** 版本化（DRAFT→STAGED→ACTIVE），支援 **Canary / Ramp / Rollback**。
* **安全上線護欄**：提交前 **Lint**（白名單/依賴/值域）、**Dry-run**（近 N 天重放）、**模擬器**（差異估計）、**敏感度分析**（±ε 擾動→flip rate / Lipschitz）。
* **零停機熱載**：廣播 `cfg:events`；客戶端（S2/S3/S4/S6/S12）按 **RCU** 原子切換 `config_rev`。
* **運行守門**：Canary/Ramp 階段滾動監控 **MaxDD / AUC / Brier / 交易密度漂移**，越界即自動回滾。
* **審計與追溯**：所有動作寫入 `promotions / simulations / strategy_events`，可重放可回溯。

---

## 2. 進出資料（ArangoDB / Redis）

### 2.1 ArangoDB Collections（必備）

* `config_bundles`：Bundle 本體（factors/rules/instruments/flags…，含 lint/dryrun/sim summary）
* `config_active`：全域指標（`{ key:"active", rev, bundle_id, activated_at }`）
* `strategy_rules`：DSL 規則庫（供 Lint/模擬引用）
* `factor_registry` / `instrument_registry`：因子/標的註冊表（Lint 依賴）
* `simulations`：模擬與敏感度結果
* `promotions`：推廣/回滾記錄（含 guardrails 與最終結論）
* `strategy_events`：`PROMOTE / ROLLBACK / CONFIG_HOTLOAD` 等審計事件

> 索引：`config_bundles(bundle_id)`, `config_bundles(rev)`, `config_active(key) [TTL無]`, `simulations(sim_id)`, `promotions(promotion_id)`，及 `created_at`/`status` 常用查詢欄位的二級索引。

### 2.2 Redis（Cluster）Keys & Streams

* **事件廣播**

  * `cfg:events` *(Stream)*：`{rev,bundle_id,mode,actor,ts}`
  * `cfg:lock:promote` *(String/Mutex)*：推廣互斥鎖
* **即時狀態**

  * `cfg:active_rev` *(String)*：快取 `rev`（客戶端啟動快速讀取）
  * `health:system:state` *(String)*：S11 合成健康度（S3/S4/S6 會依此降級）
* **批次隊列**

  * `sim:queue` / `sim:result:<sim_id>` *(Stream)*：模擬任務/結果

> TTL：廣播事件保留 7–14 天，結果類 key 視容量 7–30 天。

---

## 3. API（對外）

> 所有 API 皆有 `GET /health`。`/ready` 用於 K8s **Readiness**。

### 3.1 配置管理

* `GET /active` → `ActiveConfigResponse { bundle_id, rev, activated_at }`
* `POST /bundles` → 建立/更新 **DRAFT**（冪等鍵：`bundle_id` + `rev`）
* `POST /bundles/{id}/stage` → 進入 **STAGED**（需先通過 Lint/Dry-run）
* `POST /promote` → 進入 **CANARY / RAMP / FULL / ROLLBACK**

### 3.2 模擬與敏感度

* `POST /simulate` → 觸發 **差異重放 + 敏感度**（可附 `epsilon, topk, window`）
* `GET /simulations/{sim_id}` → 取結果（`summary / approx_effect / sensitivity_analysis`）

> **安全**：`/promote` 僅允許具備 `release-manager` 角色。

---

## 4. 內部流程（Hop-by-Hop）

### 4.1 Lint（提交即觸發）

* **Schema 完整**：`rule_id / priority / applies / when / action / status`
* **白名單**：比較/集合/布林運算子；動作與數值邊界（`size_mult∈[0.5,1.2]`…）
* **依賴**：`when.f` 皆存在於 `factor_registry(status=ENABLED)`；`applies.symbols` 存在於 `instrument_registry`
* **衝突檢查**：短路規則（`skip_entry=true` 優先）、合成後 clamp

### 4.2 Dry-run（近 N 天 `signals` 重放）

* 產出：`skip_rate / size_mult>1 比率 / policy shift Jaccard / 交易密度變化`
* **門檻**（示例，可配置）：

  * `skip_rate ≤ 0.60`、`size_mult>1 ≤ 0.50`
  * `policy_shift_jaccard ≥ 0.35`（避免過猛偏移）

### 4.3 模擬器（差異估計）

* 對 `signals` 套新 Bundle：比較 `trades_ref vs trades_new`、`delta_trades_pct`、規則命中分布、覆蓋率
* 產出樣例：

  ```json
  {
    "summary": { "trades_ref": 50, "trades_new": 65, "delta_trades_pct": 0.30 },
    "approx_effect": {
      "winrate_ref":0.52, "winrate_new":0.55,
      "net_roi_med_ref":0.007, "net_roi_med_new":0.009
    }
  }
  ```

### 4.4 敏感度分析（±ε 擾動）

* **決策翻轉率**：`flip_pct = # { d(x) ≠ d(x+δ) } / N`
* **Local Lipschitz 近似**（連續輸出如 `size_mult`）：

  $$
  L \approx \operatorname{median}\Big(\frac{|f(x+\delta)-f(x)|}{\|\delta\|}\Big),\ \ \|\delta\|=\epsilon\cdot\|x\|
  $$
* **穩健分數**：`robustness = 1 - flip_pct`；若 `fragile_features` 覆蓋率 > 15% → 不允許推廣。

### 4.5 Promote 守門（Guardrails）

* **必要條件**（全部通過）

  * Lint/Dry-run **passed**
  * 模擬：`delta_trades_pct ∈ [-25%, +50%]`、`overrides_max ≤ 30%`
  * 敏感度：`overall.robustness_score ≥ 0.60`
  * 線上健康（近 30 日）：`MaxDD ≤ 15%`；`AUC ≥ 基準-0.10`、`Brier` 未劣化
* **Canary / Ramp**

  * Canary：10% 流量 7 天 → 若 **MaxDD 超基準 +3% 或 AUC/Brier** 越界 → **自動回滾**
  * Ramp：50% → 100%

### 4.6 熱載（RCU）

* 寫 `config_active` 與快取 `cfg:active_rev` → 發 `cfg:events`
* 客戶端 Watcher：

  1. 收到 `cfg:events` → 拉 `GET /active`
  2. 本地 **Lint+Guard 再驗**（快速）→ **RCU 原子替換**（進行中請求仍用舊版）
  3. 後續決策/輸出皆記錄 `config_rev`

---

## 5. 排程與背景任務

| Job         | 週期     | 內容                                                    |
| ----------- | ------ | ----------------------------------------------------- |
| 模擬＋敏感度批次    | 每夜     | 拉近 7–14 天 `signals` 重放；寫 `simulations`；TTL 清理         |
| Active 守門巡檢 | 每 1 分鐘 | Canary/Ramp 監測 **MaxDD/AUC/Brier/交易密度**，越界→`ROLLBACK` |
| 配置保活/熱載驗證   | 每 5 分鐘 | 對比 `cfg:active_rev` 與 `config_active.rev`，異常補寫並告警     |

---

## 6. 觀測性（SLI/SLO）

* **SLI**：`promote_success_rate`、`simulate_p95_ms`、`sensitivity_runtime_ms`、`rcu_apply_ms`、`cfg_event_lag_ms`
* **SLO**（示例）：`simulate_p95 ≤ 2s`、`rcu_apply_ms ≤ 300ms`、`promote_success ≥ 99%`
* **Alert**：`cfg_event_lag_ms > 2s`、Canary 期 `MaxDD/AUC` 越界、`policy_shift` 過大

---

## 7. 錯誤處理與冪等

* **冪等鍵**：

  * `/bundles`：`bundle_id + rev`
  * `/promote`：`promotion_id`（如未提供則以 `bundle_id + to_rev + mode` 合成）
  * `/simulate`：`sim_id`（若空→服務生成並返回）
* **回滾策略**：任何 Guardrail 觸發 → 寫 `promotions(mode=ROLLBACK)` → 切回 `from_rev` → 發 `cfg:events`
* **一致性**：DB 寫入成功但事件未發出→定時保活 Job 比對 **rev** 後補發

---

## 8. 契約測試（必備清單）

* **Lint**：白名單邊界／型別／依賴缺失 → `400`
* **Dry-run**：極端 `skip_entry` 規則（>60%）→ `422`
* **Simulate**：`epsilon=0.05/topk=3` 能產出 `flip_pct` 與 `robustness`
* **Promote**：

  * Happy path：DRAFT→STAGED→CANARY→RAMP→FULL，寫入 `config_active` 並廣播
  * Canary 越界：自動 `ROLLBACK`，`config_active` 回到前一 `rev`
* **RCU**：連續 100 次熱載切換，客戶端無 500/錯置 rev

---

## 9. 安全與 RBAC

* **角色**：`researcher`（/simulate）、`risk-officer`（/stage）、`release-manager`（/promote）、`viewer`
* **審計**：所有敏感操作寫 `strategy_events(kind=PROMOTE|ROLLBACK|CONFIG_HOTLOAD)`
* **密鑰**：以 K8s Secret / KMS 管理；HTTP 必走 mTLS（內網）或 JWT（外網）

---

## 10. 前端（S12）代理建議

* 代理：`GET /active`、`POST /bundles`、`POST /simulate`、`POST /promote`
* 視覺化：差異重放（柱狀/散點）、敏感度（特徵翻轉率與邊界距離）、守門狀態（Canary 燈號）
* 人機介面：**配置模擬器**與**敏感度**的 TL;DR 敘事摘要（自動生成）

---

## 11. 快速樣例（Payload 範式）

### 11.1 `POST /simulate`（請求）

```json
{
  "bundle_id": "B-2025-09-20-001",
  "active_rev_ref": "CURRENT",
  "window": { "from": "2025-09-10T00:00:00Z", "to": "2025-09-17T00:00:00Z" },
  "symbols": ["BTCUSDT"],
  "horizons": ["24h"],
  "sensitivity": { "enabled": true, "topk": 3, "epsilon": 0.05, "n_eval": 500 }
}
```

### 11.2 `GET /simulations/{sim_id}`（回應要點）

```json
{
  "summary": { "trades_ref": 50, "trades_new": 65, "delta_trades_pct": 0.30 },
  "approx_effect": {
    "winrate_ref":0.52, "winrate_new":0.55,
    "net_roi_med_ref":0.007, "net_roi_med_new":0.009
  },
  "sensitivity_analysis": {
    "epsilon":"±5%", "n_eval":500,
    "features":[
      { "feature":"rv_pctile_30d","flip_pct":0.08,"delta_size_mult_median":0.04,"boundary_margin_p25":0.018,"label":"WATCH" }
    ],
    "overall": { "robustness_score":0.74, "fragile_features":["rv_pctile_30d"] }
  },
  "bounds_guard": { "violations": [], "passed": true },
  "status": "DONE"
}
```

---

## 12. 與其他服務的集成點（Who calls S10）

* **開機/熱載**：S2/S3/S4/S6/S12 → `GET /active`；訂閱 `cfg:events`
* **人機互動**：S12 → `/bundles` `/simulate` `/promote`
* **監控**：S11 讀 `promotions/simulations` 生成治理面板

---

> 以上補充與你既有 S10 說明（配置管理、模擬、敏感度、Promote、Canary 等）完全對齊，並把資料流、門檻、冪等、排程、觀測性與契約測試具體化，工程即可依此開始落地。
