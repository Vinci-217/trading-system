package repository

import (
	"context"
	"time"

	"stock_trader/backend/services/reconciliation-service/internal/domain/entity"
	"stock_trader/backend/services/reconciliation-service/internal/infrastructure/logger"
)

type DiscrepancyRepository struct {
	db    interface{}
	logger *logger.Logger
}

func NewDiscrepancyRepository(db interface{}, logger *logger.Logger) *DiscrepancyRepository {
	return &DiscrepancyRepository{
		db:    db,
		logger: logger,
	}
}

func (r *DiscrepancyRepository) Create(ctx context.Context, discrepancy *entity.Discrepancy) error {
	r.logger.Info("创建不一致记录",
		logger.String("id", discrepancy.ID),
		logger.String("type", string(discrepancy.Type)),
		logger.String("user_id", discrepancy.UserID))
	return nil
}

func (r *DiscrepancyRepository) GetByID(ctx context.Context, id string) (*entity.Discrepancy, error) {
	return &entity.Discrepancy{
		ID:             id,
		Type:           entity.DiscrepancyTypeFund,
		UserID:         "test-user",
		ExpectedValue:  decimal.Zero,
		ActualValue:    decimal.Zero,
		Difference:     decimal.Zero,
		DetectedAt:     time.Now(),
		Status:         entity.DiscrepancyStatusOpen,
		CreatedAt:      time.Now(),
	}, nil
}

func (r *DiscrepancyRepository) GetList(ctx context.Context, status string, discrepancyType string, limit int, offset int) ([]*entity.Discrepancy, error) {
	return []*entity.Discrepancy{}, nil
}

func (r *DiscrepancyRepository) Update(ctx context.Context, discrepancy *entity.Discrepancy) error {
	return nil
}

func (r *DiscrepancyRepository) Delete(ctx context.Context, id string) error {
	return nil
}

import "github.com/shopspring/decimal"
