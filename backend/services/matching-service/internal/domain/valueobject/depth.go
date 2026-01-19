package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type DepthLevel struct {
	Price    decimal.Decimal
	Quantity int
	Orders   int
}

type MarketDepth struct {
	Symbol    string
	Bids      []DepthLevel
	Asks      []DepthLevel
	Timestamp time.Time
}

type Quote struct {
	Symbol    string
	Price     decimal.Decimal
	Change    decimal.Decimal
	ChangePct decimal.Decimal
	Open      decimal.Decimal
	High      decimal.Decimal
	Low       decimal.Decimal
	Close     decimal.Decimal
	Volume    int64
	Amount    decimal.Decimal
	BidPrice  decimal.Decimal
	AskPrice  decimal.Decimal
	BidVolume int64
	AskVolume int64
	Timestamp time.Time
}

type KLine struct {
	Symbol    string
	Interval  string
	Open      decimal.Decimal
	High      decimal.Decimal
	Low       decimal.Decimal
	Close     decimal.Decimal
	Volume    int64
	Amount    decimal.Decimal
	StartTime time.Time
	EndTime   time.Time
}
