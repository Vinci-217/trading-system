package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

type Account struct {
	UserID          string          `json:"user_id" db:"user_id"`
	CashBalance     decimal.Decimal `json:"cash_balance" db:"cash_balance"`
	FrozenBalance   decimal.Decimal `json:"frozen_balance" db:"frozen_balance"`
	SecuritiesBalance decimal.Decimal `json:"securities_balance" db:"securities_balance"`
	TotalAssets     decimal.Decimal `json:"total_assets" db:"total_assets"`
	TotalDeposit    decimal.Decimal `json:"total_deposit" db:"total_deposit"`
	TotalWithdrawal decimal.Decimal `json:"total_withdrawal" db:"total_withdrawal"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
	Version         int64           `json:"version" db:"version"`
}

func NewAccount(userID string) *Account {
	now := time.Now()
	return &Account{
		UserID:            userID,
		CashBalance:       decimal.Zero,
		FrozenBalance:     decimal.Zero,
		SecuritiesBalance: decimal.Zero,
		TotalAssets:       decimal.Zero,
		TotalDeposit:      decimal.Zero,
		TotalWithdrawal:   decimal.Zero,
		CreatedAt:         now,
		UpdatedAt:         now,
		Version:           0,
	}
}

func (a *Account) AvailableBalance() decimal.Decimal {
	return a.CashBalance.Sub(a.FrozenBalance)
}

func (a *Account) Deposit(amount decimal.Decimal) {
	a.CashBalance = a.CashBalance.Add(amount)
	a.TotalDeposit = a.TotalDeposit.Add(amount)
	a.TotalAssets = a.TotalAssets.Add(amount)
	a.UpdatedAt = time.Now()
	a.Version++
}

func (a *Account) Withdraw(amount decimal.Decimal) error {
	available := a.AvailableBalance()
	if available.LessThan(amount) {
		return ErrInsufficientBalance
	}
	a.CashBalance = a.CashBalance.Sub(amount)
	a.TotalWithdrawal = a.TotalWithdrawal.Add(amount)
	a.TotalAssets = a.TotalAssets.Sub(amount)
	a.UpdatedAt = time.Now()
	a.Version++
	return nil
}

func (a *Account) LockFunds(amount decimal.Decimal) error {
	available := a.AvailableBalance()
	if available.LessThan(amount) {
		return ErrInsufficientBalance
	}
	a.FrozenBalance = a.FrozenBalance.Add(amount)
	a.UpdatedAt = time.Now()
	a.Version++
	return nil
}

func (a *Account) UnlockFunds(amount decimal.Decimal) {
	a.FrozenBalance = a.FrozenBalance.Sub(amount)
	if a.FrozenBalance.LessThan(decimal.Zero) {
		a.FrozenBalance = decimal.Zero
	}
	a.UpdatedAt = time.Now()
	a.Version++
}

func (a *Account) ConfirmLock(amount decimal.Decimal) {
	a.FrozenBalance = a.FrozenBalance.Sub(amount)
	a.CashBalance = a.CashBalance.Sub(amount)
	if a.FrozenBalance.LessThan(decimal.Zero) {
		a.FrozenBalance = decimal.Zero
	}
	if a.CashBalance.LessThan(decimal.Zero) {
		a.CashBalance = decimal.Zero
	}
	a.UpdatedAt = time.Now()
	a.Version++
}

type Position struct {
	ID              string          `json:"id" db:"id"`
	UserID          string          `json:"user_id" db:"user_id"`
	Symbol          string          `json:"symbol" db:"symbol"`
	Quantity        int             `json:"quantity" db:"quantity"`
	FrozenQuantity  int             `json:"frozen_quantity" db:"frozen_quantity"`
	AvgCost         decimal.Decimal `json:"avg_cost" db:"avg_cost"`
	MarketValue     decimal.Decimal `json:"market_value" db:"market_value"`
	ProfitLoss      decimal.Decimal `json:"profit_loss" db:"profit_loss"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
	Version         int64           `json:"version" db:"version"`
}

func NewPosition(userID string, symbol string) *Position {
	now := time.Now()
	return &Position{
		ID:             "",
		UserID:         userID,
		Symbol:         symbol,
		Quantity:       0,
		FrozenQuantity: 0,
		AvgCost:        decimal.Zero,
		MarketValue:    decimal.Zero,
		ProfitLoss:     decimal.Zero,
		CreatedAt:      now,
		UpdatedAt:      now,
		Version:        0,
	}
}

func (p *Position) AvailableQuantity() int {
	return p.Quantity - p.FrozenQuantity
}

func (p *Position) Buy(quantity int, price decimal.Decimal) {
	totalCost := p.AvgCost.Mul(decimal.NewFromInt(int64(p.Quantity)))
	totalCost = totalCost.Add(price.Mul(decimal.NewFromInt(int64(quantity))))
	newQuantity := p.Quantity + quantity
	if newQuantity > 0 {
		p.AvgCost = totalCost.Div(decimal.NewFromInt(int64(newQuantity)))
	} else {
		p.AvgCost = decimal.Zero
	}
	p.Quantity = newQuantity
	p.UpdatedAt = time.Now()
	p.Version++
}

func (p *Position) Sell(quantity int) {
	p.Quantity = p.Quantity - quantity
	if p.Quantity < 0 {
		p.Quantity = 0
	}
	p.UpdatedAt = time.Now()
	p.Version++
}

func (p *Position) UpdateMarketValue(currentPrice decimal.Decimal) {
	p.MarketValue = currentPrice.Mul(decimal.NewFromInt(int64(p.Quantity)))
	p.ProfitLoss = p.MarketValue.Sub(p.AvgCost.Mul(decimal.NewFromInt(int64(p.Quantity))))
	p.UpdatedAt = time.Now()
}

type FundLock struct {
	ID             string          `json:"id"`
	UserID         string          `json:"user_id"`
	OrderID        string          `json:"order_id"`
	TransactionID  string          `json:"transaction_id"`
	Amount         decimal.Decimal `json:"amount"`
	Status         string          `json:"status"`
	CreatedAt      time.Time       `json:"created_at"`
	ExpiresAt      time.Time       `json:"expires_at"`
	RetryCount     int             `json:"retry_count"`
}

func NewFundLock(userID string, orderID string, transactionID string, amount decimal.Decimal, timeout time.Duration) *FundLock {
	return &FundLock{
		ID:            "",
		UserID:        userID,
		OrderID:       orderID,
		TransactionID: transactionID,
		Amount:        amount,
		Status:        "LOCKED",
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(timeout),
		RetryCount:    0,
	}
}

type TCCTransaction struct {
	ID             string          `json:"id"`
	UserID         string          `json:"user_id"`
	OrderID        string          `json:"order_id"`
	Amount         decimal.Decimal `json:"amount"`
	Phase          string          `json:"phase"`
	Status         string          `json:"status"`
	TryTime        time.Time       `json:"try_time"`
	ConfirmTime    time.Time       `json:"confirm_time"`
	CancelTime     time.Time       `json:"cancel_time"`
	RetryCount     int             `json:"retry_count"`
	ErrorMessage   string          `json:"error_message"`
}

func NewTCCTransaction(userID string, orderID string, transactionID string, amount decimal.Decimal) *TCCTransaction {
	return &TCCTransaction{
		ID:           transactionID,
		UserID:       userID,
		OrderID:      orderID,
		Amount:       amount,
		Phase:        "TRY",
		Status:       "PENDING",
		TryTime:      time.Time{},
		ConfirmTime:  time.Time{},
		CancelTime:   time.Time{},
		RetryCount:   0,
		ErrorMessage: "",
	}
}

type AccountError struct {
	Code    string
	Message string
}

var (
	ErrAccountNotFound       = &AccountError{Code: "ACCOUNT_NOT_FOUND", Message: "账户不存在"}
	ErrInsufficientBalance   = &AccountError{Code: "INSUFFICIENT_BALANCE", Message: "余额不足"}
	ErrPositionNotFound      = &AccountError{Code: "POSITION_NOT_FOUND", Message: "持仓不存在"}
	ErrLockNotFound          = &AccountError{Code: "LOCK_NOT_FOUND", Message: "锁定记录不存在"}
	ErrTransactionNotFound   = &AccountError{Code: "TRANSACTION_NOT_FOUND", Message: "交易记录不存在"}
	ErrInvalidAmount         = &AccountError{Code: "INVALID_AMOUNT", Message: "无效金额"}
	ErrDuplicateAccount      = &AccountError{Code: "DUPLICATE_ACCOUNT", Message: "账户已存在"}
	ErrDuplicatePosition     = &AccountError{Code: "DUPLICATE_POSITION", Message: "持仓已存在"}
)

func (e *AccountError) Error() string {
	return e.Message
}
