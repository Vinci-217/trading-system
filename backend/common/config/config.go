package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Kafka    KafkaConfig    `yaml:"kafka"`
	Registry RegistryConfig `yaml:"registry"`
	Logging  LoggingConfig  `yaml:"logging"`
	RiskControl RiskControlConfig `yaml:"risk_control"`
}

type AppConfig struct {
	Name      string `yaml:"name"`
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Env       string `yaml:"env"`
	ServiceID string `yaml:"service_id"`
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
	TablePrefix     string `yaml:"table_prefix"`
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
	KeyPrefix string `yaml:"key_prefix"`
}

func (r *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type KafkaConfig struct {
	Brokers       []string       `yaml:"brokers"`
	ConsumerGroup string         `yaml:"consumer_group"`
	Producer      ProducerConfig `yaml:"producer"`
	Consumer      ConsumerConfig `yaml:"consumer"`
}

type ProducerConfig struct {
	MaxAttempts  int  `yaml:"max_attempts"`
	RequiredAcks int  `yaml:"required_acks"`
	FlushBytes   int  `yaml:"flush_bytes"`
	FlushMs      int  `yaml:"flush_ms"`
	Async        bool `yaml:"async"`
}

type ConsumerConfig struct {
	AutoOffsetReset string `yaml:"auto_offset_reset"`
	MaxPollRecords  int    `yaml:"max_poll_records"`
	SessionTimeout  int    `yaml:"session_timeout"`
}

type RegistryConfig struct {
	Addr     string `yaml:"addr"`
	Scheme   string `yaml:"scheme"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Output string `yaml:"output"`
	Format string `yaml:"format"`
}

type RiskControlConfig struct {
	MaxOrderAmount        float64 `yaml:"max_order_amount"`
	MaxDailyAmount        float64 `yaml:"max_daily_amount"`
	MaxCancelPerMinute    int     `yaml:"max_cancel_per_minute"`
	MaxOrdersPerSecond    int     `yaml:"max_orders_per_second"`
	PriceDeviationThreshold float64 `yaml:"price_deviation_threshold"`
	FrozenAccountThreshold float64 `yaml:"frozen_account_threshold"`
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

	cfg.setDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) setDefaults() {
	if c.App.Port == 0 {
		c.App.Port = 5000
	}
	if c.App.Env == "" {
		c.App.Env = "development"
	}
	if c.Database.MaxOpenConns == 0 {
		c.Database.MaxOpenConns = 100
	}
	if c.Database.MaxIdleConns == 0 {
		c.Database.MaxIdleConns = 10
	}
	if c.Database.ConnMaxLifetime == 0 {
		c.Database.ConnMaxLifetime = 3600
	}
	if c.Redis.PoolSize == 0 {
		c.Redis.PoolSize = 100
	}
	if c.RiskControl.MaxOrderAmount == 0 {
		c.RiskControl.MaxOrderAmount = 1000000
	}
	if c.RiskControl.MaxDailyAmount == 0 {
		c.RiskControl.MaxDailyAmount = 5000000
	}
	if c.RiskControl.MaxCancelPerMinute == 0 {
		c.RiskControl.MaxCancelPerMinute = 20
	}
	if c.RiskControl.MaxOrdersPerSecond == 0 {
		c.RiskControl.MaxOrdersPerSecond = 10
	}
	if c.RiskControl.PriceDeviationThreshold == 0 {
		c.RiskControl.PriceDeviationThreshold = 0.05
	}
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

func (c *Config) GetTableName(name string) string {
	if c.Database.TablePrefix != "" {
		return c.Database.TablePrefix + "_" + name
	}
	return name
}

func (c *Config) GetRedisKey(key string) string {
	if c.Redis.KeyPrefix != "" {
		return c.Redis.KeyPrefix + ":" + key
	}
	return key
}
