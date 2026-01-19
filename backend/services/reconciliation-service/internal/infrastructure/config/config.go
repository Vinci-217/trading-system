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
	Kafka         KafkaConfig         `yaml:"kafka"`
	Registry      RegistryConfig      `yaml:"registry"`
	Logging       LoggingConfig       `yaml:"logging"`
	Reconciliation ReconciliationConfig `yaml:"reconciliation"`
	Fix           FixConfig           `yaml:"fix"`
	Audit         AuditConfig         `yaml:"audit"`
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
	Brokers      []string `yaml:"brokers"`
	TopicPrefix  string   `yaml:"topic_prefix"`
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

type ReconciliationConfig struct {
	AutoReconcileInterval      int     `yaml:"auto_reconcile_interval"`
	DiscrepancyThreshold       float64 `yaml:"discrepancy_threshold"`
	MaxDiscrepanciesPerReport  int     `yaml:"max_discrepancies_per_report"`
}

func (r *ReconciliationConfig) GetAutoReconcileInterval() time.Duration {
	return time.Duration(r.AutoReconcileInterval) * time.Second
}

func (r *ReconciliationConfig) GetDiscrepancyThreshold() float64 {
	return r.DiscrepancyThreshold
}

type FixConfig struct {
	AutoFixEnabled bool    `yaml:"auto_fix_enabled"`
	RequireApproval bool   `yaml:"require_approval"`
	MaxFixAmount   float64 `yaml:"max_fix_amount"`
}

func (f *FixConfig) GetMaxFixAmount() float64 {
	return f.MaxFixAmount
}

type AuditConfig struct {
	Enabled       bool `yaml:"enabled"`
	RetentionDays int  `yaml:"retention_days"`
}

func (a *AuditConfig) GetRetentionDuration() time.Duration {
	return time.Duration(a.RetentionDays) * 24 * time.Hour
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
