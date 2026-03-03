package matching

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/stock-trading-system/internal/domain/order"
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
	Orders []*order.Order
	Index  map[string]int
	mu     sync.RWMutex
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
		Orders: make([]*order.Order, 0),
		Index:  make(map[string]int),
	}
}

func (ob *OrderBook) AddOrder(o *order.Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if o.Side == order.OrderSideBuy {
		ob.BuyOrders.Add(o)
	} else {
		ob.SellOrders.Add(o)
	}
	ob.OrderCount++
	ob.UpdatedAt = time.Now()
}

func (ob *OrderBook) AddOrderUnsafe(o *order.Order) {
	if o.Side == order.OrderSideBuy {
		ob.BuyOrders.Add(o)
	} else {
		ob.SellOrders.Add(o)
	}
	ob.OrderCount++
	ob.UpdatedAt = time.Now()
}

func (ob *OrderBook) RemoveOrder(orderID string) bool {
	ob.mu.Lock()
	defer ob.mu.Unlock()

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

func (ob *OrderBook) RemoveOrderUnsafe(orderID string) bool {
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

func (ob *OrderBook) GetBestBuyOrder() *order.Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.BuyOrders.GetBestBuy()
}

func (ob *OrderBook) GetBestSellOrder() *order.Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.SellOrders.GetBestSell()
}

func (ob *OrderBook) GetBestBuyOrderUnsafe() *order.Order {
	return ob.BuyOrders.GetBestBuy()
}

func (ob *OrderBook) GetBestSellOrderUnsafe() *order.Order {
	return ob.SellOrders.GetBestSell()
}

func (ob *OrderBook) GetOrder(orderID string) *order.Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if o := ob.BuyOrders.Get(orderID); o != nil {
		return o
	}
	return ob.SellOrders.Get(orderID)
}

func (ob *OrderBook) UpdateStats(price decimal.Decimal, quantity int) {
	ob.LastPrice = price
	ob.TotalVolume += int64(quantity)
	ob.TotalValue = ob.TotalValue.Add(price.Mul(decimal.NewFromInt(int64(quantity))))
	ob.UpdatedAt = time.Now()
}

func (q *OrderQueue) Add(o *order.Order) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.Orders = append(q.Orders, o)
	q.Index[o.OrderID] = len(q.Orders) - 1
}

func (q *OrderQueue) Remove(orderID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	idx, exists := q.Index[orderID]
	if !exists {
		return false
	}

	lastIdx := len(q.Orders) - 1
	if idx < lastIdx {
		q.Orders[idx] = q.Orders[lastIdx]
		q.Index[q.Orders[idx].OrderID] = idx
	}

	q.Orders = q.Orders[:lastIdx]
	delete(q.Index, orderID)

	return true
}

func (q *OrderQueue) Get(orderID string) *order.Order {
	q.mu.RLock()
	defer q.mu.RUnlock()

	idx, exists := q.Index[orderID]
	if !exists {
		return nil
	}
	return q.Orders[idx]
}

func (q *OrderQueue) GetBestBuy() *order.Order {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.Orders) == 0 {
		return nil
	}

	best := q.Orders[0]
	for _, o := range q.Orders {
		bestPrice := *best.Price
		oPrice := *o.Price
		if oPrice.GreaterThan(bestPrice) {
			best = o
		} else if oPrice.Equal(bestPrice) && o.CreatedAt.Before(best.CreatedAt) {
			best = o
		}
	}
	return best
}

func (q *OrderQueue) GetBestSell() *order.Order {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.Orders) == 0 {
		return nil
	}

	best := q.Orders[0]
	for _, o := range q.Orders {
		bestPrice := *best.Price
		oPrice := *o.Price
		if oPrice.LessThan(bestPrice) {
			best = o
		} else if oPrice.Equal(bestPrice) && o.CreatedAt.Before(best.CreatedAt) {
			best = o
		}
	}
	return best
}

func (q *OrderQueue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.Orders)
}

type DepthLevel struct {
	Price      decimal.Decimal
	Quantity   int
	OrderCount int
}

type MarketDepth struct {
	Symbol    string
	Bids      []DepthLevel
	Asks      []DepthLevel
	Timestamp time.Time
}

func (ob *OrderBook) GetMarketDepth(level int) *MarketDepth {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	return &MarketDepth{
		Symbol:    ob.Symbol,
		Bids:      ob.BuyOrders.GetTopLevels(level),
		Asks:      ob.SellOrders.GetTopLevels(level),
		Timestamp: ob.UpdatedAt,
	}
}

func (q *OrderQueue) GetTopLevels(limit int) []DepthLevel {
	q.mu.RLock()
	defer q.mu.RUnlock()

	priceLevels := make(map[string]int)
	orderCounts := make(map[string]int)

	for _, o := range q.Orders {
		priceStr := (*o.Price).String()
		priceLevels[priceStr] += o.RemainingQuantity()
		orderCounts[priceStr]++
	}

	var levels []DepthLevel
	for priceStr, qty := range priceLevels {
		price, _ := decimal.NewFromString(priceStr)
		levels = append(levels, DepthLevel{
			Price:      price,
			Quantity:   qty,
			OrderCount: orderCounts[priceStr],
		})
	}

	if len(levels) > limit {
		return levels[:limit]
	}
	return levels
}
