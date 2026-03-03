package trade

import (
	"database/sql/driver"
	"time"
	
	"github.com/shopspring/decimal"
)

type Trade struct {
	ID             uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	TradeID        string          `gorm:"uniqueIndex;type:varchar(32);not null" json:"trade_id"`
	OrderID        string          `gorm:"index;type:varchar(32);not null" json:"order_id"`
	UserID         string          `gorm:"index;type:varchar(64);not null" json:"user_id"`
	Symbol         string          `gorm:"index;type:varchar(16);not null" json:"symbol"`
	Side           int             `gorm:"not null" json:"side"`
	Price          decimal.Decimal `gorm:"type:decimal(20,4);not null" json:"price"`
	Quantity       int             `gorm:"not null" json:"quantity"`
	Amount         decimal.Decimal `gorm:"type:decimal(20,4);not null" json:"amount"`
	Fee            decimal.Decimal `gorm:"type:decimal(20,4);not null;default:0" json:"fee"`
	CounterUserID  string          `gorm:"type:varchar(64)" json:"counter_user_id"`
	CounterOrderID string          `gorm:"type:varchar(32)" json:"counter_order_id"`
	CreatedAt      time.Time       `gorm:"autoCreateTime" json:"created_at"`
}

func NewTrade(tradeID, orderID, userID, symbol string, side int, price decimal.Decimal, quantity int) *Trade {
	amount := price.Mul(decimal.NewFromInt(int64(quantity)))
	feeRate := decimal.NewFromFloat(0.0003)
	fee := amount.Mul(feeRate)
	
	return &Trade{
		TradeID:   tradeID,
		OrderID:   orderID,
		UserID:    userID,
		Symbol:    symbol,
		Side:      side,
		Price:     price,
		Quantity:  quantity,
		Amount:    amount,
		Fee:       fee,
		CreatedAt: time.Now(),
	}
}

func (t *Trade) SetCounterParty(userID, orderID string) {
	t.CounterUserID = userID
	t.CounterOrderID = orderID
}

func (t *Trade) TableName() string {
	return "trades"
}

type Settlement struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TradeID       string    `gorm:"uniqueIndex;type:varchar(32);not null" json:"trade_id"`
	BuyOrderID    string    `gorm:"type:varchar(32);not null" json:"buy_order_id"`
	SellOrderID   string    `gorm:"type:varchar(32);not null" json:"sell_order_id"`
	BuyUserID     string    `gorm:"type:varchar(64);not null" json:"buy_user_id"`
	SellUserID    string    `gorm:"type:varchar(64);not null" json:"sell_user_id"`
	Symbol        string    `gorm:"type:varchar(16);not null" json:"symbol"`
	Price         string    `gorm:"type:decimal(20,4);not null" json:"price"`
	Quantity      int       `gorm:"not null" json:"quantity"`
	Status        int       `gorm:"not null;default:1" json:"status"`
	Steps         string    `gorm:"type:text" json:"steps"`
	ErrorMessage  string    `gorm:"type:varchar(512)" json:"error_message"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

const (
	SettlementStatusPending    = 1
	SettlementStatusProcessing = 2
	SettlementStatusSuccess    = 3
	SettlementStatusFailed     = 4
)

func (s *Settlement) TableName() string {
	return "settlements"
}

type SettlementStep struct {
	Name         string `json:"name"`
	Status       int    `json:"status"`
	ErrorMessage string `json:"error_message"`
	ExecutedAt   int64  `json:"executed_at"`
}

const (
	StepStatusPending    = 1
	StepStatusSuccess    = 2
	StepStatusFailed     = 3
	StepStatusCompensate = 4
)

func (s SettlementStep) Value() (driver.Value, error) {
	return s.Name, nil
}

func (s *SettlementStep) Scan(value interface{}) error {
	*s = SettlementStep{Name: value.(string)}
	return nil
}
