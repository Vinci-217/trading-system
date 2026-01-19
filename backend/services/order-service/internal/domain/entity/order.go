package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

type Order struct {
	ID            string       `json:"id" db:"id"`
	UserID        string       `json:"user_id" db:"user_id"`
	Symbol        string       `json:"symbol" db:"symbol"`
	OrderType     OrderType    `json:"order_type" db:"order_type"`
	Side          OrderSide    `json:"side" db:"side"`
	Price         decimal.Decimal `json:"price" db:"price"`
	Quantity      int          `json:"quantity" db:"quantity"`
	FilledQuantity int         `json:"filled_quantity" db:"filled_quantity"`
	Status        OrderStatus  `json:"status" db:"status"`
	Fee           decimal.Decimal `json:"fee" db:"fee"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
	ClientOrderID string       `json:"client_order_id" db:"client_order_id"`
	Remarks       string       `json:"remarks" db:"remarks"`
}

type OrderType string

const (
	OrderTypeLimit  OrderType = "LIMIT"
	OrderTypeMarket OrderType = "MARKET"
)

type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "PENDING"
	OrderStatusPartial   OrderStatus = "PARTIAL"
	OrderStatusFilled    OrderStatus = "FILLED"
	OrderStatusCancelled OrderStatus = "CANCELLED"
	OrderStatusRejected  OrderStatus = "REJECTED"
)

func NewOrder(userID string, symbol string, orderType OrderType, side OrderSide, price decimal.Decimal, quantity int) *Order {
	return &Order{
		ID:             "",
		UserID:         userID,
		Symbol:         symbol,
		OrderType:      orderType,
		Side:           side,
		Price:          price,
		Quantity:       quantity,
		FilledQuantity: 0,
		Status:         OrderStatusPending,
		Fee:            decimal.Zero,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		ClientOrderID:  "",
		Remarks:        "",
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

func (o *Order) Fill(quantity int, price decimal.Decimal) {
	o.FilledQuantity += quantity
	o.UpdatedAt = time.Now()

	if o.FilledQuantity >= o.Quantity {
		o.Status = OrderStatusFilled
	} else if o.FilledQuantity > 0 {
		o.Status = OrderStatusPartial
	}

	feeRate := decimal.NewFromFloat(0.0003)
	o.Fee = o.Fee.Add(price.Mul(decimal.NewFromInt(int64(quantity))).Mul(feeRate))
}

func (o *Order) Cancel(reason string) {
	if !o.CanCancel() {
		return
	}
	o.Status = OrderStatusCancelled
	o.Remarks = reason
	o.UpdatedAt = time.Now()
}

func (o *Order) Reject(reason string) {
	o.Status = OrderStatusRejected
	o.Remarks = reason
	o.UpdatedAt = time.Now()
}

func (o *Order) TotalAmount() decimal.Decimal {
	return o.Price.Mul(decimal.NewFromInt(int64(o.Quantity)))
}

type TCCTransaction struct {
	ID             string          `json:"id"`
	OrderID        string          `json:"order_id"`
	TransactionType string         `json:"transaction_type"`
	Status         string          `json:"status"`
	TryParams      string          `json:"try_params"`
	ConfirmParams  string          `json:"confirm_params"`
	CancelParams   string          `json:"cancel_params"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	RetryCount     int             `json:"retry_count"`
	ErrorMsg       string          `json:"error_msg"`
}

func NewTCCTransaction(orderID string, transactionType string) *TCCTransaction {
	now := time.Now()
	return &TCCTransaction{
		ID:              "",
		OrderID:         orderID,
		TransactionType: transactionType,
		Status:          "TRYING",
		CreatedAt:       now,
		UpdatedAt:       now,
		RetryCount:      0,
	}
}

type OrderError struct {
	Code    string
	Message string
}

var (
	ErrOrderNotFound       = &OrderError{Code: "ORDER_NOT_FOUND", Message: "订单不存在"}
	ErrOrderInvalidStatus  = &OrderError{Code: "INVALID_STATUS", Message: "订单状态无效"}
	ErrOrderCannotCancel   = &OrderError{Code: "CANNOT_CANCEL", Message: "订单无法取消"}
	ErrOrderCannotConfirm  = &OrderError{Code: "CANNOT_CONFIRM", Message: "订单无法确认"}
	ErrDuplicateOrder      = &OrderError{Code: "DUPLICATE_ORDER", Message: "重复订单"}
	ErrInvalidQuantity     = &OrderError{Code: "INVALID_QUANTITY", Message: "无效数量"}
	ErrInvalidPrice        = &OrderError{Code: "INVALID_PRICE", Message: "无效价格"}
	ErrRateLimitExceeded   = &OrderError{Code: "RATE_LIMIT_EXCEEDED", Message: "超出频率限制"}
)

func (e *OrderError) Error() string {
	return e.Message
}
