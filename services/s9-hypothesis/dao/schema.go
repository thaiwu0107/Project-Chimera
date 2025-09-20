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
// S9 Hypothesis Orchestrator - API 結構
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
// S9 Hypothesis Orchestrator - 數據模型
// ================================

// Hypothesis 假設
type Hypothesis struct {
	HypothesisID string                 `json:"hypothesis_id"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Type         string                 `json:"type"` // WALK_FORWARD/PURGED_K_FOLD/MONTE_CARLO
	Parameters   map[string]interface{} `json:"parameters"`
	Status       string                 `json:"status"` // DRAFT/ACTIVE/DEPRECATED
	CreatedBy    string                 `json:"created_by"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// Experiment 實驗
type Experiment struct {
	ExpID        string    `json:"exp_id"`
	HypothesisID string    `json:"hypothesis_id"`
	Status       string    `json:"status"`   // QUEUED/RUNNING/DONE/FAILED
	Progress     float64   `json:"progress"` // 0-100
	Window       Window    `json:"window"`
	ConfigRev    int       `json:"config_rev"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	ErrorMsg     string    `json:"error_msg"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// WalkForwardFold Walk-Forward 折疊
type WalkForwardFold struct {
	FoldID      string    `json:"fold_id"`
	ExpID       string    `json:"exp_id"`
	FoldIndex   int       `json:"fold_index"`
	TrainWindow Window    `json:"train_window"`
	TestWindow  Window    `json:"test_window"`
	Status      string    `json:"status"` // PENDING/RUNNING/COMPLETED/FAILED
	TrainScore  float64   `json:"train_score"`
	TestScore   float64   `json:"test_score"`
	Overfitting float64   `json:"overfitting"` // 過擬合指標
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// KFoldSplit K-Fold 分割
type KFoldSplit struct {
	SplitID      string    `json:"split_id"`
	ExpID        string    `json:"exp_id"`
	SplitIndex   int       `json:"split_index"`
	TrainIndices []int     `json:"train_indices"`
	TestIndices  []int     `json:"test_indices"`
	PurgePeriod  int64     `json:"purge_period"` // 淨化期間（毫秒）
	Status       string    `json:"status"`       // PENDING/RUNNING/COMPLETED/FAILED
	TrainScore   float64   `json:"train_score"`
	TestScore    float64   `json:"test_score"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// ExperimentResult 實驗結果
type ExperimentResult struct {
	ResultID    string                 `json:"result_id"`
	ExpID       string                 `json:"exp_id"`
	FoldID      string                 `json:"fold_id,omitempty"`
	SplitID     string                 `json:"split_id,omitempty"`
	Metrics     map[string]interface{} `json:"metrics"`
	Predictions []Prediction           `json:"predictions"`
	Signals     []Signal               `json:"signals"`
	PnL         float64                `json:"pnl"`
	SharpeRatio float64                `json:"sharpe_ratio"`
	MaxDrawdown float64                `json:"max_drawdown"`
	WinRate     float64                `json:"win_rate"`
	CreatedAt   time.Time              `json:"created_at"`
}

// Prediction 預測結果
type Prediction struct {
	PredictionID   string    `json:"prediction_id"`
	ResultID       string    `json:"result_id"`
	Timestamp      int64     `json:"timestamp"`
	Symbol         string    `json:"symbol"`
	PredictedValue float64   `json:"predicted_value"`
	ActualValue    float64   `json:"actual_value"`
	Confidence     float64   `json:"confidence"`
	CreatedAt      time.Time `json:"created_at"`
}

// Signal 信號（實驗中生成）
type Signal struct {
	SignalID   string                 `json:"signal_id"`
	ResultID   string                 `json:"result_id"`
	Timestamp  int64                  `json:"timestamp"`
	Symbol     string                 `json:"symbol"`
	Market     string                 `json:"market"`
	Side       string                 `json:"side"`
	Confidence float64                `json:"confidence"`
	Features   map[string]interface{} `json:"features"`
	CreatedAt  time.Time              `json:"created_at"`
}

// ExperimentConfig 實驗配置
type ExperimentConfig struct {
	ConfigID   string                 `json:"config_id"`
	ExpID      string                 `json:"exp_id"`
	ConfigType string                 `json:"config_type"` // STRATEGY/FEATURE/RISK
	Parameters map[string]interface{} `json:"parameters"`
	Version    int                    `json:"version"`
	CreatedAt  time.Time              `json:"created_at"`
}

// ExperimentQueue 實驗隊列
type ExperimentQueue struct {
	QueueID     string    `json:"queue_id"`
	ExpID       string    `json:"exp_id"`
	Priority    int       `json:"priority"`
	Status      string    `json:"status"` // QUEUED/RUNNING/COMPLETED/FAILED
	QueuedAt    time.Time `json:"queued_at"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	WorkerID    string    `json:"worker_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
