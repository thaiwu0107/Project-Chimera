package config

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

var AppConfig Config

// TODO: S1 Exchange Connectors 配置管理待實作項目
// ================================
// 1. 環境變數配置
//    - [ ] 實現 S1_REDIS_ADDRESSES 配置
//    - [ ] 實現 S1_DB_ARANGO_URI/USER/PASS 配置
//    - [ ] 實現 S1_BINANCE_KEY/SECRET 配置
//    - [ ] 實現 S1_TESTNET 配置
//    - [ ] 實現 S1_WS_RECONNECT_INTERVAL 配置
//    - [ ] 實現 S1_DATA_RETENTION_DAYS 配置
//
// 2. WebSocket 配置
//    - [ ] 實現連接超時配置
//    - [ ] 實現重連間隔配置
//    - [ ] 實現心跳間隔配置
//    - [ ] 實現緩衝區大小配置
//    - [ ] 實現訂閱符號配置
//
// 3. Redis 配置
//    - [ ] 實現集群模式配置
//    - [ ] 實現連接池配置
//    - [ ] 實現超時配置
//    - [ ] 實現重試配置
//    - [ ] 實現 Stream 配置
//
// 4. ArangoDB 配置
//    - [ ] 實現連接配置
//    - [ ] 實現超時配置
//    - [ ] 實現重試配置
//    - [ ] 實現批量寫入配置
//    - [ ] 實現索引配置
//
// 5. Binance API 配置
//    - [ ] 實現 API 端點配置
//    - [ ] 實現速率限制配置
//    - [ ] 實現超時配置
//    - [ ] 實現重試配置
//    - [ ] 實現簽名配置
//
// 6. Treasury 配置
//    - [ ] 實現最大重試次數配置
//    - [ ] 實現重試間隔配置
//    - [ ] 實現超時配置
//    - [ ] 實現速率限制配置
//    - [ ] 實現最小/最大劃轉金額配置
//
// 7. 監控配置
//    - [ ] 實現內存監控配置
//    - [ ] 實現 Goroutine 監控配置
//    - [ ] 實現 GC 監控配置
//    - [ ] 實現告警閾值配置
//    - [ ] 實現指標收集配置
//
// 8. 日誌配置
//    - [ ] 實現日誌級別配置
//    - [ ] 實現日誌格式配置
//    - [ ] 實現日誌輸出配置
//    - [ ] 實現服務代碼配置
//    - [ ] 實現結構化日誌配置
//
// 9. APM 配置
//    - [ ] 實現服務名稱配置
//    - [ ] 實現環境配置
//    - [ ] 實現服務器 URL 配置
//    - [ ] 實現啟用/禁用配置
//    - [ ] 實現採樣率配置
//
// 10. 健康檢查配置
//     - [ ] 實現檢查間隔配置
//     - [ ] 實現超時配置
//     - [ ] 實現依賴檢查配置
//     - [ ] 實現狀態回報配置
//     - [ ] 實現告警配置

type Config struct {
	Redis struct {
		Addr         string `yaml:"addr"`
		Password     string `yaml:"password"`
		PoolSize     int    `yaml:"poolSize"`
		DB           int    `yaml:"db"`
		Timeout      string `yaml:"timeout"`
		ReadTimeout  string `yaml:"read_timeout"`
		WriteTimeout string `yaml:"write_timeout"`
	} `yaml:"redis"`
	ArangoDB struct {
		Addr           string `yaml:"addr"`
		Database       string `yaml:"database"`
		Username       string `yaml:"username"`
		Password       string `yaml:"password"`
		Timeout        string `yaml:"timeout"`
		MaxConnections int    `yaml:"max_connections"`
	} `yaml:"arangodb"`
	Exchange struct {
		Binance struct {
			APIKey     string `yaml:"api_key"`
			SecretKey  string `yaml:"secret_key"`
			BaseURL    string `yaml:"base_url"`
			FuturesURL string `yaml:"futures_url"`
			Testnet    bool   `yaml:"testnet"`
			Timeout    string `yaml:"timeout"`
			RateLimit  int    `yaml:"rate_limit"`
		} `yaml:"binance"`
	} `yaml:"exchange"`
	WebSocket struct {
		ReconnectInterval string `yaml:"reconnect_interval"`
		PingInterval      string `yaml:"ping_interval"`
		PongTimeout       string `yaml:"pong_timeout"`
		BufferSize        int    `yaml:"buffer_size"`
	} `yaml:"websocket"`
	Service struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
		Port    int    `yaml:"port"`
		Host    string `yaml:"host"`
	} `yaml:"service"`
	Logging struct {
		Level       string `yaml:"level"`
		Format      string `yaml:"format"`
		Output      string `yaml:"output"`
		ServiceCode string `yaml:"service_code"`
	} `yaml:"logging"`
	APM struct {
		ServiceName        string   `yaml:"service_name"`
		ServiceEnvironment string   `yaml:"service_environment"`
		ServerURLs         []string `yaml:"server_urls"`
		Enabled            bool     `yaml:"enabled"`
	} `yaml:"apm"`
	Health struct {
		CheckInterval string   `yaml:"check_interval"`
		Timeout       string   `yaml:"timeout"`
		Dependencies  []string `yaml:"dependencies"`
	} `yaml:"health"`
	MemoryMonitoring struct {
		Enabled               bool   `yaml:"enabled"`
		MonitorInterval       string `yaml:"monitor_interval"`
		LeakDetectionInterval string `yaml:"leak_detection_interval"`
		HeapThresholdMB       int    `yaml:"heap_threshold_mb"`
		SystemThresholdMB     int    `yaml:"system_threshold_mb"`
		GoroutineThreshold    int    `yaml:"goroutine_threshold"`
		GCThreshold           int    `yaml:"gc_threshold"`
	} `yaml:"memory_monitoring"`
}

// LoadConfig loads configuration file with priority: env.local.yaml > env.yaml > config.yaml
func LoadConfig(path string) error {
	// If no path specified, try to load default configuration files
	if path == "" {
		// Priority: env.local.yaml (for local development)
		if _, err := os.Stat("env.local.yaml"); err == nil {
			path = "env.local.yaml"
		} else if _, err := os.Stat("env.yaml"); err == nil {
			path = "env.yaml"
		} else if _, err := os.Stat("config.yaml"); err == nil {
			path = "config.yaml"
		} else {
			path = "env.yaml" // Default to env.yaml
		}
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(buf, &AppConfig)
	if err != nil {
		return err
	}
	return nil
}
