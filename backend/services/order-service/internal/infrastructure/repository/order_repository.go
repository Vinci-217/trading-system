package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"stock_trader/backend/services/order-service/internal/domain/entity"
	"stock_trader/backend/services/order-service/internal/infrastructure/logger"

	"github.com/shopspring/decimal"
)

type PostgresOrderRepository struct {
	db    *sql.DB
	logger *logger.Logger
}

func NewPostgresOrderRepository(db *sql.DB, logger *logger.Logger) *PostgresOrderRepository {
	return &PostgresOrderRepository{
		db:    db,
		logger: logger,
	}
}

func (r *PostgresOrderRepository) Create(ctx context.Context, order *entity.Order) error {
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
		return fmt.Errorf("创建订单失败: %w", err)
	}

	return nil
}

func (r *PostgresOrderRepository) Get(ctx context.Context, orderID string) (*entity.Order, error) {
	query := `
		SELECT id, user_id, symbol, order_type, side, price, quantity, 
			filled_quantity, status, fee, created_at, updated_at, client_order_id, remarks
		FROM orders WHERE id = ?
	`

	var order entity.Order
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
		return nil, entity.ErrOrderNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("获取订单失败: %w", err)
	}

	if clientOrderID.Valid {
		order.ClientOrderID = clientOrderID.String
	}
	if remarks.Valid {
		order.Remarks = remarks.String
	}

	return &order, nil
}

func (r *PostgresOrderRepository) GetForUpdate(ctx context.Context, orderID string) (*entity.Order, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	query := `
		SELECT id, user_id, symbol, order_type, side, price, quantity, 
			filled_quantity, status, fee, created_at, updated_at, client_order_id, remarks
		FROM orders WHERE id = ? FOR UPDATE
	`

	var order entity.Order
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
		return nil, entity.ErrOrderNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("锁定订单失败: %w", err)
	}

	if clientOrderID.Valid {
		order.ClientOrderID = clientOrderID.String
	}
	if remarks.Valid {
		order.Remarks = remarks.String
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	return &order, nil
}

func (r *PostgresOrderRepository) Update(ctx context.Context, order *entity.Order) error {
	query := `
		UPDATE orders SET 
			filled_quantity = ?, status = ?, fee = ?, updated_at = ?, remarks = ?
		WHERE id = ?
	`

	order.UpdatedAt = time.Now()
	result, err := r.db.ExecContext(ctx, query,
		order.FilledQuantity,
		string(order.Status),
		order.Fee.String(),
		order.UpdatedAt,
		order.Remarks,
		order.ID,
	)

	if err != nil {
		return fmt.Errorf("更新订单失败: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("订单不存在: %s", order.ID)
	}

	return nil
}

func (r *PostgresOrderRepository) GetByUserID(ctx context.Context, userID string, status string, limit int) ([]*entity.Order, error) {
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
		return nil, fmt.Errorf("查询订单列表失败: %w", err)
	}
	defer rows.Close()

	var orders []*entity.Order
	for rows.Next() {
		var order entity.Order
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
			return nil, fmt.Errorf("扫描订单数据失败: %w", err)
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

func (r *PostgresOrderRepository) GetBySymbol(ctx context.Context, symbol string, status string, limit int) ([]*entity.Order, error) {
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
		return nil, fmt.Errorf("查询订单列表失败: %w", err)
	}
	defer rows.Close()

	var orders []*entity.Order
	for rows.Next() {
		var order entity.Order
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
			return nil, fmt.Errorf("扫描订单数据失败: %w", err)
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

type PostgresTCCTransactionRepository struct {
	db    *sql.DB
	logger *logger.Logger
}

func NewPostgresTCCTransactionRepository(db *sql.DB, logger *logger.Logger) *PostgresTCCTransactionRepository {
	return &PostgresTCCTransactionRepository{
		db:    db,
		logger: logger,
	}
}

func (r *PostgresTCCTransactionRepository) Create(ctx context.Context, tx *entity.TCCTransaction) error {
	query := `
		INSERT INTO tcc_transactions (id, order_id, transaction_type, status, try_params, confirm_params, cancel_params, created_at, updated_at, retry_count, error_msg)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		tx.ID,
		tx.OrderID,
		tx.TransactionType,
		tx.Status,
		tx.TryParams,
		tx.ConfirmParams,
		tx.CancelParams,
		tx.CreatedAt,
		tx.UpdatedAt,
		tx.RetryCount,
		tx.ErrorMsg,
	)

	if err != nil {
		return fmt.Errorf("创建TCC事务失败: %w", err)
	}

	return nil
}

func (r *PostgresTCCTransactionRepository) Get(ctx context.Context, id string) (*entity.TCCTransaction, error) {
	query := `
		SELECT id, order_id, transaction_type, status, try_params, confirm_params, cancel_params, created_at, updated_at, retry_count, error_msg
		FROM tcc_transactions WHERE id = ?
	`

	var tx entity.TCCTransaction
	var tryParams, confirmParams, cancelParams, errorMsg sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tx.ID,
		&tx.OrderID,
		&tx.TransactionType,
		&tx.Status,
		&tryParams,
		&confirmParams,
		&cancelParams,
		&tx.CreatedAt,
		&tx.UpdatedAt,
		&tx.RetryCount,
		&errorMsg,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("TCC事务不存在: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("获取TCC事务失败: %w", err)
	}

	if tryParams.Valid {
		tx.TryParams = tryParams.String
	}
	if confirmParams.Valid {
		tx.ConfirmParams = confirmParams.String
	}
	if cancelParams.Valid {
		tx.CancelParams = cancelParams.String
	}
	if errorMsg.Valid {
		tx.ErrorMsg = errorMsg.String
	}

	return &tx, nil
}

func (r *PostgresTCCTransactionRepository) Update(ctx context.Context, tx *entity.TCCTransaction) error {
	query := `
		UPDATE tcc_transactions SET 
			status = ?, updated_at = ?, retry_count = ?, error_msg = ?
		WHERE id = ?
	`

	tx.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, query,
		tx.Status,
		tx.UpdatedAt,
		tx.RetryCount,
		tx.ErrorMsg,
		tx.ID,
	)

	if err != nil {
		return fmt.Errorf("更新TCC事务失败: %w", err)
	}

	return nil
}

func (r *PostgresTCCTransactionRepository) GetByOrderID(ctx context.Context, orderID string) (*entity.TCCTransaction, error) {
	query := `
		SELECT id, order_id, transaction_type, status, try_params, confirm_params, cancel_params, created_at, updated_at, retry_count, error_msg
		FROM tcc_transactions WHERE order_id = ?
	`

	var tx entity.TCCTransaction
	var tryParams, confirmParams, cancelParams, errorMsg sql.NullString

	err := r.db.QueryRowContext(ctx, query, orderID).Scan(
		&tx.ID,
		&tx.OrderID,
		&tx.TransactionType,
		&tx.Status,
		&tryParams,
		&confirmParams,
		&cancelParams,
		&tx.CreatedAt,
		&tx.UpdatedAt,
		&tx.RetryCount,
		&errorMsg,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("TCC事务不存在: %s", orderID)
	}

	if err != nil {
		return nil, fmt.Errorf("获取TCC事务失败: %w", err)
	}

	if tryParams.Valid {
		tx.TryParams = tryParams.String
	}
	if confirmParams.Valid {
		tx.ConfirmParams = confirmParams.String
	}
	if cancelParams.Valid {
		tx.CancelParams = cancelParams.String
	}
	if errorMsg.Valid {
		tx.ErrorMsg = errorMsg.String
	}

	return &tx, nil
}
