package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"stock_trader/backend/services/market-service/internal/domain/entity"
	"stock_trader/backend/services/market-service/internal/infrastructure/logger"

	"github.com/redis/go-redis/v9"
)

type Publisher struct {
	redis  *redis.Client
	logger *logger.Logger
}

func NewPublisher(redis *redis.Client, logger *logger.Logger) *Publisher {
	return &Publisher{
		redis:  redis,
		logger: logger,
	}
}

func (p *Publisher) Publish(ctx context.Context, channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	if err := p.redis.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("发布消息失败: %w", err)
	}

	p.logger.Debug("消息已发布",
		logger.String("channel", channel))

	return nil
}

func (p *Publisher) PublishQuote(ctx context.Context, quote *entity.Quote) error {
	eventData := map[string]interface{}{
		"event":     "quote",
		"symbol":    quote.Symbol,
		"price":     quote.Price.String(),
		"change":    quote.Change.String(),
		"change_pct": quote.ChangePct.String(),
		"volume":    quote.Volume,
		"timestamp": time.Now().UnixMilli(),
	}
	return p.Publish(ctx, "market:quotes", eventData)
}

func (p *Publisher) PublishKLine(ctx context.Context, kline *entity.KLine) error {
	eventData := map[string]interface{}{
		"event":     "kline",
		"symbol":    kline.Symbol,
		"interval":  kline.Interval,
		"open":      kline.Open.String(),
		"high":      kline.High.String(),
		"low":       kline.Low.String(),
		"close":     kline.Close.String(),
		"volume":    kline.Volume,
		"timestamp": time.Now().UnixMilli(),
	}
	return p.Publish(ctx, "market:klines", eventData)
}

func (p *Publisher) CacheQuote(ctx context.Context, quote *entity.Quote) error {
	key := fmt.Sprintf("quote:%s", quote.Symbol)
	data, _ := json.Marshal(quote)
	return p.redis.Set(ctx, key, data, 5*time.Second).Err()
}

func (p *Publisher) GetCachedQuote(ctx context.Context, symbol string) (*entity.Quote, error) {
	key := fmt.Sprintf("quote:%s", symbol)
	data, err := p.redis.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var quote entity.Quote
	if err := json.Unmarshal(data, &quote); err != nil {
		return nil, err
	}

	return &quote, nil
}

type Subscriber struct {
	redis  *redis.Client
	logger *logger.Logger
}

func NewSubscriber(redis *redis.Client, logger *logger.Logger) *Subscriber {
	return &Subscriber{
		redis:  redis,
		logger: logger,
	}
}

func (s *Subscriber) Subscribe(ctx context.Context, channel string) (<-chan string, error) {
	pubsub := s.redis.Subscribe(ctx, channel)
	ch := pubsub.Channel()

	messages := make(chan string, 100)
	go func() {
		for msg := range ch {
			messages <- msg.Payload
		}
		close(messages)
	}()

	return messages, nil
}
