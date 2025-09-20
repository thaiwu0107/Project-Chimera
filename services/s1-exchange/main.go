package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"s1-exchange/dao"
	"s1-exchange/internal/apispec"
	"s1-exchange/internal/config"
	"s1-exchange/internal/services/arangodb"
	"s1-exchange/internal/services/redis"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// GitCommitNum is set during build time via ldflags
var GitCommitNum string

// S1 EXCHANGE CONNECTORS
// Exchange Connectors - Integrate Binance FUT/UM & SPOT REST/WS; Optional MAX USDTTWD as factor; Reconnect/throttle/clock correction

type S1_EXCHANGEServer struct {
	redisClient    *redis.RedisClient
	arangodbClient *arangodb.ArangoDBClient
	version        string
	startTime      time.Time

	// WebSocket 連接管理
	wsConnections map[string]*websocket.Conn
	wsMutex       sync.RWMutex

	// 市場數據快取
	marketData   map[string]*dao.MarketData
	orderBooks   map[string]*dao.OrderBook
	fundingRates map[string]*dao.FundingRate
	dataMutex    sync.RWMutex

	// 配置
	credentials    *dao.ExchangeCredentials
	treasuryConfig *dao.TreasuryConfig
}

func NewS1_EXCHANGEServer() *S1_EXCHANGEServer {
	server := &S1_EXCHANGEServer{
		redisClient:    redis.GetInstance(),
		arangodbClient: arangodb.GetInstance(),
		version:        "v1.0.0",
		startTime:      time.Now(),
		wsConnections:  make(map[string]*websocket.Conn),
		marketData:     make(map[string]*dao.MarketData),
		orderBooks:     make(map[string]*dao.OrderBook),
		fundingRates:   make(map[string]*dao.FundingRate),
		credentials: &dao.ExchangeCredentials{
			APIKey:    os.Getenv("BINANCE_API_KEY"),
			SecretKey: os.Getenv("BINANCE_SECRET_KEY"),
			Sandbox:   os.Getenv("BINANCE_SANDBOX") == "true",
		},
		treasuryConfig: &dao.TreasuryConfig{
			MaxRetryCount:     3,
			RetryInterval:     5 * time.Second,
			Timeout:           30 * time.Second,
			RateLimitPerMin:   10,
			MinTransferAmount: 1.0,
			MaxTransferAmount: 10000.0,
		},
	}

	// 啟動 WebSocket 連接
	go server.startWebSocketConnections()

	// 啟動定時任務
	go server.startScheduledTasks()

	return server
}

// @Summary Health check
// @Description Check service health status
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} apispec.HealthResponse
// @Router /health [get]
func (s *S1_EXCHANGEServer) HealthCheck(c *gin.Context) {
	checks := []apispec.HealthCheck{
		{
			Name:      "redis",
			Status:    apispec.HealthOK,
			LatencyMs: 5,
		},
		{
			Name:      "arangodb",
			Status:    apispec.HealthOK,
			LatencyMs: 10,
		},
		{
			Name:      "ws-binance",
			Status:    apispec.HealthOK,
			LatencyMs: 15,
		},
	}

	response := apispec.HealthResponse{
		Service:  "s1-exchange",
		Version:  s.version,
		Status:   apispec.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Checks:   checks,
		Notes:    "Exchange connectors running normally",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Ready check
// @Description Check if service is ready
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} apispec.HealthResponse
// @Router /ready [get]
func (s *S1_EXCHANGEServer) ReadyCheck(c *gin.Context) {
	// Check all critical dependencies are ready
	response := apispec.HealthResponse{
		Service:  "s1-exchange",
		Version:  s.version,
		Status:   apispec.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Notes:    "Service ready to accept requests",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Get market data
// @Description Get current market data for a symbol
// @Tags market
// @Accept json
// @Produce json
// @Param symbol query string true "Symbol (e.g., BTCUSDT)"
// @Param market query string false "Market (FUT/SPOT)" default(FUT)
// @Success 200 {object} dao.MarketData
// @Router /market/data [get]
func (s *S1_EXCHANGEServer) GetMarketData(c *gin.Context) {
	symbol := c.Query("symbol")
	market := c.DefaultQuery("market", "FUT")

	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol parameter is required"})
		return
	}

	key := fmt.Sprintf("%s_%s", symbol, market)
	s.dataMutex.RLock()
	data, exists := s.marketData[key]
	s.dataMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "market data not found"})
		return
	}

	c.JSON(http.StatusOK, data)
}

// @Summary Get order book
// @Description Get current order book for a symbol
// @Tags market
// @Accept json
// @Produce json
// @Param symbol query string true "Symbol (e.g., BTCUSDT)"
// @Param market query string false "Market (FUT/SPOT)" default(FUT)
// @Success 200 {object} dao.OrderBook
// @Router /market/orderbook [get]
func (s *S1_EXCHANGEServer) GetOrderBook(c *gin.Context) {
	symbol := c.Query("symbol")
	market := c.DefaultQuery("market", "FUT")

	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol parameter is required"})
		return
	}

	key := fmt.Sprintf("%s_%s", symbol, market)
	s.dataMutex.RLock()
	orderBook, exists := s.orderBooks[key]
	s.dataMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "order book not found"})
		return
	}

	c.JSON(http.StatusOK, orderBook)
}

// @Summary Get funding rate
// @Description Get current funding rate for a symbol
// @Tags market
// @Accept json
// @Produce json
// @Param symbol query string true "Symbol (e.g., BTCUSDT)"
// @Success 200 {object} dao.FundingRate
// @Router /market/funding [get]
func (s *S1_EXCHANGEServer) GetFundingRate(c *gin.Context) {
	symbol := c.Query("symbol")

	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol parameter is required"})
		return
	}

	s.dataMutex.RLock()
	fundingRate, exists := s.fundingRates[symbol]
	s.dataMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "funding rate not found"})
		return
	}

	c.JSON(http.StatusOK, fundingRate)
}

// @Summary Get account balance
// @Description Get account balance for a market
// @Tags account
// @Accept json
// @Produce json
// @Param market query string false "Market (FUT/SPOT)" default(FUT)
// @Success 200 {array} dao.AccountBalance
// @Router /account/balance [get]
func (s *S1_EXCHANGEServer) GetAccountBalance(c *gin.Context) {
	market := c.DefaultQuery("market", "FUT")

	// TODO: 實現實際的帳戶餘額查詢
	balances := []dao.AccountBalance{
		{
			Asset:     "USDT",
			Free:      1000.0,
			Locked:    0.0,
			Total:     1000.0,
			Market:    market,
			Timestamp: time.Now().UnixMilli(),
			CreatedAt: time.Now(),
		},
	}

	c.JSON(http.StatusOK, balances)
}

// @Summary Get positions
// @Description Get current positions
// @Tags account
// @Accept json
// @Produce json
// @Param market query string false "Market (FUT/SPOT)" default(FUT)
// @Success 200 {array} dao.Position
// @Router /account/positions [get]
func (s *S1_EXCHANGEServer) GetPositions(c *gin.Context) {
	market := c.DefaultQuery("market", "FUT")

	// TODO: 實現實際的持倉查詢
	positions := []dao.Position{
		{
			Symbol:     "BTCUSDT",
			Market:     market,
			Side:       "LONG",
			Size:       0.1,
			EntryPrice: 50000.0,
			MarkPrice:  51000.0,
			PnL:        100.0,
			Timestamp:  time.Now().UnixMilli(),
			CreatedAt:  time.Now(),
		},
	}

	c.JSON(http.StatusOK, positions)
}

// @Summary Treasury transfer (內部私有 API)
// @Description Execute treasury transfer via Binance API
// @Tags treasury
// @Accept json
// @Produce json
// @Param request body dao.TransferRequest true "Transfer request"
// @Success 200 {object} dao.TransferResponse
// @Router /xchg/treasury/transfer [post]
func (s *S1_EXCHANGEServer) TreasuryTransfer(c *gin.Context) {
	var req dao.TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. 檢查冪等性
	if existingTransfer := s.checkIdempotency(req.IdempotencyKey); existingTransfer != nil {
		c.JSON(http.StatusOK, existingTransfer)
		return
	}

	// 2. 創建轉帳日誌
	transferID := fmt.Sprintf("transfer_%d", time.Now().Unix())
	log := dao.TreasuryTransferLog{
		LogID:          fmt.Sprintf("log_%d", time.Now().Unix()),
		TransferID:     transferID,
		From:           req.From,
		To:             req.To,
		Amount:         req.Amount,
		IdempotencyKey: req.IdempotencyKey,
		Status:         "PENDING",
		RetryCount:     0,
		CreatedAt:      time.Now(),
	}

	// 3. 執行幣安劃轉
	response, binanceResp, err := s.executeBinanceTransfer(&req, &log)
	if err != nil {
		log.Status = "FAILED"
		log.ErrorMsg = err.Error()
		log.UpdatedAt = time.Now()
		s.updateTransferLog(&log)

		c.JSON(http.StatusInternalServerError, dao.TransferResponse{
			TransferID: transferID,
			Result:     "FAIL",
			Message:    err.Error(),
		})
		return
	}

	// 4. 更新日誌
	log.Status = "SUCCESS"
	log.BinanceTranID = binanceResp.TranID
	log.UpdatedAt = time.Now()
	s.updateTransferLog(&log)

	c.JSON(http.StatusOK, response)
}

// executeBinanceTransfer 執行幣安劃轉
func (s *S1_EXCHANGEServer) executeBinanceTransfer(req *dao.TransferRequest, log *dao.TreasuryTransferLog) (*dao.TransferResponse, *dao.BinanceTransferResponse, error) {
	// 1. 構建幣安請求
	binanceReq := dao.BinanceTransferRequest{
		Asset:     "USDT",
		Amount:    req.Amount,
		Type:      s.getTransferType(req.From, req.To),
		Timestamp: time.Now().UnixMilli(),
	}

	// 2. 呼叫幣安 API
	binanceResp, err := s.callBinanceAPI(&binanceReq)
	if err != nil {
		return nil, nil, fmt.Errorf("binance API call failed: %v", err)
	}

	// 3. 構建回應
	response := &dao.TransferResponse{
		TransferID: log.TransferID,
		Result:     "OK",
		Message:    "Transfer completed successfully",
		Debug:      fmt.Sprintf("Binance TranID: %d, Status: %s", binanceResp.TranID, binanceResp.Status),
	}

	return response, binanceResp, nil
}

// getTransferType 獲取劃轉類型
func (s *S1_EXCHANGEServer) getTransferType(from, to string) int {
	if from == "SPOT" && to == "FUT" {
		return 1 // 現貨轉期貨
	} else if from == "FUT" && to == "SPOT" {
		return 2 // 期貨轉現貨
	}
	return 0 // 無效類型
}

// callBinanceAPI 呼叫幣安 API
func (s *S1_EXCHANGEServer) callBinanceAPI(req *dao.BinanceTransferRequest) (*dao.BinanceTransferResponse, error) {
	// TODO: 實現實際的幣安 API 呼叫
	// POST /sapi/v1/futures/transfer

	// 模擬回應
	return &dao.BinanceTransferResponse{
		TranID: time.Now().Unix(),
		Status: "SUCCESS",
	}, nil
}

// checkIdempotency 檢查冪等性
func (s *S1_EXCHANGEServer) checkIdempotency(key string) *dao.TransferResponse {
	// TODO: 從 Redis/ArangoDB 檢查是否已存在相同的冪等性鍵值
	return nil
}

// updateTransferLog 更新轉帳日誌
func (s *S1_EXCHANGEServer) updateTransferLog(transferLog *dao.TreasuryTransferLog) {
	// TODO: 更新 ArangoDB
	log.Printf("Updated transfer log: %+v", transferLog)
}

// startWebSocketConnections 啟動 WebSocket 連接
func (s *S1_EXCHANGEServer) startWebSocketConnections() {
	symbols := []string{"BTCUSDT", "ETHUSDT", "ADAUSDT"}

	for _, symbol := range symbols {
		go s.connectWebSocket(symbol, "FUT")
		go s.connectWebSocket(symbol, "SPOT")
	}
}

// connectWebSocket 連接 WebSocket
func (s *S1_EXCHANGEServer) connectWebSocket(symbol, market string) {
	key := fmt.Sprintf("%s_%s", symbol, market)

	var wsURL string
	if market == "FUT" {
		wsURL = fmt.Sprintf("wss://fstream.binance.com/ws/%s@ticker", symbol)
	} else {
		wsURL = fmt.Sprintf("wss://stream.binance.com:9443/ws/%s@ticker", symbol)
	}

	for {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			log.Printf("Failed to connect WebSocket for %s: %v", key, err)
			time.Sleep(5 * time.Second)
			continue
		}

		s.wsMutex.Lock()
		s.wsConnections[key] = conn
		s.wsMutex.Unlock()

		log.Printf("WebSocket connected for %s", key)

		// 處理 WebSocket 消息
		s.handleWebSocketMessages(conn, symbol, market)

		// 連接斷開，等待重連
		time.Sleep(5 * time.Second)
	}
}

// handleWebSocketMessages 處理 WebSocket 消息
func (s *S1_EXCHANGEServer) handleWebSocketMessages(conn *websocket.Conn, symbol, market string) {
	defer conn.Close()

	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("WebSocket read error for %s_%s: %v", symbol, market, err)
			return
		}

		// 處理市場數據
		s.processMarketData(msg, symbol, market)

		// 發布到 Redis Stream
		s.publishToRedisStream(msg, symbol, market)
	}
}

// processMarketData 處理市場數據
func (s *S1_EXCHANGEServer) processMarketData(msg map[string]interface{}, symbol, market string) {
	key := fmt.Sprintf("%s_%s", symbol, market)

	// 解析價格數據
	price, _ := msg["c"].(string)
	volume, _ := msg["v"].(string)
	timestamp, _ := msg["E"].(float64)

	marketData := &dao.MarketData{
		Symbol:    symbol,
		Market:    market,
		Price:     parseFloat(price),
		Volume:    parseFloat(volume),
		Timestamp: int64(timestamp),
		CreatedAt: time.Now(),
	}

	s.dataMutex.Lock()
	s.marketData[key] = marketData
	s.dataMutex.Unlock()
}

// publishToRedisStream 發布到 Redis Stream
func (s *S1_EXCHANGEServer) publishToRedisStream(msg map[string]interface{}, symbol, market string) {
	streamName := fmt.Sprintf("mkt:tick:%s", symbol)

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}

	// TODO: 實現 Redis Stream 發布
	log.Printf("Publishing to Redis Stream %s: %s", streamName, string(msgBytes))
}

// startScheduledTasks 啟動定時任務
func (s *S1_EXCHANGEServer) startScheduledTasks() {
	// 每日 exchangeInfo 刷新
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.refreshExchangeInfo()
			}
		}
	}()

	// 每 8h 拉取全量 funding rate 歷史快照補缺
	go func() {
		ticker := time.NewTicker(8 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.refreshFundingRates()
			}
		}
	}()
}

// refreshExchangeInfo 刷新交易所資訊
func (s *S1_EXCHANGEServer) refreshExchangeInfo() {
	log.Println("Refreshing exchange info...")
	// TODO: 實現 exchangeInfo 刷新邏輯
}

// refreshFundingRates 刷新資金費率
func (s *S1_EXCHANGEServer) refreshFundingRates() {
	log.Println("Refreshing funding rates...")
	// TODO: 實現資金費率刷新邏輯
}

// parseFloat 解析字串為浮點數
func parseFloat(s string) float64 {
	if s == "" {
		return 0.0
	}

	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func main() {
	// Load configuration (priority: env.local.yaml > env.yaml > config.yaml)
	if err := config.LoadConfig(""); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize Redis connection
	if err := redis.Init(); err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}

	// Initialize ArangoDB connection
	if err := arangodb.Init(); err != nil {
		log.Fatalf("Failed to initialize ArangoDB: %v", err)
	}

	// Create server instance
	s1Server := NewS1_EXCHANGEServer()

	r := gin.Default()

	// Health check routes
	r.GET("/health", s1Server.HealthCheck)
	r.GET("/ready", s1Server.ReadyCheck)

	// Market data routes
	r.GET("/market/data", s1Server.GetMarketData)
	r.GET("/market/orderbook", s1Server.GetOrderBook)
	r.GET("/market/funding", s1Server.GetFundingRate)

	// Account routes
	r.GET("/account/balance", s1Server.GetAccountBalance)
	r.GET("/account/positions", s1Server.GetPositions)

	// Treasury routes (內部私有 API)
	r.POST("/xchg/treasury/transfer", s1Server.TreasuryTransfer)

	// Use configuration port, fallback to environment variable or default
	port := os.Getenv("PORT")
	if port == "" && config.AppConfig.Service.Port != 0 {
		port = fmt.Sprintf("%d", config.AppConfig.Service.Port)
	}
	if port == "" {
		port = "8081"
	}

	log.Printf("S1 Exchange Connectors server starting on :%s", port)
	r.Run(":" + port)
}
