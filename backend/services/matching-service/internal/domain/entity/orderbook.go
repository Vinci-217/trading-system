package domain

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

type OrderBook struct {
	Symbol      string
	BuyOrders   *OrderQueue
	SellOrders  *OrderQueue
	LastPrice   decimal.Decimal
	TotalVolume int64
	TotalValue  decimal.Decimal
	OrderCount  int64
	UpdatedAt   time.Time
	mu          sync.RWMutex
}

type OrderQueue struct {
	Orders []*Order
	Index  map[string]int
}

func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		Symbol:     symbol,
		BuyOrders:  NewOrderQueue(),
		SellOrders: NewOrderQueue(),
		LastPrice:  decimal.NewFromFloat(100.00),
		TotalValue: decimal.Zero,
		UpdatedAt:  time.Now(),
	}
}

func NewOrderQueue() *OrderQueue {
	return &OrderQueue{
		Orders: make([]*Order, 0),
		Index:  make(map[string]int),
	}
}

func (ob *OrderBook) AddOrder(order *Order) {
	if order.Side == OrderSideBuy {
		ob.BuyOrders.Add(order)
	} else {
		ob.SellOrders.Add(order)
	}
	ob.OrderCount++
	ob.UpdatedAt = time.Now()
}

func (ob *OrderBook) RemoveOrder(orderID string) bool {
	if ob.BuyOrders.Remove(orderID) {
		ob.UpdatedAt = time.Now()
		return true
	}
	if ob.SellOrders.Remove(orderID) {
		ob.UpdatedAt = time.Now()
		return true
	}
	return false
}

func (ob *OrderBook) GetBestBuyOrder() *Order {
	return ob.BuyOrders.GetBest()
}

func (ob *OrderBook) GetBestSellOrder() *Order {
	return ob.SellOrders.GetBest()
}

func (ob *OrderBook) HasOrders() bool {
	return ob.BuyOrders.Size() > 0 && ob.SellOrders.Size() > 0
}

func (ob *OrderBook) GetMarketDepth() *MarketDepth {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	return &MarketDepth{
		Symbol:    ob.Symbol,
		Bids:      ob.BuyOrders.GetTopLevels(100),
		Asks:      ob.SellOrders.GetTopLevels(100),
		Timestamp: ob.UpdatedAt,
	}
}

func (q *OrderQueue) Add(order *Order) {
	q.Orders = append(q.Orders, order)
	q.Index[order.ID] = len(q.Orders) - 1
}

func (q *OrderQueue) Remove(orderID string) bool {
	idx, exists := q.Index[orderID]
	if !exists {
		return false
	}

	lastIdx := len(q.Orders) - 1
	if idx < lastIdx {
		q.Orders[idx] = q.Orders[lastIdx]
		q.Index[q.Orders[idx].ID] = idx
	}

	q.Orders = q.Orders[:lastIdx]
	delete(q.Index, orderID)

	return true
}

func (q *OrderQueue) GetBest() *Order {
	if len(q.Orders) == 0 {
		return nil
	}

	best := q.Orders[0]
	for _, order := range q.Orders {
		if order.Timestamp.Before(best.Timestamp) {
			best = order
		}
	}

	return best
}

func (q *OrderQueue) Size() int {
	return len(q.Orders)
}

func (q *OrderQueue) GetTopLevels(limit int) []DepthLevel {
	priceLevels := make(map[string]int)
	orderCounts := make(map[string]int)

	for _, order := range q.Orders {
		priceStr := order.Price.String()
		priceLevels[priceStr] += order.Quantity
		orderCounts[priceStr]++
	}

	var levels []DepthLevel
	for priceStr, qty := range priceLevels {
		price, _ := decimal.NewFromString(priceStr)
		levels = append(levels, DepthLevel{
			Price:    price,
			Quantity: qty,
			Orders:   orderCounts[priceStr],
		})
	}

	return levels[:min(len(levels), limit)]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
