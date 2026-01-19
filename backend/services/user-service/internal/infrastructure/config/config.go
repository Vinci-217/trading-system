package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Kafka    KafkaConfig    `yaml:"kafka"`
	JWT      JWTConfig      `yaml:"jwt"`
	Logging  LoggingConfig  `yaml:"logging"`
	GRPCPort int            `yaml:"grpc_port"`
	HTTPPort int            `yaml:"http_port"`
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
	Brokers []string `yaml:"brokers"`
}

type JWTConfig struct {
	Secret     string `yaml:"secret"`
	Expiration int    `yaml:"expiration"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
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
		cfg.GRPCPort = 5001
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8081
	}

	return &cfg, nil
}
