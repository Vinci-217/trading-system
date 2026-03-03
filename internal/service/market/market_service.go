package market

import (
	"context"
	"sync"
	"time"

	"github.com/stock-trading-system/internal/domain/matching"
	"github.com/stock-trading-system/pkg/logger"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Quote struct {
	Symbol      string          `json:"symbol"`
	LastPrice   decimal.Decimal `json:"last_price"`
	OpenPrice   decimal.Decimal `json:"open_price"`
	HighPrice   decimal.Decimal `json:"high_price"`
	LowPrice    decimal.Decimal `json:"low_price"`
	Volume      int64           `json:"volume"`
	Amount      decimal.Decimal `json:"amount"`
	BidPrice    decimal.Decimal `json:"bid_price"`
	BidQuantity int             `json:"bid_quantity"`
	AskPrice    decimal.Decimal `json:"ask_price"`
	AskQuantity int             `json:"ask_quantity"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type MarketDepth struct {
	Symbol    string        `json:"symbol"`
	Bids      []DepthLevel  `json:"bids"`
	Asks      []DepthLevel  `json:"asks"`
	Timestamp time.Time     `json:"timestamp"`
}

type DepthLevel struct {
	Price      decimal.Decimal `json:"price"`
	Quantity   int             `json:"quantity"`
	OrderCount int             `json:"order_count"`
}

type KLine struct {
	Symbol    string          `json:"symbol"`
	Interval  string          `json:"interval"`
	OpenTime  time.Time       `json:"open_time"`
	CloseTime time.Time       `json:"close_time"`
	Open      decimal.Decimal `json:"open"`
	High      decimal.Decimal `json:"high"`
	Low       decimal.Decimal `json:"low"`
	Close     decimal.Decimal `json:"close"`
	Volume    int64           `json:"volume"`
	Amount    decimal.Decimal `json:"amount"`
}

type MarketService struct {
	db             *gorm.DB
	matchingEngine *matching.MatchingEngine
	quotes         map[string]*Quote
	mu             sync.RWMutex
	logger         *logger.Logger
}

func NewMarketService(db *gorm.DB, matchingEngine *matching.MatchingEngine, logger *logger.Logger) *MarketService {
	ms := &MarketService{
		db:             db,
		matchingEngine: matchingEngine,
		quotes:         make(map[string]*Quote),
		logger:         logger,
	}
	go ms.startQuoteUpdater()
	return ms
}

func (s *MarketService) GetQuote(ctx context.Context, symbol string) (*Quote, error) {
	s.mu.RLock()
	quote, exists := s.quotes[symbol]
	s.mu.RUnlock()

	if exists {
		return quote, nil
	}

	return s.updateQuote(ctx, symbol)
}

func (s *MarketService) GetAllQuotes(ctx context.Context) ([]*Quote, error) {
	orderBooks := s.matchingEngine.GetAllOrderBooks()
	quotes := make([]*Quote, 0, len(orderBooks))

	for symbol := range orderBooks {
		quote, err := s.GetQuote(ctx, symbol)
		if err != nil {
			continue
		}
		quotes = append(quotes, quote)
	}

	return quotes, nil
}

func (s *MarketService) GetMarketDepth(ctx context.Context, symbol string, level int) (*MarketDepth, error) {
	depth := s.matchingEngine.GetMarketDepth(symbol, level)
	
	result := &MarketDepth{
		Symbol:    symbol,
		Timestamp: time.Now(),
		Bids:      make([]DepthLevel, 0, len(depth.Bids)),
		Asks:      make([]DepthLevel, 0, len(depth.Asks)),
	}

	for _, d := range depth.Bids {
		result.Bids = append(result.Bids, DepthLevel{
			Price:      d.Price,
			Quantity:   d.Quantity,
			OrderCount: d.OrderCount,
		})
	}

	for _, d := range depth.Asks {
		result.Asks = append(result.Asks, DepthLevel{
			Price:      d.Price,
			Quantity:   d.Quantity,
			OrderCount: d.OrderCount,
		})
	}

	return result, nil
}

func (s *MarketService) GetKLines(ctx context.Context, symbol string, interval string, startTime, endTime time.Time) ([]*KLine, error) {
	var klines []*KLine
	
	return klines, nil
}

func (s *MarketService) updateQuote(ctx context.Context, symbol string) (*Quote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	orderBook := s.matchingEngine.GetOrderBook(symbol)
	if orderBook == nil {
		return nil, nil
	}

	quote := &Quote{
		Symbol:    symbol,
		LastPrice: s.matchingEngine.GetLastPrice(symbol),
		Volume:    orderBook.TotalVolume,
		Amount:    orderBook.TotalValue,
		UpdatedAt: time.Now(),
	}

	depth := s.matchingEngine.GetMarketDepth(symbol, 1)
	if len(depth.Bids) > 0 {
		quote.BidPrice = depth.Bids[0].Price
		quote.BidQuantity = depth.Bids[0].Quantity
	}
	if len(depth.Asks) > 0 {
		quote.AskPrice = depth.Asks[0].Price
		quote.AskQuantity = depth.Asks[0].Quantity
	}

	s.quotes[symbol] = quote
	return quote, nil
}

func (s *MarketService) startQuoteUpdater() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		orderBooks := s.matchingEngine.GetAllOrderBooks()
		for symbol := range orderBooks {
			_, _ = s.updateQuote(context.Background(), symbol)
		}
	}
}

func (s *MarketService) SubscribeQuote(ctx context.Context, symbol string, ch chan<- *Quote) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			quote, err := s.GetQuote(ctx, symbol)
			if err == nil && quote != nil {
				select {
				case ch <- quote:
				default:
				}
			}
		}
	}
}
