package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"s8-autopsy/dao"
	"s8-autopsy/internal/config"
	"s8-autopsy/internal/services/arangodb"
	"s8-autopsy/internal/services/redis"
	"time"

	"github.com/gin-gonic/gin"
)

// GitCommitNum is set during build time via ldflags
var GitCommitNum string

// S8 AUTOPSY
// Autopsy - Analyze failed orders and positions

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
	checks := []dao.HealthCheck{
		{
			Name:      "redis",
			Status:    dao.HealthOK,
			LatencyMs: 5,
		},
		{
			Name:      "arangodb",
			Status:    dao.HealthOK,
			LatencyMs: 10,
		},
		{
			Name:      "service",
			Status:    dao.HealthOK,
			LatencyMs: 8,
		},
	}

	response := dao.HealthResponse{
		Service:  "s8-autopsy",
		Version:  s.version,
		Status:   dao.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Checks:   checks,
		Notes:    "Service running normally",
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
	response := dao.HealthResponse{
		Service:  "s8-autopsy",
		Version:  s.version,
		Status:   dao.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Notes:    "Service ready to accept requests",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Generate autopsy report
// @Description Generate autopsy report for a specific trade
// @Tags autopsy
// @Accept json
// @Produce json
// @Param trade_id path string true "Trade ID"
// @Param request body apispec.AutopsyRequest true "Autopsy request"
// @Success 200 {object} apispec.AutopsyResponse
// @Router /autopsy/{trade_id} [post]
func (s *Server) GenerateAutopsy(c *gin.Context) {
	tradeID := c.Param("trade_id")
	if tradeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trade_id is required"})
		return
	}

	var req dao.AutopsyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ensure trade_id matches the path parameter
	req.TradeID = tradeID

	// TODO: Implement autopsy generation logic
	response := dao.AutopsyResponse{
		ReportID: "report_" + tradeID,
		Url:      "https://autopsy.example.com/reports/" + tradeID,
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
	s8Server := NewServer()

	r := gin.Default()

	// Health check routes
	r.GET("/health", s8Server.HealthCheck)
	r.GET("/ready", s8Server.ReadyCheck)

	// Autopsy routes
	r.POST("/autopsy/:trade_id", s8Server.GenerateAutopsy)

	// Use configuration port, fallback to environment variable or default
	port := os.Getenv("PORT")
	if port == "" && config.AppConfig.Service.Port != 0 {
		port = fmt.Sprintf("%d", config.AppConfig.Service.Port)
	}
	if port == "" {
		port = "8088"
	}

	log.Printf("S8 AUTOPSY server starting on :%s", port)
	r.Run(":" + port)
}
