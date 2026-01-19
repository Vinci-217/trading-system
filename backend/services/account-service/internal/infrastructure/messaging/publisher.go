package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"stock_trader/backend/services/account-service/internal/infrastructure/logger"

	"github.com/redis/go-redis/v9"
)

type Publisher struct {
	redis  *redis.Client
	logger *logger.Logger
	mu     sync.Mutex
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
		logger.String("channel", channel),
		logger.String("message", string(data)))

	return nil
}

func (p *Publisher) PublishAccountEvent(ctx context.Context, event string, data map[string]interface{}) error {
	eventData := map[string]interface{}{
		"event":     event,
		"timestamp": time.Now().UnixMilli(),
		"data":      data,
	}
	return p.Publish(ctx, "account:events", eventData)
}

func (p *Publisher) PublishFundLockEvent(ctx context.Context, event string, data map[string]interface{}) error {
	eventData := map[string]interface{}{
		"event":     event,
		"timestamp": time.Now().UnixMilli(),
		"data":      data,
	}
	return p.Publish(ctx, "fund:lock:events", eventData)
}

func (p *Publisher) PublishPositionEvent(ctx context.Context, event string, data map[string]interface{}) error {
	eventData := map[string]interface{}{
		"event":     event,
		"timestamp": time.Now().UnixMilli(),
		"data":      data,
	}
	return p.Publish(ctx, "position:events", eventData)
}

type Subscriber struct {
	redis    *redis.Client
	logger   *logger.Logger
	pubsub   *redis.PubSub
	handlers map[string]func(message interface{})
}

func NewSubscriber(redis *redis.Client, logger *logger.Logger) *Subscriber {
	return &Subscriber{
		redis:    redis,
		logger:   logger,
		handlers: make(map[string]func(message interface{})),
	}
}

func (s *Subscriber) Subscribe(ctx context.Context, channel string) error {
	s.pubsub = s.redis.Subscribe(ctx, channel)

	ch := s.pubsub.Channel()
	go func() {
		for msg := range ch {
			var data interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &data); err != nil {
				s.logger.Error("解析消息失败", logger.Error(err))
				continue
			}

			if handler, ok := s.handlers[channel]; ok {
				handler(data)
			}
		}
	}()

	return nil
}

func (s *Subscriber) RegisterHandler(channel string, handler func(message interface{})) {
	s.handlers[channel] = handler
}

func (s *Subscriber) Close() error {
	if s.pubsub != nil {
		return s.pubsub.Close()
	}
	return nil
}

import "sync"
