package matching

import (
	"context"
	"sync"
	"time"

	"github.com/stock-trading-system/internal/domain/matching"
	"github.com/stock-trading-system/internal/domain/order"
	"github.com/stock-trading-system/internal/domain/trade"
	"github.com/stock-trading-system/pkg/logger"

	"github.com/shopspring/decimal"
)

type MatchingService struct {
	engine *matching.MatchingEngine
	logger *logger.Logger
	mu     sync.RWMutex
}

func NewMatchingService(logger *logger.Logger) *MatchingService {
	return &MatchingService{
		engine: matching.NewMatchingEngine(),
		logger: logger,
	}
}

func (s *MatchingService) SubmitOrder(ctx context.Context, o *order.Order) ([]*trade.Trade, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	trades, err := s.engine.SubmitOrder(ctx, o)
	if err != nil {
		s.logger.Errorw("failed to submit order to matching engine",
			"order_id", o.OrderID,
			"symbol", o.Symbol,
			"side", o.Side,
			"price", o.Price.String(),
			"quantity", o.Quantity,
			"error", err)
		return nil, err
	}

	if len(trades) > 0 {
		s.logger.Info("order matched",
			"order_id", o.OrderID,
			"symbol", o.Symbol,
			"trade_count", len(trades))
	}

	return trades, nil
}

func (s *MatchingService) CancelOrder(ctx context.Context, symbol, orderID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	success := s.engine.CancelOrder(symbol, orderID)
	if !success {
		s.logger.Warn("failed to cancel order, order not found in matching engine",
			"order_id", orderID,
			"symbol", symbol)
		return nil
	}

	s.logger.Info("order cancelled from matching engine",
		"order_id", orderID,
		"symbol", symbol)
	return nil
}

func (s *MatchingService) GetOrderBook(ctx context.Context, symbol string) *matching.OrderBook {
	return s.engine.GetOrderBook(symbol)
}

func (s *MatchingService) GetMarketDepth(ctx context.Context, symbol string, level int) *matching.MarketDepth {
	return s.engine.GetMarketDepth(symbol, level)
}

func (s *MatchingService) GetLastPrice(ctx context.Context, symbol string) decimal.Decimal {
	return s.engine.GetLastPrice(symbol)
}

func (s *MatchingService) GetAllOrderBooks(ctx context.Context) map[string]*matching.OrderBook {
	return s.engine.GetAllOrderBooks()
}

func (s *MatchingService) GetBestBid(ctx context.Context, symbol string) (decimal.Decimal, int) {
	ob := s.engine.GetOrderBook(symbol)
	if ob == nil {
		return decimal.Zero, 0
	}
	best := ob.GetBestBuyOrder()
	if best == nil || best.Price == nil {
		return decimal.Zero, 0
	}
	return *best.Price, best.RemainingQuantity()
}

func (s *MatchingService) GetBestAsk(ctx context.Context, symbol string) (decimal.Decimal, int) {
	ob := s.engine.GetOrderBook(symbol)
	if ob == nil {
		return decimal.Zero, 0
	}
	best := ob.GetBestSellOrder()
	if best == nil || best.Price == nil {
		return decimal.Zero, 0
	}
	return *best.Price, best.RemainingQuantity()
}

func (s *MatchingService) GetSpread(ctx context.Context, symbol string) decimal.Decimal {
	bidPrice, _ := s.GetBestBid(ctx, symbol)
	askPrice, _ := s.GetBestAsk(ctx, symbol)
	if bidPrice.IsZero() || askPrice.IsZero() {
		return decimal.Zero
	}
	return askPrice.Sub(bidPrice)
}

func (s *MatchingService) GetMidPrice(ctx context.Context, symbol string) decimal.Decimal {
	bidPrice, _ := s.GetBestBid(ctx, symbol)
	askPrice, _ := s.GetBestAsk(ctx, symbol)
	if bidPrice.IsZero() || askPrice.IsZero() {
		return decimal.Zero
	}
	return bidPrice.Add(askPrice).Div(decimal.NewFromInt(2))
}

type OrderBookSnapshot struct {
	Symbol    string         `json:"symbol"`
	Bids      []PriceLevel   `json:"bids"`
	Asks      []PriceLevel   `json:"asks"`
	Timestamp time.Time      `json:"timestamp"`
}

type PriceLevel struct {
	Price    decimal.Decimal `json:"price"`
	Quantity int             `json:"quantity"`
	Orders   int             `json:"orders"`
}

func (s *MatchingService) GetOrderBookSnapshot(ctx context.Context, symbol string, depth int) *OrderBookSnapshot {
	marketDepth := s.engine.GetMarketDepth(symbol, depth)
	
	snapshot := &OrderBookSnapshot{
		Symbol:    symbol,
		Timestamp: time.Now(),
		Bids:      make([]PriceLevel, 0, len(marketDepth.Bids)),
		Asks:      make([]PriceLevel, 0, len(marketDepth.Asks)),
	}

	for _, d := range marketDepth.Bids {
		snapshot.Bids = append(snapshot.Bids, PriceLevel{
			Price:    d.Price,
			Quantity: d.Quantity,
			Orders:   d.OrderCount,
		})
	}

	for _, d := range marketDepth.Asks {
		snapshot.Asks = append(snapshot.Asks, PriceLevel{
			Price:    d.Price,
			Quantity: d.Quantity,
			Orders:   d.OrderCount,
		})
	}

	return snapshot
}

func (s *MatchingService) LoadPendingOrders(ctx context.Context, orders []*order.Order) int {
	loadedCount := 0
	for _, o := range orders {
		if o.Price == nil {
			p := decimal.NewFromFloat(0)
			o.Price = &p
		}
		if o.RemainingQuantity() <= 0 {
			continue
		}
		ob := s.engine.GetOrCreateOrderBook(o.Symbol)
		ob.AddOrder(o)
		loadedCount++
	}
	s.logger.Info("pending orders loaded to matching engine", "count", loadedCount)
	return loadedCount
}
