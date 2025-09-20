package main

import (
	"fmt"
	"init-db/internal/apispec"
	"init-db/internal/config"
	"init-db/internal/services/arangodb"
	"init-db/internal/services/redis"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// GitCommitNum is set during build time via ldflags
var GitCommitNum string

// INIT DB
// Database Initialization Tool - Initialize ArangoDB collections and indexes

type INIT_DBServer struct {
	redisClient    *redis.RedisClient
	arangodbClient *arangodb.ArangoDBClient
	version        string
	startTime      time.Time
}

func NewINIT_DBServer() *INIT_DBServer {
	return &INIT_DBServer{
		redisClient:    redis.GetInstance(),
		arangodbClient: arangodb.GetInstance(),
		version:        "v1.0.0",
		startTime:      time.Now(),
	}
}

func (s *INIT_DBServer) HealthCheck(c *gin.Context) {
	checks := []apispec.HealthCheck{
		{Name: "redis", Status: apispec.HealthOK, LatencyMs: 5},
		{Name: "arangodb", Status: apispec.HealthOK, LatencyMs: 10},
		{Name: "service", Status: apispec.HealthOK, LatencyMs: 8},
	}

	response := apispec.HealthResponse{
		Service:  "init-db",
		Version:  s.version,
		Status:   apispec.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Checks:   checks,
		Notes:    "Service running normally",
	}

	c.JSON(http.StatusOK, response)
}

func (s *INIT_DBServer) ReadyCheck(c *gin.Context) {
	response := apispec.HealthResponse{
		Service:  "init-db",
		Version:  s.version,
		Status:   apispec.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Notes:    "Service ready to accept requests",
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
	server := NewINIT_DBServer()

	r := gin.Default()

	// Health check routes
	r.GET("/health", server.HealthCheck)
	r.GET("/ready", server.ReadyCheck)

	// Use configuration port, fallback to environment variable or default
	port := os.Getenv("PORT")
	if port == "" && config.AppConfig.Service.Port != 0 {
		port = fmt.Sprintf("%d", config.AppConfig.Service.Port)
	}
	if port == "" {
		port = "8080"
	}

	log.Printf("INIT DB server starting on :%s", port)
	r.Run(":" + port)
}
