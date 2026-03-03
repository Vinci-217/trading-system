package account

import (
	"context"
	"fmt"

	"github.com/stock-trading-system/internal/domain/account"
	"github.com/stock-trading-system/internal/infrastructure/cache"
	"github.com/stock-trading-system/internal/infrastructure/idgen"
	"github.com/stock-trading-system/pkg/errors"
	"github.com/stock-trading-system/pkg/logger"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type AccountService struct {
	db     *gorm.DB
	redis  *cache.RedisClient
	idgen  *idgen.IDGenerator
	logger *logger.Logger
}

func NewAccountService(db *gorm.DB, redis *cache.RedisClient, idgen *idgen.IDGenerator, logger *logger.Logger) *AccountService {
	return &AccountService{
		db:     db,
		redis:  redis,
		idgen:  idgen,
		logger: logger,
	}
}

func (s *AccountService) GetAccount(ctx context.Context, userID string) (*account.Account, error) {
	var acc account.Account
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&acc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrAccountNotFound
		}
		return nil, err
	}
	return &acc, nil
}

func (s *AccountService) CreateAccount(ctx context.Context, userID string) (*account.Account, error) {
	acc := account.NewAccount(userID)
	err := s.db.WithContext(ctx).Create(acc).Error
	if err != nil {
		return nil, err
	}
	return acc, nil
}

func (s *AccountService) Deposit(ctx context.Context, userID string, amount decimal.Decimal) error {
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var acc account.Account
	err := tx.Where("user_id = ?", userID).First(&acc).Error
	if err != nil {
		tx.Rollback()
		return errors.ErrAccountNotFound
	}

	transactionID := s.idgen.GenerateTransactionID()
	flowID := s.idgen.GenerateFlowID("D")

	balanceBefore := acc.CashBalance
	err = acc.Deposit(amount)
	if err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Save(&acc).Error; err != nil {
		tx.Rollback()
		return err
	}

	flow := &account.FundFlow{
		FlowID:        flowID,
		TransactionID: transactionID,
		UserID:        userID,
		Amount:        amount,
		FlowType:      account.FlowTypeDeposit,
		BalanceBefore: balanceBefore,
		BalanceAfter:  acc.CashBalance,
		Status:        account.FlowStatusSuccess,
	}

	if err := tx.Create(flow).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (s *AccountService) Withdraw(ctx context.Context, userID string, amount decimal.Decimal) error {
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var acc account.Account
	err := tx.Where("user_id = ?", userID).First(&acc).Error
	if err != nil {
		tx.Rollback()
		return errors.ErrAccountNotFound
	}

	transactionID := s.idgen.GenerateTransactionID()
	flowID := s.idgen.GenerateFlowID("W")

	balanceBefore := acc.CashBalance
	err = acc.Withdraw(amount)
	if err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Save(&acc).Error; err != nil {
		tx.Rollback()
		return err
	}

	flow := &account.FundFlow{
		FlowID:        flowID,
		TransactionID: transactionID,
		UserID:        userID,
		Amount:        amount,
		FlowType:      account.FlowTypeWithdraw,
		BalanceBefore: balanceBefore,
		BalanceAfter:  acc.CashBalance,
		Status:        account.FlowStatusSuccess,
	}

	if err := tx.Create(flow).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (s *AccountService) FreezeFunds(ctx context.Context, userID, orderID string, amount decimal.Decimal) error {
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var acc account.Account
	err := tx.Where("user_id = ?", userID).First(&acc).Error
	if err != nil {
		tx.Rollback()
		return errors.ErrAccountNotFound
	}

	transactionID := s.idgen.GenerateTransactionID()

	flowID := s.idgen.GenerateFlowID("F")
	balanceBefore := acc.CashBalance
	frozenBefore := acc.FrozenBalance

	result := tx.Model(&account.Account{}).
		Where("user_id = ? AND (cash_balance - frozen_balance) >= ? AND version = ?",
			userID, amount, acc.Version).
		Updates(map[string]interface{}{
			"frozen_balance": gorm.Expr("frozen_balance + ?", amount),
			"version":        gorm.Expr("version + 1"),
			"updated_at":     gorm.Expr("NOW()"),
		})

	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}

	if result.RowsAffected == 0 {
		tx.Rollback()
		return errors.ErrInsufficientBalance
	}

	flow := &account.FundFlow{
		FlowID:        flowID,
		TransactionID: transactionID,
		UserID:        userID,
		OrderID:       orderID,
		Amount:        amount,
		FlowType:      account.FlowTypeFreeze,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceBefore,
		FrozenBefore:  frozenBefore,
		FrozenAfter:   frozenBefore.Add(amount),
		Status:        account.FlowStatusSuccess,
	}

	if err := tx.Create(flow).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (s *AccountService) UnfreezeFunds(ctx context.Context, userID, orderID string, amount decimal.Decimal) error {
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var acc account.Account
	err := tx.Where("user_id = ?", userID).First(&acc).Error
	if err != nil {
		tx.Rollback()
		return errors.ErrAccountNotFound
	}

	transactionID := s.idgen.GenerateTransactionID()
	flowID := s.idgen.GenerateFlowID("U")

	balanceBefore := acc.CashBalance
	frozenBefore := acc.FrozenBalance

	result := tx.Model(&account.Account{}).
		Where("user_id = ? AND frozen_balance >= ? AND version = ?",
			userID, amount, acc.Version).
		Updates(map[string]interface{}{
			"frozen_balance": gorm.Expr("frozen_balance - ?", amount),
			"version":        gorm.Expr("version + 1"),
			"updated_at":     gorm.Expr("NOW()"),
		})

	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}

	if result.RowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("insufficient frozen balance")
	}

	flow := &account.FundFlow{
		FlowID:        flowID,
		TransactionID: transactionID,
		UserID:        userID,
		OrderID:       orderID,
		Amount:        amount,
		FlowType:      account.FlowTypeUnfreeze,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceBefore,
		FrozenBefore:  frozenBefore,
		FrozenAfter:   frozenBefore.Sub(amount),
		Status:        account.FlowStatusSuccess,
	}

	if err := tx.Create(flow).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (s *AccountService) ConfirmFreeze(ctx context.Context, userID, orderID string, amount decimal.Decimal) error {
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var acc account.Account
	err := tx.Where("user_id = ?", userID).First(&acc).Error
	if err != nil {
		tx.Rollback()
		return errors.ErrAccountNotFound
	}

	transactionID := s.idgen.GenerateTransactionID()
	flowID := s.idgen.GenerateFlowID("C")

	balanceBefore := acc.CashBalance
	frozenBefore := acc.FrozenBalance

	result := tx.Model(&account.Account{}).
		Where("user_id = ? AND frozen_balance >= ? AND version = ?",
			userID, amount, acc.Version).
		Updates(map[string]interface{}{
			"cash_balance":   gorm.Expr("cash_balance - ?", amount),
			"frozen_balance": gorm.Expr("frozen_balance - ?", amount),
			"version":        gorm.Expr("version + 1"),
			"updated_at":     gorm.Expr("NOW()"),
		})

	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}

	if result.RowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("insufficient frozen balance")
	}

	flow := &account.FundFlow{
		FlowID:        flowID,
		TransactionID: transactionID,
		UserID:        userID,
		OrderID:       orderID,
		Amount:        amount,
		FlowType:      account.FlowTypeDeduct,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceBefore.Sub(amount),
		FrozenBefore:  frozenBefore,
		FrozenAfter:   frozenBefore.Sub(amount),
		Status:        account.FlowStatusSuccess,
	}

	if err := tx.Create(flow).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (s *AccountService) CreditFunds(ctx context.Context, userID, orderID string, amount decimal.Decimal) error {
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var acc account.Account
	err := tx.Where("user_id = ?", userID).First(&acc).Error
	if err != nil {
		tx.Rollback()
		return errors.ErrAccountNotFound
	}

	transactionID := s.idgen.GenerateTransactionID()
	flowID := s.idgen.GenerateFlowID("CR")

	balanceBefore := acc.CashBalance

	result := tx.Model(&account.Account{}).
		Where("user_id = ? AND version = ?", userID, acc.Version).
		Updates(map[string]interface{}{
			"cash_balance": gorm.Expr("cash_balance + ?", amount),
			"total_assets": gorm.Expr("total_assets + ?", amount),
			"version":      gorm.Expr("version + 1"),
			"updated_at":   gorm.Expr("NOW()"),
		})

	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}

	flow := &account.FundFlow{
		FlowID:        flowID,
		TransactionID: transactionID,
		UserID:        userID,
		OrderID:       orderID,
		Amount:        amount,
		FlowType:      account.FlowTypeCredit,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceBefore.Add(amount),
		Status:        account.FlowStatusSuccess,
	}

	if err := tx.Create(flow).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (s *AccountService) GetPositions(ctx context.Context, userID string) ([]*account.Position, error) {
	var positions []*account.Position
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&positions).Error
	if err != nil {
		return nil, err
	}
	return positions, nil
}

func (s *AccountService) GetPosition(ctx context.Context, userID, symbol string) (*account.Position, error) {
	var pos account.Position
	err := s.db.WithContext(ctx).Where("user_id = ? AND symbol = ?", userID, symbol).First(&pos).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrInsufficientPosition
		}
		return nil, err
	}
	return &pos, nil
}

func (s *AccountService) AddPosition(ctx context.Context, userID, symbol string, quantity int, price decimal.Decimal) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var pos account.Position
		err := tx.Where("user_id = ? AND symbol = ?", userID, symbol).First(&pos).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				pos = *account.NewPosition(userID, symbol)
			} else {
				return err
			}
		}
		pos.Buy(quantity, price)
		return tx.Save(&pos).Error
	})
}

func (s *AccountService) ReducePosition(ctx context.Context, userID, symbol string, quantity int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var pos account.Position
		err := tx.Where("user_id = ? AND symbol = ?", userID, symbol).First(&pos).Error
		if err != nil {
			return errors.ErrInsufficientPosition
		}

		if pos.AvailableQuantity() < quantity {
			return errors.ErrInsufficientPosition
		}

		pos.Sell(quantity)
		return tx.Save(&pos).Error
	})
}
