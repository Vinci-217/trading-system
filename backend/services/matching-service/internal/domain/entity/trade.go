package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type Trade struct {
	ID          string
	Symbol      string
	BuyOrderID   string
	SellOrderID  string
	BuyUserID    string
	SellUserID   string
	Price       decimal.Decimal
	Quantity    int
	Timestamp   time.Time
	Fee         decimal.Decimal
}

func NewTrade(id string, symbol string, buyOrder *Order, sellOrder *Order, price decimal.Decimal, quantity int) *Trade {
	return &Trade{
		ID:          id,
		Symbol:      symbol,
		BuyOrderID:   buyOrder.ID,
		SellOrderID:  sellOrder.ID,
		BuyUserID:    buyOrder.UserID,
		SellUserID:   sellOrder.UserID,
		Price:       price,
		Quantity:    quantity,
		Timestamp:   time.Now(),
		Fee:         decimal.Zero,
	}
}

func (t *Trade) TotalAmount() decimal.Decimal {
	return t.Price.Mul(decimal.NewFromInt(int64(t.Quantity)))
}
