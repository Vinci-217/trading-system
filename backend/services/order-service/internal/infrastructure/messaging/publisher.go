package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"stock_trader/backend/services/order-service/internal/infrastructure/logger"

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

func (p *Publisher) PublishOrderEvent(ctx context.Context, event string, data map[string]interface{}) error {
	eventData := map[string]interface{}{
		"event":     event,
		"timestamp": time.Now().UnixMilli(),
		"data":      data,
	}
	return p.Publish(ctx, "order_events", eventData)
}

func (p *Publisher) PublishOrderCreated(ctx context.Context, orderID string, userID string, symbol string, side string, quantity int, price string) error {
	return p.PublishOrderEvent(ctx, "order_created", map[string]interface{}{
		"order_id": orderID,
		"user_id":  userID,
		"symbol":   symbol,
		"side":     side,
		"quantity": quantity,
		"price":    price,
	})
}

func (p *Publisher) PublishOrderCancelled(ctx context.Context, orderID string, userID string, reason string) error {
	return p.PublishOrderEvent(ctx, "order_cancelled", map[string]interface{}{
		"order_id": orderID,
		"user_id":  userID,
		"reason":   reason,
	})
}

func (p *Publisher) PublishOrderFilled(ctx context.Context, orderID string, userID string, symbol string, filledQuantity int, remainingQuantity int) error {
	return p.PublishOrderEvent(ctx, "order_filled", map[string]interface{}{
		"order_id":           orderID,
		"user_id":            userID,
		"symbol":             symbol,
		"filled_quantity":    filledQuantity,
		"remaining_quantity": remainingQuantity,
	})
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
