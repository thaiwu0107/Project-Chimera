# S12 — Web UI / API Gateway（補充規格 v3）

> 本文件在你現有的 S12 草案基礎上擴充，整合前述白皮書 v3、功能規格書、時序圖與「路過的服務」約定，補足行為定義、資料存取點、錯誤處置、觀測性與契約測試要點。

---

## 1) 角色與邊界（Scope & Boundaries）

* **定位**：唯一對外入口（REST + Web），負責驗票、RBAC、請求轉發、跨服務工作流與少量管控動作（Kill-switch、金庫劃轉審批）。
* **不做**：不直接做業務計算（不產生特徵、不下單、不算指標）；一切交易與分析行為交由後端服務（S1–S11）。
* **有狀態**：僅限輕量狀態（JWT/Session 黑名單、審批單暫態、UI 快取 TTL）。其他皆走 Redis/DB 或直連後端。

---

## 2) 代理矩陣（API Proxy Matrix）

> 所有代理路由皆經過：AuthN → RBAC → Schema 驗證（OpenAPI）→ 轉發 → 響應過濾/遮罩（敏感欄位）。

| 目標          | 路徑（S12 對外） → 內部轉發          | 方法        | 權限       | 備註                      |          |       |           |
| ----------- | -------------------------- | --------- | -------- | ----------------------- | -------- | ----- | --------- |
| 健康檢查        | `/health`                  | GET       | public   | 聚合自身與依賴探活（see §8）       |          |       |           |
| 特徵重算        | `/features/recompute` → S2 | POST      | ops      | 批次/補洞                   |          |       |           |
| 決策試算/觸發     | `/decide` → S3             | POST      | trader   | 支援 `dry_run`            |          |       |           |
| 下單/撤單       | `/orders` `/cancel` → S4   | POST      | trader   | FUT/SPOT、OCO/守護         |          |       |           |
| 持倉治理        | `/positions/manage` → S6   | POST      | trader   | 移動停損/分批/加倉              |          |       |           |
| 對帳          | `/reconcile` → S5          | POST      | ops      | 模式：ALL/ORDERS/POSITIONS |          |       |           |
| 標籤回填        | `/labels/backfill` → S7    | POST      | research | 12/24/36h               |          |       |           |
| 復盤          | `/autopsy/{trade_id}` → S8 | POST      | research | 事件/批次                   |          |       |           |
| 實驗          | `/experiments/run` → S9    | POST      | research | WF/統計檢定                 |          |       |           |
| 配置          | \`/bundles                 | /simulate | /promote | /active\` → S10         | GET/POST | admin | 模擬＋敏感度＋守門 |
| 指標/告警       | `/metrics` `/alerts` → S11 | GET       | viewer   | 面板資料                    |          |       |           |
| Kill-switch | `/kill-switch`             | POST      | admin    | 發佈停機事件（§4）              |          |       |           |
| 金庫劃轉        | `/treasury/transfer` → S1  | POST      | admin    | 冪等＋審批（§5）               |          |       |           |

---

## 3) 資料寫入點（When & Where We Write）

* **Redis**

  * `prod:kill_switch`（string+TTL）：Kill-switch 狀態旗標（S12 設置；各服務輪詢/訂閱）。
  * `ops:events`（XADD Stream）：營運事件（Kill-switch、審批、回滾通知）。
  * `ui:cache:*`（string/hash, TTL）：前端快取（active\_rev、health 摘要、告警摘要）。
  * `lock:treasury:<from>:<to>`（分散鎖）：避免劃轉併發。
* **ArangoDB**

  * `treasury_transfers`：劃轉審批單（S12 建/改，S1 執行結果回寫）。
  * `strategy_events`：系統級事件鏡像（例如 `KILL_SWITCH_ON`、`TREASURY_TRANSFER_*`）。
  * `audit_logs`（建議新增）：S12 所有敏感操作審計（user/ip/ua/payload 摘要/結果/耗時）。

> **不直接寫**：`orders/fills/signals/...` 皆由後端服務（S4/S6/S2 等）寫入，S12 僅代理或顯示。

---

## 4) Kill-switch 行為定義

**路徑**：`POST /kill-switch`
**流程**：

1. RBAC：`admin` 專屬；寫 `audit_logs`。
2. 設 `prod:kill_switch=ON`（TTL 可配置，如 15min，避免遺忘常開）。
3. XADD `ops:events`：`{kind:"KILL_SWITCH_ON", by, reason, ts}`。
4. 回傳 `{enabled:true, ttl_ms}`。

**各服務接入約定**（由各服務實作）：

* S3/S4：拒絕新倉 intent；允許平倉/撤單；S4 可選「一鍵平倉」工作流。
* S6：僅執行減風險計畫（移動停損/平倉）。
* S10：阻止 Promote；允許 Rollback。
* S11：將狀態提升為 RED，發告警。

**恢復**：`POST /kill-switch {enable:false}` → 刪 key + 廣播 `KILL_SWITCH_OFF`。

---

## 5) 金庫劃轉（SPOT ↔ FUT）審批 + 冪等

**路徑**：`POST /treasury/transfer`
**請求**：

```json
{ "from":"SPOT", "to":"FUT", "amount_usdt":1000.0, "reason":"margin top-up", "request_id":"uuid" }
```

**步驟**：

1. Schema 驗證 + RBAC（admin）。
2. 分散鎖：`lock:treasury:SPOT:FUT`；冪等：`request_id` 去重。
3. 建立/更新 `treasury_transfers`（`PENDING`）。
4. 內呼 S1 私有 API：`/xchg/treasury/transfer`（附帶冪等鍵）。
5. 成功→更新 `EXECUTED`（寫交易哈希/txid/成本）；失敗→`FAILED` + 重試策略。
6. XADD `ops:events` 與 `strategy_events(kind=TREASURY_TRANSFER_*)`；審計寫 `audit_logs`。

**錯誤處置**：

* S1 超時：退避重試（固定 + 抖動），最多 N 次，仍失敗升級 `ERROR/FATAL` 告警。
* 併發衝突：鎖等待或回覆 409 再試。

---

## 6) 安全（AuthN/RBAC/輸入驗證/遮罩）

* **AuthN**：JWT（HS/RS），支援短期存活 + Refresh；可選單用戶簡化配置。
* **RBAC**：`admin | trader | research | ops | viewer`（可多角色）。
* **Schema 驗證**：對所有代理請求做 OpenAPI schema 校驗；拒絕多餘欄位與越界值。
* **敏感遮罩**：對回應/日誌遮罩 API key、簽名、個資；審計存雜湊摘要。

---

## 7) 可觀測性（Metrics/Logs/Tracing）

* **應用指標（/metrics 暴露給 S11/Prometheus）**

  * `http_requests_total{route,code}`、`latency_seconds_bucket{route}`、`upstream_latency_seconds{service,route}`
  * `proxy_errors_total{service,kind}`、`rbac_denied_total{role,route}`
  * `kill_switch_state`（0/1）、`ops_events_published_total`
* **分散追蹤**：Propagate trace id/header 至後端；將 upstream span 連結回前端請求。
* **日誌**：結構化 JSON，含 `req_id/user/role/route/upstream/duration_ms/outcome`。

---

## 8) 健康與就緒（Health/Ready 聚合）

* `GET /health`：

  * 自身：CPU/Mem/GC、Redis ping、Arango 連線測試。
  * 依賴：對 S10 `/active`、S11 `/metrics`、Redis Cluster slots 做輕探活（**soft 依賴**，失敗降級為 `DEGRADED`）。
* `GET /ready`：

  * 必要依賴：Redis 連線、配置載入完成（有 `active_rev` 快取）、OpenAPI 套件初始化成功。

---

## 9) UI 快取與定時任務

* **任務**（每 5 分鐘）：刷新

  * `ui:cache:active_rev`（S10 `/active`）、
  * `ui:cache:health_summary`（聚合 `GET /health`）、
  * `ui:cache:alerts_digest`（S11 `/alerts`）
* **TTL 失效監測**：若發現 TTL < 閾值且尚無新值 → 立刻刷新一次；仍失敗 → `WARN` 告警。

---

## 10) 失敗注入與降級策略

* **後端超時**：統一 3 類回應：`GATEWAY_TIMEOUT`（可重試提示）、`SERVICE_UNAVAILABLE`（降級）、`FORBIDDEN/UNAUTHORIZED`（Auth 類）。
* **熔斷**：對單目標服務的錯誤率/延遲超閾值 → 短期熔斷 + 快速失敗；視覺化到面板。
* **重試**：只對 **冪等** GET/某些 POST（顯式標記）採固定+抖動退避；不可重試的 API 直接回錯。
* **降級**：指標頁讀「昨日快照」、配置頁讀「上次成功快取」。

---

## 11) 合約測試（Contract Tests）

* **代理 Schema 測試**：對每條代理路由做「成功/缺欄/多欄/越界/異常碼轉譯」。
* **權限矩陣**：針對角色×路由表跑全組合測試（最少：admin/trader/viewer 三角）。
* **冪等測試**：`/treasury/transfer` 同 `request_id` 連送，伺服器應回同 `transfer_id`。
* **邊界**：大負載（多筆回放）、後端慢回、高錯誤率、Redis slot 移轉。

---

## 12) 典型序列（端到端）

### 12.1 配置推廣（Canary）

1. 使用者（admin） → S12 `/bundles`（代理 S10 建 DRAFT）
2. `/simulate`（差異＋敏感度）→ 通過守門
3. `/bundles/{id}/stage` → `/promote(mode=CANARY,10%)`
4. S10 廣播 `cfg:events`；S12 顯示 Active Rev 與 Canary 狀態；面板讀 S11 監控
5. 觸犯 guardrail → S10 回滾；S12 顯示 ROLLBACK 事件與原因

### 12.2 對帳處置（手動）

1. ops → S12 `/reconcile {mode:ALL}`（代理 S5）
2. S5 匯報差異 → S12 呈現孤兒單/殘單列表
3. 使用者選擇「撤單/接管/平倉」→ S12 代理 S4/S6 執行
4. 結果寫 `strategy_events` + `ops:events`；S12 更新 UI

---

## 13) DB 物件（建議 Schema 摘要）

### 13.1 `treasury_transfers`

```json
{
  "transfer_id": "uuid",
  "from": "SPOT",
  "to": "FUT",
  "amount_usdt": 1000.0,
  "reason": "margin top-up",
  "request_id": "uuid",          // 冪等鍵
  "status": "PENDING|EXECUTED|FAILED",
  "requested_by": "user@id",
  "approved_by": "user@id|null",
  "exchange_txid": "string|null",
  "created_at": 1737072000000,
  "executed_at": 1737072060000,
  "error": "string|null"
}
```

**索引**：`hash(transfer_id)`、`hash(request_id, unique)`、`skiplist(created_at)`

### 13.2 `audit_logs`

```json
{
  "audit_id": "uuid",
  "actor": "user@id",
  "ip": "1.2.3.4",
  "ua": "Mozilla/5.0",
  "route": "POST /kill-switch",
  "payload_hash": "sha256:...",
  "result": "OK|FAIL",
  "latency_ms": 48,
  "ts": 1737072000000
}
```

**索引**：`skiplist(ts)`、`hash(actor)`

---

## 14) Redis Key/Stream 命名（S12 範圍）

* `prod:kill_switch` → `"ON" | "OFF"`（TTL）
* `ops:events` → `XADD` fields: `{event_id, kind, by, reason?, ts}`
* `ui:cache:active_rev` → `{rev, bundle_id, ts}`（TTL）
* `ui:cache:health_summary` → `{status, deps:{s10:UP,...}, ts}`（TTL）
* `ui:cache:alerts_digest` → `[{id,severity,msg,ts}...]`（TTL）
* `lock:treasury:<from>:<to>` → 任務期間持鎖（自動過期）

---

## 15) 組態（Config）

```yaml
server:
  port: 8092
security:
  jwt:
    issuer: chimera
    audience: chimera-users
    jwks_url: ""            # 或 HS256 secret
rbac:
  roles: ["admin","trader","research","ops","viewer"]
  policies:
    - role: admin
      allow: ["POST /kill-switch", "POST /treasury/transfer", "ANY /bundles*", "ANY /promote*", "ANY /simulate*"]
    - role: trader
      allow: ["POST /orders", "POST /cancel", "POST /positions/manage", "POST /decide"]
    - role: research
      allow: ["POST /features/recompute", "POST /experiments/run", "POST /labels/backfill", "POST /autopsy/*"]
    - role: viewer
      allow: ["GET /metrics", "GET /alerts", "GET /active"]
upstreams:
  s1: http://s1:8090
  s2: http://s2:8090
  s3: http://s3:8090
  s4: http://s4:8090
  s5: http://s5:8090
  s6: http://s6:8090
  s7: http://s7:8090
  s8: http://s8:8090
  s9: http://s9:8090
  s10: http://s10:8090
  s11: http://s11:8090
redis:
  addrs: ["redis-0:6379","redis-1:6379","redis-2:6379"]
  username: ""
  password: ""
arango:
  url: http://arangodb:8529
  user: chimera
  password: ***
timeouts:
  upstream_ms: 1500
  connect_ms: 300
  kill_switch_ttl_ms: 900000
retries:
  max: 2
  base_ms: 200
  jitter_ms: 150
```

---

## 16) 驗收標準（DoD）

* `GET /health` 聚合狀態正確、具可讀依賴樹。
* Kill-switch 能即時生效於 S3/S4（拒新倉）且 TTL 到期自動恢復。
* `/treasury/transfer` 具冪等性、可觀察審批→執行→審計全鏈路。
* 代理路由均通過：AuthN、RBAC、Schema 驗證；錯誤碼轉譯一致。
* 指標完整上報，Grafana 有「API 門戶」面板（QPS、p95、錯誤率、熔斷次數、後端延遲）。

---

## 17) 風險與緩解

* **單點風險**（API Gateway）：高可用（副本≥2 + LB + readiness gate）；上游降級路徑與快取兜底。
* **Redis Cluster slot 移轉**：官方 cluster client + 關鍵操作重試、啟用 `MOVED/ASK` 透明處理。
* **敏感操作濫用**：RBAC 嚴格、2FA（可選）、Audit 全記錄、異常行為告警（高頻劃轉/連續 Kill-switch）。

---

## 18) 測試清單（最小集）

* **Auth/RBAC**：無 JWT/錯 JWT/過期；角色越權；多角色疊加。
* **Schema**：缺欄、多欄、越界、錯型別；成功/失敗回傳遮罩。
* **Proxy**：上游 2xx/4xx/5xx、超時、慢回；熔斷與重試是否符合策略。
* **Kill-switch**：開/關/TTL 到期；核心服務行為變更驗證（mock S3/S4）。
* **Treasury**：冪等、鎖競爭、上游失敗、恢復後續。

---

> **備註**：本補充僅新增說明，不移除任何既有數學公式與邏輯的約束；與「功能規格書」「白皮書 v3」「Hop-by-Hop 補遺」「核心時序圖」相互對齊，可直接作為 S12 的工程開發藍本使用。
