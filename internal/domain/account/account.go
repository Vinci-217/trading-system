package account

import (
	"database/sql/driver"
	"errors"
	"time"
	
	"github.com/shopspring/decimal"
)

type Account struct {
	ID             uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         string          `gorm:"uniqueIndex;type:varchar(64);not null" json:"user_id"`
	CashBalance    decimal.Decimal `gorm:"type:decimal(20,4);not null;default:0" json:"cash_balance"`
	FrozenBalance  decimal.Decimal `gorm:"type:decimal(20,4);not null;default:0" json:"frozen_balance"`
	TotalAssets    decimal.Decimal `gorm:"type:decimal(20,4);not null;default:0" json:"total_assets"`
	TotalDeposit   decimal.Decimal `gorm:"type:decimal(20,4);not null;default:0" json:"total_deposit"`
	TotalWithdraw  decimal.Decimal `gorm:"type:decimal(20,4);not null;default:0" json:"total_withdraw"`
	Version        uint            `gorm:"not null;default:0" json:"version"`
	CreatedAt      time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

func NewAccount(userID string) *Account {
	now := time.Now()
	return &Account{
		UserID:        userID,
		CashBalance:   decimal.Zero,
		FrozenBalance: decimal.Zero,
		TotalAssets:   decimal.Zero,
		TotalDeposit:  decimal.Zero,
		TotalWithdraw: decimal.Zero,
		Version:       0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func (a *Account) AvailableBalance() decimal.Decimal {
	return a.CashBalance.Sub(a.FrozenBalance)
}

func (a *Account) Deposit(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return errors.New("invalid amount")
	}
	a.CashBalance = a.CashBalance.Add(amount)
	a.TotalDeposit = a.TotalDeposit.Add(amount)
	a.TotalAssets = a.TotalAssets.Add(amount)
	a.Version++
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Account) Withdraw(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return errors.New("invalid amount")
	}
	available := a.AvailableBalance()
	if available.LessThan(amount) {
		return errors.New("insufficient balance")
	}
	a.CashBalance = a.CashBalance.Sub(amount)
	a.TotalWithdraw = a.TotalWithdraw.Add(amount)
	a.TotalAssets = a.TotalAssets.Sub(amount)
	a.Version++
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Account) Freeze(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return errors.New("invalid amount")
	}
	available := a.AvailableBalance()
	if available.LessThan(amount) {
		return errors.New("insufficient balance")
	}
	a.FrozenBalance = a.FrozenBalance.Add(amount)
	a.Version++
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Account) Unfreeze(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return errors.New("invalid amount")
	}
	if a.FrozenBalance.LessThan(amount) {
		return errors.New("insufficient frozen balance")
	}
	a.FrozenBalance = a.FrozenBalance.Sub(amount)
	a.Version++
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Account) ConfirmFreeze(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return errors.New("invalid amount")
	}
	if a.FrozenBalance.LessThan(amount) {
		return errors.New("insufficient frozen balance")
	}
	a.FrozenBalance = a.FrozenBalance.Sub(amount)
	a.CashBalance = a.CashBalance.Sub(amount)
	a.TotalAssets = a.TotalAssets.Sub(amount)
	a.Version++
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Account) TableName() string {
	return "accounts"
}

type Position struct {
	ID             uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         string          `gorm:"uniqueIndex:uk_user_symbol;type:varchar(64);not null" json:"user_id"`
	Symbol         string          `gorm:"uniqueIndex:uk_user_symbol;type:varchar(16);not null" json:"symbol"`
	Quantity       int             `gorm:"not null;default:0" json:"quantity"`
	FrozenQuantity int             `gorm:"not null;default:0" json:"frozen_quantity"`
	AvgCost        decimal.Decimal `gorm:"type:decimal(20,4);not null;default:0" json:"avg_cost"`
	MarketValue    decimal.Decimal `gorm:"type:decimal(20,4);not null;default:0" json:"market_value"`
	ProfitLoss     decimal.Decimal `gorm:"type:decimal(20,4);not null;default:0" json:"profit_loss"`
	Version        uint            `gorm:"not null;default:0" json:"version"`
	CreatedAt      time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

func NewPosition(userID, symbol string) *Position {
	now := time.Now()
	return &Position{
		UserID:    userID,
		Symbol:    symbol,
		Quantity:  0,
		AvgCost:   decimal.Zero,
		Version:   0,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (p *Position) AvailableQuantity() int {
	return p.Quantity - p.FrozenQuantity
}

func (p *Position) Buy(quantity int, price decimal.Decimal) {
	if quantity <= 0 {
		return
	}
	totalCost := p.AvgCost.Mul(decimal.NewFromInt(int64(p.Quantity)))
	totalCost = totalCost.Add(price.Mul(decimal.NewFromInt(int64(quantity))))
	newQuantity := p.Quantity + quantity
	if newQuantity > 0 {
		p.AvgCost = totalCost.Div(decimal.NewFromInt(int64(newQuantity)))
	} else {
		p.AvgCost = decimal.Zero
	}
	p.Quantity = newQuantity
	p.Version++
	p.UpdatedAt = time.Now()
}

func (p *Position) Sell(quantity int) {
	if quantity <= 0 {
		return
	}
	p.Quantity = p.Quantity - quantity
	if p.Quantity < 0 {
		p.Quantity = 0
	}
	p.Version++
	p.UpdatedAt = time.Now()
}

func (p *Position) Freeze(quantity int) error {
	if quantity <= 0 {
		return errors.New("invalid quantity")
	}
	if p.AvailableQuantity() < quantity {
		return errors.New("insufficient available quantity")
	}
	p.FrozenQuantity += quantity
	p.Version++
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Position) Unfreeze(quantity int) error {
	if quantity <= 0 {
		return errors.New("invalid quantity")
	}
	if p.FrozenQuantity < quantity {
		return errors.New("insufficient frozen quantity")
	}
	p.FrozenQuantity -= quantity
	p.Version++
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Position) ConfirmFreeze(quantity int) error {
	if quantity <= 0 {
		return errors.New("invalid quantity")
	}
	if p.FrozenQuantity < quantity {
		return errors.New("insufficient frozen quantity")
	}
	p.FrozenQuantity -= quantity
	p.Quantity -= quantity
	p.Version++
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Position) UpdateMarketValue(currentPrice decimal.Decimal) {
	p.MarketValue = currentPrice.Mul(decimal.NewFromInt(int64(p.Quantity)))
	p.ProfitLoss = p.MarketValue.Sub(p.AvgCost.Mul(decimal.NewFromInt(int64(p.Quantity))))
	p.UpdatedAt = time.Now()
}

func (p *Position) TableName() string {
	return "positions"
}

type FundFlow struct {
	ID            uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	FlowID        string          `gorm:"uniqueIndex;type:varchar(32);not null" json:"flow_id"`
	TransactionID string          `gorm:"uniqueIndex;type:varchar(64);not null" json:"transaction_id"`
	UserID        string          `gorm:"index;type:varchar(64);not null" json:"user_id"`
	OrderID       string          `gorm:"index;type:varchar(32)" json:"order_id"`
	Amount        decimal.Decimal `gorm:"type:decimal(20,4);not null" json:"amount"`
	FlowType      int             `gorm:"not null" json:"flow_type"`
	BalanceBefore decimal.Decimal `gorm:"type:decimal(20,4);not null" json:"balance_before"`
	BalanceAfter  decimal.Decimal `gorm:"type:decimal(20,4);not null" json:"balance_after"`
	FrozenBefore  decimal.Decimal `gorm:"type:decimal(20,4);not null;default:0" json:"frozen_before"`
	FrozenAfter   decimal.Decimal `gorm:"type:decimal(20,4);not null;default:0" json:"frozen_after"`
	Status        int             `gorm:"not null;default:1" json:"status"`
	Remark        string          `gorm:"type:varchar(256)" json:"remark"`
	CreatedAt     time.Time       `gorm:"autoCreateTime" json:"created_at"`
}

const (
	FlowTypeDeposit   = 1
	FlowTypeWithdraw  = 2
	FlowTypeFreeze    = 3
	FlowTypeUnfreeze  = 4
	FlowTypeDeduct    = 5
	FlowTypeCredit    = 6
)

const (
	FlowStatusPending = 1
	FlowStatusSuccess = 2
	FlowStatusFailed  = 3
)

func (f *FundFlow) TableName() string {
	return "fund_flows"
}

type Decimal decimal.Decimal

func (d *Decimal) Scan(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return errors.New("failed to scan decimal")
	}
	val, err := decimal.NewFromString(str)
	if err != nil {
		return err
	}
	*d = Decimal(val)
	return nil
}

func (d Decimal) Value() (driver.Value, error) {
	return decimal.Decimal(d).String(), nil
}

type Discrepancy struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      string    `gorm:"index;type:varchar(64);not null" json:"user_id"`
	Symbol      string    `gorm:"type:varchar(16)" json:"symbol"`
	Type        string    `gorm:"type:varchar(32);not null" json:"type"`
	Expected    string    `gorm:"type:varchar(64);not null" json:"expected"`
	Actual      string    `gorm:"type:varchar(64);not null" json:"actual"`
	Details     string    `gorm:"type:text" json:"details"`
	Status      int       `gorm:"not null;default:1" json:"status"`
	Resolution  string    `gorm:"type:text" json:"resolution"`
	ResolvedAt  *time.Time `json:"resolved_at"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (d *Discrepancy) TableName() string {
	return "discrepancies"
}
