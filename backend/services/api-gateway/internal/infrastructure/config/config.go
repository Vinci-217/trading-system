package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App           AppConfig           `yaml:"app"`
	Database      DatabaseConfig      `yaml:"database"`
	Redis         RedisConfig         `yaml:"redis"`
	Registry      RegistryConfig      `yaml:"registry"`
	Logging       LoggingConfig       `yaml:"logging"`
	JWT           JWTConfig           `yaml:"jwt"`
	RateLimit     RateLimitConfig     `yaml:"rate_limit"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	Services      ServicesConfig      `yaml:"services"`
}

type AppConfig struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
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

type RegistryConfig struct {
	Type    string `yaml:"type"`
	Address string `yaml:"address"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

type JWTConfig struct {
	Secret     string `yaml:"secret"`
	Expiration int    `yaml:"expiration"`
}

func (j *JWTConfig) GetExpirationDuration() time.Duration {
	return time.Duration(j.Expiration) * time.Second
}

type RateLimitConfig struct {
	WindowSeconds int `yaml:"window_seconds"`
	MaxRequests   int `yaml:"max_requests"`
	BurstSize     int `yaml:"burst_size"`
}

func (r *RateLimitConfig) GetWindow() time.Duration {
	return time.Duration(r.WindowSeconds) * time.Second
}

type CircuitBreakerConfig struct {
	MaxRequests    uint32 `yaml:"max_requests"`
	IntervalSeconds int   `yaml:"interval_seconds"`
	TimeoutSeconds  int   `yaml:"timeout_seconds"`
}

func (c *CircuitBreakerConfig) GetInterval() time.Duration {
	return time.Duration(c.IntervalSeconds) * time.Second
}

func (c *CircuitBreakerConfig) GetTimeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

type ServicesConfig struct {
	UserService           ServiceConfig `yaml:"user_service"`
	OrderService          ServiceConfig `yaml:"order_service"`
	AccountService        ServiceConfig `yaml:"account_service"`
	MarketService         ServiceConfig `yaml:"market_service"`
	ReconciliationService ServiceConfig `yaml:"reconciliation_service"`
}

type ServiceConfig struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Timeout int    `yaml:"timeout"`
}

func (s *ServiceConfig) GetAddr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
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
