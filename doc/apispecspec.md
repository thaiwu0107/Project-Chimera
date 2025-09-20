# Project Chimera — API Contract v3.1

你可以直接把下面內容存成 `internal/apispec/spec.go` 使用。

## 說明

1. 本檔定義各服務(S1~S12)常用路徑的 Request/Response 型別，以及「每個服務都必備」的 GET /health。
2. 你可以以這份型別檔作為 server 的 contract，或加上 swaggo / go-swagger 註解產出 OpenAPI。
3. 時間戳一律 epoch ms；金額一律 USDT；百分比使用小數（0.10 = 10%）。
4. /health 採「統一結構」，各服務可回報自身與相依元件檢查明細（Redis/Arango/Exchange/WS 等）。

## 建議

K8s Liveness/Readiness Probe 均指向 GET /health：
- **liveness 判斷**：Status==OK | DEGRADED 皆可視為存活；ERROR 視情況判定（可另提供 /ready）
- **readiness 判斷**：Status==OK 才視為就緒。

## Go 程式碼

```go
package apispec

// internal/apispec/spec.go
// Project Chimera — API Contract v3.1 (commented)
// ------------------------------------------------

// ================================
// 共用列舉型別與常數
// ================================

// Market 代表交易市場（期貨/現貨）
type Market string

const (
	MarketFUT  Market = "FUT"  // 幣安永續期貨 (USDT-M)
	MarketSPOT Market = "SPOT" // 幣安現貨
)

// Side 代表下單方向（買/賣）
type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

// PosSide 代表持倉方向（多/空/空倉）
type PosSide string

const (
	PosLong  PosSide = "LONG"
	PosShort PosSide = "SHORT"
	PosFlat  PosSide = "FLAT"
)

// Severity 告警等級
type Severity string

const (
	SevInfo  Severity = "INFO"
	SevWarn  Severity = "WARN"
	SevError Severity = "ERROR"
	SevFatal Severity = "FATAL"
)

// HealthLevel 策略健康度四段燈號（策略層用）
type HealthLevel string

const (
	HealthGreen  HealthLevel = "GREEN"  // 正常
	HealthYellow HealthLevel = "YELLOW" // 降槓桿/限速
	HealthOrange HealthLevel = "ORANGE" // 嚴重警戒/凍結新倉
	HealthRed    HealthLevel = "RED"    // 緊急停機/平倉
)

// HealthStatus 服務健康狀態（/health 統一回傳）
type HealthStatus string

const (
	HealthOK       HealthStatus = "OK"       // 一切正常
	HealthDegraded HealthStatus = "DEGRADED" // 部分相依異常但服務仍可運作（降級）
	HealthError    HealthStatus = "ERROR"    // 關鍵相依失效，功能不可用
)

// DecisionAction 決策動作：入場或跳過
type DecisionAction string

const (
	DecisionOpen DecisionAction = "open"
	DecisionSkip DecisionAction = "skip"
)

// OrderIntentKind 訂單意圖類型：入場/加倉/減倉/停利/停損
type OrderIntentKind string

const (
	IntentEntry OrderIntentKind = "ENTRY"
	IntentAdd   OrderIntentKind = "ADD"
	IntentExit  OrderIntentKind = "EXIT"
	IntentTP    OrderIntentKind = "TP"
	IntentSL    OrderIntentKind = "SL"
)

// ReconcileMode 對帳模式
type ReconcileMode string

const (
	ReconcileAll       ReconcileMode = "ALL"
	ReconcileOrders    ReconcileMode = "ORDERS"
	ReconcilePositions ReconcileMode = "POSITIONS"
	ReconcileHoldings  ReconcileMode = "HOLDINGS"
)

// ================================
// /health 統一結構（所有服務共用）
// ================================

// HealthCheck 單一相依檢查結果
type HealthCheck struct {
	Name      string       `json:"name"`                 // 相依名：redis/arango/ws-binance/stream-lag 等
	Status    HealthStatus `json:"status"`               // OK/DEGRADED/ERROR
	LatencyMs int64        `json:"latency_ms,omitempty"` // 檢查耗時（若適用）
	Error     string       `json:"error,omitempty"`      // 錯誤訊息（若非 OK）
}

// HealthResponse 各服務 GET /health 的統一回傳
type HealthResponse struct {
	Service    string        `json:"service"`                // 服務代號：s1-exchange / s2-feature ...
	Version    string        `json:"version"`                // 服務版本（git tag/commit）
	Status     HealthStatus  `json:"status"`                 // 匯總狀態：OK/DEGRADED/ERROR
	Ts         int64         `json:"ts"`                     // 回應時間 epoch ms
	UptimeMs   int64         `json:"uptime_ms"`              // 服務啟動至今毫秒數
	ConfigRev  int           `json:"config_rev,omitempty"`   // 當前策略配置版本（能取得者回報）
	Checks     []HealthCheck `json:"checks,omitempty"`       // 相依檢查明細
	Notes      string        `json:"notes,omitempty"`        // 補充說明
}

// ================================
// 共用結構
// ================================

// Window 查詢時間窗（epoch ms）
type Window struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// SymbolRef 用於標的標識
type SymbolRef struct {
	Symbol string `json:"symbol"` // 交易對：如 "BTCUSDT"
	Market Market `json:"market"` // FUT|SPOT
}

// FeatureSet 決策/模型可用的特徵集合（鍵：特徵名，值：數值/字串/布林）
type FeatureSet map[string]any

// MetricPoint 單點指標（S11 輸出）
type MetricPoint struct {
	Metric string            `json:"metric"`         // 指標名稱（如 "router_p95_ms"）
	Value  float64           `json:"value"`          // 數值
	Ts     int64             `json:"ts"`             // epoch ms
	Tags   map[string]string `json:"tags,omitempty"` // 維度標籤（symbol/market/rev 等）
}

// Alert 告警事件（S11 輸出）
type Alert struct {
	AlertID  string   `json:"alert_id"`
	Severity Severity `json:"severity"`
	Source   string   `json:"source"`
	Message  string   `json:"message"`
	Ts       int64    `json:"ts"`
}

// ================================
// S1 Exchange Connectors
// 行情/深度/資金費/帳戶（WS/REST）→ 寫入 DB/Redis；對外僅健康檢查
// 路徑：GET /health
// ================================
// 使用 HealthResponse（見上）

// ================================
// S2 Feature Generator
// 計算 ATR/RV/相關性/深度衍生特徵；提供補算入口
// 路徑：GET /health
// 路徑：POST /features/recompute?symbol=BTCUSDT
// ================================

type RecomputeFeaturesRequest struct {
	Symbol string `json:"symbol"`            // 標的：如 "BTCUSDT"
	FromMs int64  `json:"from_ms,omitempty"` // 起：epoch ms（空=自動）
	ToMs   int64  `json:"to_ms,omitempty"`   // 迄：epoch ms（空=自動）
}

type RecomputeFeaturesResponse struct {
	Computed int    `json:"computed"`          // 本次補算筆數
	Message  string `json:"message,omitempty"` // 補算摘要
}

// ================================
// S3 Strategy Engine
// 規則引擎 + 守門 + 置信度模型 → Decision/OrderIntents（FUT/ SPOT）
// 路徑：GET /health
// 路徑：POST /decide
// ================================

type DecideRequest struct {
	SignalID  string     `json:"signal_id"`  // 唯一信號 ID；用於追蹤/標籤回填
	Symbol    string     `json:"symbol"`     // 交易對
	Market    Market     `json:"market"`     // FUT|SPOT
	Features  FeatureSet `json:"features"`   // 由 S2 產生的特徵快照
	ConfigRev int        `json:"config_rev"` // 當下配置版本（寫入 signals 以追溯）
}

type Decision struct {
	Action   DecisionAction `json:"action"`              // open|skip
	SizeMult float64        `json:"size_mult,omitempty"` // 初始倉位倍率（預設 1.0）
	TPMult   float64        `json:"tp_mult,omitempty"`   // 停利倍率
	SLMult   float64        `json:"sl_mult,omitempty"`   // 停損倍率
	Reason   string         `json:"reason,omitempty"`    // 可讀解釋（規則命中/模型分數等）
}

// OCO SPOT 一單兩腿（TakeProfitPx/StopLossPx 以「價格」定義）
type OCO struct {
	TakeProfitPx float64 `json:"take_profit_px"` // 停利觸發價
	StopLossPx   float64 `json:"stop_loss_px"`   // 停損觸發價
}

// ExecPolicy 路由器執行策略（Maker/TWAP/OCO/守護停損）
type ExecPolicy struct {
	PreferMaker     bool    `json:"prefer_maker"`         // 優先掛單（失敗回退）
	MakerWaitMs     int     `json:"maker_wait_ms"`        // 掛單等待毫秒（超時撤單）
	TWAPSlices      int     `json:"twap_slices"`          // 分批切片數（≥1：啟用 TWAP）
	GuardStopEnable bool    `json:"guard_stop_enable"`    // SPOT：本地守護停損
	TPPct           float64 `json:"tp_pct,omitempty"`     // 絕對停利%
	SLPct           float64 `json:"sl_pct,omitempty"`     // 絕對停損%
	OCO             *OCO    `json:"oco,omitempty"`        // SPOT：OCO 兩腿
}

type OrderIntent struct {
	IntentID     string          `json:"intent_id"`          // 唯一意圖 ID（冪等鍵）
	Symbol       string          `json:"symbol"`             // 交易對（如 BTCUSDT）
	Market       Market          `json:"market"`             // FUT|SPOT
	Kind         OrderIntentKind `json:"kind"`               // ENTRY/ADD/EXIT/TP/SL
	Side         Side            `json:"side"`               // BUY/SELL
	NotionalUSDT float64         `json:"notional_usdt"`      // 名目 USDT 金額
	Leverage     int             `json:"leverage,omitempty"` // FUT：槓桿；SPOT 留空
	ExecPolicy   ExecPolicy      `json:"exec_policy"`        // 執行策略
}

type DecideResponse struct {
	Decision Decision      `json:"decision"`          // 決策
	Intents  []OrderIntent `json:"intents,omitempty"` // 需要執行的下單意圖
}

// ================================
// S4 Order Router
// 對接交易所（FUT/ SPOT），實作 Maker→Taker 回退、TWAP、OCO、守護停損
// 路徑：GET /health
// 路徑：POST /orders, POST /cancel
// ================================

type OrderCmdRequest struct {
	Intent OrderIntent `json:"intent"` // 由 S3 產生
}

type Fill struct {
	FillID      string  `json:"fill_id"`
	Price       float64 `json:"price"`
	Qty         float64 `json:"qty"`
	FeeUSDT     float64 `json:"fee_usdt"`
	MidAtSend   float64 `json:"mid_at_send,omitempty"`
	SlippageBps float64 `json:"slippage_bps,omitempty"`
	Timestamp   int64   `json:"timestamp"`
}

type OrderResult struct {
	OrderID        string  `json:"order_id"`
	ClientOrderID  string  `json:"client_order_id"`
	Status         string  `json:"status"`                   // NEW/FILLED/PARTIALLY_FILLED/CANCELED/REJECTED
	AvgPrice       float64 `json:"avg_price,omitempty"`
	ExecutedQty    float64 `json:"executed_qty,omitempty"`
	Fills          []Fill  `json:"fills,omitempty"`
	GuardStopArmed bool    `json:"guard_stop_armed,omitempty"` // SPOT 守護停損佈署狀態
	Message        string  `json:"message,omitempty"`
}

type CancelRequest struct {
	Market   Market `json:"market"`                      // FUT|SPOT
	Symbol   string `json:"symbol"`                      // 交易對
	OrderID  string `json:"order_id,omitempty"`          // 交易所 ID（或）
	ClientID string `json:"client_order_id,omitempty"`   // 自定冪等 ID
}

type CancelResponse struct {
	Canceled bool   `json:"canceled"`
	Message  string `json:"message,omitempty"`
}

// ================================
// S5 Reconciler
// 啟動對帳 + 事務狀態機（PENDING→ACTIVE→PENDING_CLOSING→CLOSED）
// 路徑：GET /health
// 路徑：POST /reconcile
// ================================

type ReconcileRequest struct {
	Mode ReconcileMode `json:"mode"` // ALL/ORDERS/POSITIONS/HOLDINGS
}

type ReconcileResponse struct {
	FixedOrders     int    `json:"fixed_orders"`
	FixedPositions  int    `json:"fixed_positions"`
	FixedHoldings   int    `json:"fixed_holdings"`
	OrphansClosed   int    `json:"orphans_closed"`
	Message         string `json:"message,omitempty"`
}

// ================================
// S6 Position Manager
// 持倉治理（移動停損、分批止盈、加倉），FUT 與 SPOT 通用
// 路徑：GET /health
// 路徑：POST /positions/manage
// ================================

type ManagePositionsRequest struct {
	Symbol string `json:"symbol,omitempty"` // 空字串：全標的
}

type StopMove struct {
	OldPx  float64 `json:"old_px"`
	NewPx  float64 `json:"new_px"`
	Reason string  `json:"reason"`
}

type ManagePlan struct {
	StopMoves []StopMove    `json:"stop_moves"`
	Reduce    []OrderIntent `json:"reduce"`
	Adds      []OrderIntent `json:"adds"`
}

type ManagePositionsResponse struct {
	Plan   ManagePlan    `json:"plan"`
	Orders []OrderResult `json:"orders,omitempty"`
}

// ================================
// S7 Label Backfill
// 針對過往 signals 依 12/24/36h 計算淨利與標籤
// 路徑：GET /health
// 路徑：POST /labels/backfill
// ================================

type BackfillRequest struct {
	HorizonH int   `json:"horizon_h"`        // 12|24|36
	SinceMs  int64 `json:"since_ms,omitempty"`
	UntilMs  int64 `json:"until_ms,omitempty"`
}

type BackfillResponse struct {
	Updated int    `json:"updated"`
	Message string `json:"message,omitempty"`
}

// ================================
// S8 Autopsy Generator
// 交易復盤（TL;DR、圖表、反事實、Peer 對比、TCA），輸出連結或報表 ID
// 路徑：GET /health
// 路徑：POST /autopsy/{trade_id}
// ================================

type AutopsyRequest struct {
	TradeID string `json:"trade_id"` // 交易唯一 ID（策略事件關聯）
}

type AutopsyResponse struct {
	ReportID string `json:"report_id"`
	Url      string `json:"url,omitempty"`
}

// ================================
// S9 Hypothesis Orchestrator
// 假設驗證/回測實驗（Walk-Forward、Purged K-Fold）
// 路徑：GET /health
// 路徑：POST /experiments/run
// ================================

type ExperimentRunRequest struct {
	HypothesisID string `json:"hypothesis_id"`
	Window       Window `json:"window"`
	ConfigRev    int    `json:"config_rev"`
}

type ExperimentRunResponse struct {
	ExpID  string `json:"exp_id"`
	Status string `json:"status"` // QUEUED/RUNNING/DONE
}

// ================================
// S10 Config Service
// 因子/規則/標的 bundle 管理；Lint/Dry-run；模擬器＋敏感度；Promote/Rollback
// 路徑：GET /health
// 路徑：POST /bundles, POST /bundles/{id}/stage, POST /promote, POST /simulate, GET /active
// ================================

type BundleUpsertRequest struct {
	BundleID    string         `json:"bundle_id"`
	Rev         int            `json:"rev"`
	Factors     []string       `json:"factors"`
	Rules       []string       `json:"rules"`
	Instruments []string       `json:"instruments"`
	Flags       map[string]any `json:"flags,omitempty"`
	Status      string         `json:"status"` // DRAFT/STAGED/ACTIVE/ROLLBACK/REVOKED
}

type BundleUpsertResponse struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type BundleStageResponse struct {
	Ok bool `json:"ok"`
}

type PromoteRequest struct {
	BundleID string `json:"bundle_id"`
	Mode     string `json:"mode"` // CANARY/RAMP/FULL/ROLLBACK
}

type PromoteResponse struct {
	PromotionID string `json:"promotion_id"`
	Status      string `json:"status"` // PENDING/ACTIVE/ROLLED_BACK/DONE
}

type SensitivitySpec struct {
	Enabled bool    `json:"enabled"`
	TopK    int     `json:"topk"`
	Epsilon float64 `json:"epsilon"` // 0.05 = ±5%
	NEval   int     `json:"n_eval"`
}

type SimulateRequest struct {
	BundleID     string           `json:"bundle_id"`
	ActiveRevRef string           `json:"active_rev_ref"` // "CURRENT"
	Window       Window           `json:"window"`
	Symbols      []string         `json:"symbols"`
	Horizons     []string         `json:"horizons"` // "24h" 等
	Sensitivity  *SensitivitySpec `json:"sensitivity,omitempty"`
}

type SimulateResponse struct {
	SimID  string `json:"sim_id"`
	Status string `json:"status"` // QUEUED/RUNNING/DONE/FAILED
}

type ActiveConfigResponse struct {
	Rev         int    `json:"rev"`
	BundleID    string `json:"bundle_id"`
	ActivatedAt int64  `json:"activated_at"`
}

// ================================
// S11 Metrics & Health
// 指標彙整對外 API（給 S12 前端）
// 路徑：GET /health
// 路徑：GET /metrics, GET /alerts
// ================================

type MetricsResponse struct {
	Points []MetricPoint `json:"points"`
}

type AlertsResponse struct {
	Items []Alert `json:"items"`
}

// ================================
// S12 Web UI / API Gateway
// 前端共用 API：Kill-switch、Treasury Transfer（可選）
// 路徑：GET /health
// 路徑：POST /kill-switch, POST /treasury/transfer
// ================================

type KillSwitchRequest struct {
	Enable bool `json:"enable"` // true=啟動停機；false=解除
}

type KillSwitchResponse struct {
	Enabled bool `json:"enabled"`
}

// TransferRequest 期貨/現貨之間資金劃轉（若啟用）
type TransferRequest struct {
	From   string  `json:"from"`         // "SPOT" | "FUT"
	To     string  `json:"to"`           // "FUT" | "SPOT"
	Amount float64 `json:"amount_usdt"`  // 劃轉 USDT 數
	Reason string  `json:"reason"`       // 記帳理由
}

type TransferResponse struct {
	TransferID string `json:"transfer_id"`
	Result     string `json:"result"`       // OK|FAIL
	Message    string `json:"message,omitempty"`
}
```

## 使用說明（重點）

所有服務都遵循相同的 GET /health 回傳 HealthResponse。

**Status**：聚合判讀（OK/DEGRADED/ERROR）。

**Checks**：細項相依（例如 redis, arango, ws-binance, stream-lag）。

**ConfigRev**：能取得者（S3/S6/S10/S12）建議一併回報，方便排錯。

你可以把 HealthResponse 同時用在 liveness 與 readiness，或額外提供 /ready（若要嚴格把 OK 才視為就緒）。

若要產出 OpenAPI，建議在每個區塊的路徑註解改用 swaggo/swag 註解（例如 // @Summary、// @Router /decide [post] 等）。