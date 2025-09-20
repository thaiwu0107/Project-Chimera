package apispec

// internal/apispec/spec.go
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
	Status         string  `json:"status"` // NEW/FILLED/PARTIALLY_FILLED/CANCELED/REJECTED
	AvgPrice       float64 `json:"avg_price,omitempty"`
	ExecutedQty    float64 `json:"executed_qty,omitempty"`
	Fills          []Fill  `json:"fills,omitempty"`
	GuardStopArmed bool    `json:"guard_stop_armed,omitempty"` // SPOT 守護停損佈署狀態
	Message        string  `json:"message,omitempty"`
}

type CancelRequest struct {
	Market   Market `json:"market"`                    // FUT|SPOT
	Symbol   string `json:"symbol"`                    // 交易對
	OrderID  string `json:"order_id,omitempty"`        // 交易所 ID（或）
	ClientID string `json:"client_order_id,omitempty"` // 自定冪等 ID
}

type CancelResponse struct {
	Canceled bool   `json:"canceled"`
	Message  string `json:"message,omitempty"`
}

// OrderIntent 訂單意圖（S4 需要引用）
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

// ExecPolicy 路由器執行策略（Maker/TWAP/OCO/守護停損）
type ExecPolicy struct {
	PreferMaker     bool    `json:"prefer_maker"`      // 優先掛單（失敗回退）
	MakerWaitMs     int     `json:"maker_wait_ms"`     // 掛單等待毫秒（超時撤單）
	TWAPSlices      int     `json:"twap_slices"`       // 分批切片數（≥1：啟用 TWAP）
	GuardStopEnable bool    `json:"guard_stop_enable"` // SPOT：本地守護停損
	TPPct           float64 `json:"tp_pct,omitempty"`  // 絕對停利%
	SLPct           float64 `json:"sl_pct,omitempty"`  // 絕對停損%
	OCO             *OCO    `json:"oco,omitempty"`     // SPOT：OCO 兩腿
}

// OCO SPOT 一單兩腿（TakeProfitPx/StopLossPx 以「價格」定義）
type OCO struct {
	TakeProfitPx float64 `json:"take_profit_px"` // 停利觸發價
	StopLossPx   float64 `json:"stop_loss_px"`   // 停損觸發價
}
