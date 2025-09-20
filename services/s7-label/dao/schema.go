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
// S7 Label Backfill - API 結構
// ================================

type BackfillRequest struct {
	HorizonH int   `json:"horizon_h" validate:"required,oneof=12 24 36"` // 12|24|36
	FromTs   int64 `json:"from_ts,omitempty" validate:"omitempty,gt=0"`  // 開始時間戳
	ToTs     int64 `json:"to_ts,omitempty" validate:"omitempty,gt=0"`    // 結束時間戳
}

type BackfillResponse struct {
	Updated int    `json:"updated"`
	Message string `json:"message,omitempty"`
}

// ================================
// S7 Label Backfill - 數據模型
// ================================

// Signal 交易信號
type Signal struct {
	SignalID  string                 `json:"signal_id"`
	Symbol    string                 `json:"symbol"`
	Market    string                 `json:"market"`
	Features  map[string]interface{} `json:"features"`
	ConfigRev int                    `json:"config_rev"`
	Timestamp int64                  `json:"timestamp"`
	CreatedAt time.Time              `json:"created_at"`
}

// SignalLabel 信號標籤
type SignalLabel struct {
	LabelID     string    `json:"label_id"`
	SignalID    string    `json:"signal_id"`
	HorizonH    int       `json:"horizon_h"`    // 12|24|36
	PnL         float64   `json:"pnl"`          // 淨利
	PnLPct      float64   `json:"pnl_pct"`      // 淨利百分比
	MaxDrawdown float64   `json:"max_drawdown"` // 最大回撤
	SharpeRatio float64   `json:"sharpe_ratio"` // 夏普比率
	WinRate     float64   `json:"win_rate"`     // 勝率
	Label       string    `json:"label"`        // POSITIVE/NEGATIVE/NEUTRAL
	Confidence  float64   `json:"confidence"`   // 置信度 0-1
	Timestamp   int64     `json:"timestamp"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BackfillTask 回填任務
type BackfillTask struct {
	TaskID      string    `json:"task_id"`
	HorizonH    int       `json:"horizon_h"`
	SinceMs     int64     `json:"since_ms"`
	UntilMs     int64     `json:"until_ms"`
	Status      string    `json:"status"`    // PENDING/RUNNING/COMPLETED/FAILED
	Progress    float64   `json:"progress"`  // 0-100
	Processed   int       `json:"processed"` // 已處理數量
	Total       int       `json:"total"`     // 總數量
	ErrorMsg    string    `json:"error_msg"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// LabelRule 標籤規則
type LabelRule struct {
	RuleID     string    `json:"rule_id"`
	RuleName   string    `json:"rule_name"`
	HorizonH   int       `json:"horizon_h"`
	Conditions string    `json:"conditions"` // JSON 字符串
	Label      string    `json:"label"`      // POSITIVE/NEGATIVE/NEUTRAL
	Priority   int       `json:"priority"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// PerformanceMetrics 績效指標
type PerformanceMetrics struct {
	MetricsID   string    `json:"metrics_id"`
	SignalID    string    `json:"signal_id"`
	HorizonH    int       `json:"horizon_h"`
	PnL         float64   `json:"pnl"`
	PnLPct      float64   `json:"pnl_pct"`
	MaxDrawdown float64   `json:"max_drawdown"`
	SharpeRatio float64   `json:"sharpe_ratio"`
	WinRate     float64   `json:"win_rate"`
	Volatility  float64   `json:"volatility"`
	Timestamp   int64     `json:"timestamp"`
	CreatedAt   time.Time `json:"created_at"`
}

// LabelStatistics 標籤統計
type LabelStatistics struct {
	StatsID      string    `json:"stats_id"`
	HorizonH     int       `json:"horizon_h"`
	TotalSignals int       `json:"total_signals"`
	Positive     int       `json:"positive"`
	Negative     int       `json:"negative"`
	Neutral      int       `json:"neutral"`
	AvgPnL       float64   `json:"avg_pnl"`
	AvgPnLPct    float64   `json:"avg_pnl_pct"`
	WinRate      float64   `json:"win_rate"`
	Timestamp    int64     `json:"timestamp"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
