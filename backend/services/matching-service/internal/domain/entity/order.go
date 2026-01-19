package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

type OrderType string

const (
	OrderTypeLimit  OrderType = "LIMIT"
	OrderTypeMarket OrderType = "MARKET"
)

type Order struct {
	ID        string
	UserID    string
	Symbol    string
	OrderType OrderType
	Side      OrderSide
	Price     decimal.Decimal
	Quantity  int
	FilledQuantity int
	Timestamp time.Time
	Status    OrderStatus
}

type OrderStatus string

const (
	OrderStatusPending  OrderStatus = "PENDING"
	OrderStatusPartial  OrderStatus = "PARTIAL"
	OrderStatusFilled   OrderStatus = "FILLED"
	OrderStatusCancelled OrderStatus = "CANCELLED"
)

func NewOrder(id, userID, symbol string, orderType OrderType, side OrderSide, price decimal.Decimal, quantity int) *Order {
	return &Order{
		ID:        id,
		UserID:    userID,
		Symbol:    symbol,
		OrderType: orderType,
		Side:      side,
		Price:     price,
		Quantity:  quantity,
		Timestamp: time.Now(),
		Status:    OrderStatusPending,
	}
}

func (o *Order) IsFilled() bool {
	return o.FilledQuantity >= o.Quantity
}

func (o *Order) RemainingQuantity() int {
	return o.Quantity - o.FilledQuantity
}

func (o *Order) Fill(quantity int) {
	o.FilledQuantity += quantity
	if o.IsFilled() {
		o.Status = OrderStatusFilled
	} else if o.FilledQuantity > 0 {
		o.Status = OrderStatusPartial
	}
}
