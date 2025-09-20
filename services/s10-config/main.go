package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"s10-config/dao"
	"s10-config/internal/config"
	"s10-config/internal/services/arangodb"
	"s10-config/internal/services/redis"
	"time"

	"github.com/gin-gonic/gin"
)

// GitCommitNum is set during build time via ldflags
var GitCommitNum string

// S10 CONFIG
// Config Manager - Manage configuration and settings

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
		Service:  "s10-config",
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
		Service:  "s10-config",
		Version:  s.version,
		Status:   dao.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Notes:    "Service ready to accept requests",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Upsert bundle
// @Description Create or update configuration bundle
// @Tags bundles
// @Accept json
// @Produce json
// @Param request body apispec.BundleUpsertRequest true "Bundle upsert request"
// @Success 200 {object} apispec.BundleUpsertResponse
// @Router /bundles [post]
func (s *Server) UpsertBundle(c *gin.Context) {
	var req dao.BundleUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement bundle upsert logic
	response := dao.BundleUpsertResponse{
		Ok:      true,
		Message: "Bundle upserted successfully",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Stage bundle
// @Description Stage configuration bundle for promotion
// @Tags bundles
// @Accept json
// @Produce json
// @Param id path string true "Bundle ID"
// @Success 200 {object} apispec.BundleStageResponse
// @Router /bundles/{id}/stage [post]
func (s *Server) StageBundle(c *gin.Context) {
	bundleID := c.Param("id")
	if bundleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bundle_id is required"})
		return
	}

	// TODO: Implement bundle staging logic
	response := dao.BundleStageResponse{
		Ok: true,
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Promote bundle
// @Description Promote configuration bundle to active
// @Tags bundles
// @Accept json
// @Produce json
// @Param request body apispec.PromoteRequest true "Promote request"
// @Success 200 {object} apispec.PromoteResponse
// @Router /promote [post]
func (s *Server) PromoteBundle(c *gin.Context) {
	var req dao.PromoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement bundle promotion logic
	response := dao.PromoteResponse{
		PromotionID: "promo_" + req.BundleID,
		Status:      "PENDING",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Simulate bundle
// @Description Run simulation for configuration bundle
// @Tags bundles
// @Accept json
// @Produce json
// @Param request body apispec.SimulateRequest true "Simulate request"
// @Success 200 {object} apispec.SimulateResponse
// @Router /simulate [post]
func (s *Server) SimulateBundle(c *gin.Context) {
	var req dao.SimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement bundle simulation logic
	response := dao.SimulateResponse{
		SimID:  "sim_" + req.BundleID,
		Status: "QUEUED",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Get active config
// @Description Get currently active configuration
// @Tags config
// @Accept json
// @Produce json
// @Success 200 {object} apispec.ActiveConfigResponse
// @Router /active [get]
func (s *Server) GetActiveConfig(c *gin.Context) {
	// TODO: Implement get active config logic
	response := dao.ActiveConfigResponse{
		Rev:         1,
		BundleID:    "bundle_active",
		ActivatedAt: time.Now().UnixMilli(),
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
	s10Server := NewServer()

	r := gin.Default()

	// Health check routes
	r.GET("/health", s10Server.HealthCheck)
	r.GET("/ready", s10Server.ReadyCheck)

	// Bundle routes
	r.POST("/bundles", s10Server.UpsertBundle)
	r.POST("/bundles/:id/stage", s10Server.StageBundle)
	r.POST("/promote", s10Server.PromoteBundle)
	r.POST("/simulate", s10Server.SimulateBundle)
	r.GET("/active", s10Server.GetActiveConfig)

	// Use configuration port, fallback to environment variable or default
	port := os.Getenv("PORT")
	if port == "" && config.AppConfig.Service.Port != 0 {
		port = fmt.Sprintf("%d", config.AppConfig.Service.Port)
	}
	if port == "" {
		port = "8090"
	}

	log.Printf("S10 CONFIG server starting on :%s", port)
	r.Run(":" + port)
}
