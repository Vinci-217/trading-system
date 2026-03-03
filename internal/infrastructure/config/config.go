package config

import (
	"os"
	"sync"
	
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Kafka    KafkaConfig    `yaml:"kafka"`
	Log      LogConfig      `yaml:"log"`
	Trading  TradingConfig  `yaml:"trading"`
}

type ServerConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	HTTP    struct {
		Addr string `yaml:"addr"`
		Port int    `yaml:"port"`
	} `yaml:"http"`
	GRPC struct {
		Addr string `yaml:"addr"`
		Port int    `yaml:"port"`
	} `yaml:"grpc"`
}

type DatabaseConfig struct {
	Driver          string `yaml:"driver"`
	DSN             string `yaml:"dsn"`
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
	Brokers []string `yaml:"brokers"`
	GroupID string   `yaml:"group_id"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type TradingConfig struct {
	Order struct {
		MaxQuantity    int     `yaml:"max_quantity"`
		MinQuantity    int     `yaml:"min_quantity"`
		MaxAmount      float64 `yaml:"max_amount"`
		PriceDeviation float64 `yaml:"price_deviation"`
		ExpireTime     int     `yaml:"expire_time"`
	} `yaml:"order"`
	RateLimit struct {
		OrderCreate int `yaml:"order_create"`
		OrderCancel int `yaml:"order_cancel"`
	} `yaml:"rate_limit"`
	Reconciliation struct {
		DiscrepancyThreshold float64 `yaml:"discrepancy_threshold"`
		AutoFixEnabled       bool    `yaml:"auto_fix_enabled"`
		AutoFixMaxAmount     float64 `yaml:"auto_fix_max_amount"`
	} `yaml:"reconciliation"`
}

var (
	cfg  *Config
	once sync.Once
)

func Load(path string) (*Config, error) {
	var err error
	once.Do(func() {
		cfg = &Config{}
		var data []byte
		data, err = os.ReadFile(path)
		if err != nil {
			return
		}
		err = yaml.Unmarshal(data, cfg)
	})
	return cfg, err
}

func Get() *Config {
	return cfg
}
