package apispec

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

type HealthStatus string

const (
	HealthOK       HealthStatus = "OK"
	HealthDegraded HealthStatus = "DEGRADED"
	HealthError    HealthStatus = "ERROR"
)

type HealthCheck struct {
	Name      string      `json:"name"`
	Status    HealthStatus `json:"status"`
	LatencyMs int64       `json:"latency_ms,omitempty"`
	Error     string      `json:"error,omitempty"`
}

type Market string

const (
	MarketFUT  Market = "FUT"
	MarketSPOT Market = "SPOT"
)

type DecisionAction string

const (
	ActionOpen DecisionAction = "open"
	ActionSkip DecisionAction = "skip"
)

type OrderIntentKind string

const (
	IntentEntry OrderIntentKind = "ENTRY"
	IntentAdd   OrderIntentKind = "ADD"
	IntentExit  OrderIntentKind = "EXIT"
	IntentHedge OrderIntentKind = "HEDGE"
)

type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

type Window struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type MetricPoint struct {
	Metric string            `json:"metric"`
	Value  float64           `json:"value"`
	Tags   map[string]string `json:"tags"`
	Ts     int64             `json:"ts"`
}

type Alert struct {
	AlertId  string `json:"alert_id"`
	Severity string `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
	Ts       int64  `json:"ts"`
}
