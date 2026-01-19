package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

type Quote struct {
	Symbol    string          `json:"symbol"`
	Price     decimal.Decimal `json:"price"`
	Change    decimal.Decimal `json:"change"`
	ChangePct decimal.Decimal `json:"change_pct"`
	Open      decimal.Decimal `json:"open"`
	High      decimal.Decimal `json:"high"`
	Low       decimal.Decimal `json:"low"`
	Close     decimal.Decimal `json:"close"`
	Volume    int64           `json:"volume"`
	Amount    decimal.Decimal `json:"amount"`
	BidPrice  decimal.Decimal `json:"bid_price"`
	AskPrice  decimal.Decimal `json:"ask_price"`
	BidVolume int64           `json:"bid_volume"`
	AskVolume int64           `json:"ask_volume"`
	Timestamp time.Time       `json:"timestamp"`
}

func NewQuote(symbol string, basePrice decimal.Decimal) *Quote {
	return &Quote{
		Symbol:    symbol,
		Price:     basePrice,
		Change:    decimal.Zero,
		ChangePct: decimal.Zero,
		Open:      basePrice,
		High:      basePrice,
		Low:       basePrice,
		Close:     basePrice,
		Volume:    0,
		Amount:    decimal.Zero,
		BidPrice:  basePrice.Sub(decimal.NewFromFloat(0.01)),
		AskPrice:  basePrice.Add(decimal.NewFromFloat(0.01)),
		BidVolume: 1000,
		AskVolume: 1000,
		Timestamp: time.Now(),
	}
}

func (q *Quote) UpdatePrice(newPrice decimal.Decimal) {
	q.Price = newPrice
	q.Change = newPrice.Sub(q.Open)
	q.ChangePct = q.Change.Div(q.Open).Mul(decimal.NewFromInt(100))
	q.Close = newPrice
	q.Timestamp = time.Now()

	if newPrice.GreaterThan(q.High) {
		q.High = newPrice
	}
	if newPrice.LessThan(q.Low) {
		q.Low = newPrice
	}

	q.BidPrice = newPrice.Sub(decimal.NewFromFloat(0.01))
	q.AskPrice = newPrice.Add(decimal.NewFromFloat(0.01))

	q.Volume += int64(100 + int(newPrice.Mul(decimal.NewFromInt(10)).IntPart())%1000)
	q.Amount = q.Amount.Add(newPrice.Mul(decimal.NewFromInt(100)))
}

type KLine struct {
	Symbol    string          `json:"symbol"`
	Interval  string          `json:"interval"`
	Open      decimal.Decimal `json:"open"`
	High      decimal.Decimal `json:"high"`
	Low       decimal.Decimal `json:"low"`
	Close     decimal.Decimal `json:"close"`
	Volume    int64           `json:"volume"`
	Amount    decimal.Decimal `json:"amount"`
	StartTime time.Time       `json:"start_time"`
	EndTime   time.Time       `json:"end_time"`
}

func NewKLine(symbol string, interval string, price decimal.Decimal) *KLine {
	now := time.Now()
	return &KLine{
		Symbol:    symbol,
		Interval:  interval,
		Open:      price,
		High:      price,
		Low:       price,
		Close:     price,
		Volume:    0,
		Amount:    decimal.Zero,
		StartTime: now,
		EndTime:   now,
	}
}

func (k *KLine) Update(price decimal.Decimal) {
	k.Close = price
	k.EndTime = time.Now()

	if price.GreaterThan(k.High) {
		k.High = price
	}
	if price.LessThan(k.Low) {
		k.Low = price
	}

	k.Volume += 100
	k.Amount = k.Amount.Add(price.Mul(decimal.NewFromInt(100)))
}

func (k *KLine) IsExpired(interval string) bool {
	now := time.Now()
	elapsed := now.Sub(k.StartTime)

	switch interval {
	case "1m":
		return elapsed >= 1*time.Minute
	case "5m":
		return elapsed >= 5*time.Minute
	case "15m":
		return elapsed >= 15*time.Minute
	case "1h":
		return elapsed >= 1*time.Hour
	case "1d":
		return elapsed >= 24*time.Hour
	default:
		return false
	}
}

type Symbol struct {
	Symbol    string          `json:"symbol"`
	Name      string          `json:"name"`
	BasePrice decimal.Decimal `json:"base_price"`
}

func NewSymbol(symbol string, name string, basePrice decimal.Decimal) *Symbol {
	return &Symbol{
		Symbol:    symbol,
		Name:      name,
		BasePrice: basePrice,
	}
}

type MarketError struct {
	Code    string
	Message string
}

var (
	ErrSymbolNotFound   = &MarketError{Code: "SYMBOL_NOT_FOUND", Message: "股票代码不存在"}
	ErrQuoteNotFound    = &MarketError{Code: "QUOTE_NOT_FOUND", Message: "行情不存在"}
	ErrKLineNotFound    = &MarketError{Code: "KLINE_NOT_FOUND", Message: "K线不存在"}
	ErrInvalidInterval  = &MarketError{Code: "INVALID_INTERVAL", Message: "无效的K线周期"}
	ErrInvalidSymbol    = &MarketError{Code: "INVALID_SYMBOL", Message: "无效的股票代码"}
)

func (e *MarketError) Error() string {
	return e.Message
}
