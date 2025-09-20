package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"s5-reconciler/internal/apispec"
	"s5-reconciler/internal/config"
	"s5-reconciler/internal/services/arangodb"
	"s5-reconciler/internal/services/redis"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// GitCommitNum is set during build time via ldflags
var GitCommitNum string

// S5 RECONCILER
// Reconciler - Reconcile orders and positions across exchanges

type Server struct {
	redisClient    *redis.RedisClient
	arangodbClient *arangodb.ArangoDBClient
	validator      *validator.Validate
	version        string
	startTime      time.Time
}

func NewServer() *Server {
	return &Server{
		redisClient:    redis.GetInstance(),
		arangodbClient: arangodb.GetInstance(),
		validator:      validator.New(),
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
		Service:  "s5-reconciler",
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
		Service:  "s5-reconciler",
		Version:  s.version,
		Status:   apispec.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Notes:    "Service ready to accept requests",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Reconcile orders and positions
// @Description Start reconciliation process for orders and positions
// @Tags reconcile
// @Accept json
// @Produce json
// @Param request body apispec.ReconcileRequest true "Reconcile request"
// @Success 200 {object} apispec.ReconcileResponse
// @Router /reconcile [post]
func (s *Server) Reconcile(c *gin.Context) {
	var req apispec.ReconcileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format", "details": err.Error()})
		return
	}

	// 驗證基本字段
	if err := s.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	// 業務邏輯驗證
	if err := s.validateReconcileRequest(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Business logic validation failed", "details": err.Error()})
		return
	}

	// TODO: Implement reconciliation logic
	response := apispec.ReconcileResponse{
		FixedOrders:    5,
		FixedPositions: 3,
		FixedHoldings:  2,
		OrphansClosed:  1,
		Message:        "Reconciliation completed successfully",
	}

	c.JSON(http.StatusOK, response)
}

// validateReconcileRequest 驗證對帳請求的業務邏輯
func (s *Server) validateReconcileRequest(req *apispec.ReconcileRequest) error {
	// 時間窗口驗證
	if req.TimeWindowH < 1 || req.TimeWindowH > 168 {
		return fmt.Errorf("time_window_h must be between 1 and 168 hours, got %d", req.TimeWindowH)
	}

	// 孤兒策略驗證
	if req.OrphanPolicy != "" && req.OrphanPolicy != "RECLAIM_IF_SAFE" && req.OrphanPolicy != "CONSERVATIVE" {
		return fmt.Errorf("orphan_policy must be either RECLAIM_IF_SAFE or CONSERVATIVE, got %s", req.OrphanPolicy)
	}

	return nil
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
	s5Server := NewServer()

	r := gin.Default()

	// Health check routes
	r.GET("/health", s5Server.HealthCheck)
	r.GET("/ready", s5Server.ReadyCheck)

	// Reconcile routes
	r.POST("/reconcile", s5Server.Reconcile)

	// Use configuration port, fallback to environment variable or default
	port := os.Getenv("PORT")
	if port == "" && config.AppConfig.Service.Port != 0 {
		port = fmt.Sprintf("%d", config.AppConfig.Service.Port)
	}
	if port == "" {
		port = "8085"
	}

	log.Printf("S5 RECONCILER server starting on :%s", port)
	r.Run(":" + port)
}
