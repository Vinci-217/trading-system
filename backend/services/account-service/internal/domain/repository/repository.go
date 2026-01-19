package repository

import (
	"context"

	"stock_trader/backend/services/account-service/internal/domain/entity"
)

type AccountRepository interface {
	Create(ctx context.Context, account *entity.Account) error
	Get(ctx context.Context, userID string) (*entity.Account, error)
	GetForUpdate(ctx context.Context, userID string) (*entity.Account, error)
	Update(ctx context.Context, account *entity.Account) error
	GetAll(ctx context.Context) ([]*entity.Account, error)
	Lock(ctx context.Context, userID string) error
	Unlock(ctx context.Context, userID string) error
}

type PositionRepository interface {
	Create(ctx context.Context, position *entity.Position) error
	Get(ctx context.Context, userID string, symbol string) (*entity.Position, error)
	GetForUpdate(ctx context.Context, userID string, symbol string) (*entity.Position, error)
	Update(ctx context.Context, position *entity.Position) error
	GetByUserID(ctx context.Context, userID string) ([]*entity.Position, error)
}

type FundLockRepository interface {
	Create(ctx context.Context, lock *entity.FundLock) error
	Get(ctx context.Context, id string) (*entity.FundLock, error)
	GetByOrderID(ctx context.Context, orderID string) (*entity.FundLock, error)
	Update(ctx context.Context, lock *entity.FundLock) error
	Delete(ctx context.Context, id string) error
	GetExpired(ctx context.Context) ([]*entity.FundLock, error)
}

type TCCTransactionRepository interface {
	Create(ctx context.Context, tx *entity.TCCTransaction) error
	Get(ctx context.Context, id string) (*entity.TCCTransaction, error)
	GetByOrderID(ctx context.Context, orderID string) (*entity.TCCTransaction, error)
	Update(ctx context.Context, tx *entity.TCCTransaction) error
}
