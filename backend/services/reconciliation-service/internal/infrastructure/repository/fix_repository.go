package repository

import (
	"context"
	"time"

	"stock_trader/backend/services/reconciliation-service/internal/domain/entity"
	"stock_trader/backend/services/reconciliation-service/internal/infrastructure/logger"
)

type FixRecordRepository struct {
	db    interface{}
	logger *logger.Logger
}

func NewFixRecordRepository(db interface{}, logger *logger.Logger) *FixRecordRepository {
	return &FixRecordRepository{
		db:    db,
		logger: logger,
	}
}

func (r *FixRecordRepository) Create(ctx context.Context, fixRecord *entity.FixRecord) error {
	r.logger.Info("创建修复记录",
		logger.String("id", fixRecord.ID),
		logger.String("discrepancy_id", fixRecord.DiscrepancyID),
		logger.String("fix_type", fixRecord.FixType))
	return nil
}

func (r *FixRecordRepository) GetByID(ctx context.Context, id string) (*entity.FixRecord, error) {
	return &entity.FixRecord{
		ID:            id,
		DiscrepancyID: "test-discrepancy",
		FixType:       "ADJUST_BALANCE",
		FixDetails:    "Test fix",
		FixResult:     "SUCCESS",
		ExecutedBy:    "SYSTEM",
		ExecutedAt:    time.Now(),
	}, nil
}

func (r *FixRecordRepository) GetByDiscrepancyID(ctx context.Context, discrepancyID string) ([]*entity.FixRecord, error) {
	return []*entity.FixRecord{}, nil
}
