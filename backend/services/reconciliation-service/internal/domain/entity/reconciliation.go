package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

type Discrepancy struct {
	ID             string          `json:"id" db:"id"`
	Type           DiscrepancyType `json:"type" db:"type"`
	UserID         string          `json:"user_id" db:"user_id"`
	Symbol         string          `json:"symbol" db:"symbol,omitempty"`
	ExpectedValue  decimal.Decimal `json:"expected_value" db:"expected_value"`
	ActualValue    decimal.Decimal `json:"actual_value" db:"actual_value"`
	Difference     decimal.Decimal `json:"difference" db:"difference"`
	DetectedAt     time.Time       `json:"detected_at" db:"detected_at"`
	Status         DiscrepancyStatus `json:"status" db:"status"`
	ResolvedAt     *time.Time      `json:"resolved_at" db:"resolved_at,omitempty"`
	Resolution     string          `json:"resolution" db:"resolution,omitempty"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
}

type DiscrepancyType string

const (
	DiscrepancyTypeFund     DiscrepancyType = "FUND"
	DiscrepancyTypePosition DiscrepancyType = "POSITION"
	DiscrepancyTypeTrade    DiscrepancyType = "TRADE"
)

type DiscrepancyStatus string

const (
	DiscrepancyStatusOpen     DiscrepancyStatus = "OPEN"
	DiscrepancyStatusResolved DiscrepancyStatus = "RESOLVED"
	DiscrepancyStatusIgnored  DiscrepancyStatus = "IGNORED"
)

func NewDiscrepancy(discrepancyType DiscrepancyType, userID string, symbol string, expected decimal.Decimal, actual decimal.Decimal) *Discrepancy {
	difference := actual.Sub(expected)
	return &Discrepancy{
		ID:            "",
		Type:          discrepancyType,
		UserID:        userID,
		Symbol:        symbol,
		ExpectedValue: expected,
		ActualValue:   actual,
		Difference:    difference,
		DetectedAt:    time.Now(),
		Status:        DiscrepancyStatusOpen,
		CreatedAt:     time.Now(),
	}
}

func (d *Discrepancy) IsFundType() bool {
	return d.Type == DiscrepancyTypeFund
}

func (d *Discrepancy) IsPositionType() bool {
	return d.Type == DiscrepancyTypePosition
}

func (d *Discrepancy) IsTradeType() bool {
	return d.Type == DiscrepancyTypeTrade
}

func (d *Discrepancy) IsSignificant(threshold decimal.Decimal) bool {
	return d.Difference.Abs().GreaterThan(threshold)
}

func (d *Discrepancy) Resolve(resolution string) {
	d.Status = DiscrepancyStatusResolved
	d.ResolvedAt = new(time.Time)
	*d.ResolvedAt = time.Now()
	d.Resolution = resolution
}

func (d *Discrepancy) Ignore() {
	d.Status = DiscrepancyStatusIgnored
}

type ReconciliationReport struct {
	ID                  string          `json:"id" db:"id"`
	ReportType          ReportType      `json:"report_type" db:"report_type"`
	Scope               string          `json:"scope" db:"scope"`
	StartTime           time.Time       `json:"start_time" db:"start_time"`
	EndTime             time.Time       `json:"end_time" db:"end_time"`
	TotalAccounts       int             `json:"total_accounts" db:"total_accounts"`
	CheckedAccounts     int             `json:"checked_accounts" db:"checked_accounts"`
	DiscrepanciesFound  int             `json:"discrepancies_found" db:"discrepancies_found"`
	DiscrepanciesResolved int           `json:"discrepancies_resolved" db:"discrepancies_resolved"`
	Status              ReportStatus    `json:"status" db:"status"`
	ReportData          string          `json:"report_data" db:"report_data"`
	CreatedAt           time.Time       `json:"created_at" db:"created_at"`
	CompletedAt         *time.Time      `json:"completed_at" db:"completed_at,omitempty"`
}

type ReportType string

const (
	ReportTypeFund     ReportType = "FUND"
	ReportTypePosition ReportType = "POSITION"
	ReportTypeTrade    ReportType = "TRADE"
	ReportTypeFull     ReportType = "FULL"
)

type ReportStatus string

const (
	ReportStatusRunning   ReportStatus = "RUNNING"
	ReportStatusCompleted ReportStatus = "COMPLETED"
	ReportStatusFailed    ReportStatus = "FAILED"
)

func NewReconciliationReport(reportType ReportType, scope string) *ReconciliationReport {
	now := time.Now()
	return &ReconciliationReport{
		ID:         "",
		ReportType: reportType,
		Scope:      scope,
		StartTime:  now,
		EndTime:    now,
		Status:     ReportStatusRunning,
		CreatedAt:  now,
	}
}

func (r *ReconciliationReport) Complete() {
	r.Status = ReportStatusCompleted
	now := time.Now()
	r.CompletedAt = &now
	r.EndTime = now
}

func (r *ReconciliationReport) Fail() {
	r.Status = ReportStatusFailed
	now := time.Now()
	r.CompletedAt = &now
	r.EndTime = now
}

type FixRecord struct {
	ID            string    `json:"id" db:"id"`
	DiscrepancyID string    `json:"discrepancy_id" db:"discrepancy_id"`
	FixType       string    `json:"fix_type" db:"fix_type"`
	FixDetails    string    `json:"fix_details" db:"fix_details"`
	FixResult     string    `json:"fix_result" db:"fix_result"`
	ErrorMessage  string    `json:"error_message" db:"error_message,omitempty"`
	ExecutedBy    string    `json:"executed_by" db:"executed_by"`
	ExecutedAt    time.Time `json:"executed_at" db:"executed_at"`
}

func NewFixRecord(discrepancyID string, fixType string, fixDetails string, executedBy string) *FixRecord {
	return &FixRecord{
		ID:            "",
		DiscrepancyID: discrepancyID,
		FixType:       fixType,
		FixDetails:    fixDetails,
		FixResult:     "SUCCESS",
		ExecutedBy:    executedBy,
		ExecutedAt:    time.Now(),
	}
}

type FundReconciliationResult struct {
	UserID           string          `json:"user_id"`
	ExpectedBalance  decimal.Decimal `json:"expected_balance"`
	ActualBalance    decimal.Decimal `json:"actual_balance"`
	FrozenBalance    decimal.Decimal `json:"frozen_balance"`
	Discrepancy      decimal.Decimal `json:"discrepancy"`
	PendingOrders    int             `json:"pending_orders"`
	PendingTrades    int             `json:"pending_trades"`
	IsConsistent     bool            `json:"is_consistent"`
}

type PositionReconciliationResult struct {
	UserID            string `json:"user_id"`
	Symbol            string `json:"symbol"`
	ExpectedPosition  int    `json:"expected_position"`
	ActualPosition    int    `json:"actual_position"`
	Discrepancy       int    `json:"discrepancy"`
	PendingOrders     int    `json:"pending_orders"`
	IsConsistent      bool   `json:"is_consistent"`
}

type TradeReconciliationResult struct {
	UserID            string          `json:"user_id"`
	Symbol            string          `json:"symbol"`
	TradeCount        int             `json:"trade_count"`
	OrderCount        int             `json:"order_count"`
	TradeVolume       int             `json:"trade_volume"`
	OrderVolume       int             `json:"order_volume"`
	VolumeDiscrepancy int             `json:"volume_discrepancy"`
	AmountDiscrepancy decimal.Decimal `json:"amount_discrepancy"`
	IsConsistent      bool            `json:"is_consistent"`
}

type ReconciliationError struct {
	Code    string
	Message string
}

var (
	ErrDiscrepancyNotFound   = &ReconciliationError{Code: "DISCREPANCY_NOT_FOUND", Message: "不一致记录不存在"}
	ErrReportNotFound        = &ReconciliationError{Code: "REPORT_NOT_FOUND", Message: "报告不存在"}
	ErrInvalidDiscrepancyType = &ReconciliationError{Code: "INVALID_TYPE", Message: "无效的不一致类型"}
	ErrFixNotAllowed         = &ReconciliationError{Code: "FIX_NOT_ALLOWED", Message: "不允许自动修复"}
	ErrAmountTooLarge        = &ReconciliationError{Code: "AMOUNT_TOO_LARGE", Message: "修复金额超出限制"}
	ErrApprovalRequired      = &ReconciliationError{Code: "APPROVAL_REQUIRED", Message: "需要审批"}
)

func (e *ReconciliationError) Error() string {
	return e.Message
}
