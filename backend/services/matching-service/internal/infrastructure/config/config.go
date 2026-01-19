package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App              AppConfig      `yaml:"app"`
	Database         DatabaseConfig `yaml:"database"`
	Redis            RedisConfig    `yaml:"redis"`
	Kafka            KafkaConfig    `yaml:"kafka"`
	Registry         RegistryConfig `yaml:"registry"`
	Logging          LoggingConfig  `yaml:"logging"`
	Matching         MatchingConfig `yaml:"matching"`
	Symbols          []string       `yaml:"symbols"`
	GRPCPort         int            `yaml:"grpc_port"`
	HTTPPort         int            `yaml:"http_port"`
	MatchingIntervalMs int64        `yaml:"matching_interval_ms"`
}

type AppConfig struct {
	Name    string `yaml:"name"`
	Host    string `yaml:"host"`
	Version string `yaml:"version"`
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
	TradeEventsTopic string  `yaml:"trade_events_topic"`
	MarketDataTopic  string  `yaml:"market_data_topic"`
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

type MatchingConfig struct {
	PriceTick        float64 `yaml:"price_tick"`
	MaxPriceLevels   int     `yaml:"max_price_levels"`
	OrderQueueSize   int     `yaml:"order_queue_size"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.GRPCPort == 0 {
		cfg.GRPCPort = 5005
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8085
	}
	if cfg.MatchingIntervalMs == 0 {
		cfg.MatchingIntervalMs = 100
	}

	return &cfg, nil
}
