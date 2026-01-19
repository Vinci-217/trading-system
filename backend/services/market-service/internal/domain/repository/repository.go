package repository

import (
	"context"

	"stock_trader/backend/services/market-service/internal/domain/entity"
)

type QuoteRepository interface {
	Save(ctx context.Context, quote *entity.Quote) error
	Get(ctx context.Context, symbol string) (*entity.Quote, error)
	GetAll(ctx context.Context) ([]*entity.Quote, error)
}

type KLineRepository interface {
	Save(ctx context.Context, kline *entity.KLine) error
	Get(ctx context.Context, symbol string, interval string) (*entity.KLine, error)
	GetHistory(ctx context.Context, symbol string, interval string, limit int) ([]*entity.KLine, error)
}

type SymbolRepository interface {
	Save(ctx context.Context, symbol *entity.Symbol) error
	Get(ctx context.Context, symbol string) (*entity.Symbol, error)
	GetAll(ctx context.Context) ([]*entity.Symbol, error)
}
