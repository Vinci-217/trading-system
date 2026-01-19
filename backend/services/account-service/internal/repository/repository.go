package main

import (
	"context"
	"database/sql"
	"fmt"

	"stock_trader/common/model"

	"github.com/shopspring/decimal"
)

type PostgresAccountRepository struct {
	db *sql.DB
}

func NewPostgresAccountRepository(db *sql.DB) *PostgresAccountRepository {
	return &PostgresAccountRepository{db: db}
}

func (r *PostgresAccountRepository) CreateAccount(ctx context.Context, account *model.Account) error {
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
		return fmt.Errorf("failed to create account: %w", err)
	}

	return nil
}

func (r *PostgresAccountRepository) GetAccount(ctx context.Context, userID string) (*model.Account, error) {
	query := `
		SELECT user_id, cash_balance, frozen_balance, securities_balance, total_assets, total_deposit, total_withdrawal, created_at, updated_at, version
		FROM accounts WHERE user_id = ?
	`

	var account model.Account
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&account.UserID,
		&account.CashBalance,
		&account.FrozenBalance,
		&account.SecuritiesBalance,
		&account.TotalAssets,
		&account.TotalDeposit,
		&account.TotalWithdrawal,
		&account.CreatedAt,
		&account.UpdatedAt,
		&account.Version,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("account not found: %s", userID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &account, nil
}

func (r *PostgresAccountRepository) UpdateAccount(ctx context.Context, account *model.Account) error {
	query := `
		UPDATE accounts SET 
			cash_balance = ?, frozen_balance = ?, securities_balance = ?, total_assets = ?,
			total_deposit = ?, total_withdrawal = ?, updated_at = ?, version = ?
		WHERE user_id = ? AND version = ?
	`

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
		return fmt.Errorf("failed to update account: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("optimistic lock failed for account: %s", account.UserID)
	}

	account.Version++
	return nil
}

func (r *PostgresAccountRepository) GetAccountForUpdate(ctx context.Context, userID string) (*model.Account, error) {
	query := `
		SELECT user_id, cash_balance, frozen_balance, securities_balance, total_assets, total_deposit, total_withdrawal, created_at, updated_at, version
		FROM accounts WHERE user_id = ? FOR UPDATE
	`

	var account model.Account
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&account.UserID,
		&account.CashBalance,
		&account.FrozenBalance,
		&account.SecuritiesBalance,
		&account.TotalAssets,
		&account.TotalDeposit,
		&account.TotalWithdrawal,
		&account.CreatedAt,
		&account.UpdatedAt,
		&account.Version,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("account not found: %s", userID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get account for update: %w", err)
	}

	return &account, nil
}

func (r *PostgresAccountRepository) LockAccount(ctx context.Context, userID string) error {
	query := `SELECT user_id FROM accounts WHERE user_id = ? FOR UPDATE`

	var dummy string
	return r.db.QueryRowContext(ctx, query, userID).Scan(&dummy)
}

func (r *PostgresAccountRepository) UnlockAccount(ctx context.Context, userID string) error {
	return nil
}

type PostgresPositionRepository struct {
	db *sql.DB
}

func NewPostgresPositionRepository(db *sql.DB) *PostgresPositionRepository {
	return &PostgresPositionRepository{db: db}
}

func (r *PostgresPositionRepository) CreatePosition(ctx context.Context, position *model.Position) error {
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
		return fmt.Errorf("failed to create position: %w", err)
	}

	return nil
}

func (r *PostgresPositionRepository) GetPosition(ctx context.Context, userID string, symbol string) (*model.Position, error) {
	query := `
		SELECT id, user_id, symbol, quantity, frozen_quantity, avg_cost, market_value, profit_loss, created_at, updated_at, version
		FROM positions WHERE user_id = ? AND symbol = ?
	`

	var position model.Position
	err := r.db.QueryRowContext(ctx, query, userID, symbol).Scan(
		&position.ID,
		&position.UserID,
		&position.Symbol,
		&position.Quantity,
		&position.FrozenQuantity,
		&position.AvgCost,
		&position.MarketValue,
		&position.ProfitLoss,
		&position.CreatedAt,
		&position.UpdatedAt,
		&position.Version,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get position: %w", err)
	}

	return &position, nil
}

func (r *PostgresPositionRepository) GetPositions(ctx context.Context, userID string) ([]*model.Position, error) {
	query := `
		SELECT id, user_id, symbol, quantity, frozen_quantity, avg_cost, market_value, profit_loss, created_at, updated_at, version
		FROM positions WHERE user_id = ?
		ORDER BY symbol
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query positions: %w", err)
	}
	defer rows.Close()

	var positions []*model.Position
	for rows.Next() {
		var position model.Position
		if err := rows.Scan(
			&position.ID,
			&position.UserID,
			&position.Symbol,
			&position.Quantity,
			&position.FrozenQuantity,
			&position.AvgCost,
			&position.MarketValue,
			&position.ProfitLoss,
			&position.CreatedAt,
			&position.UpdatedAt,
			&position.Version,
		); err != nil {
			return nil, fmt.Errorf("failed to scan position: %w", err)
		}
		positions = append(positions, &position)
	}

	return positions, nil
}

func (r *PostgresPositionRepository) UpdatePosition(ctx context.Context, position *model.Position) error {
	query := `
		UPDATE positions SET 
			quantity = ?, frozen_quantity = ?, avg_cost = ?, market_value = ?, profit_loss = ?, updated_at = ?, version = ?
		WHERE id = ? AND version = ?
	`

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
		return fmt.Errorf("failed to update position: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("optimistic lock failed for position: %s", position.ID)
	}

	position.Version++
	return nil
}

func (r *PostgresPositionRepository) GetPositionForUpdate(ctx context.Context, userID string, symbol string) (*model.Position, error) {
	query := `
		SELECT id, user_id, symbol, quantity, frozen_quantity, avg_cost, market_value, profit_loss, created_at, updated_at, version
		FROM positions WHERE user_id = ? AND symbol = ? FOR UPDATE
	`

	var position model.Position
	err := r.db.QueryRowContext(ctx, query, userID, symbol).Scan(
		&position.ID,
		&position.UserID,
		&position.Symbol,
		&position.Quantity,
		&position.FrozenQuantity,
		&position.AvgCost,
		&position.MarketValue,
		&position.ProfitLoss,
		&position.CreatedAt,
		&position.UpdatedAt,
		&position.Version,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get position for update: %w", err)
	}

	return &position, nil
}

type MemoryAccountRepository struct {
	accounts map[string]*model.Account
	mu       sync.RWMutex
}

func NewMemoryAccountRepository() *MemoryAccountRepository {
	return &MemoryAccountRepository{
		accounts: make(map[string]*model.Account),
	}
}

func (r *MemoryAccountRepository) CreateAccount(ctx context.Context, account *model.Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.accounts[account.UserID]; exists {
		return fmt.Errorf("account already exists: %s", account.UserID)
	}

	r.accounts[account.UserID] = account
	return nil
}

func (r *MemoryAccountRepository) GetAccount(ctx context.Context, userID string) (*model.Account, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	account, exists := r.accounts[userID]
	if !exists {
		return nil, fmt.Errorf("account not found: %s", userID)
	}

	return account, nil
}

func (r *MemoryAccountRepository) UpdateAccount(ctx context.Context, account *model.Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.accounts[account.UserID]
	if !exists {
		return fmt.Errorf("account not found: %s", account.UserID)
	}

	if account.Version != existing.Version {
		return fmt.Errorf("optimistic lock failed")
	}

	account.Version++
	r.accounts[account.UserID] = account
	return nil
}

func (r *MemoryAccountRepository) GetAccountForUpdate(ctx context.Context, userID string) (*model.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	account, exists := r.accounts[userID]
	if !exists {
		return nil, fmt.Errorf("account not found: %s", userID)
	}

	return account, nil
}

func (r *MemoryAccountRepository) LockAccount(ctx context.Context, userID string) error {
	return nil
}

func (r *MemoryAccountRepository) UnlockAccount(ctx context.Context, userID string) error {
	return nil
}

type MemoryPositionRepository struct {
	positions map[string]*model.Position
	mu        sync.RWMutex
}

func NewMemoryPositionRepository() *MemoryPositionRepository {
	return &MemoryPositionRepository{
		positions: make(map[string]*model.Position),
	}
}

func (r *MemoryPositionRepository) CreatePosition(ctx context.Context, position *model.Position) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s", position.UserID, position.Symbol)
	if _, exists := r.positions[key]; exists {
		return fmt.Errorf("position already exists: %s", key)
	}

	r.positions[key] = position
	return nil
}

func (r *MemoryPositionRepository) GetPosition(ctx context.Context, userID string, symbol string) (*model.Position, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", userID, symbol)
	position, exists := r.positions[key]
	if !exists {
		return nil, sql.ErrNoRows
	}

	return position, nil
}

func (r *MemoryPositionRepository) GetPositions(ctx context.Context, userID string) ([]*model.Position, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var positions []*model.Position
	for _, position := range r.positions {
		if position.UserID == userID {
			positions = append(positions, position)
		}
	}

	return positions, nil
}

func (r *MemoryPositionRepository) UpdatePosition(ctx context.Context, position *model.Position) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s", position.UserID, position.Symbol)
	existing, exists := r.positions[key]
	if !exists {
		return fmt.Errorf("position not found: %s", key)
	}

	if position.Version != existing.Version {
		return fmt.Errorf("optimistic lock failed")
	}

	position.Version++
	r.positions[key] = position
	return nil
}

func (r *MemoryPositionRepository) GetPositionForUpdate(ctx context.Context, userID string, symbol string) (*model.Position, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s", userID, symbol)
	position, exists := r.positions[key]
	if !exists {
		return nil, sql.ErrNoRows
	}

	return position, nil
}

import (
	"sync"
	"fmt"
)
