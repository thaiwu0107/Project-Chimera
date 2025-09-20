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
// S10 Config Service - API 結構
// ================================

type BundleUpsertRequest struct {
	BundleID    string         `json:"bundle_id" validate:"required,min=1,max=128"`                           // Bundle ID
	Rev         int            `json:"rev" validate:"required,gt=0"`                                          // 版本號（必須遞增）
	Factors     []string       `json:"factors" validate:"required,min=1,dive,min=1"`                          // 因子列表（非空）
	Rules       []string       `json:"rules" validate:"required,min=1,dive,min=1"`                            // 規則列表（非空）
	Instruments []string       `json:"instruments" validate:"required,min=1,dive,min=3,regexp=^[A-Z0-9]+$"`   // 交易對列表（非空）
	Flags       map[string]any `json:"flags,omitempty"`                                                       // 標誌
	Status      string         `json:"status" validate:"required,oneof=DRAFT STAGED ACTIVE ROLLBACK REVOKED"` // DRAFT/STAGED/ACTIVE/ROLLBACK/REVOKED
}

type BundleUpsertResponse struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type BundleStageResponse struct {
	Ok bool `json:"ok"`
}

type PromoteRequest struct {
	BundleID   string `json:"bundle_id" validate:"required,min=1,max=128"`              // Bundle ID
	ToRev      int    `json:"to_rev" validate:"required,gt=0"`                          // 目標版本號（必須大於現行）
	Mode       string `json:"mode" validate:"required,oneof=CANARY RAMP FULL ROLLBACK"` // CANARY/RAMP/FULL/ROLLBACK
	TrafficPct int    `json:"traffic_pct,omitempty" validate:"omitempty,min=1,max=50"`  // Canary 流量百分比（1-50）
	DurationH  int    `json:"duration_h,omitempty" validate:"omitempty,min=24,max=336"` // Canary 持續時間（24-336小時）
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
// S10 Config Service - 數據模型
// ================================

// ConfigBundle 配置包
type ConfigBundle struct {
	BundleID    string                 `json:"bundle_id"`
	Rev         int                    `json:"rev"`
	Factors     []string               `json:"factors"`
	Rules       []string               `json:"rules"`
	Instruments []string               `json:"instruments"`
	Flags       map[string]interface{} `json:"flags"`
	Status      string                 `json:"status"` // DRAFT/STAGED/ACTIVE/ROLLBACK/REVOKED
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Factor 因子定義
type Factor struct {
	FactorID    string                 `json:"factor_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"` // TECHNICAL/FUNDAMENTAL/SENTIMENT
	Parameters  map[string]interface{} `json:"parameters"`
	Formula     string                 `json:"formula"`
	Status      string                 `json:"status"` // ACTIVE/DEPRECATED
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Rule 規則定義
type Rule struct {
	RuleID      string                 `json:"rule_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"` // ENTRY/EXIT/RISK
	Conditions  map[string]interface{} `json:"conditions"`
	Actions     map[string]interface{} `json:"actions"`
	Priority    int                    `json:"priority"`
	Status      string                 `json:"status"` // ACTIVE/DEPRECATED
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Instrument 標的定義
type Instrument struct {
	InstrumentID string                 `json:"instrument_id"`
	Symbol       string                 `json:"symbol"`
	Market       string                 `json:"market"`
	Type         string                 `json:"type"` // FUTURE/SPOT/OPTION
	Parameters   map[string]interface{} `json:"parameters"`
	Status       string                 `json:"status"` // ACTIVE/DEPRECATED
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// Promotion 推廣記錄
type Promotion struct {
	PromotionID string    `json:"promotion_id"`
	BundleID    string    `json:"bundle_id"`
	Mode        string    `json:"mode"`     // CANARY/RAMP/FULL/ROLLBACK
	Status      string    `json:"status"`   // PENDING/ACTIVE/ROLLED_BACK/DONE
	Progress    float64   `json:"progress"` // 0-100
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	ErrorMsg    string    `json:"error_msg"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Simulation 模擬任務
type Simulation struct {
	SimID        string                 `json:"sim_id"`
	BundleID     string                 `json:"bundle_id"`
	ActiveRevRef string                 `json:"active_rev_ref"`
	Window       Window                 `json:"window"`
	Symbols      []string               `json:"symbols"`
	Horizons     []string               `json:"horizons"`
	Sensitivity  *SensitivitySpec       `json:"sensitivity"`
	Status       string                 `json:"status"`   // QUEUED/RUNNING/DONE/FAILED
	Progress     float64                `json:"progress"` // 0-100
	Results      map[string]interface{} `json:"results"`
	ErrorMsg     string                 `json:"error_msg"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	CompletedAt  time.Time              `json:"completed_at"`
}

// ActiveConfig 當前活躍配置
type ActiveConfig struct {
	ConfigID    string    `json:"config_id"`
	Rev         int       `json:"rev"`
	BundleID    string    `json:"bundle_id"`
	ActivatedAt int64     `json:"activated_at"`
	ActivatedBy string    `json:"activated_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ConfigValidation 配置驗證
type ConfigValidation struct {
	ValidationID string            `json:"validation_id"`
	BundleID     string            `json:"bundle_id"`
	Type         string            `json:"type"`   // LINT/DRY_RUN/SIMULATION
	Status       string            `json:"status"` // PASS/FAIL/WARN
	Issues       []ValidationIssue `json:"issues"`
	CreatedAt    time.Time         `json:"created_at"`
}

// ValidationIssue 驗證問題
type ValidationIssue struct {
	IssueID   string `json:"issue_id"`
	Type      string `json:"type"`      // ERROR/WARNING/INFO
	Severity  string `json:"severity"`  // HIGH/MEDIUM/LOW
	Component string `json:"component"` // FACTOR/RULE/INSTRUMENT
	Message   string `json:"message"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
}

// ConfigHistory 配置歷史
type ConfigHistory struct {
	HistoryID string    `json:"history_id"`
	BundleID  string    `json:"bundle_id"`
	Rev       int       `json:"rev"`
	Action    string    `json:"action"`  // CREATE/UPDATE/DELETE/PROMOTE/ROLLBACK
	Changes   string    `json:"changes"` // JSON 字符串
	UserID    string    `json:"user_id"`
	Timestamp int64     `json:"timestamp"`
	CreatedAt time.Time `json:"created_at"`
}
