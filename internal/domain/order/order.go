package order

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

type Order struct {
	ID              uint              `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID         string            `gorm:"uniqueIndex;type:varchar(32);not null" json:"order_id"`
	UserID          string            `gorm:"index;type:varchar(64);not null" json:"user_id"`
	Symbol          string            `gorm:"index;type:varchar(16);not null" json:"symbol"`
	Side            int               `gorm:"not null" json:"side"`
	OrderType       int               `gorm:"not null" json:"order_type"`
	Price           *decimal.Decimal `gorm:"type:decimal(20,4);not null" json:"price"`
	Quantity        int               `gorm:"not null" json:"quantity"`
	FilledQuantity  int               `gorm:"not null;default:0" json:"filled_quantity"`
	AvgFilledPrice  *decimal.Decimal `gorm:"type:decimal(20,4);not null;default:0" json:"avg_filled_price"`
	Status          int               `gorm:"index;not null" json:"status"`
	ClientOrderID   string            `gorm:"uniqueIndex;type:varchar(64)" json:"client_order_id"`
	RejectReason   string            `gorm:"type:varchar(256)" json:"reject_reason"`
	Fee             *decimal.Decimal `gorm:"type:decimal(20,4);not null;default:0" json:"fee"`
	CreatedAt       time.Time         `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
}

const (
	OrderSideBuy  = 1
	OrderSideSell = 2
)

const (
	OrderTypeLimit  = 1
	OrderTypeMarket = 2
)

const (
	OrderStatusCreated   = 1
	OrderStatusPending   = 2
	OrderStatusPartial   = 3
	OrderStatusFilled    = 4
	OrderStatusCancelled = 5
	OrderStatusRejected  = 6
	OrderStatusExpired   = 7
)

var orderStatusTransitions = map[int][]int{
	OrderStatusCreated:   {OrderStatusPending, OrderStatusRejected, OrderStatusExpired},
	OrderStatusPending:   {OrderStatusPartial, OrderStatusFilled, OrderStatusCancelled},
	OrderStatusPartial:   {OrderStatusFilled, OrderStatusCancelled},
	OrderStatusFilled:    {},
	OrderStatusCancelled: {},
	OrderStatusRejected:  {},
	OrderStatusExpired:   {},
}

func NewOrder(orderID, userID, symbol string, side, orderType int, price decimal.Decimal, quantity int) *Order {
	now := time.Now()
	zero := decimal.Zero
	return &Order{
		OrderID:        orderID,
		UserID:         userID,
		Symbol:         symbol,
		Side:           side,
		OrderType:      orderType,
		Price:          &price,
		Quantity:       quantity,
		FilledQuantity: 0,
		AvgFilledPrice:  &zero,
		Status:         OrderStatusCreated,
		Fee:            &zero,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (o *Order) RemainingQuantity() int {
	return o.Quantity - o.FilledQuantity
}

func (o *Order) IsActive() bool {
	return o.Status == OrderStatusPending || o.Status == OrderStatusPartial
}

func (o *Order) CanCancel() bool {
	return o.IsActive()
}

func (o *Order) CanTransitionTo(target int) bool {
	allowed, exists := orderStatusTransitions[o.Status]
	if !exists {
		return false
	}
	for _, s := range allowed {
		if s == target {
			return true
		}
	}
	return false
}

func (o *Order) Fill(quantity int, price decimal.Decimal) error {
	if quantity <= 0 {
		return errors.New("invalid quantity")
	}
	if quantity > o.RemainingQuantity() {
		return errors.New("quantity exceeds remaining")
	}

	if o.AvgFilledPrice == nil {
		o.AvgFilledPrice = new(decimal.Decimal)
	}
	*o.AvgFilledPrice = o.AvgFilledPrice.Mul(decimal.NewFromInt(int64(o.FilledQuantity)))
	newFilled := price.Mul(decimal.NewFromInt(int64(quantity)))
	o.FilledQuantity += quantity
	*o.AvgFilledPrice = o.AvgFilledPrice.Add(newFilled).Div(decimal.NewFromInt(int64(o.FilledQuantity)))

	if o.FilledQuantity >= o.Quantity {
		o.Status = OrderStatusFilled
	} else if o.FilledQuantity > 0 {
		o.Status = OrderStatusPartial
	}

	feeRate := decimal.NewFromFloat(0.0003)
	fee := price.Mul(decimal.NewFromInt(int64(quantity))).Mul(feeRate)
	if o.Fee == nil {
		o.Fee = new(decimal.Decimal)
	}
	*o.Fee = o.Fee.Add(fee)
	o.UpdatedAt = time.Now()

	return nil
}

func (o *Order) Cancel(reason string) error {
	if !o.CanCancel() {
		return errors.New("order cannot be cancelled")
	}
	o.Status = OrderStatusCancelled
	o.RejectReason = reason
	o.UpdatedAt = time.Now()
	return nil
}

func (o *Order) Reject(reason string) {
	o.Status = OrderStatusRejected
	o.RejectReason = reason
	o.UpdatedAt = time.Now()
}

func (o *Order) SetPending() {
	o.Status = OrderStatusPending
	o.UpdatedAt = time.Now()
}

func (o *Order) TotalAmount() decimal.Decimal {
	if o.Price == nil {
		return decimal.Zero
	}
	return o.Price.Mul(decimal.NewFromInt(int64(o.Quantity)))
}

func (o *Order) FilledAmount() decimal.Decimal {
	if o.AvgFilledPrice == nil {
		return decimal.Zero
	}
	return o.AvgFilledPrice.Mul(decimal.NewFromInt(int64(o.FilledQuantity)))
}

func (o *Order) TableName() string {
	return "orders"
}

func GetOrderSideName(side int) string {
	if side == OrderSideBuy {
		return "BUY"
	}
	return "SELL"
}

func GetOrderTypeName(orderType int) string {
	if orderType == OrderTypeLimit {
		return "LIMIT"
	}
	return "MARKET"
}

func GetOrderStatusName(status int) string {
	switch status {
	case OrderStatusCreated:
		return "CREATED"
	case OrderStatusPending:
		return "PENDING"
	case OrderStatusPartial:
		return "PARTIAL"
	case OrderStatusFilled:
		return "FILLED"
	case OrderStatusCancelled:
		return "CANCELLED"
	case OrderStatusRejected:
		return "REJECTED"
	case OrderStatusExpired:
		return "EXPIRED"
	default:
		return "UNKNOWN"
	}
}
