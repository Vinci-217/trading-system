package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"stock_trader/common/model"
)

type PostgresOrderRepository struct {
	db *sql.DB
}

func NewPostgresOrderRepository(db *sql.DB) *PostgresOrderRepository {
	return &PostgresOrderRepository{db: db}
}

func (r *PostgresOrderRepository) CreateOrder(ctx context.Context, order *model.Order) error {
	query := `
		INSERT INTO orders (id, user_id, symbol, order_type, side, price, quantity, 
			filled_quantity, status, fee, created_at, updated_at, client_order_id, remarks)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		order.ID,
		order.UserID,
		order.Symbol,
		string(order.OrderType),
		string(order.Side),
		order.Price.String(),
		order.Quantity,
		order.FilledQuantity,
		string(order.Status),
		order.Fee.String(),
		order.CreatedAt,
		order.UpdatedAt,
		order.ClientOrderID,
		order.Remarks,
	)

	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	return nil
}

func (r *PostgresOrderRepository) GetOrder(ctx context.Context, orderID string) (*model.Order, error) {
	query := `
		SELECT id, user_id, symbol, order_type, side, price, quantity, 
			filled_quantity, status, fee, created_at, updated_at, client_order_id, remarks
		FROM orders WHERE id = ?
	`

	var order model.Order
	var clientOrderID, remarks sql.NullString

	err := r.db.QueryRowContext(ctx, query, orderID).Scan(
		&order.ID,
		&order.UserID,
		&order.Symbol,
		&order.OrderType,
		&order.Side,
		&order.Price,
		&order.Quantity,
		&order.FilledQuantity,
		&order.Status,
		&order.Fee,
		&order.CreatedAt,
		&order.UpdatedAt,
		&clientOrderID,
		&remarks,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	if clientOrderID.Valid {
		order.ClientOrderID = clientOrderID.String
	}
	if remarks.Valid {
		order.Remarks = remarks.String
	}

	return &order, nil
}

func (r *PostgresOrderRepository) UpdateOrder(ctx context.Context, order *model.Order) error {
	query := `
		UPDATE orders SET 
			filled_quantity = ?, status = ?, fee = ?, updated_at = ?, remarks = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		order.FilledQuantity,
		string(order.Status),
		order.Fee.String(),
		order.UpdatedAt,
		order.Remarks,
		order.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("order not found: %s", order.ID)
	}

	return nil
}

func (r *PostgresOrderRepository) GetOrdersByUser(ctx context.Context, userID string, status string, limit int) ([]*model.Order, error) {
	var query string
	var args []interface{}

	if status != "" {
		query = `
			SELECT id, user_id, symbol, order_type, side, price, quantity,
				filled_quantity, status, fee, created_at, updated_at, client_order_id, remarks
			FROM orders WHERE user_id = ? AND status = ?
			ORDER BY created_at DESC LIMIT ?
		`
		args = []interface{}{userID, status, limit}
	} else {
		query = `
			SELECT id, user_id, symbol, order_type, side, price, quantity,
				filled_quantity, status, fee, created_at, updated_at, client_order_id, remarks
			FROM orders WHERE user_id = ?
			ORDER BY created_at DESC LIMIT ?
		`
		args = []interface{}{userID, limit}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	var orders []*model.Order
	for rows.Next() {
		var order model.Order
		var clientOrderID, remarks sql.NullString

		if err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.Symbol,
			&order.OrderType,
			&order.Side,
			&order.Price,
			&order.Quantity,
			&order.FilledQuantity,
			&order.Status,
			&order.Fee,
			&order.CreatedAt,
			&order.UpdatedAt,
			&clientOrderID,
			&remarks,
		); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}

		if clientOrderID.Valid {
			order.ClientOrderID = clientOrderID.String
		}
		if remarks.Valid {
			order.Remarks = remarks.String
		}

		orders = append(orders, &order)
	}

	return orders, nil
}

func (r *PostgresOrderRepository) GetOrdersBySymbol(ctx context.Context, symbol string, status string, limit int) ([]*model.Order, error) {
	var query string
	var args []interface{}

	if status != "" {
		query = `
			SELECT id, user_id, symbol, order_type, side, price, quantity,
				filled_quantity, status, fee, created_at, updated_at, client_order_id, remarks
			FROM orders WHERE symbol = ? AND status = ?
			ORDER BY created_at DESC LIMIT ?
		`
		args = []interface{}{symbol, status, limit}
	} else {
		query = `
			SELECT id, user_id, symbol, order_type, side, price, quantity,
				filled_quantity, status, fee, created_at, updated_at, client_order_id, remarks
			FROM orders WHERE symbol = ?
			ORDER BY created_at DESC LIMIT ?
		`
		args = []interface{}{symbol, limit}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	var orders []*model.Order
	for rows.Next() {
		var order model.Order
		var clientOrderID, remarks sql.NullString

		if err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.Symbol,
			&order.OrderType,
			&order.Side,
			&order.Price,
			&order.Quantity,
			&order.FilledQuantity,
			&order.Status,
			&order.Fee,
			&order.CreatedAt,
			&order.UpdatedAt,
			&clientOrderID,
			&remarks,
		); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}

		if clientOrderID.Valid {
			order.ClientOrderID = clientOrderID.String
		}
		if remarks.Valid {
			order.Remarks = remarks.String
		}

		orders = append(orders, &order)
	}

	return orders, nil
}

func (r *PostgresOrderRepository) LockOrderForUpdate(ctx context.Context, orderID string) (*model.Order, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		SELECT id, user_id, symbol, order_type, side, price, quantity,
			filled_quantity, status, fee, created_at, updated_at, client_order_id, remarks
		FROM orders WHERE id = ? FOR UPDATE
	`

	var order model.Order
	var clientOrderID, remarks sql.NullString

	err = tx.QueryRowContext(ctx, query, orderID).Scan(
		&order.ID,
		&order.UserID,
		&order.Symbol,
		&order.OrderType,
		&order.Side,
		&order.Price,
		&order.Quantity,
		&order.FilledQuantity,
		&order.Status,
		&order.Fee,
		&order.CreatedAt,
		&order.UpdatedAt,
		&clientOrderID,
		&remarks,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to lock order: %w", err)
	}

	if clientOrderID.Valid {
		order.ClientOrderID = clientOrderID.String
	}
	if remarks.Valid {
		order.Remarks = remarks.String
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &order, nil
}

func (r *PostgresOrderRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
}

type MemoryOrderRepository struct {
	orders     map[string]*model.Order
	userOrders map[string][]string
	mu         sync.RWMutex
}

func NewMemoryOrderRepository() *MemoryOrderRepository {
	return &MemoryOrderRepository{
		orders:     make(map[string]*model.Order),
		userOrders: make(map[string][]string),
	}
}

func (r *MemoryOrderRepository) CreateOrder(ctx context.Context, order *model.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.orders[order.ID]; exists {
		return fmt.Errorf("order already exists: %s", order.ID)
	}

	r.orders[order.ID] = order
	r.userOrders[order.UserID] = append(r.userOrders[order.UserID], order.ID)

	return nil
}

func (r *MemoryOrderRepository) GetOrder(ctx context.Context, orderID string) (*model.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order, exists := r.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	return order, nil
}

func (r *MemoryOrderRepository) UpdateOrder(ctx context.Context, order *model.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.orders[order.ID]
	if !exists {
		return fmt.Errorf("order not found: %s", order.ID)
	}

	*existing = *order
	return nil
}

func (r *MemoryOrderRepository) GetOrdersByUser(ctx context.Context, userID string, status string, limit int) ([]*model.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	orderIDs, exists := r.userOrders[userID]
	if !exists {
		return []*model.Order{}, nil
	}

	var orders []*model.Order
	for _, orderID := range orderIDs {
		if len(orders) >= limit {
			break
		}

		order := r.orders[orderID]
		if status == "" || order.Status == model.OrderStatus(status) {
			orders = append(orders, order)
		}
	}

	return orders, nil
}

func (r *MemoryOrderRepository) GetOrdersBySymbol(ctx context.Context, symbol string, status string, limit int) ([]*model.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var orders []*model.Order
	for _, order := range r.orders {
		if len(orders) >= limit {
			break
		}

		if order.Symbol == symbol && (status == "" || order.Status == model.OrderStatus(status)) {
			orders = append(orders, order)
		}
	}

	return orders, nil
}

func (r *MemoryOrderRepository) LockOrderForUpdate(ctx context.Context, orderID string) (*model.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, exists := r.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	return order, nil
}

func (r *MemoryOrderRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return nil, fmt.Errorf("not supported in memory repository")
}

import (
	"sync"
)
