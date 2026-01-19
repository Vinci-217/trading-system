package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App         AppConfig        `yaml:"app"`
	Database    DatabaseConfig   `yaml:"database"`
	Redis       RedisConfig      `yaml:"redis"`
	Kafka       KafkaConfig      `yaml:"kafka"`
	Registry    RegistryConfig   `yaml:"registry"`
	Logging     LoggingConfig    `yaml:"logging"`
	TCC         TCCConfig        `yaml:"tcc"`
	RateLimit   RateLimitConfig  `yaml:"rate_limit"`
	RiskControl RiskControlConfig `yaml:"risk_control"`
}

type AppConfig struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	GRPCPort int    `yaml:"grpc_port"`
	HTTPPort int    `yaml:"http_port"`
	Version  string `yaml:"version"`
}

type DatabaseConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	Name            string `yaml:"name"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"`
}

func (d *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		d.Username, d.Password, d.Host, d.Port, d.Name)
}

type RedisConfig struct {
	Addr         string `yaml:"addr"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	PoolSize     int    `yaml:"pool_size"`
	MinIdleConns int    `yaml:"min_idle_conns"`
}

type KafkaConfig struct {
	Brokers         []string `yaml:"brokers"`
	TopicPrefix     string   `yaml:"topic_prefix"`
	OrderEventsTopic string  `yaml:"order_events_topic"`
}

type RegistryConfig struct {
	Type    string `yaml:"type"`
	Address string `yaml:"address"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

type TCCConfig struct {
	TimeoutSeconds int `yaml:"timeout_seconds"`
	MaxRetries     int `yaml:"max_retries"`
	RetryInterval  int `yaml:"retry_interval"`
}

func (t *TCCConfig) GetTimeout() time.Duration {
	return time.Duration(t.TimeoutSeconds) * time.Second
}

type RateLimitConfig struct {
	OrdersPerMinute int `yaml:"orders_per_minute"`
	OrdersPerHour   int `yaml:"orders_per_hour"`
	CancelsPerMinute int `yaml:"cancels_per_minute"`
}

type RiskControlConfig struct {
	MaxOrderQuantity int     `yaml:"max_order_quantity"`
	MaxOrderAmount   float64 `yaml:"max_order_amount"`
	PriceDeviationLimit float64 `yaml:"price_deviation_limit"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}
