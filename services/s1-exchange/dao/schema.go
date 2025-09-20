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
// S1 Exchange Connectors - 數據模型
// ================================

// MarketData 市場數據結構
type MarketData struct {
	Symbol    string    `json:"symbol"`    // 交易對
	Market    string    `json:"market"`    // FUT/SPOT
	Price     float64   `json:"price"`     // 價格
	Volume    float64   `json:"volume"`    // 成交量
	Timestamp int64     `json:"timestamp"` // 時間戳
	CreatedAt time.Time `json:"created_at"`
}

// OrderBook 訂單簿數據
type OrderBook struct {
	Symbol    string    `json:"symbol"`
	Market    string    `json:"market"`
	Bids      []BidAsk  `json:"bids"` // 買盤
	Asks      []BidAsk  `json:"asks"` // 賣盤
	Timestamp int64     `json:"timestamp"`
	CreatedAt time.Time `json:"created_at"`
}

// BidAsk 買賣盤數據
type BidAsk struct {
	Price float64 `json:"price"`
	Qty   float64 `json:"qty"`
}

// FundingRate 資金費率
type FundingRate struct {
	Symbol    string    `json:"symbol"`
	Rate      float64   `json:"rate"`
	NextRate  float64   `json:"next_rate"`
	Timestamp int64     `json:"timestamp"`
	CreatedAt time.Time `json:"created_at"`
}

// AccountBalance 帳戶餘額
type AccountBalance struct {
	Asset     string    `json:"asset"`
	Free      float64   `json:"free"`
	Locked    float64   `json:"locked"`
	Total     float64   `json:"total"`
	Market    string    `json:"market"`
	Timestamp int64     `json:"timestamp"`
	CreatedAt time.Time `json:"created_at"`
}

// Position 持倉信息
type Position struct {
	Symbol     string    `json:"symbol"`
	Market     string    `json:"market"`
	Side       string    `json:"side"` // LONG/SHORT
	Size       float64   `json:"size"`
	EntryPrice float64   `json:"entry_price"`
	MarkPrice  float64   `json:"mark_price"`
	PnL        float64   `json:"pnl"`
	Timestamp  int64     `json:"timestamp"`
	CreatedAt  time.Time `json:"created_at"`
}

// WebSocketConnection WebSocket 連接狀態
type WebSocketConnection struct {
	Symbol     string    `json:"symbol"`
	Market     string    `json:"market"`
	Status     string    `json:"status"` // CONNECTED/DISCONNECTED/RECONNECTING
	LastPing   int64     `json:"last_ping"`
	ErrorCount int       `json:"error_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ================================
// S1 Exchange - Treasury 子模組
// ================================

// TransferRequest 期貨/現貨之間資金劃轉請求（內部使用）
type TransferRequest struct {
	From           string  `json:"from"`            // "SPOT" | "FUT"
	To             string  `json:"to"`              // "FUT" | "SPOT"
	Amount         float64 `json:"amount_usdt"`     // 劃轉 USDT 數
	Reason         string  `json:"reason"`          // 記帳理由
	IdempotencyKey string  `json:"idempotency_key"` // 冪等性鍵值
}

// TransferResponse 資金劃轉回應（內部使用）
type TransferResponse struct {
	TransferID string `json:"transfer_id"`
	Result     string `json:"result"` // OK|FAIL
	Message    string `json:"message,omitempty"`
	Debug      string `json:"debug,omitempty"` // 原始交易所回執
}

// BinanceTransferRequest 幣安劃轉請求
type BinanceTransferRequest struct {
	Asset     string  `json:"asset"`  // USDT
	Amount    float64 `json:"amount"` // 劃轉數量
	Type      int     `json:"type"`   // 1: 現貨轉期貨, 2: 期貨轉現貨
	Timestamp int64   `json:"timestamp"`
}

// BinanceTransferResponse 幣安劃轉回應
type BinanceTransferResponse struct {
	TranID int64  `json:"tranId"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// TreasuryTransferLog 資金劃轉日誌
type TreasuryTransferLog struct {
	LogID          string    `json:"log_id"`
	TransferID     string    `json:"transfer_id"`
	From           string    `json:"from"`
	To             string    `json:"to"`
	Amount         float64   `json:"amount_usdt"`
	IdempotencyKey string    `json:"idempotency_key"`
	BinanceTranID  int64     `json:"binance_tran_id"`
	Status         string    `json:"status"` // PENDING/SUCCESS/FAILED
	ErrorMsg       string    `json:"error_msg"`
	RetryCount     int       `json:"retry_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ExchangeCredentials 交易所憑證
type ExchangeCredentials struct {
	APIKey     string `json:"api_key"`
	SecretKey  string `json:"secret_key"`
	Passphrase string `json:"passphrase,omitempty"`
	Sandbox    bool   `json:"sandbox"`
}

// TreasuryConfig 資金劃轉配置
type TreasuryConfig struct {
	MaxRetryCount     int           `json:"max_retry_count"`
	RetryInterval     time.Duration `json:"retry_interval"`
	Timeout           time.Duration `json:"timeout"`
	RateLimitPerMin   int           `json:"rate_limit_per_min"`
	MinTransferAmount float64       `json:"min_transfer_amount"`
	MaxTransferAmount float64       `json:"max_transfer_amount"`
}
