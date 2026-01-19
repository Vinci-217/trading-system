package matching

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"stock_trader/internal/kafka"
	"stock_trader/internal/model"
	"stock_trader/internal/repository"
)

type Order struct {
	OrderID   string
	UserID    int64
	Symbol    string
	Side      model.OrderSide
	Price     float64
	Quantity  int
	Timestamp time.Time
	Index     int
}

type OrderQueue []*Order

func (o OrderQueue) Len() int { return len(o) }

func (o OrderQueue) Less(i, j int) bool {
	if o[i].Price == o[j].Price {
		return o[i].Timestamp.Before(o[j].Timestamp)
	}
	return o[i].Price > o[j].Price
}

func (o OrderQueue) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
	o[i].Index = i
	o[j].Index = j
}

func (o *OrderQueue) Push(x interface{}) {
	n := len(*o)
	order := x.(*Order)
	order.Index = n
	*o = append(*o, order)
}

func (o *OrderQueue) Pop() interface{} {
	old := *o
	n := len(old)
	order := old[n-1]
	old[n-1] = nil
	order.Index = -1
	*o = old[0 : n-1]
	return order
}

func (o *OrderQueue) Peek() *Order {
	if len(*o) == 0 {
		return nil
	}
	return (*o)[0]
}

type Engine struct {
	repo   *repository.Repository
	kafka  kafka.Producer

	orderBooks map[string]*OrderBook
	mu         sync.RWMutex

	tradeChan chan *TradeResult
}

type OrderBook struct {
	Symbol     string
	BuyOrders  *OrderQueue
	SellOrders *OrderQueue
}

type TradeResult struct {
	TradeID        string
	OrderID        string
	CounterOrderID string
	UserID         int64
	CounterUserID  int64
	Symbol         string
	Side           model.OrderSide
	Price          float64
	Quantity       int
	Amount         float64
	Timestamp      time.Time
}

func NewEngine(repo *repository.Repository, kafkaClient kafka.Producer) *Engine {
	e := &Engine{
		repo:      repo,
		kafka:     kafkaClient,
		orderBooks: make(map[string]*OrderBook),
		tradeChan:  make(chan *TradeResult, 10000),
	}

	buyQueue := make(OrderQueue, 0)
	sellQueue := make(OrderQueue, 0)
	heap.Init(&buyQueue)
	heap.Init(&sellQueue)

	e.orderBooks["default"] = &OrderBook{
		Symbol:     "default",
		BuyOrders:  &buyQueue,
		SellOrders: &sellQueue,
	}

	go e.processTrades()

	return e
}

func (e *Engine) Match(order *Order) []*TradeResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	orderBook, exists := e.orderBooks[order.Symbol]
	if !exists {
		buyQueue := make(OrderQueue, 0)
		sellQueue := make(OrderQueue, 0)
		heap.Init(&buyQueue)
		heap.Init(&sellQueue)

		orderBook = &OrderBook{
			Symbol:     order.Symbol,
			BuyOrders:  &buyQueue,
			SellOrders: &sellQueue,
		}
		e.orderBooks[order.Symbol] = orderBook
	}

	var trades []*TradeResult

	if order.Side == model.OrderSideBuy {
		trades = e.matchBuyOrder(order, orderBook)
	} else {
		trades = e.matchSellOrder(order, orderBook)
	}

	return trades
}

func (e *Engine) matchBuyOrder(order *Order, orderBook *OrderBook) []*TradeResult {
	var trades []*TradeResult

	for order.Quantity > 0 && orderBook.SellOrders.Len() > 0 {
		bestSell := orderBook.SellOrders.Peek()

		if !e.canMatch(order.Price, bestSell.Price) {
			break
		}

		tradeQty := min(order.Quantity, bestSell.Quantity)
		tradePrice := e.determinePrice(order.Price, bestSell.Price)

		trade := &TradeResult{
			TradeID:        uuid.New().String(),
			OrderID:        order.OrderID,
			CounterOrderID: bestSell.OrderID,
			UserID:         order.UserID,
			CounterUserID:  bestSell.UserID,
			Symbol:         order.Symbol,
			Side:           model.OrderSideBuy,
			Price:          tradePrice,
			Quantity:       tradeQty,
			Amount:         tradePrice * float64(tradeQty),
			Timestamp:      time.Now(),
		}
		trades = append(trades, trade)

		order.Quantity -= tradeQty
		bestSell.Quantity -= tradeQty

		heap.Pop(orderBook.SellOrders)

		if bestSell.Quantity > 0 {
			heap.Push(orderBook.SellOrders, bestSell)
		}
	}

	if order.Quantity > 0 {
		heap.Push(orderBook.BuyOrders, order)
	}

	return trades
}

func (e *Engine) matchSellOrder(order *Order, orderBook *OrderBook) []*TradeResult {
	var trades []*TradeResult

	for order.Quantity > 0 && orderBook.BuyOrders.Len() > 0 {
		bestBuy := orderBook.BuyOrders.Peek()

		if !e.canMatch(order.Price, bestBuy.Price) {
			break
		}

		tradeQty := min(order.Quantity, bestBuy.Quantity)
		tradePrice := e.determinePrice(bestBuy.Price, order.Price)

		trade := &TradeResult{
			TradeID:        uuid.New().String(),
			OrderID:        order.OrderID,
			CounterOrderID: bestBuy.OrderID,
			UserID:         order.UserID,
			CounterUserID:  bestBuy.UserID,
			Symbol:         order.Symbol,
			Side:           model.OrderSideSell,
			Price:          tradePrice,
			Quantity:       tradeQty,
			Amount:         tradePrice * float64(tradeQty),
			Timestamp:      time.Now(),
		}
		trades = append(trades, trade)

		order.Quantity -= tradeQty
		bestBuy.Quantity -= tradeQty

		heap.Pop(orderBook.BuyOrders)

		if bestBuy.Quantity > 0 {
			heap.Push(orderBook.BuyOrders, bestBuy)
		}
	}

	if order.Quantity > 0 {
		heap.Push(orderBook.SellOrders, order)
	}

	return trades
}

func (e *Engine) canMatch(buyPrice, sellPrice float64) bool {
	return buyPrice >= sellPrice
}

func (e *Engine) determinePrice(buyPrice, sellPrice float64) float64 {
	return sellPrice
}

func (e *Engine) processTrades() {
	for trade := range e.tradeChan {
		e.publishTrade(trade)
	}
}

func (e *Engine) publishTrade(trade *TradeResult) {
	ctx := context.Background()

	tradeEvent := &kafka.TradeEvent{
		TradeID:        trade.TradeID,
		OrderID:        trade.OrderID,
		CounterOrderID: trade.CounterOrderID,
		UserID:         trade.UserID,
		Symbol:         trade.Symbol,
		Side:           string(trade.Side),
		Price:          trade.Price,
		Quantity:       trade.Quantity,
		Amount:         trade.Amount,
		Fee:            trade.Amount * 0.0003,
	}

	if err := e.kafka.SendTradeExecuted(ctx, tradeEvent); err != nil {
		fmt.Printf("发送成交事件失败: %v\n", err)
	}
}

func (e *Engine) AddTrade(trade *TradeResult) {
	e.tradeChan <- trade
}

func (e *Engine) GetOrderBook(symbol string) (buyOrders, sellOrders []*Order) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	orderBook, exists := e.orderBooks[symbol]
	if !exists {
		return nil, nil
	}

	for _, order := range *orderBook.BuyOrders {
		buyOrders = append(buyOrders, order)
	}
	for _, order := range *orderBook.SellOrders {
		sellOrders = append(sellOrders, order)
	}

	return buyOrders, sellOrders
}

func (e *Engine) GetOrderBookDepth(symbol string, depth int) (buyDepth, sellDepth []DepthLevel) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	orderBook, exists := e.orderBooks[symbol]
	if !exists {
		return nil, nil
	}

	buyMap := make(map[float64]int)
	for _, order := range *orderBook.BuyOrders {
		buyMap[order.Price] += order.Quantity
	}
	for price, qty := range buyMap {
		buyDepth = append(buyDepth, DepthLevel{Price: price, Quantity: qty})
	}

	sellMap := make(map[float64]int)
	for _, order := range *orderBook.SellOrders {
		sellMap[order.Price] += order.Quantity
	}
	for price, qty := range sellMap {
		sellDepth = append(sellDepth, DepthLevel{Price: price, Quantity: qty})
	}

	return buyDepth, sellDepth
}

type DepthLevel struct {
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
