package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化数据失败: %w", err)
	}

	if c.client == nil {
		return nil
	}

	return c.client.Set(ctx, key, data, expiration).Err()
}

func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	if c.client == nil {
		return nil
	}

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return fmt.Errorf("获取数据失败: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("反序列化数据失败: %w", err)
	}

	return nil
}

func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	if c.client == nil {
		return nil
	}

	return c.client.Del(ctx, keys...).Err()
}

func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	if c.client == nil {
		return false, nil
	}

	result, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

func (c *RedisCache) Increment(ctx context.Context, key string) (int64, error) {
	if c.client == nil {
		return 0, nil
	}
	return c.client.Incr(ctx, key).Result()
}

func (c *RedisCache) Decrement(ctx context.Context, key string) (int64, error) {
	if c.client == nil {
		return 0, nil
	}
	return c.client.Decr(ctx, key).Result()
}

func (c *RedisCache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	if c.client == nil {
		return false, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("序列化数据失败: %w", err)
	}

	return c.client.SetNX(ctx, key, data, expiration).Result()
}

func (c *RedisCache) Publish(ctx context.Context, channel string, message interface{}) error {
	if c.client == nil {
		return nil
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	return c.client.Publish(ctx, channel, data).Err()
}

func (c *RedisCache) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	if c.client == nil {
		return nil
	}
	return c.client.Subscribe(ctx, channel)
}

func (c *RedisCache) GetQuoteKey(symbol string) string {
	return fmt.Sprintf("quote:%s", symbol)
}

func (c *RedisCache) GetOrderBookKey(symbol string, side string) string {
	return fmt.Sprintf("orderbook:%s:%s", symbol, side)
}

func (c *RedisCache) GetUserOrdersKey(userID int64) string {
	return fmt.Sprintf("user:%d:orders", userID)
}

func (c *RedisCache) GetUserPositionsKey(userID int64) string {
	return fmt.Sprintf("user:%d:positions", userID)
}

func (c *RedisCache) GetAccountKey(userID int64) string {
	return fmt.Sprintf("user:%d:account", userID)
}

func (c *RedisCache) GetMarketDataKey(symbol string) string {
	return fmt.Sprintf("market:%s", symbol)
}

func (c *RedisCache) GetHotSymbolsKey() string {
	return "market:hot_symbols"
}

func (c *RedisCache) GetOrderRateLimitKey(userID int64) string {
	return fmt.Sprintf("ratelimit:order:%d", userID)
}

func (c *RedisCache) GetCancelRateLimitKey(userID int64) string {
	return fmt.Sprintf("ratelimit:cancel:%d", userID)
}

func (c *RedisCache) IsOrderRateLimited(ctx context.Context, userID int64, maxOrders int, window time.Duration) (bool, error) {
	key := c.GetOrderRateLimitKey(userID)

	count, err := c.Increment(ctx, key)
	if err != nil {
		return false, err
	}

	if count == 1 {
		c.client.Expire(ctx, key, window)
	}

	return count > int64(maxOrders), nil
}

func (c *RedisCache) IsCancelRateLimited(ctx context.Context, userID int64, maxCancels int, window time.Duration) (bool, error) {
	key := c.GetCancelRateLimitKey(userID)

	count, err := c.Increment(ctx, key)
	if err != nil {
		return false, err
	}

	if count == 1 {
		c.client.Expire(ctx, key, window)
	}

	return count > int64(maxCancels), nil
}

func (c *RedisCache) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}
