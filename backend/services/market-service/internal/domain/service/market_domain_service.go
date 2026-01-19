package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"stock_trader/backend/services/market-service/internal/domain/entity"
	"stock_trader/backend/services/market-service/internal/infrastructure/logger"

	"github.com/shopspring/decimal"
)

type MarketDomainService struct {
	quotes        *sync.Map
	klines        *sync.Map
	symbols       *sync.Map
	priceGenerator *PriceGenerator
	logger        *logger.Logger
}

type PriceGenerator struct {
	basePrices map[string]decimal.Decimal
	volatility map[string]decimal.Decimal
	mu         sync.RWMutex
}

func NewPriceGenerator() *PriceGenerator {
	return &PriceGenerator{
		basePrices: map[string]decimal.Decimal{
			"600519": decimal.NewFromFloat(1800.00),
			"000001": decimal.NewFromFloat(3500.00),
			"300750": decimal.NewFromFloat(400.00),
			"601398": decimal.NewFromFloat(5.50),
			"601988": decimal.NewFromFloat(3.50),
		},
		volatility: map[string]decimal.Decimal{
			"600519": decimal.NewFromFloat(0.02),
			"000001": decimal.NewFromFloat(0.015),
			"300750": decimal.NewFromFloat(0.03),
			"601398": decimal.NewFromFloat(0.01),
			"601988": decimal.NewFromFloat(0.01),
		},
	}
}

func (pg *PriceGenerator) GeneratePrice(symbol string, currentPrice decimal.Decimal) decimal.Decimal {
	pg.mu.RLock()
	volatility, exists := pg.volatility[symbol]
	if !exists {
		volatility = decimal.NewFromFloat(0.02)
	}
	basePrice, exists := pg.basePrices[symbol]
	if !exists {
		basePrice = currentPrice
	}
	pg.mu.RUnlock()

	changePercent := volatility.Mul(decimal.NewFromFloat(float64(time.Now().UnixNano()%1000-500) / 50000))
	newPrice := currentPrice.Mul(decimal.One.Add(changePercent))

	if newPrice.LessThan(basePrice.Mul(decimal.NewFromFloat(0.8))) {
		return basePrice.Mul(decimal.NewFromFloat(0.8))
	}
	if newPrice.GreaterThan(basePrice.Mul(decimal.NewFromFloat(1.2))) {
		return basePrice.Mul(decimal.NewFromFloat(1.2))
	}

	return newPrice.Round(2)
}

func NewMarketDomainService(logger *logger.Logger) *MarketDomainService {
	return &MarketDomainService{
		quotes:        &sync.Map{},
		klines:        &sync.Map{},
		symbols:       &sync.Map{},
		priceGenerator: NewPriceGenerator(),
		logger:        logger,
	}
}

func (s *MarketDomainService) Initialize(symbols []entity.Symbol) {
	for _, symbol := range symbols {
		s.symbols.Store(symbol.Symbol, &symbol)

		quote := entity.NewQuote(symbol.Symbol, symbol.BasePrice)
		s.quotes.Store(symbol.Symbol, quote)

		for _, interval := range []string{"1m", "5m", "15m", "1h", "1d"} {
			key := fmt.Sprintf("%s:%s", symbol.Symbol, interval)
			kline := entity.NewKLine(symbol.Symbol, interval, symbol.BasePrice)
			s.klines.Store(key, kline)
		}
	}

	s.logger.Info("行情服务初始化完成",
		logger.Int("symbols", len(symbols)))
}

func (s *MarketDomainService) GetQuote(ctx context.Context, symbol string) (*entity.Quote, error) {
	quote, exists := s.quotes.Load(symbol)
	if !exists {
		return nil, entity.ErrQuoteNotFound
	}
	return quote.(*entity.Quote), nil
}

func (s *MarketDomainService) GetBatchQuotes(ctx context.Context, symbols []string) ([]*entity.Quote, error) {
	quotes := make([]*entity.Quote, 0, len(symbols))
	for _, symbol := range symbols {
		quote, exists := s.quotes.Load(symbol)
		if !exists {
			continue
		}
		quotes = append(quotes, quote.(*entity.Quote))
	}
	return quotes, nil
}

func (s *MarketDomainService) GetKLine(ctx context.Context, symbol string, interval string) (*entity.KLine, error) {
	key := fmt.Sprintf("%s:%s", symbol, interval)
	kline, exists := s.klines.Load(key)
	if !exists {
		return nil, entity.ErrKLineNotFound
	}
	return kline.(*entity.KLine), nil
}

func (s *MarketDomainService) GetKLines(ctx context.Context, symbol string, interval string, limit int) ([]*entity.KLine, error) {
	var klines []*entity.KLine
	for i := 0; i < limit; i++ {
		key := fmt.Sprintf("%s:%s:%d", symbol, interval, i)
		kline, exists := s.klines.Load(key)
		if !exists {
			break
		}
		klines = append(klines, kline.(*entity.KLine))
	}
	return klines, nil
}

func (s *MarketDomainService) UpdateMarketData() {
	symbols := s.getAllSymbols()

	for _, symbol := range symbols {
		quote, exists := s.quotes.Load(symbol)
		if !exists {
			continue
		}

		q := quote.(*entity.Quote)
		newPrice := s.priceGenerator.GeneratePrice(symbol, q.Price)
		q.UpdatePrice(newPrice)
		s.quotes.Store(symbol, q)

		s.updateKLines(symbol, q.Price)
	}
}

func (s *MarketDomainService) updateKLines(symbol string, price decimal.Decimal) {
	intervals := []string{"1m", "5m", "15m", "1h", "1d"}

	for _, interval := range intervals {
		key := fmt.Sprintf("%s:%s", symbol, interval)
		kline, exists := s.klines.Load(key)
		if !exists {
			continue
		}

		k := kline.(*entity.KLine)
		if k.IsExpired(interval) {
			newKLine := entity.NewKLine(symbol, interval, price)
			s.klines.Store(key, newKLine)
		} else {
			k.Update(price)
			s.klines.Store(key, k)
		}
	}
}

func (s *MarketDomainService) getAllSymbols() []string {
	symbols := make([]string, 0)
	s.symbols.Range(func(key, value interface{}) bool {
		symbols = append(symbols, key.(string))
		return true
	})
	return symbols
}

func (s *MarketDomainService) GetAllQuotes() []*entity.Quote {
	quotes := make([]*entity.Quote, 0)
	s.quotes.Range(func(key, value interface{}) bool {
		quotes = append(quotes, value.(*entity.Quote))
		return true
	})
	return quotes
}

func (s *MarketDomainService) AddSymbol(symbol entity.Symbol) {
	s.symbols.Store(symbol.Symbol, &symbol)

	quote := entity.NewQuote(symbol.Symbol, symbol.BasePrice)
	s.quotes.Store(symbol.Symbol, quote)

	for _, interval := range []string{"1m", "5m", "15m", "1h", "1d"} {
		key := fmt.Sprintf("%s:%s", symbol.Symbol, interval)
		kline := entity.NewKLine(symbol.Symbol, interval, symbol.BasePrice)
		s.klines.Store(key, kline)
	}

	s.logger.Info("添加股票",
		logger.String("symbol", symbol.Symbol),
		logger.String("name", symbol.Name))
}
