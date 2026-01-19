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
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string     `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	PasswordHash string     `gorm:"type:varchar(255);not null" json:"-"`
	Email        string     `gorm:"type:varchar(100)" json:"email"`
	Phone        string     `gorm:"type:varchar(20)" json:"phone"`
	Status       UserStatus `gorm:"type:varchar(20);default:ACTIVE" json:"status"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
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
	ID            int64         `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        int64         `gorm:"uniqueIndex;not null" json:"user_id"`
	CashBalance   float64       `gorm:"type:decimal(18,4);default:0" json:"cash_balance"`
	FrozenBalance float64       `gorm:"type:decimal(18,4);default:0" json:"frozen_balance"`
	TotalAssets   float64       `gorm:"type:decimal(18,4);default:0" json:"total_assets"`
	TotalProfit   float64       `gorm:"type:decimal(18,4);default:0" json:"total_profit"`
	Version       int           `gorm:"default:0" json:"version"`
	Status        AccountStatus `gorm:"type:varchar(20);default:ACTIVE" json:"status"`
	CreatedAt     time.Time     `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time     `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Account) TableName() string {
	return "accounts"
}

type Position struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           int64     `gorm:"index;not null" json:"user_id"`
	Symbol           string    `gorm:"type:varchar(20);not null" json:"symbol"`
	SymbolName       string    `gorm:"type:varchar(100)" json:"symbol_name"`
	Quantity         int       `gorm:"default:0" json:"quantity"`
	AvailableQuantity int      `gorm:"default:0" json:"available_quantity"`
	CostPrice        float64   `gorm:"type:decimal(18,4);default:0" json:"cost_price"`
	CurrentPrice     float64   `gorm:"type:decimal(18,4);default:0" json:"current_price"`
	MarketValue      float64   `gorm:"type:decimal(18,4);default:0" json:"market_value"`
	ProfitLoss       float64   `gorm:"type:decimal(18,4);default:0" json:"profit_loss"`
	ProfitLossRate   float64   `gorm:"type:decimal(10,4);default:0" json:"profit_loss_rate"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	User   User     `gorm:"foreignKey:UserID" json:"-"`
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
	ID              int64       `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID         string      `gorm:"type:varchar(64);uniqueIndex;not null" json:"order_id"`
	UserID          int64       `gorm:"index;not null" json:"user_id"`
	Symbol          string      `gorm:"type:varchar(20);not null" json:"symbol"`
	SymbolName      string      `gorm:"type:varchar(100)" json:"symbol_name"`
	OrderType       OrderType   `gorm:"type:varchar(20);default:LIMIT" json:"order_type"`
	Side            OrderSide   `gorm:"type:varchar(10);not null" json:"side"`
	Price           float64     `gorm:"type:decimal(18,4)" json:"price"`
	Quantity        int         `gorm:"not null" json:"quantity"`
	FilledQuantity  int         `gorm:"default:0" json:"filled_quantity"`
	Status          OrderStatus `gorm:"type:varchar(20);default:PENDING" json:"status"`
	Fee             float64     `gorm:"type:decimal(18,4);default:0" json:"fee"`
	RejectReason    string      `gorm:"type:varchar(255)" json:"reject_reason"`
	TXID            string      `gorm:"type:varchar(64)" json:"tx_id"`
	Version         int         `gorm:"default:0" json:"version"`
	CreatedAt       time.Time   `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time   `gorm:"autoUpdateTime" json:"updated_at"`
	FilledAt        *time.Time  `json:"filled_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (Order) TableName() string {
	return "orders"
}

type Trade struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TradeID        string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"trade_id"`
	OrderID        string    `gorm:"index;not null" json:"order_id"`
	CounterOrderID string    `gorm:"type:varchar(64)" json:"counter_order_id"`
	UserID         int64     `gorm:"index;not null" json:"user_id"`
	Symbol         string    `gorm:"type:varchar(20);not null" json:"symbol"`
	Side           OrderSide `gorm:"type:varchar(10);not null" json:"side"`
	Price          float64   `gorm:"type:decimal(18,4);not null" json:"price"`
	Quantity       int       `gorm:"not null" json:"quantity"`
	Amount         float64   `gorm:"type:decimal(18,4);not null" json:"amount"`
	Fee            float64   `gorm:"type:decimal(18,4);default:0" json:"fee"`
	ProfitLoss     float64   `gorm:"type:decimal(18,4)" json:"profit_loss"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
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
	ID          int64       `gorm:"primaryKey;autoIncrement" json:"id"`
	Symbol      string      `gorm:"type:varchar(20);uniqueIndex;not null" json:"symbol"`
	SymbolName  string      `gorm:"type:varchar(100);not null" json:"symbol_name"`
	Exchange    string      `gorm:"type:varchar(20)" json:"exchange"`
	Currency    string      `gorm:"type:varchar(10);default:CNY" json:"currency"`
	LotSize     int         `gorm:"default:100" json:"lot_size"`
	PriceTick   float64     `gorm:"type:decimal(10,2);default:0.01" json:"price_tick"`
	LimitUp     float64     `gorm:"type:decimal(18,4)" json:"limit_up"`
	LimitDown   float64     `gorm:"type:decimal(18,4)" json:"limit_down"`
	Industry    string      `gorm:"type:varchar(50)" json:"industry"`
	Status      StockStatus `gorm:"type:varchar(20);default:ACTIVE" json:"status"`
	CreatedAt   time.Time   `gorm:"autoCreateTime" json:"created_at"`
}

func (Stock) TableName() string {
	return "stocks"
}

type ReconciliationCheckType string

const (
	ReconCheckTypeFund     ReconciliationCheckType = "FUND"
	ReconCheckTypePosition ReconciliationCheckType = "POSITION"
	ReconCheckTypeTrade    ReconciliationCheckType = "TRADE"
	ReconCheckTypeOrder    ReconciliationCheckType = "ORDER"
	ReconCheckTypeFull     ReconciliationCheckType = "FULL"
	ReconCheckTypeDeep     ReconciliationCheckType = "DEEP"
)

type ReconciliationRecord struct {
	ID             int64                    `gorm:"primaryKey;autoIncrement" json:"id"`
	ReportID       string                   `gorm:"type:varchar(64);uniqueIndex;not null" json:"report_id"`
	CheckTime      time.Time               `gorm:"not null" json:"check_time"`
	CheckType      ReconciliationCheckType `gorm:"type:varchar(20);not null" json:"check_type"`
	Scope          string                  `gorm:"type:varchar(50);not null" json:"scope"`
	TotalUsers     int                     `gorm:"default:0" json:"total_users"`
	CheckedUsers   int                     `gorm:"default:0" json:"checked_users"`
	PassedUsers    int                     `gorm:"default:0" json:"passed_users"`
	IssueUsers     int                     `gorm:"default:0" json:"issue_users"`
	CriticalCount  int                     `gorm:"default:0" json:"critical_count"`
	HighCount      int                     `gorm:"default:0" json:"high_count"`
	MediumCount    int                     `gorm:"default:0" json:"medium_count"`
	LowCount       int                     `gorm:"default:0" json:"low_count"`
	AutoRepaired   int                     `gorm:"default:0" json:"auto_repaired"`
	ManualRepaired int                     `gorm:"default:0" json:"manual_repaired"`
	PendingRepair  int                     `gorm:"default:0" json:"pending_repair"`
	DurationMs     int                     `gorm:"default:0" json:"duration_ms"`
	ReportData     string                  `gorm:"type:json" json:"report_data"`
	CreatedAt      time.Time               `gorm:"autoCreateTime" json:"created_at"`
}

func (ReconciliationRecord) TableName() string {
	return "reconciliation_records"
}

type IssueSeverity string

const (
	IssueSeverityCritical IssueSeverity = "CRITICAL"
	IssueSeverityHigh     IssueSeverity = "HIGH"
	IssueSeverityMedium   IssueSeverity = "MEDIUM"
	IssueSeverityLow      IssueSeverity = "LOW"
)

type IssueCategory string

const (
	IssueCategoryAssetImbalance    IssueCategory = "ASSET_IMBALANCE"
	IssueCategoryCashMismatch      IssueCategory = "CASH_MISMATCH"
	IssueCategoryFeeMismatch       IssueCategory = "FEE_MISMATCH"
	IssueCategoryNegativeBalance   IssueCategory = "NEGATIVE_BALANCE"
	IssueCategoryQuantityMismatch  IssueCategory = "QUANTITY_MISMATCH"
	IssueCategoryCostMismatch      IssueCategory = "COST_MISMATCH"
	IssueCategoryNegativeAvailable IssueCategory = "NEGATIVE_AVAILABLE"
	IssueCategoryOverTrade         IssueCategory = "OVER_TRADE"
	IssueCategoryOrphanTrade       IssueCategory = "ORPHAN_TRADE"
	IssueCategoryInvalidPrice      IssueCategory = "INVALID_PRICE"
	IssueCategoryInvalidQuantity   IssueCategory = "INVALID_QUANTITY"
	IssueCategoryAmountMismatch    IssueCategory = "AMOUNT_MISMATCH"
	IssueCategoryTimeDisorder      IssueCategory = "TIME_DISORDER"
	IssueCategoryStatusError       IssueCategory = "STATUS_ERROR"
)

type IssueStatus string

const (
	IssueStatusPending       IssueStatus = "PENDING"
	IssueStatusAutoRepaired  IssueStatus = "AUTO_REPAIRED"
	IssueStatusManualRepaired IssueStatus = "MANUAL_REPAIRED"
	IssueStatusIgnored       IssueStatus = "IGNORED"
)

type ReconciliationIssue struct {
	ID            int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	IssueID       string         `gorm:"type:varchar(64);uniqueIndex;not null" json:"issue_id"`
	ReportID      string         `gorm:"index;not null" json:"report_id"`
	UserID        *int64         `gorm:"index" json:"user_id"`
	Symbol        string         `gorm:"type:varchar(20)" json:"symbol"`
	Category      IssueCategory  `gorm:"type:varchar(50);not null" json:"category"`
	Severity      IssueSeverity  `gorm:"type:varchar(20);not null" json:"severity"`
	Description   string         `gorm:"type:text;not null" json:"description"`
	ExpectedValue float64        `gorm:"type:decimal(18,4)" json:"expected_value"`
	ActualValue   float64        `gorm:"type:decimal(18,4)" json:"actual_value"`
	Difference    float64        `gorm:"type:decimal(18,4)" json:"difference"`
	Status        IssueStatus    `gorm:"type:varchar(30);default:PENDING" json:"status"`
	RepairInfo    string         `gorm:"type:json" json:"repair_info"`
	RepairedAt    *time.Time     `json:"repaired_at"`
	RepairedBy    string         `gorm:"type:varchar(50)" json:"repaired_by"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

func (ReconciliationIssue) TableName() string {
	return "reconciliation_issues"
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
