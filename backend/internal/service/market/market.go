package market

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"stock_trader/internal/cache"
	"stock_trader/internal/kafka"
	"stock_trader/internal/model"
)

type Service struct {
	cache  *cache.RedisCache
	kafka  kafka.Producer
	logger interface{}

	symbols map[string]*model.Stock
	mu      sync.RWMutex
}

func NewService(cache *cache.RedisCache, kafkaClient kafka.Producer, logger interface{}) *Service {
	svc := &Service{
		cache:   cache,
		kafka:   kafkaClient,
		logger:  logger,
		symbols: make(map[string]*model.Stock),
	}
	svc.initMockData()
	go svc.simulateMarket()
	return svc
}

func (s *Service) initMockData() {
	mockStocks := []model.Stock{
		{Symbol: "600519", SymbolName: "贵州茅台", Exchange: "SHSE", Currency: "CNY", LotSize: 100, PriceTick: 0.01, Status: model.StockStatusActive},
		{Symbol: "000001", SymbolName: "平安银行", Exchange: "SZSE", Currency: "CNY", LotSize: 100, PriceTick: 0.01, Status: model.StockStatusActive},
		{Symbol: "600036", SymbolName: "招商银行", Exchange: "SHSE", Currency: "CNY", LotSize: 100, PriceTick: 0.01, Status: model.StockStatusActive},
		{Symbol: "000002", SymbolName: "万 科Ａ", Exchange: "SZSE", Currency: "CNY", LotSize: 100, PriceTick: 0.01, Status: model.StockStatusActive},
		{Symbol: "600276", SymbolName: "恒瑞医药", Exchange: "SHSE", Currency: "CNY", LotSize: 100, PriceTick: 0.01, Status: model.StockStatusActive},
		{Symbol: "000651", SymbolName: "格力电器", Exchange: "SZSE", Currency: "CNY", LotSize: 100, PriceTick: 0.01, Status: model.StockStatusActive},
		{Symbol: "600030", SymbolName: "中信证券", Exchange: "SHSE", Currency: "CNY", LotSize: 100, PriceTick: 0.01, Status: model.StockStatusActive},
		{Symbol: "000725", SymbolName: "京东方Ａ", Exchange: "SZSE", Currency: "CNY", LotSize: 100, PriceTick: 0.01, Status: model.StockStatusActive},
		{Symbol: "600887", SymbolName: "伊利股份", Exchange: "SHSE", Currency: "CNY", LotSize: 100, PriceTick: 0.01, Status: model.StockStatusActive},
		{Symbol: "002594", SymbolName: "比亚迪", Exchange: "SZSE", Currency: "CNY", LotSize: 100, PriceTick: 0.01, Status: model.StockStatusActive},
		{Symbol: "AAPL", SymbolName: "苹果公司", Exchange: "NASDAQ", Currency: "USD", LotSize: 1, PriceTick: 0.01, Status: model.StockStatusActive},
		{Symbol: "GOOGL", SymbolName: "谷歌公司", Exchange: "NASDAQ", Currency: "USD", LotSize: 1, PriceTick: 0.01, Status: model.StockStatusActive},
		{Symbol: "MSFT", SymbolName: "微软公司", Exchange: "NASDAQ", Currency: "USD", LotSize: 1, PriceTick: 0.01, Status: model.StockStatusActive},
	}

	s.mu.Lock()
	for i := range mockStocks {
		stock := &mockStocks[i]
		switch stock.Symbol {
		case "600519":
			stock.LimitUp, stock.LimitDown = 1700, 1400
		case "000001":
			stock.LimitUp, stock.LimitDown = 12, 10
		case "600036":
			stock.LimitUp, stock.LimitDown = 45, 38
		default:
			stock.LimitUp, stock.LimitDown = stock.PriceTick*100, -stock.PriceTick*100
		}
		s.symbols[stock.Symbol] = stock
	}
	s.mu.Unlock()
}

func (s *Service) GetQuote(ctx context.Context, symbol string) (*model.Quote, error) {
	s.mu.RLock()
	stock, exists := s.symbols[symbol]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("股票不存在: %s", symbol)
	}

	price := s.getCurrentPrice(symbol)
	prevClose := price * (1 + (rand.Float64()-0.5)*0.02)
	change := price - prevClose
	changePercent := change / prevClose * 100

	return &model.Quote{
		Symbol:        symbol,
		SymbolName:    stock.SymbolName,
		Price:         price,
		PrevClose:     prevClose,
		Change:        change,
		ChangePercent: changePercent,
		High:          price * 1.02,
		Low:           price * 0.98,
		Open:          prevClose * (1 + (rand.Float64()-0.5)*0.01),
		Volume:        int64(rand.Intn(10000000) + 1000000),
		Amount:        price * float64(rand.Intn(10000000)+1000000),
		BidPrice:      price * 0.9998,
		AskPrice:      price * 1.0002,
		BidVolume:     int64(rand.Intn(10000) + 1000),
		AskVolume:     int64(rand.Intn(10000) + 1000),
		Timestamp:     time.Now(),
	}, nil
}

func (s *Service) GetKLine(ctx context.Context, symbol, period string, limit int) ([]*model.KLine, error) {
	s.mu.RLock()
	_, exists := s.symbols[symbol]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("股票不存在: %s", symbol)
	}

	klines := make([]*model.KLine, 0, limit)
	basePrice := s.getCurrentPrice(symbol)
	now := time.Now()

	for i := limit - 1; i >= 0; i-- {
		var interval time.Duration
		switch period {
		case "1m":
			interval = time.Minute
		case "5m":
			interval = 5 * time.Minute
		case "15m":
			interval = 15 * time.Minute
		case "1h":
			interval = time.Hour
		case "4h":
			interval = 4 * time.Hour
		case "1d":
			interval = 24 * time.Hour
		case "1w":
			interval = 7 * 24 * time.Hour
		default:
			interval = time.Hour
		}

		timestamp := now.Add(-interval * time.Duration(i+1))
		volatility := 0.02
		open := basePrice * (1 + (rand.Float64()-0.5)*volatility*2)
		high := open * (1 + rand.Float64()*volatility)
		low := open * (1 - rand.Float64()*volatility)
		close := (open + high + low) / 3 * (1 + (rand.Float64()-0.5)*volatility)
		volume := int64(rand.Intn(10000000) + 1000000)

		klines = append(klines, &model.KLine{
			Symbol:    symbol,
			Period:    period,
			Timestamp: timestamp,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			Amount:    close * float64(volume),
		})
	}

	return klines, nil
}

func (s *Service) GetSymbols() ([]*model.Stock, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.Stock, 0, len(s.symbols))
	for _, stock := range s.symbols {
		result = append(result, stock)
	}
	return result, nil
}

func (s *Service) getCurrentPrice(symbol string) float64 {
	s.mu.RLock()
	stock, exists := s.symbols[symbol]
	s.mu.RUnlock()

	if !exists {
		return 0
	}

	basePrice := s.getBasePrice(symbol)
	volatility := 0.005
	change := (rand.Float64() - 0.5) * 2 * volatility
	return basePrice * (1 + change)
}

func (s *Service) getBasePrice(symbol string) float64 {
	prices := map[string]float64{
		"600519": 1600,
		"000001": 11,
		"600036": 42,
		"000002": 12,
		"600276": 48,
		"000651": 38,
		"600030": 22,
		"000725": 7,
		"600887": 18,
		"002594": 250,
		"AAPL":   185,
		"GOOGL":  140,
		"MSFT":   380,
	}
	return prices[symbol]
}

func (s *Service) simulateMarket() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		for symbol, stock := range s.symbols {
			basePrice := s.getBasePrice(symbol)
			change := (rand.Float64() - 0.5) * 0.01
			stock.LimitUp = basePrice * 1.10
			stock.LimitDown = basePrice * 0.90
		}
		s.mu.Unlock()
	}
}

func (s *Service) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var req struct {
			Symbol string `json:"symbol"`
			Action string `json:"action"`
		}
		json.Unmarshal(message, &req)

		if req.Action == "subscribe" {
			go s.subscribeQuote(conn, req.Symbol)
		}
	}
}

func (s *Service) subscribeQuote(conn *websocket.Conn, symbol string) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			quote, _ := s.GetQuote(context.Background(), symbol)
			if quote != nil {
				data, _ := json.Marshal(quote)
				conn.WriteMessage(websocket.TextMessage, data)
			}
		}
	}
}

import (
	"encoding/json"
	"net/http"
	"github.com/gorilla/websocket"
)
