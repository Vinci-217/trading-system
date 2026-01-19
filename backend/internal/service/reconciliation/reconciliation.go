package reconciliation

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"stock_trader/internal/model"
	"stock_trader/internal/repository"
)

type Service struct {
	repo  *repository.Repository
	log   *zap.Logger
}

func NewService(repo *repository.Repository, logger *zap.Logger) *Service {
	return &Service{
		repo: repo,
		log:  logger,
	}
}

type IssueInfo struct {
	IssueID       string  `json:"issue_id"`
	ReportID      string  `json:"report_id"`
	UserID        *int64  `json:"user_id,omitempty"`
	Symbol        string  `json:"symbol,omitempty"`
	Category      string  `json:"category"`
	Severity      string  `json:"severity"`
	Description   string  `json:"description"`
	ExpectedValue float64 `json:"expected_value,omitempty"`
	ActualValue   float64 `json:"actual_value,omitempty"`
	Difference    float64 `json:"difference,omitempty"`
	Status        string  `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type ReportInfo struct {
	ReportID       string    `json:"report_id"`
	CheckTime      time.Time `json:"check_time"`
	CheckType      string    `json:"check_type"`
	Scope          string    `json:"scope"`
	TotalUsers     int       `json:"total_users"`
	IssueUsers     int       `json:"issue_users"`
	CriticalCount  int       `json:"critical_count"`
	HighCount      int       `json:"high_count"`
	MediumCount    int       `json:"medium_count"`
	LowCount       int       `json:"low_count"`
	AutoRepaired   int       `json:"auto_repaired"`
	DurationMs     int       `json:"duration_ms"`
}

func (s *Service) RunFullReconciliation(ctx context.Context) (*model.ReconciliationRecord, error) {
	startTime := time.Now()
	reportID := uuid.New().String()

	accounts, err := s.repo.GetAllAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取账户列表失败: %w", err)
	}

	allPositions, err := s.repo.GetAllPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取持仓列表失败: %w", err)
	}

	trades, err := s.repo.GetAllTrades(ctx, startTime.Add(-24*time.Hour), startTime)
	if err != nil {
		return nil, fmt.Errorf("获取成交记录失败: %w", err)
	}

	var issues []model.ReconciliationIssue
	criticalCount, highCount, mediumCount, lowCount := 0, 0, 0, 0
	checkedUsers := 0
	issueUsers := make(map[int64]bool)

	for _, account := range accounts {
		checkedUsers++
		userID := account.UserID

		positionMap := make(map[string]*model.Position)
		for _, pos := range allPositions {
			if pos.UserID == userID {
				positionMap[pos.Symbol] = pos
			}
		}

		userTrades := make([]*model.Trade, 0)
		for _, trade := range trades {
			if trade.UserID == userID {
				userTrades = append(userTrades, trade)
			}
		}

		fundIssues := s.checkFundReconciliation(account, userTrades)
		issues = append(issues, fundIssues...)

		for _, issue := range fundIssues {
			if issue.UserID != nil {
				issueUsers[*issue.UserID] = true
			}
			switch issue.Severity {
			case model.IssueSeverityCritical:
				criticalCount++
			case model.IssueSeverityHigh:
				highCount++
			case model.IssueSeverityMedium:
				mediumCount++
			case model.IssueSeverityLow:
				lowCount++
			}
		}

		for _, trade := range userTrades {
			position, exists := positionMap[trade.Symbol]
			if exists {
				delete(positionMap, trade.Symbol)

				if trade.Side == model.OrderSideBuy {
					position.Quantity += trade.Quantity
				} else {
					position.Quantity -= trade.Quantity
				}
			}
		}

		for symbol, position := range positionMap {
			if position.Quantity != 0 {
				issues = append(issues, model.ReconciliationIssue{
					IssueID:     uuid.New().String(),
					ReportID:    reportID,
					UserID:      &userID,
					Symbol:      symbol,
					Category:    model.IssueCategoryQuantityMismatch,
					Severity:    model.IssueSeverityHigh,
					Description: fmt.Sprintf("持仓数量不匹配: 记录=%d", position.Quantity),
					Status:      model.IssueStatusPending,
				})
				highCount++
				issueUsers[userID] = true
			}
		}
	}

	if len(issues) > 0 {
		for i := range issues {
			issues[i].ReportID = reportID
		}
		if err := s.repo.CreateReconciliationIssues(ctx, issues); err != nil {
			s.log.Error("保存对账差异失败", zap.Error(err))
		}
	}

	duration := time.Since(startTime)
	record := &model.ReconciliationRecord{
		ReportID:      reportID,
		CheckTime:     startTime,
		CheckType:     model.ReconCheckTypeFull,
		Scope:         "ALL",
		TotalUsers:    len(accounts),
		CheckedUsers:  checkedUsers,
		PassedUsers:   len(accounts) - len(issueUsers),
		IssueUsers:    len(issueUsers),
		CriticalCount: criticalCount,
		HighCount:     highCount,
		MediumCount:   mediumCount,
		LowCount:      lowCount,
		DurationMs:    int(duration.Milliseconds()),
	}

	if err := s.repo.CreateReconciliationRecord(ctx, record); err != nil {
		s.log.Error("保存对账记录失败", zap.Error(err))
	}

	return record, nil
}

func (s *Service) checkFundReconciliation(account *model.Account, trades []*model.Trade) []model.ReconciliationIssue {
	var issues []model.ReconciliationIssue

	expectedBalance := 100000.0
	expectedFrozen := 0.0

	for _, trade := range trades {
		if trade.Side == model.OrderSideBuy {
			expectedBalance -= trade.Amount
		} else {
			expectedBalance += trade.Amount
		}
	}

	balanceDiff := account.CashBalance - expectedBalance
	if math.Abs(balanceDiff) > 0.01 {
		userID := account.UserID
		issues = append(issues, model.ReconciliationIssue{
			IssueID:       uuid.New().String(),
			ReportID:      "",
			UserID:        &userID,
			Category:      model.IssueCategoryCashMismatch,
			Severity:      model.IssueSeverityHigh,
			Description:   fmt.Sprintf("现金余额不匹配: 记录=%f, 计算=%f", account.CashBalance, expectedBalance),
			ExpectedValue: expectedBalance,
			ActualValue:   account.CashBalance,
			Difference:    balanceDiff,
			Status:        model.IssueStatusPending,
		})
	}

	if account.CashBalance < 0 {
		userID := account.UserID
		issues = append(issues, model.ReconciliationIssue{
			IssueID:       uuid.New().String(),
			ReportID:      "",
			UserID:        &userID,
			Category:      model.IssueCategoryNegativeBalance,
			Severity:      model.IssueSeverityCritical,
			Description:   fmt.Sprintf("现金余额为负数: %f", account.CashBalance),
			ActualValue:   account.CashBalance,
			Status:        model.IssueStatusPending,
		})
	}

	totalFee := 0.0
	for _, trade := range trades {
		totalFee += trade.Fee
	}

	return issues
}

func (s *Service) RunPositionReconciliation(ctx context.Context) (*model.ReconciliationRecord, error) {
	startTime := time.Now()
	reportID := uuid.New().String()

	positions, err := s.repo.GetAllPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取持仓列表失败: %w", err)
	}

	var issues []model.ReconciliationIssue
	criticalCount, highCount, mediumCount, lowCount := 0, 0, 0, 0

	for _, pos := range positions {
		if pos.AvailableQuantity < 0 {
			issues = append(issues, model.ReconciliationIssue{
				IssueID:       uuid.New().String(),
				ReportID:      reportID,
				UserID:        &pos.UserID,
				Symbol:        pos.Symbol,
				Category:      model.IssueCategoryNegativeAvailable,
				Severity:      model.IssueSeverityCritical,
				Description:   fmt.Sprintf("可用股份为负数: %d", pos.AvailableQuantity),
				ActualValue:   float64(pos.AvailableQuantity),
				Status:        model.IssueStatusPending,
			})
			criticalCount++
		}

		if pos.Quantity < pos.AvailableQuantity {
			issues = append(issues, model.ReconciliationIssue{
				IssueID:       uuid.New().String(),
				ReportID:      reportID,
				UserID:        &pos.UserID,
				Symbol:        pos.Symbol,
				Category:      model.IssueCategoryQuantityMismatch,
				Severity:      model.IssueSeverityHigh,
				Description:   fmt.Sprintf("持仓数量小于可用数量: 持仓=%d, 可用=%d", pos.Quantity, pos.AvailableQuantity),
				Status:        model.IssueStatusPending,
			})
			highCount++
		}
	}

	if len(issues) > 0 {
		for i := range issues {
			issues[i].ReportID = reportID
		}
		if err := s.repo.CreateReconciliationIssues(ctx, issues); err != nil {
			s.log.Error("保存对账差异失败", zap.Error(err))
		}
	}

	duration := time.Since(startTime)
	uniqueUsers := make(map[int64]bool)
	for _, issue := range issues {
		if issue.UserID != nil {
			uniqueUsers[*issue.UserID] = true
		}
	}

	record := &model.ReconciliationRecord{
		ReportID:      reportID,
		CheckTime:     startTime,
		CheckType:     model.ReconCheckTypePosition,
		Scope:         "ALL",
		TotalUsers:    len(uniqueUsers),
		CheckedUsers:  len(positions),
		PassedUsers:   len(positions) - len(issues),
		IssueUsers:    len(uniqueUsers),
		CriticalCount: criticalCount,
		HighCount:     highCount,
		MediumCount:   mediumCount,
		LowCount:      lowCount,
		DurationMs:    int(duration.Milliseconds()),
	}

	if err := s.repo.CreateReconciliationRecord(ctx, record); err != nil {
		s.log.Error("保存对账记录失败", zap.Error(err))
	}

	return record, nil
}

func (s *Service) GetReports(ctx context.Context, startTime, endTime time.Time) ([]*ReportInfo, error) {
	records, err := s.repo.GetReconciliationRecords(ctx, startTime, endTime)
	if err != nil {
		return nil, err
	}

	result := make([]*ReportInfo, 0, len(records))
	for _, record := range records {
		result = append(result, &ReportInfo{
			ReportID:      record.ReportID,
			CheckTime:     record.CheckTime,
			CheckType:     string(record.CheckType),
			Scope:         record.Scope,
			TotalUsers:    record.TotalUsers,
			IssueUsers:    record.IssueUsers,
			CriticalCount: record.CriticalCount,
			HighCount:     record.HighCount,
			MediumCount:   record.MediumCount,
			LowCount:      record.LowCount,
			AutoRepaired:  record.AutoRepaired,
			DurationMs:    record.DurationMs,
		})
	}

	return result, nil
}

func (s *Service) GetIssues(ctx context.Context, reportID, status string) ([]*IssueInfo, error) {
	dbIssues, err := s.repo.GetReconciliationIssues(ctx, reportID, status)
	if err != nil {
		return nil, err
	}

	result := make([]*IssueInfo, 0, len(dbIssues))
	for _, issue := range dbIssues {
		result = append(result, &IssueInfo{
			IssueID:       issue.IssueID,
			ReportID:      issue.ReportID,
			UserID:        issue.UserID,
			Symbol:        issue.Symbol,
			Category:      string(issue.Category),
			Severity:      string(issue.Severity),
			Description:   issue.Description,
			ExpectedValue: issue.ExpectedValue,
			ActualValue:   issue.ActualValue,
			Difference:    issue.Difference,
			Status:        string(issue.Status),
			CreatedAt:     issue.CreatedAt,
		})
	}

	return result, nil
}

func (s *Service) AutoRepairIssue(ctx context.Context, issueID string) error {
	issues, err := s.repo.GetReconciliationIssues(ctx, issueID, "")
	if err != nil || len(issues) == 0 {
		return fmt.Errorf("问题不存在")
	}

	issue := issues[0]

	if issue.Category == model.IssueCategoryCashMismatch && math.Abs(issue.Difference) < 0.01 {
		issue.Status = model.IssueStatusAutoRepaired
		issue.RepairedAt = timePtr(time.Now())
		issue.RepairedBy = "system"
		if err := s.repo.UpdateReconciliationIssue(ctx, issue); err != nil {
			return err
		}
	}

	return nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}
