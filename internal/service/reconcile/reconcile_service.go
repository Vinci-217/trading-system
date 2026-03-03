package reconcile

import (
	"context"
	"fmt"
	"time"

	"github.com/stock-trading-system/internal/domain/account"
	"github.com/stock-trading-system/internal/domain/trade"
	"github.com/stock-trading-system/pkg/logger"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ReconcileService struct {
	db     *gorm.DB
	logger *logger.Logger
}

func NewReconcileService(db *gorm.DB, logger *logger.Logger) *ReconcileService {
	return &ReconcileService{
		db:     db,
		logger: logger,
	}
}

type ReconcileResult struct {
	UserID             string          `json:"user_id"`
	Symbol             string          `json:"symbol"`
	AccountBalance     decimal.Decimal `json:"account_balance"`
	ExpectedBalance    decimal.Decimal `json:"expected_balance"`
	PositionQuantity   int             `json:"position_quantity"`
	ExpectedPosition   int             `json:"expected_position"`
	FrozenBalance      decimal.Decimal `json:"frozen_balance"`
	ExpectedFrozen     decimal.Decimal `json:"expected_frozen"`
	HasDiscrepancy     bool            `json:"has_discrepancy"`
	DiscrepancyDetails []string        `json:"discrepancy_details"`
	ReconcileTime      time.Time       `json:"reconcile_time"`
}

type DailyReconcileReport struct {
	Date             string            `json:"date"`
	TotalAccounts    int               `json:"total_accounts"`
	MatchedAccounts  int               `json:"matched_accounts"`
	DiscrepancyCount int               `json:"discrepancy_count"`
	Results          []*ReconcileResult `json:"results"`
}

func (s *ReconcileService) ReconcileAccount(ctx context.Context, userID string) (*ReconcileResult, error) {
	result := &ReconcileResult{
		UserID:             userID,
		ReconcileTime:      time.Now(),
		DiscrepancyDetails: make([]string, 0),
	}

	var acc account.Account
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&acc).Error
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	result.AccountBalance = acc.CashBalance
	result.FrozenBalance = acc.FrozenBalance

	var fundFlows []account.FundFlow
	s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&fundFlows)

	expectedBalance := decimal.Zero
	for _, flow := range fundFlows {
		switch flow.FlowType {
		case account.FlowTypeDeposit:
			expectedBalance = expectedBalance.Add(flow.Amount)
		case account.FlowTypeWithdraw:
			expectedBalance = expectedBalance.Sub(flow.Amount)
		case account.FlowTypeDeduct:
			expectedBalance = expectedBalance.Sub(flow.Amount)
		case account.FlowTypeCredit:
			expectedBalance = expectedBalance.Add(flow.Amount)
		}
	}
	result.ExpectedBalance = expectedBalance

	if !result.AccountBalance.Equal(result.ExpectedBalance) {
		result.HasDiscrepancy = true
		result.DiscrepancyDetails = append(result.DiscrepancyDetails,
			fmt.Sprintf("资金余额不一致: 账户余额=%s, 预期余额=%s",
				result.AccountBalance.String(), result.ExpectedBalance.String()))
	}

	var frozenFlows []account.FundFlow
	s.db.WithContext(ctx).Where("user_id = ? AND flow_type IN ?", userID,
		[]int{account.FlowTypeFreeze, account.FlowTypeUnfreeze}).Find(&frozenFlows)

	expectedFrozen := decimal.Zero
	for _, flow := range frozenFlows {
		switch flow.FlowType {
		case account.FlowTypeFreeze:
			expectedFrozen = expectedFrozen.Add(flow.Amount)
		case account.FlowTypeUnfreeze:
			expectedFrozen = expectedFrozen.Sub(flow.Amount)
		}
	}
	result.ExpectedFrozen = expectedFrozen

	if !result.FrozenBalance.Equal(result.ExpectedFrozen) {
		result.HasDiscrepancy = true
		result.DiscrepancyDetails = append(result.DiscrepancyDetails,
			fmt.Sprintf("冻结金额不一致: 账户冻结=%s, 预期冻结=%s",
				result.FrozenBalance.String(), result.ExpectedFrozen.String()))
	}

	if result.HasDiscrepancy {
		s.saveDiscrepancy(ctx, result)
	}

	return result, nil
}

func (s *ReconcileService) ReconcilePosition(ctx context.Context, userID, symbol string) (*ReconcileResult, error) {
	result := &ReconcileResult{
		UserID:             userID,
		Symbol:             symbol,
		ReconcileTime:      time.Now(),
		DiscrepancyDetails: make([]string, 0),
	}

	var pos account.Position
	err := s.db.WithContext(ctx).Where("user_id = ? AND symbol = ?", userID, symbol).First(&pos).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			result.PositionQuantity = 0
			result.ExpectedPosition = 0
			return result, nil
		}
		return nil, fmt.Errorf("position not found: %w", err)
	}

	result.PositionQuantity = pos.Quantity

	var trades []trade.Trade
	s.db.WithContext(ctx).Where("user_id = ? AND symbol = ?", userID, symbol).Find(&trades)

	expectedPosition := 0
	for _, t := range trades {
		if t.Side == 1 {
			expectedPosition += t.Quantity
		} else {
			expectedPosition -= t.Quantity
		}
	}
	result.ExpectedPosition = expectedPosition

	if result.PositionQuantity != result.ExpectedPosition {
		result.HasDiscrepancy = true
		result.DiscrepancyDetails = append(result.DiscrepancyDetails,
			fmt.Sprintf("持仓数量不一致: 账户持仓=%d, 预期持仓=%d",
				result.PositionQuantity, result.ExpectedPosition))
	}

	if result.HasDiscrepancy {
		s.saveDiscrepancy(ctx, result)
	}

	return result, nil
}

func (s *ReconcileService) DailyReconcile(ctx context.Context, date string) (*DailyReconcileReport, error) {
	report := &DailyReconcileReport{
		Date:    date,
		Results: make([]*ReconcileResult, 0),
	}

	var accounts []account.Account
	s.db.WithContext(ctx).Find(&accounts)

	report.TotalAccounts = len(accounts)

	for _, acc := range accounts {
		result, err := s.ReconcileAccount(ctx, acc.UserID)
		if err != nil {
			s.logger.Errorw("failed to reconcile account", "user_id", acc.UserID, "error", err)
			continue
		}

		report.Results = append(report.Results, result)
		if result.HasDiscrepancy {
			report.DiscrepancyCount++
		} else {
			report.MatchedAccounts++
		}
	}

	s.logger.Info("daily reconcile completed",
		"date", date,
		"total_accounts", report.TotalAccounts,
		"matched", report.MatchedAccounts,
		"discrepancies", report.DiscrepancyCount)

	return report, nil
}

func (s *ReconcileService) saveDiscrepancy(ctx context.Context, result *ReconcileResult) error {
	details := ""
	for i, d := range result.DiscrepancyDetails {
		if i > 0 {
			details += "; "
		}
		details += d
	}

	discrepancy := &account.Discrepancy{
		UserID:     result.UserID,
		Symbol:     result.Symbol,
		Type:       "balance",
		Expected:   result.ExpectedBalance.String(),
		Actual:     result.AccountBalance.String(),
		Details:    details,
		Status:     1,
		CreatedAt:  time.Now(),
	}

	return s.db.WithContext(ctx).Create(discrepancy).Error
}

func (s *ReconcileService) GetDiscrepancies(ctx context.Context, userID string, startDate, endDate time.Time) ([]*account.Discrepancy, error) {
	var discrepancies []*account.Discrepancy

	query := s.db.WithContext(ctx).Model(&account.Discrepancy{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if !startDate.IsZero() {
		query = query.Where("created_at >= ?", startDate)
	}
	if !endDate.IsZero() {
		query = query.Where("created_at <= ?", endDate)
	}

	err := query.Order("created_at DESC").Find(&discrepancies).Error
	return discrepancies, err
}

func (s *ReconcileService) ResolveDiscrepancy(ctx context.Context, discrepancyID uint, resolution string) error {
	return s.db.WithContext(ctx).Model(&account.Discrepancy{}).
		Where("id = ?", discrepancyID).
		Updates(map[string]interface{}{
			"status":     2,
			"resolved_at": time.Now(),
			"resolution": resolution,
		}).Error
}

func (s *ReconcileService) ReconcileAllPositions(ctx context.Context) ([]*ReconcileResult, error) {
	var positions []account.Position
	s.db.WithContext(ctx).Find(&positions)

	results := make([]*ReconcileResult, 0)
	for _, pos := range positions {
		result, err := s.ReconcilePosition(ctx, pos.UserID, pos.Symbol)
		if err != nil {
			s.logger.Errorw("failed to reconcile position",
				"user_id", pos.UserID,
				"symbol", pos.Symbol,
				"error", err)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}
