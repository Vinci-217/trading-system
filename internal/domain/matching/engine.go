package matching

import (
	"context"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/stock-trading-system/internal/domain/order"
	"github.com/stock-trading-system/internal/domain/trade"
)

type MatchingEngine struct {
	orderBooks map[string]*OrderBook
	mu         sync.RWMutex
}

func NewMatchingEngine() *MatchingEngine {
	return &MatchingEngine{
		orderBooks: make(map[string]*OrderBook),
	}
}

func (e *MatchingEngine) GetOrCreateOrderBook(symbol string) *OrderBook {
	e.mu.RLock()
	ob, exists := e.orderBooks[symbol]
	e.mu.RUnlock()

	if exists {
		return ob
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if ob, exists = e.orderBooks[symbol]; exists {
		return ob
	}

	ob = NewOrderBook(symbol)
	e.orderBooks[symbol] = ob

	return ob
}

func (e *MatchingEngine) SubmitOrder(ctx context.Context, o *order.Order) ([]*trade.Trade, error) {
	orderBook := e.GetOrCreateOrderBook(o.Symbol)

	orderBook.mu.Lock()
	defer orderBook.mu.Unlock()

	var trades []*trade.Trade

	if o.Side == order.OrderSideBuy {
		trades = e.matchBuyOrder(orderBook, o)
	} else {
		trades = e.matchSellOrder(orderBook, o)
	}

	if o.RemainingQuantity() > 0 {
		orderBook.AddOrderUnsafe(o)
	}

	orderBook.UpdatedAt = time.Now()

	return trades, nil
}

func (e *MatchingEngine) matchBuyOrder(orderBook *OrderBook, buyOrder *order.Order) []*trade.Trade {
	var trades []*trade.Trade

	for buyOrder.RemainingQuantity() > 0 {
		sellOrder := orderBook.GetBestSellOrderUnsafe()

		if sellOrder == nil {
			break
		}

		buyPrice := *buyOrder.Price
		sellPrice := *sellOrder.Price

		if buyPrice.LessThan(sellPrice) {
			break
		}

		tradeQty := min(buyOrder.RemainingQuantity(), sellOrder.RemainingQuantity())
		tradePrice := sellPrice

		t := trade.NewTrade(
			generateTradeID(),
			buyOrder.OrderID,
			buyOrder.UserID,
			buyOrder.Symbol,
			int(buyOrder.Side),
			tradePrice,
			tradeQty,
		)
		t.SetCounterParty(sellOrder.UserID, sellOrder.OrderID)
		trades = append(trades, t)

		buyOrder.Fill(tradeQty, tradePrice)
		sellOrder.Fill(tradeQty, tradePrice)

		orderBook.UpdateStats(tradePrice, tradeQty)

		if sellOrder.RemainingQuantity() == 0 {
			orderBook.RemoveOrderUnsafe(sellOrder.OrderID)
		}
	}

	return trades
}

func (e *MatchingEngine) matchSellOrder(orderBook *OrderBook, sellOrder *order.Order) []*trade.Trade {
	var trades []*trade.Trade

	for sellOrder.RemainingQuantity() > 0 {
		buyOrder := orderBook.GetBestBuyOrderUnsafe()

		if buyOrder == nil {
			break
		}

		sellPrice := *sellOrder.Price
		buyPrice := *buyOrder.Price

		if sellPrice.GreaterThan(buyPrice) {
			break
		}

		tradeQty := min(sellOrder.RemainingQuantity(), buyOrder.RemainingQuantity())
		tradePrice := buyPrice

		t := trade.NewTrade(
			generateTradeID(),
			sellOrder.OrderID,
			sellOrder.UserID,
			sellOrder.Symbol,
			int(sellOrder.Side),
			tradePrice,
			tradeQty,
		)
		t.SetCounterParty(buyOrder.UserID, buyOrder.OrderID)
		trades = append(trades, t)

		sellOrder.Fill(tradeQty, tradePrice)
		buyOrder.Fill(tradeQty, tradePrice)

		orderBook.UpdateStats(tradePrice, tradeQty)

		if buyOrder.RemainingQuantity() == 0 {
			orderBook.RemoveOrderUnsafe(buyOrder.OrderID)
		}
	}

	return trades
}

func (e *MatchingEngine) CancelOrder(symbol, orderID string) bool {
	orderBook := e.GetOrderBook(symbol)
	if orderBook == nil {
		return false
	}
	return orderBook.RemoveOrder(orderID)
}

func (e *MatchingEngine) GetOrderBook(symbol string) *OrderBook {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.orderBooks[symbol]
}

func (e *MatchingEngine) GetAllOrderBooks() map[string]*OrderBook {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]*OrderBook)
	for k, v := range e.orderBooks {
		result[k] = v
	}
	return result
}

func generateTradeID() string {
	return "T" + time.Now().Format("20060102150405") + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (e *MatchingEngine) GetMarketDepth(symbol string, level int) *MarketDepth {
	orderBook := e.GetOrderBook(symbol)
	if orderBook == nil {
		return &MarketDepth{Symbol: symbol}
	}
	return orderBook.GetMarketDepth(level)
}

func (e *MatchingEngine) GetLastPrice(symbol string) decimal.Decimal {
	orderBook := e.GetOrderBook(symbol)
	if orderBook == nil {
		return decimal.Zero
	}
	return orderBook.LastPrice
}
