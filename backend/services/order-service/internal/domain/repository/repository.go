package repository

import (
	"context"

	"stock_trader/backend/services/order-service/internal/domain/entity"
)

type OrderRepository interface {
	Create(ctx context.Context, order *entity.Order) error
	Get(ctx context.Context, orderID string) (*entity.Order, error)
	GetForUpdate(ctx context.Context, orderID string) (*entity.Order, error)
	Update(ctx context.Context, order *entity.Order) error
	GetByUserID(ctx context.Context, userID string, status string, limit int) ([]*entity.Order, error)
	GetBySymbol(ctx context.Context, symbol string, status string, limit int) ([]*entity.Order, error)
}

type TCCTransactionRepository interface {
	Create(ctx context.Context, tx *entity.TCCTransaction) error
	Get(ctx context.Context, id string) (*entity.TCCTransaction, error)
	Update(ctx context.Context, tx *entity.TCCTransaction) error
	GetByOrderID(ctx context.Context, orderID string) (*entity.TCCTransaction, error)
}
