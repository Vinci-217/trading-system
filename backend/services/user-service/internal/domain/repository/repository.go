package repository

import (
	"context"

	"stock_trader/user-service/internal/domain/entity"
)

type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	GetByID(ctx context.Context, id string) (*entity.User, error)
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	Update(ctx context.Context, user *entity.User) error
	Delete(ctx context.Context, id string) error
}

type AccountRepository interface {
	Create(ctx context.Context, account *entity.Account) error
	GetByID(ctx context.Context, id string) (*entity.Account, error)
	GetByUserID(ctx context.Context, userID string) ([]*entity.Account, error)
	Update(ctx context.Context, account *entity.Account) error
}
