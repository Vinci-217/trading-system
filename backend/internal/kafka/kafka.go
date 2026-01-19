package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
)

type Producer interface {
	SendOrderCreated(ctx context.Context, event *OrderEvent) error
	SendOrderCancelled(ctx context.Context, event *OrderEvent) error
	SendTradeExecuted(ctx context.Context, event *TradeEvent) error
	SendQuoteUpdate(ctx context.Context, event *QuoteEvent) error
	Close() error
}

type KafkaProducer struct {
	producer sarama.SyncProducer
	brokers  []string
}

type OrderEvent struct {
	OrderID    string    `json:"order_id"`
	UserID     int64     `json:"user_id"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	OrderType  string    `json:"order_type"`
	Price      float64   `json:"price"`
	Quantity   int       `json:"quantity"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type TradeEvent struct {
	TradeID       string    `json:"trade_id"`
	OrderID       string    `json:"order_id"`
	CounterOrderID string   `json:"counter_order_id"`
	UserID        int64     `json:"user_id"`
	Symbol        string    `json:"symbol"`
	Side          string    `json:"side"`
	Price         float64   `json:"price"`
	Quantity      int       `json:"quantity"`
	Amount        float64   `json:"amount"`
	Fee           float64   `json:"fee"`
	Timestamp     time.Time `json:"timestamp"`
}

type QuoteEvent struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Change    float64   `json:"change"`
	Volume    int64     `json:"volume"`
	Timestamp time.Time `json:"timestamp"`
}

func NewProducer(brokers []string) (*KafkaProducer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("创建Kafka生产者失败: %w", err)
	}

	return &KafkaProducer{
		producer: producer,
		brokers:  brokers,
	}, nil
}

func (p *KafkaProducer) SendOrderCreated(ctx context.Context, event *OrderEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化订单事件失败: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: "orders",
		Key:   sarama.StringEncoder(event.OrderID),
		Value: sarama.ByteEncoder(data),
	}

	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("发送订单事件失败: %w", err)
	}

	return nil
}

func (p *KafkaProducer) SendOrderCancelled(ctx context.Context, event *OrderEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化取消订单事件失败: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: "orders_cancelled",
		Key:   sarama.StringEncoder(event.OrderID),
		Value: sarama.ByteEncoder(data),
	}

	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("发送取消订单事件失败: %w", err)
	}

	return nil
}

func (p *KafkaProducer) SendTradeExecuted(ctx context.Context, event *TradeEvent) error {
	if event.TradeID == "" {
		event.TradeID = uuid.New().String()
	}
	event.Timestamp = time.Now()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化成交事件失败: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: "trades",
		Key:   sarama.StringEncoder(event.TradeID),
		Value: sarama.ByteEncoder(data),
	}

	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("发送成交事件失败: %w", err)
	}

	return nil
}

func (p *KafkaProducer) SendQuoteUpdate(ctx context.Context, event *QuoteEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化行情事件失败: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: "quotes",
		Key:   sarama.StringEncoder(event.Symbol),
		Value: sarama.ByteEncoder(data),
	}

	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("发送行情事件失败: %w", err)
	}

	return nil
}

func (p *KafkaProducer) Close() error {
	if p.producer != nil {
		return p.producer.Close()
	}
	return nil
}

type Client struct {
	producer Producer
}

func NewClient(producer Producer) *Client {
	return &Client{producer: producer}
}

func (c *Client) SendOrderCreated(ctx context.Context, event *OrderEvent) error {
	return c.producer.SendOrderCreated(ctx, event)
}

func (c *Client) SendOrderCancelled(ctx context.Context, event *OrderEvent) error {
	return c.producer.SendOrderCancelled(ctx, event)
}

func (c *Client) SendTradeExecuted(ctx context.Context, event *TradeEvent) error {
	return c.producer.SendTradeExecuted(ctx, event)
}

func (c *Client) SendQuoteUpdate(ctx context.Context, event *QuoteEvent) error {
	return c.producer.SendQuoteUpdate(ctx, event)
}
