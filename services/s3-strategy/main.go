package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"s3-strategy/dao"
	"s3-strategy/internal/config"
	"s3-strategy/internal/services/arangodb"
	"s3-strategy/internal/services/redis"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// GitCommitNum is set during build time via ldflags
var GitCommitNum string

// S3 STRATEGY
// Strategy Engine - Execute trading strategies based on features and rules

// GateKeeper L0 守門器
type GateKeeper struct {
	maxFundingAbs              float64
	spreadBpLimit              float64
	depthTop1UsdtMin           float64
	spotQuoteUsdtMax           float64
	futMarginUsdtMax           float64
	concurrentEntriesPerMarket int
}

func (gk *GateKeeper) Check(req *dao.DecideRequest, features dao.FeatureSet) (bool, string) {
	// 資金費上限檢查
	if fundingNext, ok := features["funding_next"].(float64); ok {
		if math.Abs(fundingNext) > gk.maxFundingAbs {
			return false, fmt.Sprintf("funding rate too high: %.6f > %.6f", math.Abs(fundingNext), gk.maxFundingAbs)
		}
	}

	// 流動性檢查
	if spreadBps, ok := features["spread_bps"].(float64); ok {
		if spreadBps > gk.spreadBpLimit {
			return false, fmt.Sprintf("spread too wide: %.2f bps > %.2f bps", spreadBps, gk.spreadBpLimit)
		}
	}

	if depthTop1Usdt, ok := features["depth_top1_usdt"].(float64); ok {
		if depthTop1Usdt < gk.depthTop1UsdtMin {
			return false, fmt.Sprintf("insufficient depth: %.2f USDT < %.2f USDT", depthTop1Usdt, gk.depthTop1UsdtMin)
		}
	}

	// 風險預算檢查
	if req.Market == dao.MarketSPOT {
		// TODO: 檢查現貨名義金額限制
	} else if req.Market == dao.MarketFUT {
		// TODO: 檢查期貨保證金限制
	}

	return true, "gate check passed"
}

// RuleEngine L1 規則引擎
type RuleEngine struct {
	rules map[string]*dao.StrategyRule
}

func (re *RuleEngine) Evaluate(req *dao.DecideRequest, features dao.FeatureSet) (*dao.Decision, []string) {
	var firedRules []string
	var sizeMult, tpMult, slMult float64 = 1.0, 1.0, 1.0

	for _, rule := range re.rules {
		if !rule.Enabled {
			continue
		}

		if re.evaluateRule(rule, req, features) {
			firedRules = append(firedRules, rule.RuleID)

			// 解析規則動作
			var actions map[string]interface{}
			if err := json.Unmarshal([]byte(rule.Actions), &actions); err == nil {
				if mult, ok := actions["size_mult"].(float64); ok {
					sizeMult *= mult
				}
				if mult, ok := actions["tp_mult"].(float64); ok {
					tpMult *= mult
				}
				if mult, ok := actions["sl_mult"].(float64); ok {
					slMult *= mult
				}
			}
		}
	}

	// Clamp 到白名單範圍
	sizeMult = math.Max(0.1, math.Min(2.0, sizeMult))
	tpMult = math.Max(1.0, math.Min(3.0, tpMult))
	slMult = math.Max(0.1, math.Min(1.0, slMult))

	decision := &dao.Decision{
		Action:   dao.DecisionOpen,
		SizeMult: sizeMult,
		TPMult:   tpMult,
		SLMult:   slMult,
		Reason:   fmt.Sprintf("Rules fired: %v", firedRules),
	}

	return decision, firedRules
}

func (re *RuleEngine) evaluateRule(rule *dao.StrategyRule, req *dao.DecideRequest, features dao.FeatureSet) bool {
	var conditions map[string]interface{}
	if err := json.Unmarshal([]byte(rule.Conditions), &conditions); err != nil {
		return false
	}

	// 簡化的條件評估（實際實現需要更複雜的 DSL 解析）
	if allOf, ok := conditions["allOf"].([]interface{}); ok {
		for _, condition := range allOf {
			if condMap, ok := condition.(map[string]interface{}); ok {
				if !re.evaluateCondition(condMap, features) {
					return false
				}
			}
		}
	}

	return true
}

func (re *RuleEngine) evaluateCondition(condition map[string]interface{}, features dao.FeatureSet) bool {
	feature, _ := condition["f"].(string)
	operator, _ := condition["op"].(string)
	value, _ := condition["v"].(float64)

	if featureValue, exists := features[feature]; exists {
		if fv, ok := featureValue.(float64); ok {
			switch operator {
			case "<":
				return fv < value
			case ">":
				return fv > value
			case "<=":
				return fv <= value
			case ">=":
				return fv >= value
			case "==":
				return fv == value
			}
		}
	}

	return false
}

// MLModel L2 機器學習模型
type MLModel struct {
	modelName string
	version   string
}

func (ml *MLModel) Predict(features dao.FeatureSet) (float64, float64) {
	// 模擬 ML 模型預測
	// 實際實現會調用訓練好的模型

	// 基於特徵計算置信度分數
	score := 0.5 // 基礎分數

	if atr, ok := features["atr_pct"].(float64); ok {
		if atr < 1.0 { // 低波動
			score += 0.1
		}
	}

	if rv, ok := features["rv_pct"].(float64); ok {
		if rv < 0.25 { // 低波動率分位
			score += 0.15
		}
	}

	if correlation, ok := features["correlation"].(float64); ok {
		if correlation < -0.3 { // 負相關
			score += 0.1
		}
	}

	// Clamp 分數到 [0, 1]
	score = math.Max(0.0, math.Min(1.0, score))

	// 計算置信度
	confidence := math.Min(score*2, 1.0)

	return score, confidence
}

func (ml *MLModel) GetSizeMultiplier(score float64) float64 {
	// 基於分數的倉位倍率
	if score > 0.85 {
		return 1.2
	} else if score >= 0.6 {
		return 1.0
	} else if score >= 0.4 {
		return 0.5
	} else {
		return 0.0 // skip
	}
}

// ConfigManager 配置管理器
type ConfigManager struct {
	activeConfigRev int
	configCache     map[int]*dao.StrategyConfig
}

func (cm *ConfigManager) GetActiveConfig() *dao.StrategyConfig {
	return cm.configCache[cm.activeConfigRev]
}

func (cm *ConfigManager) LoadConfig(rev int) error {
	// TODO: 從 ArangoDB 加載配置
	config := &dao.StrategyConfig{
		ConfigRev:   rev,
		ConfigName:  "default",
		Parameters:  make(map[string]interface{}),
		Rules:       []string{"R-001", "R-002"},
		Instruments: []string{"BTCUSDT", "ETHUSDT"},
		Status:      "ACTIVE",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	cm.configCache[rev] = config
	cm.activeConfigRev = rev

	return nil
}

type S3_STRATEGYServer struct {
	redisClient    *redis.RedisClient
	arangodbClient *arangodb.ArangoDBClient
	validator      *validator.Validate
	version        string
	startTime      time.Time

	// 策略引擎組件
	gateKeeper    *GateKeeper
	ruleEngine    *RuleEngine
	mlModel       *MLModel
	configManager *ConfigManager

	// 配置快取
	activeConfig *dao.StrategyConfig
	activeRules  map[string]*dao.StrategyRule
	configMutex  sync.RWMutex

	// 風險管理
	riskLimits      map[string]float64
	positionTracker map[string]float64
	riskMutex       sync.RWMutex
}

func NewS3_STRATEGYServer() *S3_STRATEGYServer {
	server := &S3_STRATEGYServer{
		redisClient:     redis.GetInstance(),
		arangodbClient:  arangodb.GetInstance(),
		validator:       validator.New(),
		version:         "v1.0.0",
		startTime:       time.Now(),
		activeRules:     make(map[string]*dao.StrategyRule),
		riskLimits:      make(map[string]float64),
		positionTracker: make(map[string]float64),
	}

	// 初始化組件
	server.gateKeeper = &GateKeeper{
		maxFundingAbs:              0.0005,
		spreadBpLimit:              3.0,
		depthTop1UsdtMin:           200.0,
		spotQuoteUsdtMax:           10000.0,
		futMarginUsdtMax:           5000.0,
		concurrentEntriesPerMarket: 1,
	}

	server.ruleEngine = &RuleEngine{
		rules: make(map[string]*dao.StrategyRule),
	}

	server.mlModel = &MLModel{
		modelName: "default_model",
		version:   "v1.0",
	}

	server.configManager = &ConfigManager{
		configCache: make(map[int]*dao.StrategyConfig),
	}

	// 加載配置和規則
	server.loadConfiguration()

	// 啟動配置監聽
	go server.startConfigWatcher()

	return server
}

// @Summary Health check
// @Description Check service health status
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} apispec.HealthResponse
// @Router /health [get]
func (s *S3_STRATEGYServer) HealthCheck(c *gin.Context) {
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
			Name:      "ws-binance",
			Status:    dao.HealthOK,
			LatencyMs: 15,
		},
	}

	response := dao.HealthResponse{
		Service:  "s3-strategy",
		Version:  s.version,
		Status:   dao.HealthOK,
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
func (s *S3_STRATEGYServer) ReadyCheck(c *gin.Context) {
	// Check all critical dependencies are ready
	response := dao.HealthResponse{
		Service:  "s3-strategy",
		Version:  s.version,
		Status:   dao.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Notes:    "Service ready to accept requests",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Make trading decision
// @Description Execute strategy rules and generate trading decisions
// @Tags strategy
// @Accept json
// @Produce json
// @Param request body apispec.DecideRequest true "Decision request"
// @Success 200 {object} apispec.DecideResponse
// @Router /decide [post]
func (s *S3_STRATEGYServer) Decide(c *gin.Context) {
	var req dao.DecideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format", "details": err.Error()})
		return
	}

	// 驗證請求
	if err := s.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	// L0 守門檢查
	passed, reason := s.gateKeeper.Check(&req, req.Features)
	if !passed {
		response := dao.DecideResponse{
			Decision: dao.Decision{
				Action: dao.DecisionSkip,
				Reason: reason,
			},
		}
		c.JSON(http.StatusOK, response)
		return
	}

	// L1 規則引擎評估
	decision, firedRules := s.ruleEngine.Evaluate(&req, req.Features)

	// L2 ML 模型評分
	mlScore, confidence := s.mlModel.Predict(req.Features)
	mlSizeMult := s.mlModel.GetSizeMultiplier(mlScore)

	// 合併規則和 ML 結果
	finalSizeMult := decision.SizeMult * mlSizeMult
	if mlSizeMult == 0.0 {
		decision.Action = dao.DecisionSkip
		decision.Reason = "ML model recommends skip"
	} else {
		decision.SizeMult = finalSizeMult
		decision.Reason = fmt.Sprintf("Rules: %v, ML Score: %.3f", firedRules, mlScore)
	}

	// 生成訂單意圖
	var intents []dao.OrderIntent
	if decision.Action == dao.DecisionOpen && !req.DryRun {
		intent := s.generateOrderIntent(&req, decision)
		intents = append(intents, intent)
	}

	// 保存信號
	s.saveSignal(&req, decision, firedRules, mlScore, confidence)

	response := dao.DecideResponse{
		Decision: *decision,
		Intents:  intents,
	}

	c.JSON(http.StatusOK, response)
}

// generateOrderIntent 生成訂單意圖
func (s *S3_STRATEGYServer) generateOrderIntent(req *dao.DecideRequest, decision *dao.Decision) dao.OrderIntent {
	// 計算倉位大小
	marginBase := 20.0 // USDT
	margin := marginBase * decision.SizeMult
	leverage := 20
	notional := margin * float64(leverage)

	// 生成執行策略
	execPolicy := dao.ExecPolicy{
		PreferMaker:     true,
		MakerWaitMs:     2000,
		TWAPSlices:      1,
		GuardStopEnable: false,
		TPPct:           decision.TPMult * 0.02, // 2% * TP倍率
		SLPct:           decision.SLMult * 0.01, // 1% * SL倍率
	}

	// 如果是 SPOT 市場，添加 OCO 策略
	if req.Market == dao.MarketSPOT {
		currentPrice := 50000.0 // 模擬價格
		execPolicy.OCO = &dao.OCO{
			TakeProfitPx: currentPrice * (1 + execPolicy.TPPct),
			StopLossPx:   currentPrice * (1 - execPolicy.SLPct),
		}
	}

	return dao.OrderIntent{
		IntentID:     fmt.Sprintf("intent_%s_%d", req.SignalID, time.Now().Unix()),
		Symbol:       req.Symbol,
		Market:       req.Market,
		Kind:         dao.IntentEntry,
		Side:         dao.SideBuy, // 簡化為買入
		NotionalUSDT: notional,
		Leverage:     leverage,
		ExecPolicy:   execPolicy,
	}
}

// saveSignal 保存交易信號
func (s *S3_STRATEGYServer) saveSignal(req *dao.DecideRequest, decision *dao.Decision, firedRules []string, mlScore, confidence float64) {
	signal := &dao.Signal{
		SignalID:  req.SignalID,
		Symbol:    req.Symbol,
		Market:    string(req.Market),
		Features:  req.Features,
		ConfigRev: 1, // TODO: 從配置管理器獲取
		Timestamp: time.Now().UnixMilli(),
		CreatedAt: time.Now(),
	}

	// TODO: 保存到 ArangoDB
	log.Printf("Saved signal: %+v", signal)

	// 發布到 Redis Stream
	s.publishSignalToRedis(signal, decision, firedRules, mlScore, confidence)
}

// publishSignalToRedis 發布信號到 Redis
func (s *S3_STRATEGYServer) publishSignalToRedis(signal *dao.Signal, decision *dao.Decision, firedRules []string, mlScore, confidence float64) {
	// TODO: 實現 Redis Stream 發布
	log.Printf("Publishing signal to Redis: %s", signal.SignalID)
}

// loadConfiguration 加載配置
func (s *S3_STRATEGYServer) loadConfiguration() {
	// 加載策略配置
	s.configManager.LoadConfig(1)

	// 加載策略規則
	s.loadStrategyRules()
}

// loadStrategyRules 加載策略規則
func (s *S3_STRATEGYServer) loadStrategyRules() {
	// 示例規則
	rule1 := &dao.StrategyRule{
		RuleID:     "R-001",
		RuleName:   "Low Volatility Entry",
		RuleType:   "ENTRY",
		Conditions: `{"allOf":[{"f":"rv_pctile_30d","op":"<","v":0.25},{"f":"rho_usdttwd_14","op":"<","v":-0.3}]}`,
		Actions:    `{"size_mult":1.2,"tp_mult":2.0,"sl_mult":0.5}`,
		Priority:   50,
		Enabled:    true,
		ConfigRev:  1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	rule2 := &dao.StrategyRule{
		RuleID:     "R-002",
		RuleName:   "High Correlation Exit",
		RuleType:   "EXIT",
		Conditions: `{"allOf":[{"f":"correlation","op":">","v":0.8}]}`,
		Actions:    `{"size_mult":0.5}`,
		Priority:   30,
		Enabled:    true,
		ConfigRev:  1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	s.configMutex.Lock()
	s.activeRules["R-001"] = rule1
	s.activeRules["R-002"] = rule2
	s.ruleEngine.rules["R-001"] = rule1
	s.ruleEngine.rules["R-002"] = rule2
	s.configMutex.Unlock()
}

// startConfigWatcher 啟動配置監聽
func (s *S3_STRATEGYServer) startConfigWatcher() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkConfigUpdates()
		}
	}
}

// checkConfigUpdates 檢查配置更新
func (s *S3_STRATEGYServer) checkConfigUpdates() {
	// TODO: 檢查 Redis cfg:events 或 ArangoDB config_active.rev
	log.Println("Checking for config updates...")
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
	s3Server := NewS3_STRATEGYServer()

	r := gin.Default()

	// Health check routes
	r.GET("/health", s3Server.HealthCheck)
	r.GET("/ready", s3Server.ReadyCheck)

	// Strategy routes
	r.POST("/decide", s3Server.Decide)

	// Use configuration port, fallback to environment variable or default
	port := os.Getenv("PORT")
	if port == "" && config.AppConfig.Service.Port != 0 {
		port = fmt.Sprintf("%d", config.AppConfig.Service.Port)
	}
	if port == "" {
		port = "8083"
	}

	log.Printf("S3 STRATEGY server starting on :%s", port)
	r.Run(":" + port)
}
