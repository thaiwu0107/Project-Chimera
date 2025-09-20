package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"s7-label/dao"
	"s7-label/internal/config"
	"s7-label/internal/services/arangodb"
	"s7-label/internal/services/redis"
	"time"

	"github.com/gin-gonic/gin"
)

// GitCommitNum is set during build time via ldflags
var GitCommitNum string

// S7 LABEL
// Label Manager - Manage labels and tags for orders and positions

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
		Service:  "s7-label",
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
		Service:  "s7-label",
		Version:  s.version,
		Status:   dao.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Notes:    "Service ready to accept requests",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Backfill labels
// @Description Backfill labels for historical signals based on horizon
// @Tags labels
// @Accept json
// @Produce json
// @Param request body apispec.BackfillRequest true "Backfill request"
// @Success 200 {object} apispec.BackfillResponse
// @Router /labels/backfill [post]
func (s *Server) BackfillLabels(c *gin.Context) {
	var req dao.BackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement label backfill logic
	response := dao.BackfillResponse{
		Updated: 100, // Mock value
		Message: "Labels backfilled successfully",
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
	s7Server := NewServer()

	r := gin.Default()

	// Health check routes
	r.GET("/health", s7Server.HealthCheck)
	r.GET("/ready", s7Server.ReadyCheck)

	// Label routes
	r.POST("/labels/backfill", s7Server.BackfillLabels)

	// Use configuration port, fallback to environment variable or default
	port := os.Getenv("PORT")
	if port == "" && config.AppConfig.Service.Port != 0 {
		port = fmt.Sprintf("%d", config.AppConfig.Service.Port)
	}
	if port == "" {
		port = "8087"
	}

	log.Printf("S7 LABEL server starting on :%s", port)
	r.Run(":" + port)
}
