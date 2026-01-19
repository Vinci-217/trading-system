package domain

import (
	"context"

	"github.com/shopspring/decimal"
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

func (e *MatchingEngine) SubmitOrder(ctx context.Context, order *Order) ([]*Trade, error) {
	orderBook := e.GetOrCreateOrderBook(order.Symbol)

	orderBook.mu.Lock()
	defer orderBook.mu.Unlock()

	var trades []*Trade

	if order.Side == OrderSideBuy {
		trades = e.matchBuyOrder(orderBook, order)
	} else {
		trades = e.matchSellOrder(orderBook, order)
	}

	if order.RemainingQuantity() > 0 {
		orderBook.AddOrder(order)
	}

	orderBook.UpdatedAt = time.Now()

	return trades, nil
}

func (e *MatchingEngine) matchBuyOrder(orderBook *OrderBook, buyOrder *Order) []*Trade {
	var trades []*Trade

	for buyOrder.RemainingQuantity() > 0 {
		sellOrder := orderBook.GetBestSellOrder()

		if sellOrder == nil {
			break
		}

		if buyOrder.Price.LessThan(sellOrder.Price) {
			break
		}

		tradeQty := min(buyOrder.RemainingQuantity(), sellOrder.RemainingQuantity())
		tradePrice := sellOrder.Price

		trade := NewTrade(
			GenerateTradeID(),
			orderBook.Symbol,
			buyOrder,
			sellOrder,
			tradePrice,
			tradeQty,
		)
		trades = append(trades, trade)

		buyOrder.Fill(tradeQty)
		sellOrder.Fill(tradeQty)

		orderBook.LastPrice = tradePrice
		orderBook.TotalVolume += int64(tradeQty)
		orderBook.TotalValue = orderBook.TotalValue.Add(tradePrice.Mul(decimal.NewFromInt(int64(tradeQty))))

		if sellOrder.IsFilled() {
			orderBook.RemoveOrder(sellOrder.ID)
		}
	}

	return trades
}

func (e *MatchingEngine) matchSellOrder(orderBook *OrderBook, sellOrder *Order) []*Trade {
	var trades []*Trade

	for sellOrder.RemainingQuantity() > 0 {
		buyOrder := orderBook.GetBestBuyOrder()

		if buyOrder == nil {
			break
		}

		if sellOrder.Price.GreaterThan(buyOrder.Price) {
			break
		}

		tradeQty := min(sellOrder.RemainingQuantity(), buyOrder.RemainingQuantity())
		tradePrice := buyOrder.Price

		trade := NewTrade(
			GenerateTradeID(),
			orderBook.Symbol,
			buyOrder,
			sellOrder,
			tradePrice,
			tradeQty,
		)
		trades = append(trades, trade)

		buyOrder.Fill(tradeQty)
		sellOrder.Fill(tradeQty)

		orderBook.LastPrice = tradePrice
		orderBook.TotalVolume += int64(tradeQty)
		orderBook.TotalValue = orderBook.TotalValue.Add(tradePrice.Mul(decimal.NewFromInt(int64(tradeQty))))

		if buyOrder.IsFilled() {
			orderBook.RemoveOrder(buyOrder.ID)
		}
	}

	return trades
}

func (e *MatchingEngine) CancelOrder(orderBook *OrderBook, orderID string) bool {
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

import (
	"sync"
	"time"
)

func GenerateTradeID() string {
	return time.Now().Format("20060102150405") + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}
