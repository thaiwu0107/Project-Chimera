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
// S12 Web UI / API Gateway - API 結構
// ================================

type KillSwitchRequest struct {
	Enable bool `json:"enable"` // true=啟動停機；false=解除
}

type KillSwitchResponse struct {
	Enabled bool `json:"enabled"`
}

// TransferRequest 期貨/現貨之間資金劃轉（若啟用）
type TransferRequest struct {
	From   string  `json:"from"`        // "SPOT" | "FUT"
	To     string  `json:"to"`          // "FUT" | "SPOT"
	Amount float64 `json:"amount_usdt"` // 劃轉 USDT 數
	Reason string  `json:"reason"`      // 記帳理由
}

type TransferResponse struct {
	TransferID string `json:"transfer_id"`
	Result     string `json:"result"` // OK|FAIL
	Message    string `json:"message,omitempty"`
	Debug      string `json:"debug,omitempty"` // 原始交易所回執（內部使用）
}

// IdempotencyKey 冪等性鍵值
type IdempotencyKey struct {
	Key       string  `json:"key"`
	From      string  `json:"from"`
	To        string  `json:"to"`
	Amount    float64 `json:"amount"`
	Timestamp int64   `json:"timestamp"`
	Hash      string  `json:"hash"`
}

// TreasuryTransferAudit 資金劃轉審計記錄
type TreasuryTransferAudit struct {
	AuditID        string    `json:"audit_id"`
	TransferID     string    `json:"transfer_id"`
	UserID         string    `json:"user_id"`
	Username       string    `json:"username"`
	From           string    `json:"from"`
	To             string    `json:"to"`
	Amount         float64   `json:"amount_usdt"`
	Reason         string    `json:"reason"`
	IdempotencyKey string    `json:"idempotency_key"`
	Status         string    `json:"status"` // PENDING/SUCCESS/FAILED
	ErrorMsg       string    `json:"error_msg"`
	IPAddress      string    `json:"ip_address"`
	UserAgent      string    `json:"user_agent"`
	Timestamp      time.Time `json:"timestamp"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TreasuryLock 資金劃轉分散鎖
type TreasuryLock struct {
	LockID    string    `json:"lock_id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// RiskBudget 風險預算配置
type RiskBudget struct {
	SpotQuoteUsdtMax    float64 `json:"spot_quote_usdt_max"`
	FutMarginUsdtMax    float64 `json:"fut_margin_usdt_max"`
	DailyTransferLimit  float64 `json:"daily_transfer_limit"`
	HourlyTransferLimit float64 `json:"hourly_transfer_limit"`
}

// ================================
// S12 Web UI / API Gateway - 數據模型
// ================================

// UserSession 用戶會話
type UserSession struct {
	SessionID   string    `json:"session_id"`
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	Role        string    `json:"role"`        // ADMIN/TRADER/VIEWER
	Permissions []string  `json:"permissions"` // kill_switch/treasury_transfer/view_metrics
	LoginTime   time.Time `json:"login_time"`
	LastActive  time.Time `json:"last_active"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// DashboardLayout 儀表板佈局
type DashboardLayout struct {
	LayoutID   string                 `json:"layout_id"`
	UserID     string                 `json:"user_id"`
	LayoutName string                 `json:"layout_name"`
	Widgets    []WidgetConfig         `json:"widgets"`
	Layout     map[string]interface{} `json:"layout"`
	IsDefault  bool                   `json:"is_default"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// WidgetConfig 小部件配置
type WidgetConfig struct {
	WidgetID    string                 `json:"widget_id"`
	WidgetType  string                 `json:"widget_type"` // CHART/TABLE/ALERT/KPI/METRICS
	Title       string                 `json:"title"`
	Service     string                 `json:"service"`  // s1-exchange/s2-feature/...
	Endpoint    string                 `json:"endpoint"` // /health/metrics/alerts
	Config      map[string]interface{} `json:"config"`
	Position    map[string]interface{} `json:"position"`
	RefreshRate int                    `json:"refresh_rate"` // 秒
	Enabled     bool                   `json:"enabled"`
}

// KillSwitchLog 停機開關日誌
type KillSwitchLog struct {
	LogID     string    `json:"log_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Action    string    `json:"action"` // ENABLE/DISABLE
	Reason    string    `json:"reason"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Timestamp time.Time `json:"timestamp"`
	CreatedAt time.Time `json:"created_at"`
}

// TreasuryTransferLog 資金劃轉日誌
type TreasuryTransferLog struct {
	TransferID string    `json:"transfer_id"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	From       string    `json:"from"` // SPOT/FUT
	To         string    `json:"to"`   // FUT/SPOT
	Amount     float64   `json:"amount_usdt"`
	Reason     string    `json:"reason"`
	Status     string    `json:"status"` // PENDING/SUCCESS/FAILED
	ErrorMsg   string    `json:"error_msg"`
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	Timestamp  time.Time `json:"timestamp"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SystemStatus 系統狀態
type SystemStatus struct {
	StatusID   string          `json:"status_id"`
	KillSwitch bool            `json:"kill_switch"`
	Services   []ServiceStatus `json:"services"`
	LastUpdate time.Time       `json:"last_update"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ServiceStatus 服務狀態
type ServiceStatus struct {
	ServiceID    string       `json:"service_id"`
	ServiceName  string       `json:"service_name"`
	Status       HealthStatus `json:"status"`
	Version      string       `json:"version"`
	UptimeMs     int64        `json:"uptime_ms"`
	LastCheck    time.Time    `json:"last_check"`
	ErrorCount   int          `json:"error_count"`
	ResponseTime int64        `json:"response_time_ms"`
}

// UserPreference 用戶偏好設定
type UserPreference struct {
	PreferenceID  string                 `json:"preference_id"`
	UserID        string                 `json:"user_id"`
	Theme         string                 `json:"theme"`    // light/dark
	Language      string                 `json:"language"` // en/zh
	Timezone      string                 `json:"timezone"`
	RefreshRate   int                    `json:"refresh_rate"` // 秒
	Notifications map[string]bool        `json:"notifications"`
	Settings      map[string]interface{} `json:"settings"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// APIGatewayConfig API 網關配置
type APIGatewayConfig struct {
	ConfigID     string    `json:"config_id"`
	ServiceName  string    `json:"service_name"`
	BaseURL      string    `json:"base_url"`
	HealthPath   string    `json:"health_path"`
	Timeout      int       `json:"timeout_ms"`
	RetryCount   int       `json:"retry_count"`
	RateLimit    int       `json:"rate_limit_per_min"`
	AuthRequired bool      `json:"auth_required"`
	Permissions  []string  `json:"permissions"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AuditLog 審計日誌
type AuditLog struct {
	LogID     string    `json:"log_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Action    string    `json:"action"`   // LOGIN/LOGOUT/KILL_SWITCH/TRANSFER/VIEW
	Resource  string    `json:"resource"` // 操作的資源
	Details   string    `json:"details"`  // 操作詳情
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Success   bool      `json:"success"`
	ErrorMsg  string    `json:"error_msg"`
	Timestamp time.Time `json:"timestamp"`
	CreatedAt time.Time `json:"created_at"`
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	ConfigID  string                 `json:"config_id"`
	UserID    string                 `json:"user_id"`
	Type      string                 `json:"type"` // EMAIL/SMS/WEBHOOK
	Endpoint  string                 `json:"endpoint"`
	Events    []string               `json:"events"` // kill_switch/transfer/alert
	Enabled   bool                   `json:"enabled"`
	Settings  map[string]interface{} `json:"settings"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// SystemMetrics 系統指標聚合
type SystemMetrics struct {
	MetricsID        string    `json:"metrics_id"`
	Timestamp        int64     `json:"timestamp"`
	TotalServices    int       `json:"total_services"`
	HealthyServices  int       `json:"healthy_services"`
	DegradedServices int       `json:"degraded_services"`
	ErrorServices    int       `json:"error_services"`
	AvgResponseTime  float64   `json:"avg_response_time_ms"`
	TotalAlerts      int       `json:"total_alerts"`
	CriticalAlerts   int       `json:"critical_alerts"`
	CreatedAt        time.Time `json:"created_at"`
}
