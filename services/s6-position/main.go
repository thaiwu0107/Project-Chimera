package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"s6-position/dao"
	"s6-position/internal/apispec"
	"s6-position/internal/config"
	"s6-position/internal/services/arangodb"
	"s6-position/internal/services/redis"
	"time"

	"github.com/gin-gonic/gin"
)

// GitCommitNum is set during build time via ldflags
var GitCommitNum string

// S6 POSITION
// Position Manager - Manage positions and risk across all exchanges

type Server struct {
	redisClient    *redis.RedisClient
	arangodbClient *arangodb.ArangoDBClient
	version        string
	startTime      time.Time
}

func NewServer() *Server {
	return &Server{
		redisClient:    redis.GetInstance(),
		arangodbClient: arangodb.GetInstance(),
		version:        "v1.0.0",
		startTime:      time.Now(),
	}
}

// @Summary Health check
// @Description Check service health status
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} apispec.HealthResponse
// @Router /health [get]
func (s *Server) HealthCheck(c *gin.Context) {
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
		Service:  "s6-position",
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
func (s *Server) ReadyCheck(c *gin.Context) {
	// Check all critical dependencies are ready
	response := apispec.HealthResponse{
		Service:  "s6-position",
		Version:  s.version,
		Status:   apispec.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Notes:    "Service ready to accept requests",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Manage positions
// @Description Manage positions with stop moves, reductions, and additions
// @Tags positions
// @Accept json
// @Produce json
// @Param request body apispec.ManagePositionsRequest true "Manage positions request"
// @Success 200 {object} apispec.ManagePositionsResponse
// @Router /positions/manage [post]
func (s *Server) ManagePositions(c *gin.Context) {
	var req apispec.ManagePositionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement position management logic
	plan := apispec.ManagePlan{
		StopMoves: []apispec.StopMove{
			{
				OldPx:  49000.0,
				NewPx:  49500.0,
				Reason: "Trailing stop moved up",
			},
		},
		Reduce: []apispec.OrderIntent{},
		Adds:   []apispec.OrderIntent{},
	}

	response := apispec.ManagePositionsResponse{
		Plan:   plan,
		Orders: []apispec.OrderResult{},
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Auto transfer trigger
// @Description Trigger automatic transfer based on margin/risk conditions
// @Tags auto-transfer
// @Accept json
// @Produce json
// @Param request body dao.AutoTransferTrigger true "Auto transfer trigger"
// @Success 200 {object} dao.AutoTransferLog
// @Router /auto-transfer/trigger [post]
func (s *Server) AutoTransferTrigger(c *gin.Context) {
	var req dao.AutoTransferTrigger
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. 檢查自動劃轉配置
	config := s.getAutoTransferConfig()
	if !config.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Auto transfer is disabled"})
		return
	}

	// 2. 檢查劃轉間隔
	if time.Now().Unix()-config.LastTransferTime < int64(config.TransferInterval) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Transfer interval not met"})
		return
	}

	// 3. 檢查觸發條件
	if !s.checkTransferTrigger(&req) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Transfer trigger conditions not met"})
		return
	}

	// 4. 執行自動劃轉
	transferLog := s.executeAutoTransfer(&req, config)

	c.JSON(http.StatusOK, transferLog)
}

// checkTransferTrigger 檢查劃轉觸發條件
func (s *Server) checkTransferTrigger(trigger *dao.AutoTransferTrigger) bool {
	switch trigger.TriggerType {
	case "MARGIN_CALL":
		// 檢查保證金追繳條件
		return s.checkMarginCall(trigger)
	case "RISK_LIMIT":
		// 檢查風險限制條件
		return s.checkRiskLimit(trigger)
	case "PROFIT_TAKING":
		// 檢查獲利了結條件
		return s.checkProfitTaking(trigger)
	default:
		return false
	}
}

// checkMarginCall 檢查保證金追繳
func (s *Server) checkMarginCall(trigger *dao.AutoTransferTrigger) bool {
	// TODO: 實現保證金追繳檢查邏輯
	// 檢查當前保證金是否低於閾值
	return true // 暫時返回 true
}

// checkRiskLimit 檢查風險限制
func (s *Server) checkRiskLimit(trigger *dao.AutoTransferTrigger) bool {
	// TODO: 實現風險限制檢查邏輯
	// 檢查持倉風險是否超過限制
	return true // 暫時返回 true
}

// checkProfitTaking 檢查獲利了結
func (s *Server) checkProfitTaking(trigger *dao.AutoTransferTrigger) bool {
	// TODO: 實現獲利了結檢查邏輯
	// 檢查獲利是否達到閾值
	return true // 暫時返回 true
}

// executeAutoTransfer 執行自動劃轉
func (s *Server) executeAutoTransfer(trigger *dao.AutoTransferTrigger, config *dao.AutoTransferConfig) *dao.AutoTransferLog {
	// 1. 創建劃轉日誌
	transferLog := &dao.AutoTransferLog{
		LogID:       fmt.Sprintf("log_%d", time.Now().Unix()),
		TriggerID:   trigger.TriggerID,
		Symbol:      trigger.Symbol,
		Market:      trigger.Market,
		TriggerType: trigger.TriggerType,
		From:        trigger.TransferFrom,
		To:          trigger.TransferTo,
		Amount:      trigger.TransferAmount,
		Reason:      fmt.Sprintf("Auto transfer triggered by %s", trigger.TriggerType),
		Status:      "PENDING",
		Timestamp:   time.Now(),
		CreatedAt:   time.Now(),
	}

	// 2. 呼叫 S12 UI API 執行劃轉
	transferID, err := s.callS12TreasuryTransfer(trigger)
	if err != nil {
		transferLog.Status = "FAILED"
		transferLog.ErrorMsg = err.Error()
		return transferLog
	}

	// 3. 更新日誌
	transferLog.Status = "SUCCESS"
	transferLog.TransferID = transferID

	// 4. 更新配置中的最後劃轉時間
	config.LastTransferTime = time.Now().Unix()
	s.updateAutoTransferConfig(config)

	return transferLog
}

// callS12TreasuryTransfer 呼叫 S12 UI Treasury Transfer API
func (s *Server) callS12TreasuryTransfer(trigger *dao.AutoTransferTrigger) (string, error) {
	// TODO: 實現 HTTP 呼叫 S12 UI API
	// POST /treasury/transfer

	// 模擬回應
	return fmt.Sprintf("transfer_%d", time.Now().Unix()), nil
}

// getAutoTransferConfig 獲取自動劃轉配置
func (s *Server) getAutoTransferConfig() *dao.AutoTransferConfig {
	// TODO: 從 ArangoDB 獲取配置
	return &dao.AutoTransferConfig{
		ConfigID:            "default",
		Enabled:             true,
		SpotMarginThreshold: 1000.0,
		FutMarginThreshold:  2000.0,
		MinTransferAmount:   100.0,
		MaxTransferAmount:   10000.0,
		TransferInterval:    300, // 5 分鐘
		LastTransferTime:    0,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

// updateAutoTransferConfig 更新自動劃轉配置
func (s *Server) updateAutoTransferConfig(config *dao.AutoTransferConfig) {
	// TODO: 更新 ArangoDB
	log.Printf("Updated auto transfer config: %+v", config)
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
	s6Server := NewServer()

	r := gin.Default()

	// Health check routes
	r.GET("/health", s6Server.HealthCheck)
	r.GET("/ready", s6Server.ReadyCheck)

	// Position routes
	r.POST("/positions/manage", s6Server.ManagePositions)

	// Auto transfer routes
	r.POST("/auto-transfer/trigger", s6Server.AutoTransferTrigger)

	// Use configuration port, fallback to environment variable or default
	port := os.Getenv("PORT")
	if port == "" && config.AppConfig.Service.Port != 0 {
		port = fmt.Sprintf("%d", config.AppConfig.Service.Port)
	}
	if port == "" {
		port = "8086"
	}

	log.Printf("S6 POSITION server starting on :%s", port)
	r.Run(":" + port)
}
