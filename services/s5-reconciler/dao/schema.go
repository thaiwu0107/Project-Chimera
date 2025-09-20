package dao

import "time"

// ReconciliationTask 對帳任務
type ReconciliationTask struct {
	TaskID    string    `json:"task_id"`
	Mode      string    `json:"mode" validate:"required,oneof=ALL ORDERS POSITIONS"` // ALL/ORDERS/POSITIONS/HOLDINGS
	Status    string    `json:"status"`                                              // PENDING/RUNNING/COMPLETED/FAILED
	StartTime int64     `json:"start_time"`
	EndTime   int64     `json:"end_time"`
	Progress  float64   `json:"progress"` // 0-100
	ErrorMsg  string    `json:"error_msg"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ReconciliationResult 對帳結果
type ReconciliationResult struct {
	ResultID        string    `json:"result_id"`
	TaskID          string    `json:"task_id"`
	Type            string    `json:"type"` // ORDER/POSITION/HOLDING
	ExchangeOrderID string    `json:"exchange_order_id"`
	InternalOrderID string    `json:"internal_order_id"`
	Symbol          string    `json:"symbol"`
	Market          string    `json:"market"`
	Status          string    `json:"status"`  // MATCHED/MISMATCHED/MISSING/ORPHAN
	Details         string    `json:"details"` // JSON 字符串
	Fixed           bool      `json:"fixed"`
	CreatedAt       time.Time `json:"created_at"`
}

// OrderState 訂單狀態
type OrderState struct {
	OrderID        string    `json:"order_id"`
	Symbol         string    `json:"symbol"`
	Market         string    `json:"market"`
	Side           string    `json:"side"`
	Quantity       float64   `json:"quantity"`
	ExecutedQty    float64   `json:"executed_qty"`
	AvgPrice       float64   `json:"avg_price"`
	Status         string    `json:"status"`
	ExchangeStatus string    `json:"exchange_status"`
	LastSyncTime   int64     `json:"last_sync_time"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// PositionState 持倉狀態
type PositionState struct {
	PositionID   string    `json:"position_id"`
	Symbol       string    `json:"symbol"`
	Market       string    `json:"market"`
	Side         string    `json:"side"`
	Size         float64   `json:"size"`
	EntryPrice   float64   `json:"entry_price"`
	MarkPrice    float64   `json:"mark_price"`
	PnL          float64   `json:"pnl"`
	ExchangeSize float64   `json:"exchange_size"`
	ExchangePnL  float64   `json:"exchange_pnl"`
	LastSyncTime int64     `json:"last_sync_time"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// HoldingState 持倉狀態
type HoldingState struct {
	HoldingID      string    `json:"holding_id"`
	Asset          string    `json:"asset"`
	Market         string    `json:"market"`
	Free           float64   `json:"free"`
	Locked         float64   `json:"locked"`
	Total          float64   `json:"total"`
	ExchangeFree   float64   `json:"exchange_free"`
	ExchangeLocked float64   `json:"exchange_locked"`
	ExchangeTotal  float64   `json:"exchange_total"`
	LastSyncTime   int64     `json:"last_sync_time"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// OrphanRecord 孤兒記錄
type OrphanRecord struct {
	OrphanID   string    `json:"orphan_id"`
	Type       string    `json:"type"` // ORDER/POSITION/HOLDING
	ExchangeID string    `json:"exchange_id"`
	Symbol     string    `json:"symbol"`
	Market     string    `json:"market"`
	Details    string    `json:"details"` // JSON 字符串
	Status     string    `json:"status"`  // OPEN/CLOSED/IGNORED
	ClosedAt   int64     `json:"closed_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SyncLog 同步日誌
type SyncLog struct {
	LogID      string    `json:"log_id"`
	TaskID     string    `json:"task_id"`
	Action     string    `json:"action"`      // SYNC/FIX/CREATE/UPDATE/DELETE
	EntityType string    `json:"entity_type"` // ORDER/POSITION/HOLDING
	EntityID   string    `json:"entity_id"`
	Message    string    `json:"message"`
	Details    string    `json:"details"` // JSON 字符串
	Timestamp  int64     `json:"timestamp"`
	CreatedAt  time.Time `json:"created_at"`
}
