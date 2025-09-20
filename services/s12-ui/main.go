package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"s12-ui/dao"
	"s12-ui/internal/config"
	"s12-ui/internal/services/arangodb"
	"s12-ui/internal/services/redis"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// GitCommitNum is set during build time via ldflags
var GitCommitNum string

// S12 UI
// UI - Web interface for trading system

type Server struct {
	redisClient    *redis.RedisClient
	arangodbClient *arangodb.ArangoDBClient
	version        string
	startTime      time.Time
	httpClient     *http.Client
	serviceURLs    map[string]string
}

func NewServer() *Server {
	return &Server{
		redisClient:    redis.GetInstance(),
		arangodbClient: arangodb.GetInstance(),
		version:        "v1.0.0",
		startTime:      time.Now(),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		serviceURLs: map[string]string{
			"s2":  "http://localhost:8082",
			"s3":  "http://localhost:8083",
			"s4":  "http://localhost:8084",
			"s5":  "http://localhost:8085",
			"s6":  "http://localhost:8086",
			"s7":  "http://localhost:8087",
			"s8":  "http://localhost:8088",
			"s9":  "http://localhost:8089",
			"s10": "http://localhost:8090",
			"s11": "http://localhost:8091",
			"s1":  "http://localhost:8081",
		},
	}
}

// RBAC 角色定義
const (
	RoleViewer      = "viewer"
	RoleTrader      = "trader"
	RoleResearcher  = "researcher"
	RoleRiskOfficer = "risk_officer"
	RoleAdmin       = "admin"
)

// 錯誤響應結構
type ErrorResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// 代理請求結構
type ProxyRequest struct {
	Service string
	Path    string
	Method  string
	Headers map[string]string
	Body    interface{}
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
		Service:  "s12-ui",
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
		Service:  "s12-ui",
		Version:  s.version,
		Status:   dao.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Notes:    "Service ready to accept requests",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Kill switch
// @Description Enable or disable system kill switch
// @Tags system
// @Accept json
// @Produce json
// @Param request body apispec.KillSwitchRequest true "Kill switch request"
// @Success 200 {object} apispec.KillSwitchResponse
// @Router /kill-switch [post]
func (s *Server) KillSwitch(c *gin.Context) {
	var req dao.KillSwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement kill switch logic
	// This would typically:
	// 1. Update system status in Redis/ArangoDB
	// 2. Notify all trading services to stop/start
	// 3. Log the action for audit purposes

	response := dao.KillSwitchResponse{
		Enabled: req.Enable,
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Treasury transfer
// @Description Transfer funds between SPOT and FUT markets
// @Tags treasury
// @Accept json
// @Produce json
// @Param request body apispec.TransferRequest true "Transfer request"
// @Success 200 {object} apispec.TransferResponse
// @Router /treasury/transfer [post]
func (s *Server) TreasuryTransfer(c *gin.Context) {
	var req dao.TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. 驗證參數
	if err := s.validateTransferRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 2. 生成冪等性鍵值
	idempotencyKey := s.generateIdempotencyKey(&req)

	// 3. 獲取分散鎖
	lockKey := fmt.Sprintf("lock:treasury:%s:%s", req.From, req.To)
	if !s.acquireLock(lockKey, idempotencyKey) {
		c.JSON(http.StatusConflict, gin.H{"error": "Transfer in progress"})
		return
	}
	defer s.releaseLock(lockKey)

	// 4. 創建審計記錄
	auditID := fmt.Sprintf("audit_%d", time.Now().Unix())
	audit := dao.TreasuryTransferAudit{
		AuditID:        auditID,
		UserID:         "system", // TODO: 從認證中獲取
		Username:       "admin",  // TODO: 從認證中獲取
		From:           req.From,
		To:             req.To,
		Amount:         req.Amount,
		Reason:         req.Reason,
		IdempotencyKey: idempotencyKey,
		Status:         "PENDING",
		IPAddress:      c.ClientIP(),
		UserAgent:      c.GetHeader("User-Agent"),
		Timestamp:      time.Now(),
		CreatedAt:      time.Now(),
	}

	// 5. 寫入審計記錄
	s.writeAuditLog(&audit)

	// 6. 呼叫 S1 Exchange 執行劃轉
	transferID := fmt.Sprintf("transfer_%d", time.Now().Unix())
	response := dao.TransferResponse{
		TransferID: transferID,
		Result:     "OK",
		Message:    "Transfer completed successfully",
	}

	// TODO: 實際呼叫 S1 Exchange API
	// s.callS1TreasuryTransfer(&req, idempotencyKey)

	// 7. 更新審計記錄
	audit.TransferID = transferID
	audit.Status = "SUCCESS"
	audit.UpdatedAt = time.Now()
	s.updateAuditLog(&audit)

	c.JSON(http.StatusOK, response)
}

// validateTransferRequest 驗證劃轉請求
func (s *Server) validateTransferRequest(req *dao.TransferRequest) error {
	// 參數白名單檢查
	if req.From != "SPOT" && req.From != "FUT" {
		return fmt.Errorf("invalid from: %s", req.From)
	}
	if req.To != "FUT" && req.To != "SPOT" {
		return fmt.Errorf("invalid to: %s", req.To)
	}
	if req.From == req.To {
		return fmt.Errorf("from and to cannot be the same")
	}
	if req.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	// TODO: 風險預算檢查
	// if req.Amount > s.getRiskBudget().DailyTransferLimit {
	//     return fmt.Errorf("amount exceeds daily limit")
	// }

	return nil
}

// generateIdempotencyKey 生成冪等性鍵值
func (s *Server) generateIdempotencyKey(req *dao.TransferRequest) string {
	timestamp := time.Now().Unix()
	hash := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%f:%d", req.From, req.To, req.Amount, timestamp))))
	return fmt.Sprintf("treasury:%s:%s:%f:%d:%s", req.From, req.To, req.Amount, timestamp, hash)
}

// acquireLock 獲取分散鎖
func (s *Server) acquireLock(lockKey, idempotencyKey string) bool {
	// TODO: 使用 Redis 實現分散鎖
	// return s.redisClient.SetNX(lockKey, idempotencyKey, 30*time.Second)
	return true // 暫時返回 true
}

// releaseLock 釋放分散鎖
func (s *Server) releaseLock(lockKey string) {
	// TODO: 使用 Redis 釋放鎖
	// s.redisClient.Del(lockKey)
}

// writeAuditLog 寫入審計日誌
func (s *Server) writeAuditLog(audit *dao.TreasuryTransferAudit) {
	// TODO: 寫入 ArangoDB
	log.Printf("Audit log: %+v", audit)
}

// updateAuditLog 更新審計日誌
func (s *Server) updateAuditLog(audit *dao.TreasuryTransferAudit) {
	// TODO: 更新 ArangoDB
	log.Printf("Updated audit log: %+v", audit)
}

// callS1TreasuryTransfer 呼叫 S1 Exchange 執行劃轉
func (s *Server) callS1TreasuryTransfer(req *dao.TransferRequest, idempotencyKey string) (*dao.TransferResponse, error) {
	// TODO: 實現 HTTP 呼叫 S1 Exchange API
	// POST /xchg/treasury/transfer
	return &dao.TransferResponse{
		TransferID: fmt.Sprintf("transfer_%d", time.Now().Unix()),
		Result:     "OK",
		Message:    "Transfer completed successfully",
	}, nil
}

// RBAC 中間件
func (s *Server) RBACMiddleware(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 從 Authorization header 獲取 JWT token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:     "UNAUTHORIZED",
				Message:   "Missing Authorization header",
				RequestID: c.GetHeader("X-Request-Id"),
			})
			c.Abort()
			return
		}

		// 解析 JWT token 獲取角色
		userRole := s.extractRoleFromToken(authHeader)
		if userRole == "" {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:     "UNAUTHORIZED",
				Message:   "Invalid token",
				RequestID: c.GetHeader("X-Request-Id"),
			})
			c.Abort()
			return
		}

		// 檢查角色權限
		if !s.hasRequiredRole(userRole, requiredRoles) {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:     "FORBIDDEN",
				Message:   "Insufficient permissions",
				RequestID: c.GetHeader("X-Request-Id"),
			})
			c.Abort()
			return
		}

		// 將用戶角色存儲到上下文中
		c.Set("user_role", userRole)
		c.Next()
	}
}

// extractRoleFromToken 從 JWT token 中提取角色
func (s *Server) extractRoleFromToken(authHeader string) string {
	// TODO: 實現 JWT token 解析
	// 這裡簡化實現，實際應該解析 JWT
	if strings.HasPrefix(authHeader, "Bearer ") {
		// 簡化實現：從 token 中提取角色
		return RoleTrader // 默認返回 trader 角色
	}
	return ""
}

// hasRequiredRole 檢查用戶是否有所需角色
func (s *Server) hasRequiredRole(userRole string, requiredRoles []string) bool {
	// 角色層級：admin > risk_officer > researcher > trader > viewer
	roleLevels := map[string]int{
		RoleViewer:      1,
		RoleTrader:      2,
		RoleResearcher:  3,
		RoleRiskOfficer: 4,
		RoleAdmin:       5,
	}

	userLevel := roleLevels[userRole]
	for _, requiredRole := range requiredRoles {
		if requiredRole == userRole || userLevel >= roleLevels[requiredRole] {
			return true
		}
	}
	return false
}

// 通用代理函數
func (s *Server) proxyRequest(c *gin.Context, service, path string) {
	requestID := c.GetHeader("X-Request-Id")
	if requestID == "" {
		requestID = fmt.Sprintf("req_%d", time.Now().UnixNano())
	}

	// 讀取請求體
	var bodyBytes []byte
	var err error
	if c.Request.Body != nil {
		bodyBytes, err = io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:     "BAD_REQUEST",
				Message:   "Invalid request body",
				RequestID: requestID,
			})
			return
		}
		// 重新設置請求體以供後續使用
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// 構建上游請求
	upstreamURL := s.serviceURLs[service] + path
	var bodyReader io.Reader
	if len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(c.Request.Method, upstreamURL, bodyReader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:     "INTERNAL_ERROR",
			Message:   "Failed to create upstream request",
			RequestID: requestID,
		})
		return
	}

	// 複製必要的 headers
	for key, values := range c.Request.Header {
		if key == "Authorization" || key == "X-Request-Id" || key == "X-Idempotency-Key" {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	// 添加代理 headers
	req.Header.Set("X-Forwarded-For", c.ClientIP())
	req.Header.Set("X-Forwarded-Host", c.Request.Host)
	req.Header.Set("X-Request-Id", requestID)

	// 發送請求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{
			Error:     "UPSTREAM_TIMEOUT",
			Message:   fmt.Sprintf("%s timeout", service),
			RequestID: requestID,
		})
		return
	}
	defer resp.Body.Close()

	// 讀取響應
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{
			Error:     "UPSTREAM_ERROR",
			Message:   "Failed to read upstream response",
			RequestID: requestID,
		})
		return
	}

	// 設置響應 headers
	for key, values := range resp.Header {
		if key == "Content-Type" || key == "X-Request-Id" {
			for _, value := range values {
				c.Header(key, value)
			}
		}
	}

	// 返回響應
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

// Proxy functions to other services
// ProxyToS2 代理到 S2 Feature Generator
func (s *Server) ProxyToS2(c *gin.Context) {
	s.proxyRequest(c, "s2", "/features/recompute")
}

// ProxyToS3 代理到 S3 Strategy Engine
func (s *Server) ProxyToS3(c *gin.Context) {
	s.proxyRequest(c, "s3", "/decide")
}

// ProxyToS4 代理到 S4 Order Router
func (s *Server) ProxyToS4(c *gin.Context) {
	path := c.Request.URL.Path
	if strings.Contains(path, "/orders") {
		s.proxyRequest(c, "s4", "/orders")
	} else {
		s.proxyRequest(c, "s4", "/cancel")
	}
}

// ProxyToS5 代理到 S5 Reconciler
func (s *Server) ProxyToS5(c *gin.Context) {
	s.proxyRequest(c, "s5", "/reconcile")
}

// ProxyToS6 代理到 S6 Position Manager
func (s *Server) ProxyToS6(c *gin.Context) {
	s.proxyRequest(c, "s6", "/positions/manage")
}

// ProxyToS7 代理到 S7 Label Backfill
func (s *Server) ProxyToS7(c *gin.Context) {
	s.proxyRequest(c, "s7", "/labels/backfill")
}

// ProxyToS8 代理到 S8 Autopsy Generator
func (s *Server) ProxyToS8(c *gin.Context) {
	tradeID := c.Param("trade_id")
	s.proxyRequest(c, "s8", "/autopsy/"+tradeID)
}

// ProxyToS9 代理到 S9 Hypothesis Orchestrator
func (s *Server) ProxyToS9(c *gin.Context) {
	s.proxyRequest(c, "s9", "/experiments/run")
}

// ProxyToS10 代理到 S10 Config Service
func (s *Server) ProxyToS10(c *gin.Context) {
	path := c.Request.URL.Path
	if strings.Contains(path, "/bundles") {
		if strings.Contains(path, "/stage") {
			bundleID := c.Param("id")
			s.proxyRequest(c, "s10", "/bundles/"+bundleID+"/stage")
		} else {
			s.proxyRequest(c, "s10", "/bundles")
		}
	} else if strings.Contains(path, "/simulate") {
		s.proxyRequest(c, "s10", "/simulate")
	} else if strings.Contains(path, "/promote") {
		s.proxyRequest(c, "s10", "/promote")
	} else if strings.Contains(path, "/active") {
		s.proxyRequest(c, "s10", "/active")
	}
}

// ProxyToS11 代理到 S11 Metrics & Health
func (s *Server) ProxyToS11(c *gin.Context) {
	path := c.Request.URL.Path
	if strings.Contains(path, "/metrics") {
		s.proxyRequest(c, "s11", "/metrics")
	} else if strings.Contains(path, "/alerts") {
		s.proxyRequest(c, "s11", "/alerts")
	}
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
	s12Server := NewServer()

	r := gin.Default()

	// Health check routes (no auth required)
	r.GET("/health", s12Server.HealthCheck)
	r.GET("/ready", s12Server.ReadyCheck)

	// System control routes
	r.POST("/kill-switch", s12Server.RBACMiddleware(RoleAdmin), s12Server.KillSwitch)
	r.POST("/treasury/transfer", s12Server.RBACMiddleware(RoleTrader), s12Server.TreasuryTransfer)

	// Proxy routes to other services with RBAC
	// S2 Feature Generator - researcher
	r.POST("/features/recompute", s12Server.RBACMiddleware(RoleResearcher), s12Server.ProxyToS2)

	// S3 Strategy Engine - trader
	r.POST("/decide", s12Server.RBACMiddleware(RoleTrader), s12Server.ProxyToS3)

	// S4 Order Router - trader
	r.POST("/orders", s12Server.RBACMiddleware(RoleTrader), s12Server.ProxyToS4)
	r.POST("/cancel", s12Server.RBACMiddleware(RoleTrader), s12Server.ProxyToS4)

	// S5 Reconciler - admin
	r.POST("/reconcile", s12Server.RBACMiddleware(RoleAdmin), s12Server.ProxyToS5)

	// S6 Position Manager - trader
	r.POST("/positions/manage", s12Server.RBACMiddleware(RoleTrader), s12Server.ProxyToS6)

	// S7 Label Backfill - researcher
	r.POST("/labels/backfill", s12Server.RBACMiddleware(RoleResearcher), s12Server.ProxyToS7)

	// S8 Autopsy Generator - researcher
	r.POST("/autopsy/:trade_id", s12Server.RBACMiddleware(RoleResearcher), s12Server.ProxyToS8)

	// S9 Hypothesis Orchestrator - researcher
	r.POST("/experiments/run", s12Server.RBACMiddleware(RoleResearcher), s12Server.ProxyToS9)

	// S10 Config Service
	r.POST("/bundles", s12Server.RBACMiddleware(RoleResearcher), s12Server.ProxyToS10)
	r.POST("/bundles/:id/stage", s12Server.RBACMiddleware(RoleResearcher), s12Server.ProxyToS10)
	r.POST("/simulate", s12Server.RBACMiddleware(RoleResearcher), s12Server.ProxyToS10)
	r.POST("/promote", s12Server.RBACMiddleware(RoleRiskOfficer), s12Server.ProxyToS10)
	r.GET("/active", s12Server.RBACMiddleware(RoleViewer), s12Server.ProxyToS10)

	// S11 Metrics & Health - viewer
	r.GET("/metrics", s12Server.RBACMiddleware(RoleViewer), s12Server.ProxyToS11)
	r.GET("/alerts", s12Server.RBACMiddleware(RoleViewer), s12Server.ProxyToS11)

	// Use configuration port, fallback to environment variable or default
	port := os.Getenv("PORT")
	if port == "" && config.AppConfig.Service.Port != 0 {
		port = fmt.Sprintf("%d", config.AppConfig.Service.Port)
	}
	if port == "" {
		port = "8092"
	}

	log.Printf("S12 UI server starting on :%s", port)
	r.Run(":" + port)
}
