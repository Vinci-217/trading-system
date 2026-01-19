package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"stock_trader/backend/services/order-service/internal/infrastructure/config"

	_ "github.com/go-sql-driver/mysql"
)

type MySQL struct {
	*sql.DB
	config *config.DatabaseConfig
}

func NewMySQL(cfg *config.DatabaseConfig) (*MySQL, error) {
	db, err := sql.Open("mysql", cfg.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	return &MySQL{
		DB:     db,
		config: cfg,
	}, nil
}

func (m *MySQL) GetDB() *sql.DB {
	return m.DB
}

func (m *MySQL) HealthCheck(ctx context.Context) error {
	return m.PingContext(ctx)
}

func (m *MySQL) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return m.DB.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
}
