package service

import (
	"context"
	"fmt"
	"time"

	"stock_trader/backend/services/order-service/internal/domain/entity"
	"stock_trader/backend/services/order-service/internal/domain/repository"
	"stock_trader/backend/services/order-service/internal/infrastructure/logger"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type OrderDomainService struct {
	orderRepo      repository.OrderRepository
	tccRepo        repository.TCCTransactionRepository
	logger         *logger.Logger
	rateLimiter    *RateLimiter
	tccManager     *TCCManager
}

func NewOrderDomainService(orderRepo repository.OrderRepository, tccRepo repository.TCCTransactionRepository, logger *logger.Logger) *OrderDomainService {
	return &OrderDomainService{
		orderRepo:   orderRepo,
		tccRepo:     tccRepo,
		logger:      logger,
		rateLimiter: NewRateLimiter(),
		tccManager:  NewTCCManager(),
	}
}

type RateLimiter struct {
	limits map[string][]time.Time
	mu     sync.RWMutex
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limits: make(map[string][]time.Time),
	}
}

func (rl *RateLimiter) CheckLimit(userID string, limit int, window time.Duration) (bool, string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	key := fmt.Sprintf("ratelimit:%s", userID)

	requests := rl.limits[key]
	var validRequests []time.Time
	for _, t := range requests {
		if now.Sub(t) < window {
			validRequests = append(validRequests, t)
		}
	}

	if len(validRequests) >= limit {
		return true, fmt.Sprintf("频率限制: 最多 %d 次/%v", limit, window)
	}

	validRequests = append(validRequests, now)
	rl.limits[key] = validRequests
	return false, ""
}

type TCCManager struct {
	transactions map[string]*TCCTransaction
	mu           sync.RWMutex
}

func NewTCCManager() *TCCManager {
	return &TCCManager{
		transactions: make(map[string]*TCCTransaction),
	}
}

type TCCTransaction struct {
	OrderID       string
	TransactionID string
	Status        string
	TryResult     *TryResult
	ConfirmTime   time.Time
	CancelTime    time.Time
	RetryCount    int
	MaxRetries    int
}

type TryResult struct {
	FundsLocked  bool
	LockAmount   decimal.Decimal
	ErrorMsg     string
}

func (tm *TCCManager) CreateTransaction(orderID string) *TCCTransaction {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tx := &TCCTransaction{
		OrderID:       orderID,
		TransactionID: uuid.New().String(),
		Status:        "TRYING",
		MaxRetries:    3,
		RetryCount:    0,
	}
	tm.transactions[orderID] = tx
	return tx
}

func (tm *TCCManager) GetTransaction(orderID string) (*TCCTransaction, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tx, ok := tm.transactions[orderID]
	return tx, ok
}

func (tm *TCCManager) UpdateStatus(orderID string, status string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tx, ok := tm.transactions[orderID]; ok {
		tx.Status = status
	}
}

func (s *OrderDomainService) CreateOrder(ctx context.Context, userID string, symbol string, orderType entity.OrderType, side entity.OrderSide, price decimal.Decimal, quantity int, clientOrderID string) (*entity.Order, *TCCTransaction, error) {
	if quantity <= 0 {
		return nil, nil, entity.ErrInvalidQuantity
	}

	if price.LessThanOrEqual(decimal.Zero) {
		return nil, nil, entity.ErrInvalidPrice
	}

	if limited, msg := s.rateLimiter.CheckLimit(userID, 60, time.Minute); limited {
		return nil, nil, fmt.Errorf(msg)
	}

	if limited, msg := s.rateLimiter.CheckLimit(userID, 1000, time.Hour); limited {
		return nil, nil, fmt.Errorf(msg)
	}

	orderID := uuid.New().String()
	tccTx := s.tccManager.CreateTransaction(orderID)

	order := entity.NewOrder(userID, symbol, orderType, side, price, quantity)
	order.ID = orderID
	order.ClientOrderID = clientOrderID

	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, nil, fmt.Errorf("创建订单失败: %w", err)
	}

	s.logger.Info("订单创建成功",
		logger.String("order_id", orderID),
		logger.String("user_id", userID),
		logger.String("symbol", symbol),
		logger.String("side", string(side)),
		logger.Int("quantity", quantity))

	return order, tccTx, nil
}

func (s *OrderDomainService) GetOrder(ctx context.Context, orderID string) (*entity.Order, error) {
	return s.orderRepo.Get(ctx, orderID)
}

func (s *OrderDomainService) GetOrdersByUser(ctx context.Context, userID string, status string, limit int) ([]*entity.Order, error) {
	return s.orderRepo.GetByUserID(ctx, userID, status, limit)
}

func (s *OrderDomainService) GetOrdersBySymbol(ctx context.Context, symbol string, status string, limit int) ([]*entity.Order, error) {
	return s.orderRepo.GetBySymbol(ctx, symbol, status, limit)
}

func (s *OrderDomainService) CancelOrder(ctx context.Context, orderID string, userID string) error {
	if limited, msg := s.rateLimiter.CheckLimit(userID, 30, time.Minute); limited {
		return fmt.Errorf(msg)
	}

	order, err := s.orderRepo.GetForUpdate(ctx, orderID)
	if err != nil {
		return err
	}

	if order.UserID != userID {
		return fmt.Errorf("无权取消此订单")
	}

	if !order.CanCancel() {
		return entity.ErrOrderCannotCancel
	}

	order.Cancel("用户取消")
	if err := s.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("更新订单失败: %w", err)
	}

	s.logger.Info("订单取消成功",
		logger.String("order_id", orderID),
		logger.String("user_id", userID))

	return nil
}

func (s *OrderDomainService) ConfirmOrder(ctx context.Context, orderID string) error {
	tx, exists := s.tccManager.GetTransaction(orderID)
	if !exists {
		return fmt.Errorf("事务不存在")
	}

	if tx.Status != "TRY_SUCCESS" {
		return entity.ErrOrderCannotConfirm
	}

	s.tccManager.UpdateStatus(orderID, "CONFIRMED")
	tx.ConfirmTime = time.Now()

	s.logger.Info("订单确认成功",
		logger.String("order_id", orderID))

	return nil
}

func (s *OrderDomainService) FillOrder(ctx context.Context, orderID string, quantity int, price decimal.Decimal) error {
	order, err := s.orderRepo.GetForUpdate(ctx, orderID)
	if err != nil {
		return err
	}

	order.Fill(quantity, price)
	if err := s.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("更新订单失败: %w", err)
	}

	s.logger.Info("订单成交",
		logger.String("order_id", orderID),
		logger.Int("filled_quantity", order.FilledQuantity),
		logger.String("status", string(order.Status)))

	return nil
}

func (s *OrderDomainService) RejectOrder(ctx context.Context, orderID string, reason string) error {
	order, err := s.orderRepo.GetForUpdate(ctx, orderID)
	if err != nil {
		return err
	}

	order.Reject(reason)
	if err := s.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("更新订单失败: %w", err)
	}

	s.logger.Info("订单拒绝",
		logger.String("order_id", orderID),
		logger.String("reason", reason))

	return nil
}

import "sync"
