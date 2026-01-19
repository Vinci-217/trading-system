package service

import (
	"context"
	"time"

	"stock_trader/backend/services/account-service/internal/domain/entity"
	"stock_trader/backend/services/account-service/internal/domain/repository"
	"stock_trader/backend/services/account-service/internal/infrastructure/logger"

	"github.com/shopspring/decimal"
	"github.com/google/uuid"
)

type AccountDomainService struct {
	accountRepo  repository.AccountRepository
	positionRepo repository.PositionRepository
	logger       *logger.Logger
}

func NewAccountDomainService(accountRepo repository.AccountRepository, positionRepo repository.PositionRepository, logger *logger.Logger) *AccountDomainService {
	return &AccountDomainService{
		accountRepo:  accountRepo,
		positionRepo: positionRepo,
		logger:       logger,
	}
}

func (s *AccountDomainService) CreateAccount(ctx context.Context, userID string) (*entity.Account, error) {
	account := entity.NewAccount(userID)
	if err := s.accountRepo.Create(ctx, account); err != nil {
		return nil, err
	}
	s.logger.Info("账户创建成功", logger.String("user_id", userID))
	return account, nil
}

func (s *AccountDomainService) GetAccount(ctx context.Context, userID string) (*entity.Account, error) {
	return s.accountRepo.Get(ctx, userID)
}

func (s *AccountDomainService) GetAccountForUpdate(ctx context.Context, userID string) (*entity.Account, error) {
	return s.accountRepo.GetForUpdate(ctx, userID)
}

func (s *AccountDomainService) Deposit(ctx context.Context, userID string, amount decimal.Decimal) (*entity.Account, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, entity.ErrInvalidAmount
	}

	account, err := s.accountRepo.GetForUpdate(ctx, userID)
	if err != nil {
		return nil, err
	}

	account.Deposit(amount)
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, err
	}

	s.logger.Info("存款成功",
		logger.String("user_id", userID),
		logger.Decimal("amount", amount),
		logger.Decimal("new_balance", account.CashBalance))

	return account, nil
}

func (s *AccountDomainService) Withdraw(ctx context.Context, userID string, amount decimal.Decimal) (*entity.Account, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, entity.ErrInvalidAmount
	}

	account, err := s.accountRepo.GetForUpdate(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := account.Withdraw(amount); err != nil {
		return nil, err
	}

	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, err
	}

	s.logger.Info("取款成功",
		logger.String("user_id", userID),
		logger.Decimal("amount", amount),
		logger.Decimal("new_balance", account.CashBalance))

	return account, nil
}

func (s *AccountDomainService) LockFunds(ctx context.Context, userID string, orderID string, transactionID string, amount decimal.Decimal, timeout time.Duration) (*entity.FundLock, *entity.TCCTransaction, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, nil, entity.ErrInvalidAmount
	}

	account, err := s.accountRepo.GetForUpdate(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	if err := account.LockFunds(amount); err != nil {
		return nil, nil, err
	}

	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, nil, err
	}

	fundLock := entity.NewFundLock(userID, orderID, transactionID, amount, timeout)
	tccTx := entity.NewTCCTransaction(userID, orderID, transactionID, amount)
	tccTx.Status = "SUCCESS"
	tccTx.TryTime = time.Now()

	s.logger.Info("资金锁定成功",
		logger.String("user_id", userID),
		logger.String("order_id", orderID),
		logger.Decimal("amount", amount))

	return fundLock, tccTx, nil
}

func (s *AccountDomainService) UnlockFunds(ctx context.Context, userID string, orderID string, amount decimal.Decimal) error {
	account, err := s.accountRepo.GetForUpdate(ctx, userID)
	if err != nil {
		return err
	}

	account.UnlockFunds(amount)
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return err
	}

	s.logger.Info("资金解锁成功",
		logger.String("user_id", userID),
		logger.String("order_id", orderID),
		logger.Decimal("amount", amount))

	return nil
}

func (s *AccountDomainService) ConfirmLock(ctx context.Context, userID string, amount decimal.Decimal) error {
	account, err := s.accountRepo.GetForUpdate(ctx, userID)
	if err != nil {
		return err
	}

	account.ConfirmLock(amount)
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return err
	}

	s.logger.Info("锁定确认成功",
		logger.String("user_id", userID),
		logger.Decimal("amount", amount))

	return nil
}

func (s *AccountDomainService) SettleTrade(ctx context.Context, userID string, symbol string, side string, quantity int, price decimal.Decimal) (*entity.Position, error) {
	account, err := s.accountRepo.GetForUpdate(ctx, userID)
	if err != nil {
		return nil, err
	}

	amount := price.Mul(decimal.NewFromInt(int64(quantity)))

	if side == "BUY" {
		if err := account.Withdraw(amount); err != nil {
			return nil, err
		}
	} else {
		account.Deposit(amount)
	}

	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, err
	}

	position, err := s.positionRepo.GetForUpdate(ctx, userID, symbol)
	if err != nil {
		position = entity.NewPosition(userID, symbol)
	}

	if side == "BUY" {
		position.Buy(quantity, price)
	} else {
		position.Sell(quantity)
	}

	if err := s.positionRepo.Update(ctx, position); err != nil {
		return nil, err
	}

	s.logger.Info("交易结算成功",
		logger.String("user_id", userID),
		logger.String("symbol", symbol),
		logger.String("side", side),
		logger.Int("quantity", quantity),
		logger.Decimal("price", price))

	return position, nil
}

func (s *AccountDomainService) GetPosition(ctx context.Context, userID string, symbol string) (*entity.Position, error) {
	return s.positionRepo.Get(ctx, userID, symbol)
}

func (s *AccountDomainService) GetPositions(ctx context.Context, userID string) ([]*entity.Position, error) {
	return s.positionRepo.GetByUserID(ctx, userID)
}

func (s *AccountDomainService) CreatePosition(ctx context.Context, userID string, symbol string) (*entity.Position, error) {
	position := entity.NewPosition(userID, symbol)
	position.ID = uuid.New().String()
	if err := s.positionRepo.Create(ctx, position); err != nil {
		return nil, err
	}
	return position, nil
}

func (s *AccountDomainService) Reconcile(ctx context.Context) ([]string, error) {
	accounts, err := s.accountRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	var discrepancies []string

	for _, account := range accounts {
		calcAvailable := account.CashBalance.Sub(account.FrozenBalance)
		calcTotalAssets := account.CashBalance.Add(account.SecuritiesBalance)

		if account.TotalAssets.LessThan(decimal.Zero) {
			discrepancies = append(discrepancies, 
				logger.String("user_id", account.UserID),
				logger.String("issue", "negative total assets"))
		}

		if account.FrozenBalance.GreaterThan(account.CashBalance) {
			discrepancies = append(discrepancies, 
				logger.String("user_id", account.UserID),
				logger.String("issue", "frozen balance exceeds cash balance"))
		}

		if calcAvailable.LessThan(decimal.Zero) {
			discrepancies = append(discrepancies, 
				logger.String("user_id", account.UserID),
				logger.String("issue", "available balance is negative"))
		}

		account.TotalAssets = calcTotalAssets
		account.UpdatedAt = time.Now()
		s.accountRepo.Update(ctx, account)
	}

	s.logger.Info("对账完成",
		logger.Int("total_accounts", len(accounts)),
		logger.Int("discrepancies", len(discrepancies)))

	return discrepancies, nil
}
