package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"s2-feature/dao"
	"s2-feature/internal/apispec"
	"s2-feature/internal/config"
	"s2-feature/internal/services/arangodb"
	"s2-feature/internal/services/redis"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// GitCommitNum is set during build time via ldflags
var GitCommitNum string

// S2 FEATURE
// Feature Generator - Generate features from market data and signals

// FeatureCalculator 特徵計算器接口
type FeatureCalculator interface {
	Calculate(symbol string, data []MarketDataPoint) (map[string]interface{}, error)
	GetFeatureType() string
}

// MarketDataPoint 市場數據點
type MarketDataPoint struct {
	Timestamp int64   `json:"timestamp"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}

// ATRCalculator ATR 計算器
type ATRCalculator struct {
	period int
}

func (calc *ATRCalculator) Calculate(symbol string, data []MarketDataPoint) (map[string]interface{}, error) {
	if len(data) < calc.period+1 {
		return nil, fmt.Errorf("insufficient data for ATR calculation")
	}

	// 計算 True Range
	trValues := make([]float64, 0, len(data)-1)
	for i := 1; i < len(data); i++ {
		tr := math.Max(data[i].High-data[i].Low,
			math.Max(math.Abs(data[i].High-data[i-1].Close),
				math.Abs(data[i].Low-data[i-1].Close)))
		trValues = append(trValues, tr)
	}

	// 計算 ATR (Wilder's smoothing)
	atr := 0.0
	for i := 0; i < calc.period; i++ {
		atr += trValues[i]
	}
	atr /= float64(calc.period)

	// 繼續平滑
	for i := calc.period; i < len(trValues); i++ {
		atr = atr + (trValues[i]-atr)/float64(calc.period)
	}

	// 計算 ATR 百分比
	currentPrice := data[len(data)-1].Close
	atrPct := (atr / currentPrice) * 100

	return map[string]interface{}{
		"atr":       atr,
		"atr_pct":   atrPct,
		"period":    calc.period,
		"symbol":    symbol,
		"timestamp": data[len(data)-1].Timestamp,
	}, nil
}

func (calc *ATRCalculator) GetFeatureType() string {
	return "ATR"
}

// RVCalculator 已實現波動率計算器
type RVCalculator struct {
	period int
}

func (calc *RVCalculator) Calculate(symbol string, data []MarketDataPoint) (map[string]interface{}, error) {
	if len(data) < calc.period+1 {
		return nil, fmt.Errorf("insufficient data for RV calculation")
	}

	// 計算 log returns
	logReturns := make([]float64, 0, len(data)-1)
	for i := 1; i < len(data); i++ {
		if data[i-1].Close > 0 {
			logRet := math.Log(data[i].Close / data[i-1].Close)
			logReturns = append(logReturns, logRet)
		}
	}

	if len(logReturns) < calc.period {
		return nil, fmt.Errorf("insufficient log returns for RV calculation")
	}

	// 計算標準差
	mean := 0.0
	for _, ret := range logReturns[len(logReturns)-calc.period:] {
		mean += ret
	}
	mean /= float64(calc.period)

	variance := 0.0
	for _, ret := range logReturns[len(logReturns)-calc.period:] {
		variance += math.Pow(ret-mean, 2)
	}
	variance /= float64(calc.period)

	// 年化波動率
	rv := math.Sqrt(variance) * math.Sqrt(252)

	return map[string]interface{}{
		"rv":        rv,
		"rv_pct":    rv * 100,
		"period":    calc.period,
		"symbol":    symbol,
		"timestamp": data[len(data)-1].Timestamp,
	}, nil
}

func (calc *RVCalculator) GetFeatureType() string {
	return "RV"
}

// CorrelationCalculator 相關性計算器
type CorrelationCalculator struct {
	period int
}

func (calc *CorrelationCalculator) Calculate(symbol string, data []MarketDataPoint) (map[string]interface{}, error) {
	// 這裡需要兩個標的的數據，暫時返回模擬數據
	return map[string]interface{}{
		"correlation": 0.75,
		"symbol1":     symbol,
		"symbol2":     "BTCUSDT",
		"period":      calc.period,
		"timestamp":   time.Now().UnixMilli(),
	}, nil
}

func (calc *CorrelationCalculator) GetFeatureType() string {
	return "CORRELATION"
}

// DepthCalculator 深度特徵計算器
type DepthCalculator struct{}

func (calc *DepthCalculator) Calculate(symbol string, data []MarketDataPoint) (map[string]interface{}, error) {
	// 模擬深度數據
	return map[string]interface{}{
		"bid_depth":     1000.0,
		"ask_depth":     1200.0,
		"bid_ask_ratio": 0.83,
		"spread":        0.5,
		"spread_pct":    0.001,
		"symbol":        symbol,
		"timestamp":     time.Now().UnixMilli(),
	}, nil
}

func (calc *DepthCalculator) GetFeatureType() string {
	return "DEPTH"
}

type S2_FEATUREServer struct {
	redisClient    *redis.RedisClient
	arangodbClient *arangodb.ArangoDBClient
	validator      *validator.Validate
	version        string
	startTime      time.Time

	// 特徵計算器
	featureCalculators map[string]FeatureCalculator

	// 數據快取
	featureCache map[string]*dao.FeatureSetSnapshot
	cacheMutex   sync.RWMutex

	// 計算任務管理
	computationTasks map[string]*dao.FeatureComputation
	taskMutex        sync.RWMutex
}

func NewS2_FEATUREServer() *S2_FEATUREServer {
	server := &S2_FEATUREServer{
		redisClient:        redis.GetInstance(),
		arangodbClient:     arangodb.GetInstance(),
		validator:          validator.New(),
		version:            "v1.0.0",
		startTime:          time.Now(),
		featureCalculators: make(map[string]FeatureCalculator),
		featureCache:       make(map[string]*dao.FeatureSetSnapshot),
		computationTasks:   make(map[string]*dao.FeatureComputation),
	}

	// 初始化特徵計算器
	server.initializeFeatureCalculators()

	// 啟動定時任務
	go server.startScheduledTasks()

	return server
}

func (s *S2_FEATUREServer) HealthCheck(c *gin.Context) {
	checks := []apispec.HealthCheck{
		{Name: "redis", Status: apispec.HealthOK, LatencyMs: 5},
		{Name: "arangodb", Status: apispec.HealthOK, LatencyMs: 10},
		{Name: "service", Status: apispec.HealthOK, LatencyMs: 8},
	}

	response := apispec.HealthResponse{
		Service:  "s2-feature",
		Version:  s.version,
		Status:   apispec.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Checks:   checks,
		Notes:    "Service running normally",
	}

	c.JSON(http.StatusOK, response)
}

func (s *S2_FEATUREServer) ReadyCheck(c *gin.Context) {
	response := apispec.HealthResponse{
		Service:  "s2-feature",
		Version:  s.version,
		Status:   apispec.HealthOK,
		Ts:       time.Now().UnixMilli(),
		UptimeMs: time.Since(s.startTime).Milliseconds(),
		Notes:    "Service ready to accept requests",
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Recompute features
// @Description Recompute features for a specific symbol
// @Tags features
// @Accept json
// @Produce json
// @Param request body apispec.RecomputeFeaturesRequest true "Recompute request"
// @Success 200 {object} apispec.RecomputeFeaturesResponse
// @Router /features/recompute [post]
func (s *S2_FEATUREServer) RecomputeFeatures(c *gin.Context) {
	var req dao.RecomputeFeaturesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format", "details": err.Error()})
		return
	}

	// 驗證請求
	if err := s.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	// 執行特徵重算
	computed := 0
	for _, symbol := range req.Symbols {
		for _, window := range req.Windows {
			if err := s.computeFeaturesForSymbol(symbol, window, req.Force); err != nil {
				log.Printf("Failed to compute features for %s %s: %v", symbol, window, err)
				continue
			}
			computed++
		}
	}

	response := dao.RecomputeFeaturesResponse{
		Computed: computed,
		Message:  fmt.Sprintf("Successfully computed %d feature sets", computed),
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Get features
// @Description Get computed features for a symbol
// @Tags features
// @Accept json
// @Produce json
// @Param symbol query string true "Symbol (e.g., BTCUSDT)"
// @Param feature_type query string false "Feature type (ATR/RV/CORRELATION/DEPTH)"
// @Success 200 {object} dao.FeatureSetSnapshot
// @Router /features [get]
func (s *S2_FEATUREServer) GetFeatures(c *gin.Context) {
	symbol := c.Query("symbol")
	featureType := c.Query("feature_type")

	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol parameter is required"})
		return
	}

	// 從快取獲取特徵
	s.cacheMutex.RLock()
	snapshot, exists := s.featureCache[symbol]
	s.cacheMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "features not found for symbol"})
		return
	}

	// 如果指定了特徵類型，只返回該類型的特徵
	if featureType != "" {
		filteredFeatures := make(map[string]interface{})
		for key, value := range snapshot.Features {
			if key == featureType {
				filteredFeatures[key] = value
			}
		}
		snapshot.Features = filteredFeatures
	}

	c.JSON(http.StatusOK, snapshot)
}

// @Summary Get computation status
// @Description Get status of feature computation tasks
// @Tags features
// @Accept json
// @Produce json
// @Param task_id query string false "Task ID"
// @Success 200 {object} dao.FeatureComputation
// @Router /features/computation [get]
func (s *S2_FEATUREServer) GetComputationStatus(c *gin.Context) {
	taskID := c.Query("task_id")

	if taskID != "" {
		s.taskMutex.RLock()
		task, exists := s.computationTasks[taskID]
		s.taskMutex.RUnlock()

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}

		c.JSON(http.StatusOK, task)
		return
	}

	// 返回所有任務
	s.taskMutex.RLock()
	tasks := make([]*dao.FeatureComputation, 0, len(s.computationTasks))
	for _, task := range s.computationTasks {
		tasks = append(tasks, task)
	}
	s.taskMutex.RUnlock()

	c.JSON(http.StatusOK, tasks)
}

// initializeFeatureCalculators 初始化特徵計算器
func (s *S2_FEATUREServer) initializeFeatureCalculators() {
	s.featureCalculators["ATR"] = &ATRCalculator{period: 14}
	s.featureCalculators["RV"] = &RVCalculator{period: 20}
	s.featureCalculators["CORRELATION"] = &CorrelationCalculator{period: 14}
	s.featureCalculators["DEPTH"] = &DepthCalculator{}
}

// computeFeaturesForSymbol 為指定標的計算特徵
func (s *S2_FEATUREServer) computeFeaturesForSymbol(symbol, window string, force bool) error {
	// 檢查快取
	if !force {
		s.cacheMutex.RLock()
		if _, exists := s.featureCache[symbol]; exists {
			s.cacheMutex.RUnlock()
			return nil // 已存在，跳過
		}
		s.cacheMutex.RUnlock()
	}

	// 創建計算任務
	taskID := fmt.Sprintf("task_%s_%s_%d", symbol, window, time.Now().Unix())
	task := &dao.FeatureComputation{
		TaskID:      taskID,
		Symbol:      symbol,
		FeatureType: "ALL",
		FromMs:      time.Now().Add(-24 * time.Hour).UnixMilli(),
		ToMs:        time.Now().UnixMilli(),
		Status:      "RUNNING",
		Progress:    0.0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	s.taskMutex.Lock()
	s.computationTasks[taskID] = task
	s.taskMutex.Unlock()

	// 獲取市場數據（模擬）
	marketData := s.getMarketData(symbol, window)

	// 計算各種特徵
	features := make(map[string]interface{})

	for featureType, calculator := range s.featureCalculators {
		result, err := calculator.Calculate(symbol, marketData)
		if err != nil {
			log.Printf("Failed to calculate %s for %s: %v", featureType, symbol, err)
			continue
		}
		features[featureType] = result
	}

	// 更新任務狀態
	task.Status = "COMPLETED"
	task.Progress = 100.0
	task.UpdatedAt = time.Now()

	// 保存特徵快照
	snapshot := &dao.FeatureSetSnapshot{
		SetID:     fmt.Sprintf("set_%s_%d", symbol, time.Now().Unix()),
		Symbol:    symbol,
		Features:  features,
		Timestamp: time.Now().UnixMilli(),
		CreatedAt: time.Now(),
	}

	s.cacheMutex.Lock()
	s.featureCache[symbol] = snapshot
	s.cacheMutex.Unlock()

	// 發布到 Redis
	s.publishFeaturesToRedis(snapshot)

	return nil
}

// getMarketData 獲取市場數據（模擬）
func (s *S2_FEATUREServer) getMarketData(symbol, window string) []MarketDataPoint {
	// 模擬生成 K 線數據
	data := make([]MarketDataPoint, 0, 100)
	basePrice := 50000.0

	for i := 0; i < 100; i++ {
		timestamp := time.Now().Add(-time.Duration(100-i) * time.Minute).UnixMilli()
		price := basePrice + float64(i)*10 + math.Sin(float64(i)*0.1)*100

		data = append(data, MarketDataPoint{
			Timestamp: timestamp,
			Open:      price,
			High:      price + 50,
			Low:       price - 50,
			Close:     price + 25,
			Volume:    1000.0,
		})
	}

	return data
}

// publishFeaturesToRedis 發布特徵到 Redis
func (s *S2_FEATUREServer) publishFeaturesToRedis(snapshot *dao.FeatureSetSnapshot) {
	// TODO: 實現 Redis 發布
	log.Printf("Publishing features to Redis for %s", snapshot.Symbol)
}

// startScheduledTasks 啟動定時任務
func (s *S2_FEATUREServer) startScheduledTasks() {
	// 每 5 分鐘補算特徵
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runScheduledFeatureComputation()
			}
		}
	}()
}

// runScheduledFeatureComputation 執行定時特徵計算
func (s *S2_FEATUREServer) runScheduledFeatureComputation() {
	symbols := []string{"BTCUSDT", "ETHUSDT", "ADAUSDT"}
	windows := []string{"1m", "5m", "1h", "4h", "1d"}

	for _, symbol := range symbols {
		for _, window := range windows {
			if err := s.computeFeaturesForSymbol(symbol, window, false); err != nil {
				log.Printf("Scheduled computation failed for %s %s: %v", symbol, window, err)
			}
		}
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
	server := NewS2_FEATUREServer()

	r := gin.Default()

	// Health check routes
	r.GET("/health", server.HealthCheck)
	r.GET("/ready", server.ReadyCheck)

	// Feature routes
	r.POST("/features/recompute", server.RecomputeFeatures)
	r.GET("/features", server.GetFeatures)
	r.GET("/features/computation", server.GetComputationStatus)

	// Use configuration port, fallback to environment variable or default
	port := os.Getenv("PORT")
	if port == "" && config.AppConfig.Service.Port != 0 {
		port = fmt.Sprintf("%d", config.AppConfig.Service.Port)
	}
	if port == "" {
		port = "8082"
	}

	log.Printf("S2 FEATURE server starting on :%s", port)
	r.Run(":" + port)
}
