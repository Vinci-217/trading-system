package repository

import (
	"context"

	"stock_trader/backend/services/reconciliation-service/internal/domain/entity"
)

type DiscrepancyRepository interface {
	Create(ctx context.Context, discrepancy *entity.Discrepancy) error
	GetByID(ctx context.Context, id string) (*entity.Discrepancy, error)
	GetList(ctx context.Context, status string, discrepancyType string, limit int, offset int) ([]*entity.Discrepancy, error)
	Update(ctx context.Context, discrepancy *entity.Discrepancy) error
	Delete(ctx context.Context, id string) error
}

type ReportRepository interface {
	Create(ctx context.Context, report *entity.ReconciliationReport) error
	GetByID(ctx context.Context, id string) (*entity.ReconciliationReport, error)
	Update(ctx context.Context, report *entity.ReconciliationReport) error
	GetList(ctx context.Context, reportType string, limit int, offset int) ([]*entity.ReconciliationReport, error)
}

type FixRecordRepository interface {
	Create(ctx context.Context, fixRecord *entity.FixRecord) error
	GetByID(ctx context.Context, id string) (*entity.FixRecord, error)
	GetByDiscrepancyID(ctx context.Context, discrepancyID string) ([]*entity.FixRecord, error)
}

type AccountRepository interface {
	GetByUserID(ctx context.Context, userID string) (*Account, error)
	GetAll(ctx context.Context) ([]*Account, error)
}

type Account struct {
	UserID        string
	CashBalance   string
	FrozenBalance string
	Version       int64
}

type PositionRepository interface {
	GetByUserID(ctx context.Context, userID string) ([]*Position, error)
}

type Position struct {
	ID       string
	UserID   string
	Symbol   string
	Quantity int
}

type OrderRepository interface {
	GetPendingByUserID(ctx context.Context, userID string) ([]*Order, error)
	GetPendingByUserIDAndSymbol(ctx context.Context, userID string, symbol string) ([]*Order, error)
	GetFilledByUserIDAndSymbol(ctx context.Context, userID string, symbol string) ([]*Order, error)
}

type Order struct {
	ID             string
	UserID         string
	Symbol         string
	Side           string
	Quantity       int
	FilledQuantity int
	Price          string
	Status         string
}

type TradeRepository interface {
	GetPendingByUserID(ctx context.Context, userID string, startTime, endTime time.Time) ([]*Trade, error)
	GetByUserIDAndSymbol(ctx context.Context, userID string, symbol string, startTime, endTime time.Time) ([]*Trade, error)
}

type Trade struct {
	ID       string
	UserID   string
	Symbol   string
	Side     string
	Quantity int
	Price    string
}
