package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type Account struct {
	ID            string
	UserID        string
	CashBalance   decimal.Decimal
	FrozenBalance decimal.Decimal
	TotalAssets   decimal.Decimal
	TotalDeposit  decimal.Decimal
	TotalWithdrawal decimal.Decimal
	Version       int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewAccount(id, userID string, initialBalance decimal.Decimal) *Account {
	return &Account{
		ID:              id,
		UserID:          userID,
		CashBalance:     initialBalance,
		FrozenBalance:   decimal.Zero,
		TotalAssets:     initialBalance,
		TotalDeposit:    decimal.Zero,
		TotalWithdrawal: decimal.Zero,
		Version:         0,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func (a *Account) AvailableBalance() decimal.Decimal {
	return a.CashBalance.Sub(a.FrozenBalance)
}

func (a *Account) Freeze(amount decimal.Decimal) error {
	if amount.GreaterThan(a.AvailableBalance()) {
		return ErrInsufficientBalance
	}
	a.FrozenBalance = a.FrozenBalance.Add(amount)
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Account) Unfreeze(amount decimal.Decimal) {
	a.FrozenBalance = a.FrozenBalance.Sub(amount)
	if a.FrozenBalance.LessThan(decimal.Zero) {
		a.FrozenBalance = decimal.Zero
	}
	a.UpdatedAt = time.Now()
}

func (a *Account) Deposit(amount decimal.Decimal) {
	a.CashBalance = a.CashBalance.Add(amount)
	a.TotalDeposit = a.TotalDeposit.Add(amount)
	a.TotalAssets = a.TotalAssets.Add(amount)
	a.UpdatedAt = time.Now()
}

func (a *Account) Withdraw(amount decimal.Decimal) error {
	if amount.GreaterThan(a.AvailableBalance()) {
		return ErrInsufficientBalance
	}
	a.CashBalance = a.CashBalance.Sub(amount)
	a.TotalWithdrawal = a.TotalWithdrawal.Add(amount)
	a.TotalAssets = a.TotalAssets.Sub(amount)
	a.UpdatedAt = time.Now()
	return nil
}

import "errors"

var ErrInsufficientBalance = errors.New("insufficient balance")
