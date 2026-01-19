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
	MarketData  MarketDataConfig `yaml:"market_data"`
	WebSocket   WebSocketConfig  `yaml:"websocket"`
	Symbols     []SymbolConfig   `yaml:"symbols"`
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
	Brokers        []string `yaml:"brokers"`
	TopicPrefix    string   `yaml:"topic_prefix"`
	MarketDataTopic string  `yaml:"market_data_topic"`
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

type MarketDataConfig struct {
	UpdateIntervalMs int      `yaml:"update_interval_ms"`
	KLineIntervals   []string `yaml:"kline_intervals"`
}

func (m *MarketDataConfig) GetUpdateInterval() time.Duration {
	return time.Duration(m.UpdateIntervalMs) * time.Millisecond
}

type WebSocketConfig struct {
	PingInterval        int `yaml:"ping_interval"`
	PongTimeout         int `yaml:"pong_timeout"`
	MaxConnectionsPerUser int `yaml:"max_connections_per_user"`
}

func (w *WebSocketConfig) GetPingInterval() time.Duration {
	return time.Duration(w.PingInterval) * time.Second
}

func (w *WebSocketConfig) GetPongTimeout() time.Duration {
	return time.Duration(w.PongTimeout) * time.Second
}

type SymbolConfig struct {
	Symbol    string  `yaml:"symbol"`
	Name      string  `yaml:"name"`
	BasePrice float64 `yaml:"base_price"`
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
