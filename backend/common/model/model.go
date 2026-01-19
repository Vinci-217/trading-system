package model

import (
	"time"
)

type UserStatus string

const (
	UserStatusActive   UserStatus = "ACTIVE"
	UserStatusInactive UserStatus = "INACTIVE"
	UserStatusFrozen   UserStatus = "FROZEN"
)

type User struct {
	ID           int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	Username     string     `json:"username" gorm:"type:varchar(50);uniqueIndex;not null"`
	PasswordHash string     `json:"-" gorm:"type:varchar(255);not null"`
	Email        string     `json:"email" gorm:"type:varchar(100)"`
	Phone        string     `json:"phone" gorm:"type:varchar(20)"`
	Status       UserStatus `json:"status" gorm:"type:varchar(20);default:ACTIVE"`
	CreatedAt    time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (User) TableName() string {
	return "users"
}

type AccountStatus string

const (
	AccountStatusActive  AccountStatus = "ACTIVE"
	AccountStatusFrozen  AccountStatus = "FROZEN"
	AccountStatusClosed  AccountStatus = "CLOSED"
)

type Account struct {
	ID            int64         `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID        int64         `json:"user_id" gorm:"uniqueIndex;not null"`
	CashBalance   float64       `json:"cash_balance" gorm:"type:decimal(18,4);default:0"`
	FrozenBalance float64       `json:"frozen_balance" gorm:"type:decimal(18,4);default:0"`
	TotalAssets   float64       `json:"total_assets" gorm:"type:decimal(18,4);default:0"`
	TotalProfit   float64       `json:"total_profit" gorm:"type:decimal(18,4);default:0"`
	Version       int           `json:"version" gorm:"default:0"`
	Status        AccountStatus `json:"status" gorm:"type:varchar(20);default:ACTIVE"`
	CreatedAt     time.Time     `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time     `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Account) TableName() string {
	return "accounts"
}

type Position struct {
	ID               int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID           int64     `json:"user_id" gorm:"index;not null"`
	Symbol           string    `json:"symbol" gorm:"type:varchar(20);not null"`
	SymbolName       string    `json:"symbol_name" gorm:"type:varchar(100)"`
	Quantity         int       `json:"quantity" gorm:"default:0"`
	AvailableQuantity int      `json:"available_quantity" gorm:"default:0"`
	CostPrice        float64   `json:"cost_price" gorm:"type:decimal(18,4);default:0"`
	CurrentPrice     float64   `json:"current_price" gorm:"type:decimal(18,4);default:0"`
	MarketValue      float64   `json:"market_value" gorm:"type:decimal(18,4);default:0"`
	ProfitLoss       float64   `json:"profit_loss" gorm:"type:decimal(18,4);default:0"`
	ProfitLossRate   float64   `json:"profit_loss_rate" gorm:"type:decimal(10,4);default:0"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Position) TableName() string {
	return "positions"
}

type OrderType string

const (
	OrderTypeMarket OrderType = "MARKET"
	OrderTypeLimit  OrderType = "LIMIT"
	OrderTypeStop   OrderType = "STOP"
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

type Order struct {
	ID              int64       `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderID         string      `json:"order_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	UserID          int64       `json:"user_id" gorm:"index;not null"`
	Symbol          string      `json:"symbol" gorm:"type:varchar(20);not null"`
	SymbolName      string      `json:"symbol_name" gorm:"type:varchar(100)"`
	OrderType       OrderType   `json:"order_type" gorm:"type:varchar(20);default:LIMIT"`
	Side            OrderSide   `json:"side" gorm:"type:varchar(10);not null"`
	Price           float64     `json:"price" gorm:"type:decimal(18,4)"`
	Quantity        int         `json:"quantity" gorm:"not null"`
	FilledQuantity  int         `json:"filled_quantity" gorm:"default:0"`
	Status          OrderStatus `json:"status" gorm:"type:varchar(20);default:PENDING"`
	Fee             float64     `json:"fee" gorm:"type:decimal(18,4);default:0"`
	RejectReason    string      `json:"reject_reason" gorm:"type:varchar(255)"`
	TXID            string      `json:"tx_id" gorm:"type:varchar(64)"`
	Version         int         `json:"version" gorm:"default:0"`
	ClientOrderID   string      `json:"client_order_id" gorm:"type:varchar(64)"`
	CreatedAt       time.Time   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time   `json:"updated_at" gorm:"autoUpdateTime"`
	FilledAt        *time.Time  `json:"filled_at,omitempty"`
}

func (Order) TableName() string {
	return "orders"
}

type Trade struct {
	ID             int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	TradeID        string    `json:"trade_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	OrderID        string    `json:"order_id" gorm:"index;not null"`
	CounterOrderID string    `json:"counter_order_id" gorm:"type:varchar(64)"`
	UserID         int64     `json:"user_id" gorm:"index;not null"`
	Symbol         string    `json:"symbol" gorm:"type:varchar(20);not null"`
	Side           OrderSide `json:"side" gorm:"type:varchar(10);not null"`
	Price          float64   `json:"price" gorm:"type:decimal(18,4);not null"`
	Quantity       int       `json:"quantity" gorm:"not null"`
	Amount         float64   `json:"amount" gorm:"type:decimal(18,4);not null"`
	Fee            float64   `json:"fee" gorm:"type:decimal(18,4);default:0"`
	ProfitLoss     float64   `json:"profit_loss" gorm:"type:decimal(18,4)"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (Trade) TableName() string {
	return "trades"
}

type StockStatus string

const (
	StockStatusActive    StockStatus = "ACTIVE"
	StockStatusSuspended StockStatus = "SUSPENDED"
	StockStatusDelisted  StockStatus = "DELISTED"
)

type Stock struct {
	ID          int64       `json:"id" gorm:"primaryKey;autoIncrement"`
	Symbol      string      `json:"symbol" gorm:"type:varchar(20);uniqueIndex;not null"`
	SymbolName  string      `json:"symbol_name" gorm:"type:varchar(100);not null"`
	Exchange    string      `json:"exchange" gorm:"type:varchar(20)"`
	Currency    string      `json:"currency" gorm:"type:varchar(10);default:CNY"`
	LotSize     int         `json:"lot_size" gorm:"default:100"`
	PriceTick   float64     `json:"price_tick" gorm:"type:decimal(10,2);default:0.01"`
	LimitUp     float64     `json:"limit_up" gorm:"type:decimal(18,4)"`
	LimitDown   float64     `json:"limit_down" gorm:"type:decimal(18,4)"`
	Industry    string      `json:"industry" gorm:"type:varchar(50)"`
	Status      StockStatus `json:"status" gorm:"type:varchar(20);default:ACTIVE"`
	CreatedAt   time.Time   `json:"created_at" gorm:"autoCreateTime"`
}

func (Stock) TableName() string {
	return "stocks"
}

type Quote struct {
	Symbol        string    `json:"symbol"`
	SymbolName    string    `json:"symbol_name"`
	Price         float64   `json:"price"`
	PrevClose     float64   `json:"prev_close"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"change_percent"`
	High          float64   `json:"high"`
	Low           float64   `json:"low"`
	Open          float64   `json:"open"`
	Volume        int64     `json:"volume"`
	Amount        float64   `json:"amount"`
	BidPrice      float64   `json:"bid_price"`
	AskPrice      float64   `json:"ask_price"`
	BidVolume     int64     `json:"bid_volume"`
	AskVolume     int64     `json:"ask_volume"`
	Timestamp     time.Time `json:"timestamp"`
}

type KLine struct {
	Symbol    string    `json:"symbol"`
	Period    string    `json:"period"`
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    int64     `json:"volume"`
	Amount    float64   `json:"amount"`
}
