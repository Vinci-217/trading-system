package repository

import (
	"context"

	"stock_trader/matching-service/internal/domain/entity"
)

type OrderBookRepository interface {
	GetOrCreate(ctx context.Context, symbol string) (*entity.OrderBook, error)
	SaveOrderBook(ctx context.Context, orderBook *entity.OrderBook) error
	GetOrderBook(ctx context.Context, symbol string) (*entity.OrderBook, error)
	SaveTrade(ctx context.Context, trade *entity.Trade) error
	GetTrades(ctx context.Context, symbol string, limit int) ([]*entity.Trade, error)
}

type TradeRepository interface {
	Save(ctx context.Context, trade *entity.Trade) error
	GetBySymbol(ctx context.Context, symbol string, limit int) ([]*entity.Trade, error)
	GetByOrderID(ctx context.Context, orderID string) ([]*entity.Trade, error)
}
