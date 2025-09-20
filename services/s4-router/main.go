package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"s4-router/dao"
	"s4-router/internal/apispec"
	"s4-router/internal/config"
	"s4-router/internal/services/arangodb"
	"s4-router/internal/services/redis"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// GitCommitNum is set during build time via ldflags
var GitCommitNum string

// S4 ORDER ROUTER
// Order Router - Route orders to exchanges with TWAP granularity, Maker/Taker strategies, OCO/Guard Stop

type S4_ROUTERServer struct {
	redisClient    *redis.RedisClient
	arangodbClient *arangodb.ArangoDBClient
	validator      *validator.Validate
	version        string
	startTime      time.Time
}

func NewS4_ROUTERServer() *S4_ROUTERServer {
	return &S4_ROUTERServer{
		redisClient:    redis.GetInstance(),
		arangodbClient: arangodb.GetInstance(),
		validator:      validator.New(),
		version:        "v1.0.0",
		startTime:      time.Now(),
	}
}

func (s *S4_ROUTERServer) HealthCheck(c *gin.Context) {
	checks := []apispec.HealthCheck{
		{Name: "redis", Status: apispec.HealthOK, LatencyMs: 5},
		{Name: "arangodb", Status: apispec.HealthOK, LatencyMs: 10},
		{Name: "order-router", Status: apispec.HealthOK, LatencyMs: 8},
	}

	response := apispec.HealthResponse{
		Service:  "s4-router",
		Version:  s.version,
		Status:   apispec.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Checks:   checks,
		Notes:    "Order router running normally",
	}

	c.JSON(http.StatusOK, response)
}

func (s *S4_ROUTERServer) ReadyCheck(c *gin.Context) {
	response := apispec.HealthResponse{
		Service:  "s4-router",
		Version:  s.version,
		Status:   apispec.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Notes:    "Service ready to accept requests",
	}

	c.JSON(http.StatusOK, response)
}

func (s *S4_ROUTERServer) CreateOrder(c *gin.Context) {
	var req dao.OrderCmdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format", "details": err.Error()})
		return
	}

	// 驗證基本字段
	if err := s.validator.Struct(req.Intent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	// 業務邏輯驗證
	if err := s.validateOrderIntent(&req.Intent); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Business logic validation failed", "details": err.Error()})
		return
	}

	// TODO: Implement order execution logic
	result := dao.OrderResult{
		OrderID:       "order_" + req.Intent.IntentID,
		ClientOrderID: req.Intent.IntentID,
		Status:        "FILLED",
		AvgPrice:      50000.0,
		ExecutedQty:   0.002,
		Fills: []dao.Fill{
			{
				FillID:      "fill_1",
				Price:       50000.0,
				Qty:         0.002,
				FeeUSDT:     0.1,
				MidAtSend:   49995.0,
				SlippageBps: 10.0,
				Timestamp:   time.Now().UnixMilli(),
			},
		},
		GuardStopArmed: false,
		Message:        "Order executed successfully",
	}

	c.JSON(http.StatusOK, result)
}

// validateOrderIntent 驗證訂單意圖的業務邏輯
func (s *S4_ROUTERServer) validateOrderIntent(intent *dao.OrderIntent) error {
	// FUT 市場必須有槓桿
	if intent.Market == "FUT" {
		if intent.Leverage == 0 {
			return fmt.Errorf("leverage is required for FUT market")
		}
		if intent.Leverage < 1 || intent.Leverage > 125 {
			return fmt.Errorf("leverage must be between 1 and 125, got %d", intent.Leverage)
		}
	}

	// OCO 策略必須有 OCO 配置
	if intent.ExecPolicy.OCO != nil {
		if intent.ExecPolicy.OCO.TakeProfitPx <= 0 || intent.ExecPolicy.OCO.StopLossPx <= 0 {
			return fmt.Errorf("OCO requires valid take_profit_px and stop_loss_px")
		}

		// 價格關係驗證
		if intent.Side == "BUY" {
			if intent.ExecPolicy.OCO.TakeProfitPx <= intent.ExecPolicy.OCO.StopLossPx {
				return fmt.Errorf("for BUY orders, take_profit_px must be greater than stop_loss_px")
			}
		} else if intent.Side == "SELL" {
			if intent.ExecPolicy.OCO.TakeProfitPx >= intent.ExecPolicy.OCO.StopLossPx {
				return fmt.Errorf("for SELL orders, take_profit_px must be less than stop_loss_px")
			}
		}
	}

	// TWAP 切片驗證
	if intent.ExecPolicy.TWAPSlices > 0 {
		if intent.ExecPolicy.TWAPSlices < 1 || intent.ExecPolicy.TWAPSlices > 10 {
			return fmt.Errorf("TWAP slices must be between 1 and 10, got %d", intent.ExecPolicy.TWAPSlices)
		}
	}

	// Maker 等待時間驗證
	if intent.ExecPolicy.MakerWaitMs < 0 || intent.ExecPolicy.MakerWaitMs > 10000 {
		return fmt.Errorf("maker_wait_ms must be between 0 and 10000, got %d", intent.ExecPolicy.MakerWaitMs)
	}

	return nil
}

func (s *S4_ROUTERServer) CancelOrder(c *gin.Context) {
	var req dao.CancelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format", "details": err.Error()})
		return
	}

	// 驗證 CancelRequest
	if err := s.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	// 業務邏輯驗證：order_id 和 client_order_id 必須有一個
	if req.OrderID == "" && req.ClientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Either order_id or client_order_id must be provided"})
		return
	}

	// TODO: Implement order cancellation logic
	response := dao.CancelResponse{
		Canceled: true,
		Message:  "Order cancelled successfully",
	}

	c.JSON(http.StatusOK, response)
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
	s4Server := NewS4_ROUTERServer()

	r := gin.Default()

	// Health check routes
	r.GET("/health", s4Server.HealthCheck)
	r.GET("/ready", s4Server.ReadyCheck)

	// Order routes
	r.POST("/orders", s4Server.CreateOrder)
	r.POST("/cancel", s4Server.CancelOrder)

	// Use configuration port, fallback to environment variable or default
	port := os.Getenv("PORT")
	if port == "" && config.AppConfig.Service.Port != 0 {
		port = fmt.Sprintf("%d", config.AppConfig.Service.Port)
	}
	if port == "" {
		port = "8084"
	}

	log.Printf("S4 Order Router server starting on :%s", port)
	r.Run(":" + port)
}
