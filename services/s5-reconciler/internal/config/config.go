package config

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

var AppConfig Config

type Config struct {
	Redis struct {
		Addr        string `yaml:"addr"`
		Password    string `yaml:"password"`
		PoolSize    int    `yaml:"poolSize"`
		DB          int    `yaml:"db"`
		Timeout     string `yaml:"timeout"`
		ReadTimeout string `yaml:"read_timeout"`
		WriteTimeout string `yaml:"write_timeout"`
	} `yaml:"redis"`
	ArangoDB struct {
		Addr          string `yaml:"addr"`
		Database      string `yaml:"database"`
		Username      string `yaml:"username"`
		Password      string `yaml:"password"`
		Timeout       string `yaml:"timeout"`
		MaxConnections int   `yaml:"max_connections"`
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
		PingInterval     string `yaml:"ping_interval"`
		PongTimeout      string `yaml:"pong_timeout"`
		BufferSize       int    `yaml:"buffer_size"`
	} `yaml:"websocket"`
	Service struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
		Port    int    `yaml:"port"`
		Host    string `yaml:"host"`
	} `yaml:"service"`
	Logging struct {
		Level        string `yaml:"level"`
		Format       string `yaml:"format"`
		Output       string `yaml:"output"`
		ServiceCode  string `yaml:"service_code"`
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
		Enabled                bool   `yaml:"enabled"`
		MonitorInterval        string `yaml:"monitor_interval"`
		LeakDetectionInterval string `yaml:"leak_detection_interval"`
		HeapThresholdMB        int    `yaml:"heap_threshold_mb"`
		SystemThresholdMB      int    `yaml:"system_threshold_mb"`
		GoroutineThreshold     int    `yaml:"goroutine_threshold"`
		GCThreshold            int    `yaml:"gc_threshold"`
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
