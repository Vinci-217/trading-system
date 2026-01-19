package service

import (
	"context"
	"fmt"
	"time"

	"stock_trader/backend/services/reconciliation-service/internal/domain/entity"
	"stock_trader/backend/services/reconciliation-service/internal/domain/repository"
	"stock_trader/backend/services/reconciliation-service/internal/infrastructure/config"
	"stock_trader/backend/services/reconciliation-service/internal/infrastructure/logger"

	"github.com/shopspring/decimal"
	"github.com/google/uuid"
)

type ReconciliationDomainService struct {
	cfg            *config.Config
	logger         *logger.Logger
	discrepancyRepo repository.DiscrepancyRepository
	reportRepo     repository.ReportRepository
	fixRepo        repository.FixRecordRepository
	accountRepo    repository.AccountRepository
	positionRepo   repository.PositionRepository
	orderRepo      repository.OrderRepository
	tradeRepo      repository.TradeRepository
}

func NewReconciliationDomainService(
	cfg *config.Config,
	logger *logger.Logger,
	discrepancyRepo repository.DiscrepancyRepository,
	reportRepo repository.ReportRepository,
	fixRepo repository.FixRecordRepository,
) *ReconciliationDomainService {
	return &ReconciliationDomainService{
		cfg:            cfg,
		logger:         logger,
		discrepancyRepo: discrepancyRepo,
		reportRepo:     reportRepo,
		fixRepo:        fixRepo,
	}
}

func (s *ReconciliationDomainService) ReconcileFunds(ctx context.Context, userID string) (*entity.FundReconciliationResult, error) {
	account, err := s.accountRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取账户失败: %w", err)
	}

	pendingOrders, err := s.orderRepo.GetPendingByUserID(ctx, userID)
	if err != nil {
		pendingOrders = nil
	}

	pendingTrades, err := s.tradeRepo.GetPendingByUserID(ctx, userID, time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		pendingTrades = nil
	}

	expectedBalance := account.CashBalance.Add(account.FrozenBalance)

	for _, order := range pendingOrders {
		orderAmount := order.Price.Mul(decimal.NewFromInt(int64(order.Quantity)))
		if order.Side == "BUY" {
			expectedBalance = expectedBalance.Add(orderAmount)
		} else {
			expectedBalance = expectedBalance.Sub(orderAmount)
		}
	}

	for _, trade := range pendingTrades {
		tradeAmount := trade.Price.Mul(decimal.NewFromInt(int64(trade.Quantity)))
		if trade.Side == "BUY" {
			expectedBalance = expectedBalance.Add(tradeAmount)
		} else {
			expectedBalance = expectedBalance.Sub(tradeAmount)
		}
	}

	actualBalance := account.CashBalance.Add(account.FrozenBalance)
	discrepancy := actualBalance.Sub(expectedBalance)

	threshold := decimal.NewFromFloat(s.cfg.Reconciliation.DiscrepancyThreshold)
	isConsistent := discrepancy.Abs().LessThan(threshold)

	result := &entity.FundReconciliationResult{
		UserID:          userID,
		ExpectedBalance: expectedBalance,
		ActualBalance:   actualBalance,
		FrozenBalance:   account.FrozenBalance,
		Discrepancy:     discrepancy,
		PendingOrders:   len(pendingOrders),
		PendingTrades:   len(pendingTrades),
		IsConsistent:    isConsistent,
	}

	if !isConsistent {
		disc := entity.NewDiscrepancy(
			entity.DiscrepancyTypeFund,
			userID,
			"",
			expectedBalance,
			actualBalance,
		)
		disc.ID = uuid.New().String()

		if err := s.discrepancyRepo.Create(ctx, disc); err != nil {
			s.logger.Error("记录不一致失败", logger.Error(err))
		}

		s.logger.Warn("发现资金不一致",
			logger.String("user_id", userID),
			logger.Decimal("expected", expectedBalance),
			logger.Decimal("actual", actualBalance),
			logger.Decimal("discrepancy", discrepancy))
	}

	s.logger.Info("资金对账完成",
		logger.String("user_id", userID),
		logger.Bool("consistent", isConsistent),
		logger.Decimal("discrepancy", discrepancy))

	return result, nil
}

func (s *ReconciliationDomainService) ReconcilePositions(ctx context.Context, userID string, symbol string) (*entity.PositionReconciliationResult, error) {
	positions, err := s.positionRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	pendingOrders, err := s.orderRepo.GetPendingByUserIDAndSymbol(ctx, userID, symbol)
	if err != nil {
		pendingOrders = nil
	}

	var actualPosition int
	for _, pos := range positions {
		if pos.Symbol == symbol {
			actualPosition = pos.Quantity
			break
		}
	}

	orderImpact := 0
	for _, order := range pendingOrders {
		if order.Side == "BUY" {
			orderImpact += order.Quantity
		} else {
			orderImpact -= order.Quantity
		}
	}

	expectedPosition := actualPosition + orderImpact
	discrepancy := expectedPosition - actualPosition

	result := &entity.PositionReconciliationResult{
		UserID:           userID,
		Symbol:           symbol,
		ExpectedPosition: expectedPosition,
		ActualPosition:   actualPosition,
		Discrepancy:      discrepancy,
		PendingOrders:    len(pendingOrders),
		IsConsistent:     discrepancy == 0,
	}

	if discrepancy != 0 {
		disc := entity.NewDiscrepancy(
			entity.DiscrepancyTypePosition,
			userID,
			symbol,
			decimal.NewFromInt(int64(expectedPosition)),
			decimal.NewFromInt(int64(actualPosition)),
		)
		disc.ID = uuid.New().String()

		if err := s.discrepancyRepo.Create(ctx, disc); err != nil {
			s.logger.Error("记录不一致失败", logger.Error(err))
		}

		s.logger.Warn("发现持仓不一致",
			logger.String("user_id", userID),
			logger.String("symbol", symbol),
			logger.Int("expected", expectedPosition),
			logger.Int("actual", actualPosition))
	}

	return result, nil
}

func (s *ReconciliationDomainService) ReconcileTrades(ctx context.Context, userID string, symbol string, startTime time.Time, endTime time.Time) (*entity.TradeReconciliationResult, error) {
	trades, err := s.tradeRepo.GetByUserIDAndSymbol(ctx, userID, symbol, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("获取成交记录失败: %w", err)
	}

	orders, err := s.orderRepo.GetFilledByUserIDAndSymbol(ctx, userID, symbol)
	if err != nil {
		orders = nil
	}

	tradeVolume := 0
	tradeAmount := decimal.Zero
	for _, trade := range trades {
		tradeVolume += trade.Quantity
		tradeAmount = tradeAmount.Add(trade.Price.Mul(decimal.NewFromInt(int64(trade.Quantity))))
	}

	orderVolume := 0
	for _, order := range orders {
		orderVolume += order.FilledQuantity
	}

	volumeDiscrepancy := tradeVolume - orderVolume

	result := &entity.TradeReconciliationResult{
		UserID:            userID,
		Symbol:            symbol,
		TradeCount:        len(trades),
		OrderCount:        orderVolume,
		TradeVolume:       tradeVolume,
		OrderVolume:       orderVolume,
		VolumeDiscrepancy: volumeDiscrepancy,
		AmountDiscrepancy: tradeAmount,
		IsConsistent:      volumeDiscrepancy == 0,
	}

	if volumeDiscrepancy != 0 {
		disc := entity.NewDiscrepancy(
			entity.DiscrepancyTypeTrade,
			userID,
			symbol,
			decimal.NewFromInt(int64(orderVolume)),
			decimal.NewFromInt(int64(tradeVolume)),
		)
		disc.ID = uuid.New().String()

		if err := s.discrepancyRepo.Create(ctx, disc); err != nil {
			s.logger.Error("记录不一致失败", logger.Error(err))
		}

		s.logger.Warn("发现交易不一致",
			logger.String("user_id", userID),
			logger.String("symbol", symbol),
			logger.Int("trade_volume", tradeVolume),
			logger.Int("order_volume", orderVolume))
	}

	return result, nil
}

func (s *ReconciliationDomainService) GetDiscrepancies(ctx context.Context, status string, discrepancyType string, limit int, offset int) ([]*entity.Discrepancy, error) {
	return s.discrepancyRepo.GetList(ctx, status, discrepancyType, limit, offset)
}

func (s *ReconciliationDomainService) FixDiscrepancy(ctx context.Context, discrepancyID string, fixType string, notes string, executedBy string) (*entity.FixRecord, error) {
	discrepancy, err := s.discrepancyRepo.GetByID(ctx, discrepancyID)
	if err != nil {
		return nil, entity.ErrDiscrepancyNotFound
	}

	if discrepancy.Status == entity.DiscrepancyStatusResolved {
		return nil, fmt.Errorf("不一致已解决")
	}

	if s.cfg.Fix.AutoFixEnabled {
		if discrepancy.Difference.Abs().GreaterThan(decimal.NewFromFloat(s.cfg.Fix.MaxFixAmount)) {
			return nil, entity.ErrAmountTooLarge
		}
	} else {
		if s.cfg.Fix.RequireApproval {
			return nil, entity.ErrApprovalRequired
		}
	}

	fixRecord := entity.NewFixRecord(discrepancyID, fixType, notes, executedBy)

	if err := s.fixRepo.Create(ctx, fixRecord); err != nil {
		fixRecord.FixResult = "FAILED"
		fixRecord.ErrorMessage = err.Error()
		return fixRecord, err
	}

	discrepancy.Resolve(notes)
	if err := s.discrepancyRepo.Update(ctx, discrepancy); err != nil {
		return fixRecord, err
	}

	s.logger.Info("不一致已修复",
		logger.String("discrepancy_id", discrepancyID),
		logger.String("fix_type", fixType),
		logger.String("executed_by", executedBy))

	return fixRecord, nil
}

func (s *ReconciliationDomainService) RunFullReconciliation(ctx context.Context) (*entity.ReconciliationReport, error) {
	report := entity.NewReconciliationReport(entity.ReportTypeFull, "ALL")
	report.ID = uuid.New().String()

	if err := s.reportRepo.Create(ctx, report); err != nil {
		return nil, fmt.Errorf("创建报告失败: %w", err)
	}

	accounts, err := s.accountRepo.GetAll(ctx)
	if err != nil {
		report.Fail()
		s.reportRepo.Update(ctx, report)
		return nil, fmt.Errorf("获取账户列表失败: %w", err)
	}

	var totalDiscrepancies int

	for _, account := range accounts {
		result, err := s.ReconcileFunds(ctx, account.UserID)
		if err != nil {
			continue
		}

		report.CheckedAccounts++

		if !result.IsConsistent {
			totalDiscrepancies++
		}
	}

	report.DiscrepanciesFound = totalDiscrepancies
	report.Complete()

	if err := s.reportRepo.Update(ctx, report); err != nil {
		s.logger.Error("更新报告失败", logger.Error(err))
	}

	s.logger.Info("全量对账完成",
		logger.Int("checked_accounts", report.CheckedAccounts),
		logger.Int("discrepancies", report.DiscrepanciesFound),
		logger.String("duration", report.EndTime.Sub(report.StartTime).String()))

	return report, nil
}
