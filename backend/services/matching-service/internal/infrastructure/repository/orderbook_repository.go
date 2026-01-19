package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"stock_trader/matching-service/internal/domain/entity"
)

type OrderBookRepository struct {
	db *sql.DB
}

func NewOrderBookRepository(db *sql.DB) *OrderBookRepository {
	return &OrderBookRepository{db: db}
}

func (r *OrderBookRepository) GetOrCreate(ctx context.Context, symbol string) (*entity.OrderBook, error) {
	orderBook := entity.NewOrderBook(symbol)

	query := `
		SELECT id, symbol, last_price, total_volume, total_value, updated_at
		FROM order_books WHERE symbol = ?
	`

	var id string
	var lastPrice, totalValue decimal.Decimal
	var totalVolume int64
	var updatedAt time.Time

	err := r.db.QueryRowContext(ctx, query, symbol).Scan(
		&id, &symbol, &lastPrice, &totalVolume, &totalValue, &updatedAt,
	)

	if err == sql.ErrNoRows {
		return orderBook, nil
	}

	if err != nil {
		return nil, fmt.Errorf("查询订单簿失败: %w", err)
	}

	orderBook.LastPrice = lastPrice
	orderBook.TotalVolume = totalVolume
	orderBook.TotalValue = totalValue
	orderBook.UpdatedAt = updatedAt

	return orderBook, nil
}

func (r *OrderBookRepository) SaveOrderBook(ctx context.Context, orderBook *entity.OrderBook) error {
	query := `
		INSERT INTO order_books (id, symbol, last_price, total_volume, total_value, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		last_price = VALUES(last_price),
		total_volume = VALUES(total_volume),
		total_value = VALUES(total_value),
		updated_at = VALUES(updated_at)
	`

	_, err := r.db.ExecContext(ctx, query,
		orderBook.Symbol,
		orderBook.Symbol,
		orderBook.LastPrice,
		orderBook.TotalVolume,
		orderBook.TotalValue,
		orderBook.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("保存订单簿失败: %w", err)
	}

	return nil
}

func (r *OrderBookRepository) GetOrderBook(ctx context.Context, symbol string) (*entity.OrderBook, error) {
	return r.GetOrCreate(ctx, symbol)
}

func (r *OrderBookRepository) SaveTrade(ctx context.Context, trade *entity.Trade) error {
	query := `
		INSERT INTO trades (id, symbol, buy_order_id, sell_order_id, buy_user_id, sell_user_id, price, quantity, timestamp, fee)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		trade.ID,
		trade.Symbol,
		trade.BuyOrderID,
		trade.SellOrderID,
		trade.BuyUserID,
		trade.SellUserID,
		trade.Price.String(),
		trade.Quantity,
		trade.Timestamp,
		trade.Fee.String(),
	)

	if err != nil {
		return fmt.Errorf("保存交易失败: %w", err)
	}

	return nil
}

func (r *OrderBookRepository) GetTrades(ctx context.Context, symbol string, limit int) ([]*entity.Trade, error) {
	query := `
		SELECT id, symbol, buy_order_id, sell_order_id, buy_user_id, sell_user_id, price, quantity, timestamp, fee
		FROM trades WHERE symbol = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, symbol, limit)
	if err != nil {
		return nil, fmt.Errorf("查询交易失败: %w", err)
	}
	defer rows.Close()

	var trades []*entity.Trade
	for rows.Next() {
		var trade entity.Trade
		var priceStr, feeStr string

		if err := rows.Scan(
			&trade.ID,
			&trade.Symbol,
			&trade.BuyOrderID,
			&trade.SellOrderID,
			&trade.BuyUserID,
			&trade.SellUserID,
			&priceStr,
			&trade.Quantity,
			&trade.Timestamp,
			&feeStr,
		); err != nil {
			return nil, fmt.Errorf("扫描交易失败: %w", err)
		}

		trade.Price, _ = decimal.NewFromString(priceStr)
		trade.Fee, _ = decimal.NewFromString(feeStr)

		trades = append(trades, &trade)
	}

	return trades, nil
}
