package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"stock_trader/backend/services/account-service/internal/domain/entity"
	"stock_trader/backend/services/account-service/internal/infrastructure/logger"

	"github.com/shopspring/decimal"
)

type PostgresAccountRepository struct {
	db    *sql.DB
	logger *logger.Logger
}

func NewPostgresAccountRepository(db *sql.DB, logger *logger.Logger) *PostgresAccountRepository {
	return &PostgresAccountRepository{
		db:    db,
		logger: logger,
	}
}

func (r *PostgresAccountRepository) Create(ctx context.Context, account *entity.Account) error {
	query := `
		INSERT INTO accounts (user_id, cash_balance, frozen_balance, securities_balance, total_assets, total_deposit, total_withdrawal, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		account.UserID,
		account.CashBalance.String(),
		account.FrozenBalance.String(),
		account.SecuritiesBalance.String(),
		account.TotalAssets.String(),
		account.TotalDeposit.String(),
		account.TotalWithdrawal.String(),
		account.Version,
	)

	if err != nil {
		return fmt.Errorf("创建账户失败: %w", err)
	}

	return nil
}

func (r *PostgresAccountRepository) Get(ctx context.Context, userID string) (*entity.Account, error) {
	query := `
		SELECT user_id, cash_balance, frozen_balance, securities_balance, total_assets, total_deposit, total_withdrawal, created_at, updated_at, version
		FROM accounts WHERE user_id = ?
	`

	var account entity.Account
	var cashBalance, frozenBalance, securitiesBalance, totalAssets, totalDeposit, totalWithdrawal string

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&account.UserID,
		&cashBalance,
		&frozenBalance,
		&securitiesBalance,
		&totalAssets,
		&totalDeposit,
		&totalWithdrawal,
		&account.CreatedAt,
		&account.UpdatedAt,
		&account.Version,
	)

	account.CashBalance, _ = decimal.NewFromString(cashBalance)
	account.FrozenBalance, _ = decimal.NewFromString(frozenBalance)
	account.SecuritiesBalance, _ = decimal.NewFromString(securitiesBalance)
	account.TotalAssets, _ = decimal.NewFromString(totalAssets)
	account.TotalDeposit, _ = decimal.NewFromString(totalDeposit)
	account.TotalWithdrawal, _ = decimal.NewFromString(totalWithdrawal)

	if err == sql.ErrNoRows {
		return nil, entity.ErrAccountNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("获取账户失败: %w", err)
	}

	return &account, nil
}

func (r *PostgresAccountRepository) GetForUpdate(ctx context.Context, userID string) (*entity.Account, error) {
	query := `
		SELECT user_id, cash_balance, frozen_balance, securities_balance, total_assets, total_deposit, total_withdrawal, created_at, updated_at, version
		FROM accounts WHERE user_id = ? FOR UPDATE
	`

	var account entity.Account
	var cashBalance, frozenBalance, securitiesBalance, totalAssets, totalDeposit, totalWithdrawal string

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&account.UserID,
		&cashBalance,
		&frozenBalance,
		&securitiesBalance,
		&totalAssets,
		&totalDeposit,
		&totalWithdrawal,
		&account.CreatedAt,
		&account.UpdatedAt,
		&account.Version,
	)

	account.CashBalance, _ = decimal.NewFromString(cashBalance)
	account.FrozenBalance, _ = decimal.NewFromString(frozenBalance)
	account.SecuritiesBalance, _ = decimal.NewFromString(securitiesBalance)
	account.TotalAssets, _ = decimal.NewFromString(totalAssets)
	account.TotalDeposit, _ = decimal.NewFromString(totalDeposit)
	account.TotalWithdrawal, _ = decimal.NewFromString(totalWithdrawal)

	if err == sql.ErrNoRows {
		return nil, entity.ErrAccountNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("获取账户失败: %w", err)
	}

	return &account, nil
}

func (r *PostgresAccountRepository) Update(ctx context.Context, account *entity.Account) error {
	query := `
		UPDATE accounts SET 
			cash_balance = ?, frozen_balance = ?, securities_balance = ?, total_assets = ?,
			total_deposit = ?, total_withdrawal = ?, updated_at = ?, version = ?
		WHERE user_id = ? AND version = ?
	`

	account.UpdatedAt = time.Now()
	result, err := r.db.ExecContext(ctx, query,
		account.CashBalance.String(),
		account.FrozenBalance.String(),
		account.SecuritiesBalance.String(),
		account.TotalAssets.String(),
		account.TotalDeposit.String(),
		account.TotalWithdrawal.String(),
		account.UpdatedAt,
		account.Version+1,
		account.UserID,
		account.Version,
	)

	if err != nil {
		return fmt.Errorf("更新账户失败: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("乐观锁失败: %s", account.UserID)
	}

	account.Version++
	return nil
}

func (r *PostgresAccountRepository) GetAll(ctx context.Context) ([]*entity.Account, error) {
	query := `
		SELECT user_id, cash_balance, frozen_balance, securities_balance, total_assets, total_deposit, total_withdrawal, created_at, updated_at, version
		FROM accounts
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询账户列表失败: %w", err)
	}
	defer rows.Close()

	var accounts []*entity.Account
	for rows.Next() {
		var account entity.Account
		var cashBalance, frozenBalance, securitiesBalance, totalAssets, totalDeposit, totalWithdrawal string

		if err := rows.Scan(
			&account.UserID,
			&cashBalance,
			&frozenBalance,
			&securitiesBalance,
			&totalAssets,
			&totalDeposit,
			&totalWithdrawal,
			&account.CreatedAt,
			&account.UpdatedAt,
			&account.Version,
		); err != nil {
			return nil, fmt.Errorf("扫描账户数据失败: %w", err)
		}

		account.CashBalance, _ = decimal.NewFromString(cashBalance)
		account.FrozenBalance, _ = decimal.NewFromString(frozenBalance)
		account.SecuritiesBalance, _ = decimal.NewFromString(securitiesBalance)
		account.TotalAssets, _ = decimal.NewFromString(totalAssets)
		account.TotalDeposit, _ = decimal.NewFromString(totalDeposit)
		account.TotalWithdrawal, _ = decimal.NewFromString(totalWithdrawal)

		accounts = append(accounts, &account)
	}

	return accounts, nil
}

func (r *PostgresAccountRepository) Lock(ctx context.Context, userID string) error {
	query := `SELECT user_id FROM accounts WHERE user_id = ? FOR UPDATE`
	var dummy string
	return r.db.QueryRowContext(ctx, query, userID).Scan(&dummy)
}

func (r *PostgresAccountRepository) Unlock(ctx context.Context, userID string) error {
	return nil
}

type PostgresPositionRepository struct {
	db    *sql.DB
	logger *logger.Logger
}

func NewPostgresPositionRepository(db *sql.DB, logger *logger.Logger) *PostgresPositionRepository {
	return &PostgresPositionRepository{
		db:    db,
		logger: logger,
	}
}

func (r *PostgresPositionRepository) Create(ctx context.Context, position *entity.Position) error {
	query := `
		INSERT INTO positions (id, user_id, symbol, quantity, frozen_quantity, avg_cost, market_value, profit_loss, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		position.ID,
		position.UserID,
		position.Symbol,
		position.Quantity,
		position.FrozenQuantity,
		position.AvgCost.String(),
		position.MarketValue.String(),
		position.ProfitLoss.String(),
		position.Version,
	)

	if err != nil {
		return fmt.Errorf("创建持仓失败: %w", err)
	}

	return nil
}

func (r *PostgresPositionRepository) Get(ctx context.Context, userID string, symbol string) (*entity.Position, error) {
	query := `
		SELECT id, user_id, symbol, quantity, frozen_quantity, avg_cost, market_value, profit_loss, created_at, updated_at, version
		FROM positions WHERE user_id = ? AND symbol = ?
	`

	var position entity.Position
	var avgCost, marketValue, profitLoss string

	err := r.db.QueryRowContext(ctx, query, userID, symbol).Scan(
		&position.ID,
		&position.UserID,
		&position.Symbol,
		&position.Quantity,
		&position.FrozenQuantity,
		&avgCost,
		&marketValue,
		&profitLoss,
		&position.CreatedAt,
		&position.UpdatedAt,
		&position.Version,
	)

	position.AvgCost, _ = decimal.NewFromString(avgCost)
	position.MarketValue, _ = decimal.NewFromString(marketValue)
	position.ProfitLoss, _ = decimal.NewFromString(profitLoss)

	if err == sql.ErrNoRows {
		return nil, entity.ErrPositionNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	return &position, nil
}

func (r *PostgresPositionRepository) GetForUpdate(ctx context.Context, userID string, symbol string) (*entity.Position, error) {
	query := `
		SELECT id, user_id, symbol, quantity, frozen_quantity, avg_cost, market_value, profit_loss, created_at, updated_at, version
		FROM positions WHERE user_id = ? AND symbol = ? FOR UPDATE
	`

	var position entity.Position
	var avgCost, marketValue, profitLoss string

	err := r.db.QueryRowContext(ctx, query, userID, symbol).Scan(
		&position.ID,
		&position.UserID,
		&position.Symbol,
		&position.Quantity,
		&position.FrozenQuantity,
		&avgCost,
		&marketValue,
		&profitLoss,
		&position.CreatedAt,
		&position.UpdatedAt,
		&position.Version,
	)

	position.AvgCost, _ = decimal.NewFromString(avgCost)
	position.MarketValue, _ = decimal.NewFromString(marketValue)
	position.ProfitLoss, _ = decimal.NewFromString(profitLoss)

	if err == sql.ErrNoRows {
		return nil, entity.ErrPositionNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	return &position, nil
}

func (r *PostgresPositionRepository) Update(ctx context.Context, position *entity.Position) error {
	query := `
		UPDATE positions SET 
			quantity = ?, frozen_quantity = ?, avg_cost = ?, market_value = ?, profit_loss = ?, updated_at = ?, version = ?
		WHERE id = ? AND version = ?
	`

	position.UpdatedAt = time.Now()
	result, err := r.db.ExecContext(ctx, query,
		position.Quantity,
		position.FrozenQuantity,
		position.AvgCost.String(),
		position.MarketValue.String(),
		position.ProfitLoss.String(),
		position.UpdatedAt,
		position.Version+1,
		position.ID,
		position.Version,
	)

	if err != nil {
		return fmt.Errorf("更新持仓失败: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("乐观锁失败: %s", position.ID)
	}

	position.Version++
	return nil
}

func (r *PostgresPositionRepository) GetByUserID(ctx context.Context, userID string) ([]*entity.Position, error) {
	query := `
		SELECT id, user_id, symbol, quantity, frozen_quantity, avg_cost, market_value, profit_loss, created_at, updated_at, version
		FROM positions WHERE user_id = ? ORDER BY symbol
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("查询持仓列表失败: %w", err)
	}
	defer rows.Close()

	var positions []*entity.Position
	for rows.Next() {
		var position entity.Position
		var avgCost, marketValue, profitLoss string

		if err := rows.Scan(
			&position.ID,
			&position.UserID,
			&position.Symbol,
			&position.Quantity,
			&position.FrozenQuantity,
			&avgCost,
			&marketValue,
			&profitLoss,
			&position.CreatedAt,
			&position.UpdatedAt,
			&position.Version,
		); err != nil {
			return nil, fmt.Errorf("扫描持仓数据失败: %w", err)
		}

		position.AvgCost, _ = decimal.NewFromString(avgCost)
		position.MarketValue, _ = decimal.NewFromString(marketValue)
		position.ProfitLoss, _ = decimal.NewFromString(profitLoss)

		positions = append(positions, &position)
	}

	return positions, nil
}
