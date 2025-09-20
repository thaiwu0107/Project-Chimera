package dao

import "time"

// ================================
// Project Chimera — API Contract v3.1 (commented)
// ------------------------------------------------
// 說明：
// 1) 本檔定義各服務(S1~S12)常用路徑的 Request/Response 型別，以及「每個服務都必備」的 GET /health。
// 2) 你可以以這份型別檔作為 server 的 contract，或加上 swaggo / go-swagger 註解產出 OpenAPI。
// 3) 時間戳一律 epoch ms；金額一律 USDT；百分比使用小數（0.10 = 10%）。
// 4) /health 採「統一結構」，各服務可回報自身與相依元件檢查明細（Redis/Arango/Exchange/WS 等）。
//
// 建議：K8s Liveness/Readiness Probe 均指向 GET /health；
// - liveness 判斷：Status==OK | DEGRADED 皆可視為存活；ERROR 視情況判定（可另提供 /ready）
// - readiness 判斷：Status==OK 才視為就緒。

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
	Service   string        `json:"service"`              // 服務代號：s1-exchange / s2-feature ...
	Version   string        `json:"version"`              // 服務版本（git tag/commit）
	Status    HealthStatus  `json:"status"`               // 匯總狀態：OK/DEGRADED/ERROR
	Ts        int64         `json:"ts"`                   // 回應時間 epoch ms
	UptimeMs  int64         `json:"uptime_ms"`            // 服務啟動至今毫秒數
	ConfigRev int           `json:"config_rev,omitempty"` // 當前策略配置版本（能取得者回報）
	Checks    []HealthCheck `json:"checks,omitempty"`     // 相依檢查明細
	Notes     string        `json:"notes,omitempty"`      // 補充說明
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
// S11 Metrics & Health - API 結構
// ================================

type MetricsResponse struct {
	Points []MetricPoint `json:"points"`
}

type AlertsResponse struct {
	Items []Alert `json:"items"`
}

// ================================
// S11 Metrics & Health - 數據模型
// ================================

// ServiceMetrics 服務指標
type ServiceMetrics struct {
	ServiceID   string            `json:"service_id"`
	ServiceName string            `json:"service_name"`
	MetricType  string            `json:"metric_type"` // LATENCY/THROUGHPUT/ERROR_RATE/MEMORY/CPU
	Value       float64           `json:"value"`
	Unit        string            `json:"unit"` // ms/req_per_sec/percent/mb/percent
	Timestamp   int64             `json:"timestamp"`
	Tags        map[string]string `json:"tags"`
	CreatedAt   time.Time         `json:"created_at"`
}

// TradingMetrics 交易指標
type TradingMetrics struct {
	MetricID   string    `json:"metric_id"`
	Symbol     string    `json:"symbol"`
	Market     string    `json:"market"`
	MetricType string    `json:"metric_type"` // PNL/VOLUME/ORDERS/FILLS/SLIPPAGE
	Value      float64   `json:"value"`
	Unit       string    `json:"unit"` // USDT/contracts/count/bps
	Timestamp  int64     `json:"timestamp"`
	StrategyID string    `json:"strategy_id"`
	ConfigRev  int       `json:"config_rev"`
	CreatedAt  time.Time `json:"created_at"`
}

// SystemMetrics 系統指標
type SystemMetrics struct {
	MetricID   string            `json:"metric_id"`
	Component  string            `json:"component"`   // REDIS/ARANGODB/EXCHANGE/WEBHOOK
	MetricType string            `json:"metric_type"` // CONNECTION/LATENCY/ERROR_RATE/THROUGHPUT
	Value      float64           `json:"value"`
	Unit       string            `json:"unit"` // ms/req_per_sec/percent/count
	Timestamp  int64             `json:"timestamp"`
	Tags       map[string]string `json:"tags"`
	CreatedAt  time.Time         `json:"created_at"`
}

// AlertRule 告警規則
type AlertRule struct {
	RuleID     string            `json:"rule_id"`
	RuleName   string            `json:"rule_name"`
	MetricName string            `json:"metric_name"`
	Condition  string            `json:"condition"` // GREATER_THAN/LESS_THAN/EQUAL
	Threshold  float64           `json:"threshold"`
	Severity   Severity          `json:"severity"`
	Enabled    bool              `json:"enabled"`
	Tags       map[string]string `json:"tags"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// AlertHistory 告警歷史
type AlertHistory struct {
	HistoryID      string    `json:"history_id"`
	AlertID        string    `json:"alert_id"`
	RuleID         string    `json:"rule_id"`
	Severity       Severity  `json:"severity"`
	Source         string    `json:"source"`
	Message        string    `json:"message"`
	Status         string    `json:"status"` // TRIGGERED/RESOLVED/ACKNOWLEDGED
	TriggeredAt    time.Time `json:"triggered_at"`
	ResolvedAt     time.Time `json:"resolved_at"`
	AcknowledgedAt time.Time `json:"acknowledged_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// MetricsAggregation 指標聚合
type MetricsAggregation struct {
	AggregationID   string            `json:"aggregation_id"`
	MetricName      string            `json:"metric_name"`
	AggregationType string            `json:"aggregation_type"` // SUM/AVG/MAX/MIN/COUNT
	Window          Window            `json:"window"`
	Value           float64           `json:"value"`
	Count           int               `json:"count"`
	Tags            map[string]string `json:"tags"`
	CreatedAt       time.Time         `json:"created_at"`
}

// DashboardConfig 儀表板配置
type DashboardConfig struct {
	ConfigID      string                 `json:"config_id"`
	DashboardName string                 `json:"dashboard_name"`
	Widgets       []WidgetConfig         `json:"widgets"`
	Layout        map[string]interface{} `json:"layout"`
	RefreshRate   int                    `json:"refresh_rate"` // 秒
	CreatedBy     string                 `json:"created_by"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// WidgetConfig 小部件配置
type WidgetConfig struct {
	WidgetID    string                 `json:"widget_id"`
	WidgetType  string                 `json:"widget_type"` // CHART/TABLE/ALERT/KPI
	Title       string                 `json:"title"`
	MetricName  string                 `json:"metric_name"`
	Aggregation string                 `json:"aggregation"`
	Window      Window                 `json:"window"`
	Config      map[string]interface{} `json:"config"`
	Position    map[string]interface{} `json:"position"`
}

// MetricsQuery 指標查詢
type MetricsQuery struct {
	QueryID     string            `json:"query_id"`
	MetricNames []string          `json:"metric_names"`
	Tags        map[string]string `json:"tags"`
	Window      Window            `json:"window"`
	Aggregation string            `json:"aggregation"`
	GroupBy     []string          `json:"group_by"`
	Limit       int               `json:"limit"`
	CreatedAt   time.Time         `json:"created_at"`
}

// MetricsSubscription 指標訂閱
type MetricsSubscription struct {
	SubscriptionID string            `json:"subscription_id"`
	UserID         string            `json:"user_id"`
	MetricNames    []string          `json:"metric_names"`
	Tags           map[string]string `json:"tags"`
	CallbackURL    string            `json:"callback_url"`
	Enabled        bool              `json:"enabled"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// ================================
// S11 Metrics - Treasury 指標
// ================================

// TreasuryMetrics 資金劃轉指標
type TreasuryMetrics struct {
	MetricID   string    `json:"metric_id"`
	TransferID string    `json:"transfer_id"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	Amount     float64   `json:"amount_usdt"`
	LatencyMs  int64     `json:"latency_ms"`
	Status     string    `json:"status"` // SUCCESS/FAILED
	ErrorCode  string    `json:"error_code"`
	RetryCount int       `json:"retry_count"`
	Timestamp  int64     `json:"timestamp"`
	CreatedAt  time.Time `json:"created_at"`
}

// TreasurySLI Treasury 服務水平指標
type TreasurySLI struct {
	SLIID           string    `json:"sli_id"`
	MetricName      string    `json:"metric_name"`
	Value           float64   `json:"value"`
	Unit            string    `json:"unit"`
	Window          Window    `json:"window"`
	P95LatencyMs    int64     `json:"p95_latency_ms"`
	P99LatencyMs    int64     `json:"p99_latency_ms"`
	SuccessRate     float64   `json:"success_rate"`
	FailureRate     float64   `json:"failure_rate"`
	IdempotencyHits int       `json:"idempotency_hits"`
	TotalRequests   int       `json:"total_requests"`
	Timestamp       int64     `json:"timestamp"`
	CreatedAt       time.Time `json:"created_at"`
}
