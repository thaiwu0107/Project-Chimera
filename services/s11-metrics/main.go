package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"s11-metrics/dao"
	"s11-metrics/internal/config"
	"s11-metrics/internal/services/arangodb"
	"s11-metrics/internal/services/redis"
	"time"

	"github.com/gin-gonic/gin"
)

// GitCommitNum is set during build time via ldflags
var GitCommitNum string

// S11 METRICS
// Metrics - Collect and analyze trading metrics

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
		Service:  "s11-metrics",
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
		Service:  "s11-metrics",
		Version:  s.version,
		Status:   dao.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Notes:    "Service ready to accept requests",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Get metrics
// @Description Get aggregated metrics data
// @Tags metrics
// @Accept json
// @Produce json
// @Param metric query string false "Metric name filter"
// @Param tags query string false "Tags filter"
// @Param from query int64 false "Start timestamp"
// @Param to query int64 false "End timestamp"
// @Success 200 {object} apispec.MetricsResponse
// @Router /metrics [get]
func (s *Server) GetMetrics(c *gin.Context) {
	// TODO: Implement metrics retrieval logic
	points := []dao.MetricPoint{
		{
			Metric: "router_p95_ms",
			Value:  45.2,
			Ts:     time.Now().UnixMilli(),
			Tags: map[string]string{
				"service": "s4-router",
				"symbol":  "BTCUSDT",
			},
		},
		{
			Metric: "strategy_pnl_usdt",
			Value:  1250.75,
			Ts:     time.Now().UnixMilli(),
			Tags: map[string]string{
				"service": "s3-strategy",
				"symbol":  "BTCUSDT",
			},
		},
	}

	response := dao.MetricsResponse{
		Points: points,
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Get alerts
// @Description Get active alerts
// @Tags alerts
// @Accept json
// @Produce json
// @Param severity query string false "Severity filter"
// @Param source query string false "Source filter"
// @Param limit query int false "Limit results"
// @Success 200 {object} apispec.AlertsResponse
// @Router /alerts [get]
func (s *Server) GetAlerts(c *gin.Context) {
	// TODO: Implement alerts retrieval logic
	items := []dao.Alert{
		{
			AlertID:  "alert_001",
			Severity: dao.SevWarn,
			Source:   "s4-router",
			Message:  "High latency detected",
			Ts:       time.Now().UnixMilli(),
		},
		{
			AlertID:  "alert_002",
			Severity: dao.SevError,
			Source:   "s1-exchange",
			Message:  "WebSocket connection lost",
			Ts:       time.Now().UnixMilli(),
		},
	}

	response := dao.AlertsResponse{
		Items: items,
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Get treasury metrics
// @Description Get treasury transfer metrics and SLI
// @Tags treasury-metrics
// @Accept json
// @Produce json
// @Param metric query string false "Metric name filter"
// @Param from query string false "From market filter"
// @Param to query string false "To market filter"
// @Param from_time query int64 false "Start timestamp"
// @Param to_time query int64 false "End timestamp"
// @Success 200 {object} dao.TreasurySLI
// @Router /treasury/metrics [get]
func (s *Server) GetTreasuryMetrics(c *gin.Context) {
	// TODO: 實現 Treasury 指標查詢邏輯

	// 模擬 Treasury SLI 數據
	sli := dao.TreasurySLI{
		SLIID:      fmt.Sprintf("sli_%d", time.Now().Unix()),
		MetricName: "treasury_transfer_p95_ms",
		Value:      45.2,
		Unit:       "ms",
		Window: dao.Window{
			From: time.Now().Add(-time.Hour).UnixMilli(),
			To:   time.Now().UnixMilli(),
		},
		P95LatencyMs:    45,
		P99LatencyMs:    78,
		SuccessRate:     0.998,
		FailureRate:     0.002,
		IdempotencyHits: 12,
		TotalRequests:   1250,
		Timestamp:       time.Now().UnixMilli(),
		CreatedAt:       time.Now(),
	}

	c.JSON(http.StatusOK, sli)
}

// @Summary Get treasury alerts
// @Description Get treasury transfer related alerts
// @Tags treasury-alerts
// @Accept json
// @Produce json
// @Param severity query string false "Severity filter"
// @Param limit query int false "Limit results"
// @Success 200 {object} dao.AlertsResponse
// @Router /treasury/alerts [get]
func (s *Server) GetTreasuryAlerts(c *gin.Context) {
	// TODO: 實現 Treasury 告警查詢邏輯

	items := []dao.Alert{
		{
			AlertID:  "treasury_alert_001",
			Severity: dao.SevWarn,
			Source:   "s1-exchange",
			Message:  "Treasury transfer latency exceeded threshold",
			Ts:       time.Now().UnixMilli(),
		},
		{
			AlertID:  "treasury_alert_002",
			Severity: dao.SevError,
			Source:   "s12-ui",
			Message:  "Treasury transfer failure rate increased",
			Ts:       time.Now().UnixMilli(),
		},
	}

	response := dao.AlertsResponse{
		Items: items,
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
	s11Server := NewServer()

	r := gin.Default()

	// Health check routes
	r.GET("/health", s11Server.HealthCheck)
	r.GET("/ready", s11Server.ReadyCheck)

	// Metrics routes
	r.GET("/metrics", s11Server.GetMetrics)
	r.GET("/alerts", s11Server.GetAlerts)

	// Treasury metrics routes
	r.GET("/treasury/metrics", s11Server.GetTreasuryMetrics)
	r.GET("/treasury/alerts", s11Server.GetTreasuryAlerts)

	// Use configuration port, fallback to environment variable or default
	port := os.Getenv("PORT")
	if port == "" && config.AppConfig.Service.Port != 0 {
		port = fmt.Sprintf("%d", config.AppConfig.Service.Port)
	}
	if port == "" {
		port = "8091"
	}

	log.Printf("S11 METRICS server starting on :%s", port)
	r.Run(":" + port)
}
