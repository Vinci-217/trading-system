package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"

	"stock_trader/matching-service/internal/domain/entity"
)

type TradePublisher interface {
	PublishTrade(ctx context.Context, trade *entity.Trade) error
}

type MarketPublisher interface {
	PublishDepth(ctx context.Context, symbol string, depth *entity.MarketDepth) error
}

type RedisTradePublisher struct {
	redis *redis.Client
}

func NewTradePublisher(redis *redis.Client) *RedisTradePublisher {
	return &RedisTradePublisher{redis: redis}
}

func (p *RedisTradePublisher) PublishTrade(ctx context.Context, trade *entity.Trade) error {
	data, err := json.Marshal(trade)
	if err != nil {
		return fmt.Errorf("序列化交易失败: %w", err)
	}

	key := fmt.Sprintf("trades:%s", trade.Symbol)
	pipe := p.redis.Pipeline()
	pipe.LPush(ctx, key, string(data))
	pipe.LTrim(ctx, key, 0, 1000)
	pipe.Expire(ctx, key, 24*time.Hour)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("发布交易到Redis失败: %w", err)
	}

	channel := fmt.Sprintf("trade:%s", trade.Symbol)
	p.redis.Publish(ctx, channel, string(data))

	return nil
}

type RedisMarketPublisher struct {
	redis *redis.Client
}

func NewMarketPublisher(redis *redis.Client) *RedisMarketPublisher {
	return &RedisMarketPublisher{redis: redis}
}

func (p *RedisMarketPublisher) PublishDepth(ctx context.Context, symbol string, depth *entity.MarketDepth) error {
	data, err := json.Marshal(depth)
	if err != nil {
		return fmt.Errorf("序列化深度失败: %w", err)
	}

	key := fmt.Sprintf("depth:%s", symbol)
	p.redis.Set(ctx, key, string(data), 5*time.Second)

	channel := fmt.Sprintf("depth:%s", symbol)
	p.redis.Publish(ctx, channel, string(data))

	return nil
}

type TradeEvent struct {
	TradeID     string          `json:"trade_id"`
	Symbol      string          `json:"symbol"`
	BuyOrderID  string          `json:"buy_order_id"`
	SellOrderID string          `json:"sell_order_id"`
	BuyUserID   string          `json:"buy_user_id"`
	SellUserID  string          `json:"sell_user_id"`
	Price       decimal.Decimal `json:"price"`
	Quantity    int             `json:"quantity"`
	Timestamp   time.Time       `json:"timestamp"`
}
