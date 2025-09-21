# S11 — Metrics & Health（補充版 README / 實作指南）

> 角色：**指標彙整器 + 健康度裁決器 + 告警中樞**
> 任務：彙整各服務指標、計算健康等級（GREEN/YELLOW/ORANGE/RED）、維護 SLI/SLO、發佈告警與前端查詢 API。&#x20;

---

## 1) 範圍與目標（Scope）

* **指標彙整**：策略績效、執行延遲、穩定性、錯誤率、TCA 成本等聚合與落地。
* **健康裁決**：以「失效偵測門檻表」+ 權重或 max-severity，輸出單一健康狀態。
* **SLO 監控**：持續追蹤核心 SLI，對 SLO 違反做狀態與告警處理。
* **告警與面板**：將事件寫入 `alerts`，提供 `/metrics`、`/alerts` 查詢給 S12 前端；Grafana 面板接入。
* **Canary/Ramp 守門配合**：在 S10 推廣期監看 guardrail（MaxDD/AUC/Brier/Router p95），越界主動升級嚴重度。

---

## 2) 依賴與輸入（Inputs）

* **Redis Streams（事件）**

  * `metrics:events:*`：各服務推送的即時執行事件（例如 `ord:result:*` 投影、ws 心跳等）。
  * `feat:events:*`：必要時用於派生策略級指標（可選）。
* **Redis Keys（高頻快照 / 權威狀態）**

  * `prod:health:system:state`：S11 綜合健康輸出（GREEN|YELLOW|ORANGE|RED）。
  * `prod:router:p95:win`、`prod:ws:avail:win`、`prod:stream:lag` 等滑動窗口快照（S11 計算或接收）。
* **ArangoDB Collections（落地與查詢）**

  * `metrics_timeseries`（高頻）、`strategy_metrics_daily`（日彙總）
  * `alerts`（告警記錄）
  * （參考）`simulations` / `promotions`（讀取守門值，推廣觀測）

---

## 3) 產出（Outputs）

* **Redis**

  * `prod:health:system:state = GREEN|YELLOW|ORANGE|RED`（TTL: 30s；每 10s 更新）
  * `metrics:agg:1m`（可選 Stream，把聚合後的指標對外廣播）
* **ArangoDB**

  * `metrics_timeseries`: 指標點位（秒/分級）
  * `strategy_metrics_daily`: 日級 KPI（PF、Sharpe、MaxDD 等）
  * `alerts`: 告警實體（含等級、來源、訊息、關聯指標）
* **API（供 S12 前端）**

  * `GET /health`、`GET /metrics`、`GET /alerts`、`GET /metrics/history`

---

## 4) 指標定義（SLI）與數學公式

> **統一時間**：epoch ms；**比例**：小數（0.1 = 10%）

### 4.1 執行與穩定性

* **Router p95 延遲（毫秒）**

  * 來源：`ord:cmd:*` → `ord:result:*` 的端到端延遲集合
  * 度量：以滑窗（近 5–15 分鐘）計算 p95
* **Redis Stream Lag**

  * 定義：`consumer_lag = last_produced_id - last_ack_id`
  * 門檻：`lag > 2s` → WARN；`> 5s` → ERROR
* **WS 可用率**

  * `p_up = ok_calls / total_calls`，滑動窗口 1h

### 4.2 策略與風險

* **MaxDD**：

  $$
  \text{MaxDD}=\max_t\left(1-\frac{\text{Equity}_t}{\max_{\tau\le t}\text{Equity}_\tau}\right)
  $$
* **PF / Sharpe / Hit Rate**：依日/周窗口聚合
* **Maker 成交率**：

  $$
  \text{maker\_fill\_ratio}=\frac{\text{maker\_filled\_qty}}{\text{total\_qty}}
  $$
* **資金不足率（SPOT/FUT）**：

  $$
  \text{ib\_rate}=\frac{\#\text{insufficient\_balance\_errors}}{\#\text{orders}}
  $$

### 4.3 模型健壯性

* **滾動 AUC（近 100 筆）**
* **Brier Score**：

  $$
  \text{Brier}=\frac{1}{N}\sum_i (p_i - y_i)^2
  $$

---

## 5) 健康裁決（Health State）與動作矩陣

### 5.1 映射策略（兩種擇一或混合）

* **max-severity**：各項指標先映射到區間色階（GREEN/YELLOW/ORANGE/RED），取最嚴重者為系統健康。
* **加權分數**：

  $$
  S=\sum_k w_k \cdot s_k \quad \Rightarrow \quad \text{State}=
  \begin{cases}
  \text{GREEN}, & S\le \theta_1\\
  \text{YELLOW}, & \theta_1 < S\le \theta_2\\
  \text{ORANGE}, & \theta_2 < S\le \theta_3\\
  \text{RED}, & S > \theta_3
  \end{cases}
  $$

### 5.2 失效偵測門檻（示例，可配置）

* `maker_fill_ratio < 0.25`（近 1h） → **執行降級**：S4 改市價比例↑、maker 等待↓
* `ib_rate > 0.10`（近 1d） → **資金流控**：S6 啟用降額或排隊重試
* `AUC` 低於基準 0.65 的 0.1 以上 / `Brier` 升高 → **治理**：S10 禁止 Promote 或觸發 Canary 回滾
* `router_p95 > 800ms`（連續 3 個窗口） → **執行降級**：禁 TWAP / Maker

### 5.3 健康狀態對各服務的作用（建議）

| State  | S3（決策）           | S4（路由）              | S6（持倉）   |
| ------ | ---------------- | ------------------- | -------- |
| GREEN  | 正常倍率             | 正常策略                | 正常       |
| YELLOW | `size_mult ×0.9` | `maker_wait -20%`   | SL 遞進略保守 |
| ORANGE | `size_mult ×0.7` | `market_ratio +30%` | 加嚴停損、禁加倉 |
| RED    | 暫停新倉             | 僅平倉/撤單              | 風險解纜、清倉  |

---

## 6) Redis 規劃（Keys / Streams）

> 前綴：`prod:`；Cluster hash-tags 用 `{}` 括起對齊熱點（例如 `{sys}`）

### 6.1 Keys（快照）

* `prod:{health}:system:state` → `GREEN|YELLOW|ORANGE|RED`（TTL 30s）
* `prod:{sys}:router:p95:win` → 路由 p95 近期值
* `prod:{sys}:stream:lag` → 近期 lag（ms）
* `prod:{sys}:ws:avail` → 近期可用率
* `prod:{guard}:canary:*` → S10 推廣 guardrail 當前觀測（MaxDD/AUC/Brier 等）

### 6.2 Streams（事件）

* `metrics:events:s4_router`：S4 推送執行延遲/結果彙整
* `metrics:events:s1_ws`：S1 推送 ws 連線/心跳指標
* `metrics:events:s3_model`：S3 推送 AUC/Brier 原始點
* `metrics:agg:1m`：S11 對外廣播（可選，供其他消費者）

**消費者群組**：`cg:s11:agg`（S11 聚合器）、`cg:s12:dashboard`（前端即時板可選）

---

## 7) DB 結構與索引（ArangoDB）

* `metrics_timeseries`

  * `metric`（string, hash index）
  * `ds/ts`（date/ms, skiplist）
  * `tags`（object, optional）
  * **索引**：`hash(metric)`、`skiplist(ts)`、`hash(tags.symbol)`（可選）
* `strategy_metrics_daily`

  * `ds`（date, skiplist）、`metric`、`value`、`tags`
  * **索引**：`skiplist(ds)`、`hash(metric)`
* `alerts`

  * `alert_id`（hash）、`severity`（skiplist）、`source`、`message`、`ts`（skiplist）
  * **索引**：`hash(alert_id)`、`skiplist(ts)`、`hash(severity)`

---

## 8) 定時任務（Jobs / Schedules）

* **健康巡檢彙總（每 10s）**

  * 收 `metrics:events:*` & 滑窗聚合 → 判定健康 → 寫 `prod:health:system:state`（TTL 30s）
  * 同步寫 `metrics_timeseries`（必要子集）
* **分鐘聚合（每 1m）**

  * p50/p95、錯誤率、ib\_rate、maker\_fill\_ratio、stream\_lag → `metrics_timeseries`
* **日彙總（每日 00:05）**

  * PF/Sharpe/MaxDD、路由成功率/延遲統計 → `strategy_metrics_daily`
* **守門巡檢（每 1m，在推廣期）**

  * 讀 `promotions.guardrail` 與線上觀測 → 越界觸發 Canary 回滾（寫入 `alerts`，告知 S10）

---

## 9) API（入向）

* `GET /health` → `HealthResponse{status, deps:[redis, arango], lag_ms, router_p95}`
* `GET /metrics?metric=router_p95&from=...&to=...&step=1m` → `MetricsResponse{series:[{ts,value,tags}]}`
* `GET /metrics/sli` → 核心 SLI 即時點（router\_p95、ib\_rate、maker\_fill\_ratio、stream\_lag、ws\_avail、maxdd）
* `GET /alerts?severity=WARN|ERROR|FATAL&from=...` → `AlertsResponse{items:[...]}`

> **契約測試（示例）**
>
> * `GET /metrics`：空參數 → 400；合法 metric + 範圍 → 200 並按時間遞增；`from>to` → 422
> * `GET /alerts`：`severity` 非白名單 → 422；時間範圍內查無資料 → 200 + 空陣列

---

## 10) 計算流程（Hop-by-Hop）

1. **事件收集**：S1/S3/S4 將執行事件丟至 `metrics:events:*`。
2. **滑窗聚合**：S11 消費群組 `cg:s11:agg` 拉取事件 → 計算 p95、比率、lag、AUC/Brier（滾動 100 筆）。
3. **健康裁決**：以門檻表映射 & 彙總 → 得到 `state`。
4. **落地與發佈**：

   * 寫入 `metrics_timeseries`（關鍵點）/ `strategy_metrics_daily`（於日彙總）
   * 寫入 `prod:health:system:state`（TTL 30s）；必要時寫 `alerts`
5. **上下游反應**：

   * S3/S4/S6 讀健康 → 自行調整（倍率/市價比例/停損保守度）

---

## 11) SLO / 告警規則（建議初值）

* **SLO**

  * Router p95 ≤ **800ms**（連續 95% 視窗）
  * Stream Lag ≤ **2s**（95 百分位）
  * WS 可用率 ≥ **99.5%**（過去 24h）
* **告警**

  * 任一 SLO **連續 3 個窗口**違反 → `alerts(severity=WARN|ERROR)`
  * Canary guardrail 觸發 → `alerts(severity=ERROR|FATAL)` 並回滾建議

---

## 12) 測試計畫（要點）

* **單元**：滑窗 p95、lag、AUC/Brier、maker\_fill\_ratio、ib\_rate 計算函式（邊界與 NaN 處理）
* **整合**：模擬 `metrics:events:*` 高壓流量，驗證聚合正確性與吞吐
* **契約**：`/metrics`、`/alerts`、`/health` 正常/錯誤路徑
* **回歸**：失效偵測門檻表改動，健康裁決保持可重現（固定樣本）

---

## 13) Grafana 面板（建議）

* Router p50/p95、失敗率（堆疊）
* Stream Lag（當前/歷史）
* WS 可用率（S1）
* maker\_fill\_ratio、ib\_rate（SPOT/FUT 分面）
* 模型 AUC/Brier（滾動）
* 健康狀態火焰圖（GREEN→RED）

---

## 14) 安全與治理

* `/metrics`、`/alerts` 開只讀、需 Token（S12 代理）
* 門檻表（失效偵測與 SLO）進 `config_bundles` 或 `observability_policies`，由 S10 管控，審批後生效
* 所有自動降級動作需記錄 `alerts` 與 `strategy_events(kind=HEALTH_DEGRADE)`，便於稽核

---

## 15) Roadmap（S11 專屬）

1. **MVP**：收事件 → 聚合 p95/lag → 輸出健康狀態 + `/metrics`
2. **SLO/告警**：加入門檻表，Prometheus Alertmanager 整合
3. **模型面指標**：滾動 AUC/Brier，對接 S10 Canary 守門
4. **成本學習閉環**：對 S4 釋出 TCA 統計（供路由參數再估）

---

*本補充依據既有 S11 規劃整理並細化為可實作的項目清單與公式、鍵空間、DB 與 API 介面，供工程直接落地。*
