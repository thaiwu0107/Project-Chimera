package dao

import "time"

// Position 持倉
type Position struct {
	PositionID string    `json:"position_id"`
	Symbol     string    `json:"symbol"`
	Market     string    `json:"market"`
	Side       string    `json:"side"` // LONG/SHORT
	Size       float64   `json:"size"`
	EntryPrice float64   `json:"entry_price"`
	MarkPrice  float64   `json:"mark_price"`
	PnL        float64   `json:"pnl"`
	PnLPct     float64   `json:"pnl_pct"`
	Margin     float64   `json:"margin"`
	Leverage   int       `json:"leverage"`
	Status     string    `json:"status"` // OPEN/CLOSED/PARTIAL
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// StopLoss 停損設置
type StopLoss struct {
	StopLossID    string    `json:"stop_loss_id"`
	PositionID    string    `json:"position_id"`
	StopPrice     float64   `json:"stop_price"`
	StopType      string    `json:"stop_type"` // FIXED/TRAILING/BREAKEVEN
	TrailDistance float64   `json:"trail_distance"`
	Status        string    `json:"status"` // ACTIVE/TRIGGERED/CANCELLED
	TriggeredAt   int64     `json:"triggered_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TakeProfit 停利設置
type TakeProfit struct {
	TakeProfitID string    `json:"take_profit_id"`
	PositionID   string    `json:"position_id"`
	TargetPrice  float64   `json:"target_price"`
	TargetPct    float64   `json:"target_pct"`
	ReducePct    float64   `json:"reduce_pct"` // 分批止盈比例
	Status       string    `json:"status"`     // ACTIVE/TRIGGERED/CANCELLED
	TriggeredAt  int64     `json:"triggered_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PositionManagement 持倉管理計劃
type PositionManagement struct {
	PlanID     string    `json:"plan_id"`
	PositionID string    `json:"position_id"`
	Action     string    `json:"action"` // STOP_MOVE/REDUCE/ADD/CLOSE
	OldValue   float64   `json:"old_value"`
	NewValue   float64   `json:"new_value"`
	Reason     string    `json:"reason"`
	Status     string    `json:"status"` // PENDING/EXECUTED/FAILED
	ExecutedAt int64     `json:"executed_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// RiskLimit 風險限制
type RiskLimit struct {
	LimitID         string    `json:"limit_id"`
	Symbol          string    `json:"symbol"`
	Market          string    `json:"market"`
	MaxSize         float64   `json:"max_size"`
	MaxLeverage     int       `json:"max_leverage"`
	MaxDrawdown     float64   `json:"max_drawdown"`
	MaxDailyLoss    float64   `json:"max_daily_loss"`
	MaxPosition     float64   `json:"max_position"`
	MaxExposure     float64   `json:"max_exposure"`
	CurrentExposure float64   `json:"current_exposure"`
	CurrentDrawdown float64   `json:"current_drawdown"`
	Status          string    `json:"status"` // ACTIVE/SUSPENDED/OK/WARNING/BREACH
	LastCheck       time.Time `json:"last_check"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// PositionAlert 持倉警報
type PositionAlert struct {
	AlertID      string    `json:"alert_id"`
	PositionID   string    `json:"position_id"`
	AlertType    string    `json:"alert_type"`     // DRAWDOWN/MARGIN_CALL/SIZE_LIMIT
	Severity     string    `json:"alert_severity"` // INFO/WARN/ERROR/CRITICAL
	Message      string    `json:"message"`
	Threshold    float64   `json:"threshold"`
	CurrentValue float64   `json:"current_value"`
	Status       string    `json:"status"` // ACTIVE/ACKNOWLEDGED/RESOLVED
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PositionHistory 持倉歷史
type PositionHistory struct {
	HistoryID  string    `json:"history_id"`
	PositionID string    `json:"position_id"`
	Action     string    `json:"action"` // OPEN/CLOSE/REDUCE/ADD/STOP_MOVE
	OldSize    float64   `json:"old_size"`
	NewSize    float64   `json:"new_size"`
	OldPrice   float64   `json:"old_price"`
	NewPrice   float64   `json:"new_price"`
	PnL        float64   `json:"pnl"`
	Reason     string    `json:"reason"`
	Timestamp  int64     `json:"timestamp"`
	CreatedAt  time.Time `json:"created_at"`
}

// PortfolioSummary 投資組合摘要
type PortfolioSummary struct {
	SummaryID     string    `json:"summary_id"`
	TotalPnL      float64   `json:"total_pnl"`
	TotalPnLPct   float64   `json:"total_pnl_pct"`
	TotalMargin   float64   `json:"total_margin"`
	TotalExposure float64   `json:"total_exposure"`
	PositionCount int       `json:"position_count"`
	OpenPositions int       `json:"open_positions"`
	Timestamp     int64     `json:"timestamp"`
	CreatedAt     time.Time `json:"created_at"`
}

// ================================
// S6 Position - 自動劃轉功能
// ================================

// AutoTransferConfig 自動劃轉配置
type AutoTransferConfig struct {
	ConfigID            string    `json:"config_id"`
	Enabled             bool      `json:"enabled"`
	SpotMarginThreshold float64   `json:"spot_margin_threshold"` // 現貨保證金閾值
	FutMarginThreshold  float64   `json:"fut_margin_threshold"`  // 期貨保證金閾值
	MinTransferAmount   float64   `json:"min_transfer_amount"`   // 最小劃轉金額
	MaxTransferAmount   float64   `json:"max_transfer_amount"`   // 最大劃轉金額
	TransferInterval    int       `json:"transfer_interval"`     // 劃轉間隔（秒）
	LastTransferTime    int64     `json:"last_transfer_time"`    // 上次劃轉時間
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// AutoTransferTrigger 自動劃轉觸發條件
type AutoTransferTrigger struct {
	TriggerID      string    `json:"trigger_id"`
	Symbol         string    `json:"symbol"`
	Market         string    `json:"market"`
	TriggerType    string    `json:"trigger_type"` // MARGIN_CALL/RISK_LIMIT/PROFIT_TAKING
	Threshold      float64   `json:"threshold"`
	TransferFrom   string    `json:"transfer_from"` // SPOT/FUT
	TransferTo     string    `json:"transfer_to"`   // FUT/SPOT
	TransferAmount float64   `json:"transfer_amount"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AutoTransferLog 自動劃轉日誌
type AutoTransferLog struct {
	LogID       string    `json:"log_id"`
	TriggerID   string    `json:"trigger_id"`
	Symbol      string    `json:"symbol"`
	Market      string    `json:"market"`
	TriggerType string    `json:"trigger_type"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Amount      float64   `json:"amount_usdt"`
	Reason      string    `json:"reason"`
	Status      string    `json:"status"` // PENDING/SUCCESS/FAILED
	TransferID  string    `json:"transfer_id"`
	ErrorMsg    string    `json:"error_msg"`
	Timestamp   time.Time `json:"timestamp"`
	CreatedAt   time.Time `json:"created_at"`
}

// MarginCall 保證金追繳
type MarginCall struct {
	CallID         string    `json:"call_id"`
	Symbol         string    `json:"symbol"`
	Market         string    `json:"market"`
	PositionID     string    `json:"position_id"`
	RequiredMargin float64   `json:"required_margin"`
	CurrentMargin  float64   `json:"current_margin"`
	Deficit        float64   `json:"deficit"`
	Status         string    `json:"status"` // PENDING/RESOLVED/FAILED
	ResolvedAt     time.Time `json:"resolved_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
