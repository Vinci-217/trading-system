package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App            AppConfig            `yaml:"app"`
	Database       DatabaseConfig       `yaml:"database"`
	Redis          RedisConfig          `yaml:"redis"`
	Kafka          KafkaConfig          `yaml:"kafka"`
	Seata          SeataConfig          `yaml:"seata"`
	Reconciliation ReconciliationConfig `yaml:"reconciliation"`
	RiskControl    RiskControlConfig    `yaml:"risk_control"`
	Logging        LoggingConfig        `yaml:"logging"`
}

type AppConfig struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	GRPCPort int    `yaml:"grpc_port"`
	HTTPPort int    `yaml:"http_port"`
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

func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		d.Username, d.Password, d.Host, d.Port, d.Name)
}

func (d *DatabaseConfig) ConnMaxLifetimeDuration() time.Duration {
	return time.Duration(d.ConnMaxLifetime) * time.Second
}

type RedisConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Password  string `yaml:"password"`
	DB        int    `yaml:"db"`
	PoolSize  int    `yaml:"pool_size"`
}

func (r *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type KafkaConfig struct {
	Brokers       []string          `yaml:"brokers"`
	ConsumerGroup string            `yaml:"consumer_group"`
	Topics        KafkaTopicsConfig `yaml:"topics"`
}

type KafkaTopicsConfig struct {
	Orders string `yaml:"orders"`
	Trades string `yaml:"trades"`
	Quotes string `yaml:"quotes"`
}

type SeataConfig struct {
	Enabled                 bool   `yaml:"enabled"`
	TCServer                string `yaml:"tc_server"`
	ApplicationID           string `yaml:"application_id"`
	TransactionServiceGroup string `yaml:"transaction_service_group"`
}

type ReconciliationConfig struct {
	Enabled   bool                      `yaml:"enabled"`
	Schedules []ReconciliationSchedule `yaml:"schedules"`
}

type ReconciliationSchedule struct {
	Name        string `yaml:"name"`
	Cron        string `yaml:"cron"`
	Type        string `yaml:"type"`
	Scope       string `yaml:"scope"`
	Description string `yaml:"description"`
}

type RiskControlConfig struct {
	MaxOrderAmount       float64 `yaml:"max_order_amount"`
	MaxDailyAmount       float64 `yaml:"max_daily_amount"`
	MaxCancelPerMinute   int     `yaml:"max_cancel_per_minute"`
	PriceDeviationThreshold float64 `yaml:"price_deviation_threshold"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
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

	if cfg.App.GRPCPort == 0 {
		cfg.App.GRPCPort = 50051
	}
	if cfg.App.HTTPPort == 0 {
		cfg.App.HTTPPort = 8080
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = 100
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 10
	}
	if cfg.Database.ConnMaxLifetime == 0 {
		cfg.Database.ConnMaxLifetime = 3600
	}
	if cfg.Redis.PoolSize == 0 {
		cfg.Redis.PoolSize = 100
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Database.Host == "" {
		return fmt.Errorf("数据库主机不能为空")
	}
	if c.Database.Name == "" {
		return fmt.Errorf("数据库名称不能为空")
	}
	if c.Redis.Host == "" {
		return fmt.Errorf("Redis主机不能为空")
	}
	return nil
}
