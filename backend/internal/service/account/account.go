package account

import (
	"context"
	"errors"
	"fmt"
	"math"

	"stock_trader/internal/cache"
	"stock_trader/internal/model"
	"stock_trader/internal/repository"
)

var (
	ErrInsufficientBalance = errors.New("可用余额不足")
	ErrInsufficientShares  = errors.New("可用股份不足")
	ErrAccountNotFound     = errors.New("账户不存在")
	ErrPositionNotFound    = errors.New("持仓不存在")
)

type Service struct {
	repo      *repository.Repository
	cache     *cache.RedisCache
}

func NewService(repo *repository.Repository, cache *cache.RedisCache) *Service {
	return &Service{
		repo:  repo,
		cache: cache,
	}
}

type AccountInfo struct {
	UserID        int64   `json:"user_id"`
	CashBalance   float64 `json:"cash_balance"`
	FrozenBalance float64 `json:"frozen_balance"`
	TotalAssets   float64 `json:"total_assets"`
	TotalProfit   float64 `json:"total_profit"`
}

type PositionInfo struct {
	Symbol            string  `json:"symbol"`
	SymbolName        string  `json:"symbol_name"`
	Quantity          int     `json:"quantity"`
	AvailableQuantity int     `json:"available_quantity"`
	CostPrice         float64 `json:"cost_price"`
	CurrentPrice      float64 `json:"current_price"`
	MarketValue       float64 `json:"market_value"`
	ProfitLoss        float64 `json:"profit_loss"`
	ProfitLossRate    float64 `json:"profit_loss_rate"`
}

func (s *Service) GetAccount(ctx context.Context, userID int64) (*AccountInfo, error) {
	account, err := s.repo.GetAccountByUserID(ctx, userID)
	if err != nil {
		return nil, ErrAccountNotFound
	}

	return &AccountInfo{
		UserID:        userID,
		CashBalance:   account.CashBalance,
		FrozenBalance: account.FrozenBalance,
		TotalAssets:   account.TotalAssets,
		TotalProfit:   account.TotalProfit,
	}, nil
}

func (s *Service) GetPositions(ctx context.Context, userID int64) ([]*PositionInfo, error) {
	positions, err := s.repo.GetPositions(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]*PositionInfo, 0, len(positions))
	for _, pos := range positions {
		result = append(result, &PositionInfo{
			Symbol:            pos.Symbol,
			SymbolName:        pos.SymbolName,
			Quantity:          pos.Quantity,
			AvailableQuantity: pos.AvailableQuantity,
			CostPrice:         pos.CostPrice,
			CurrentPrice:      pos.CurrentPrice,
			MarketValue:       pos.MarketValue,
			ProfitLoss:        pos.ProfitLoss,
			ProfitLossRate:    pos.ProfitLossRate,
		})
	}

	return result, nil
}

func (s *Service) Freeze(ctx context.Context, userID int64, amount float64) error {
	account, err := s.repo.GetAccountForUpdate(ctx, userID)
	if err != nil {
		return ErrAccountNotFound
	}

	if account.CashBalance < amount {
		return ErrInsufficientBalance
	}

	account.CashBalance -= amount
	account.FrozenBalance += amount

	if err := s.repo.UpdateAccount(ctx, account); err != nil {
		return fmt.Errorf("冻结资金失败: %w", err)
	}

	return nil
}

func (s *Service) Unfreeze(ctx context.Context, userID int64, amount float64) error {
	account, err := s.repo.GetAccountForUpdate(ctx, userID)
	if err != nil {
		return ErrAccountNotFound
	}

	if account.FrozenBalance < amount {
		return errors.New("冻结余额不足")
	}

	account.FrozenBalance -= amount
	account.CashBalance += amount

	if err := s.repo.UpdateAccount(ctx, account); err != nil {
		return fmt.Errorf("解冻资金失败: %w", err)
	}

	return nil
}

func (s *Service) Deduct(ctx context.Context, userID int64, amount float64) error {
	account, err := s.repo.GetAccountForUpdate(ctx, userID)
	if err != nil {
		return ErrAccountNotFound
	}

	if account.FrozenBalance < amount {
		return errors.New("冻结余额不足")
	}

	account.FrozenBalance -= amount
	account.TotalAssets -= amount

	if err := s.repo.UpdateAccount(ctx, account); err != nil {
		return fmt.Errorf("扣减资金失败: %w", err)
	}

	return nil
}

func (s *Service) AddCash(ctx context.Context, userID int64, amount float64) error {
	account, err := s.repo.GetAccountForUpdate(ctx, userID)
	if err != nil {
		return ErrAccountNotFound
	}

	account.CashBalance += amount
	account.TotalAssets += amount

	if err := s.repo.UpdateAccount(ctx, account); err != nil {
		return fmt.Errorf("增加现金失败: %w", err)
	}

	return nil
}

func (s *Service) UpdatePositionPrice(ctx context.Context, symbol string, price float64) error {
	positions, err := s.repo.GetAllPositions(ctx)
	if err != nil {
		return err
	}

	for _, pos := range positions {
		if pos.Symbol == symbol {
			pos.CurrentPrice = price
			pos.MarketValue = float64(pos.Quantity) * price
			if pos.CostPrice > 0 {
				pos.ProfitLoss = pos.MarketValue - float64(pos.Quantity)*pos.CostPrice
				pos.ProfitLossRate = pos.ProfitLoss / (float64(pos.Quantity) * pos.CostPrice)
			}
			if err := s.repo.UpdatePosition(ctx, pos); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Service) Buy(ctx context.Context, userID int64, symbol, symbolName string, price float64, quantity int) (*model.Position, error) {
	amount := price * float64(quantity)

	if err := s.Freeze(ctx, userID, amount); err != nil {
		return nil, err
	}

	position, err := s.repo.GetPosition(ctx, userID, symbol)
	if err != nil {
		position = &model.Position{
			UserID:        userID,
			Symbol:        symbol,
			SymbolName:    symbolName,
			Quantity:      0,
			AvailableQuantity: 0,
			CostPrice:     0,
		}
	}

	totalCost := position.CostPrice*float64(position.Quantity) + amount
	position.Quantity += quantity
	position.AvailableQuantity += quantity
	position.CostPrice = totalCost / float64(position.Quantity)
	position.CurrentPrice = price
	position.MarketValue = float64(position.Quantity) * price
	position.ProfitLoss = 0
	position.ProfitLossRate = 0

	if err := s.repo.UpsertPosition(ctx, position); err != nil {
		s.Unfreeze(ctx, userID, amount)
		return nil, fmt.Errorf("更新持仓失败: %w", err)
	}

	if err := s.Deduct(ctx, userID, amount); err != nil {
		return nil, err
	}

	return position, nil
}

func (s *Service) Sell(ctx context.Context, userID int64, symbol string, price float64, quantity int) (*model.Position, error) {
	position, err := s.repo.GetPosition(ctx, userID, symbol)
	if err != nil {
		return nil, ErrPositionNotFound
	}

	if position.AvailableQuantity < quantity {
		return nil, ErrInsufficientShares
	}

	amount := price * float64(quantity)
	fee := amount * 0.0003
	netAmount := amount - fee

	position.Quantity -= quantity
	position.AvailableQuantity -= quantity
	position.CurrentPrice = price
	position.MarketValue = float64(position.Quantity) * price
	position.ProfitLoss = position.MarketValue - float64(position.Quantity)*position.CostPrice
	if position.Quantity > 0 && position.CostPrice > 0 {
		position.ProfitLossRate = position.ProfitLoss / (float64(position.Quantity) * position.CostPrice)
	} else {
		position.ProfitLoss = 0
		position.ProfitLossRate = 0
	}

	if err := s.repo.UpdatePosition(ctx, position); err != nil {
		return nil, fmt.Errorf("更新持仓失败: %w", err)
	}

	if err := s.AddCash(ctx, userID, netAmount); err != nil {
		return nil, err
	}

	account, err := s.repo.GetAccountByUserID(ctx, userID)
	if err == nil {
		account.TotalProfit += position.ProfitLoss
		s.repo.UpdateAccount(ctx, account)
	}

	return position, nil
}

func roundToTwoDecimals(val float64) float64 {
	return math.Round(val*100) / 100
}
