package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"stock_trader/internal/kafka"
	"stock_trader/internal/model"
	"stock_trader/internal/repository"
	acc "stock_trader/internal/service/account"
)

var (
	ErrOrderNotFound       = errors.New("订单不存在")
	ErrOrderAlreadyFilled  = errors.New("订单已完全成交")
	ErrOrderCannotCancel   = errors.New("订单无法取消")
	ErrInsufficientFunds   = errors.New("资金不足")
	ErrInsufficientShares  = errors.New("股份不足")
	ErrInvalidOrderStatus  = errors.New("无效的订单状态")
)

type Service struct {
	repo       *repository.Repository
	accountSvc *acc.Service
	kafka      kafka.Producer
}

func NewService(repo *repository.Repository, accountSvc *acc.Service, kafkaClient kafka.Producer) *Service {
	return &Service{
		repo:       repo,
		accountSvc: accountSvc,
		kafka:      kafkaClient,
	}
}

type CreateOrderRequest struct {
	UserID    int64
	Symbol    string
	OrderType string
	Side      string
	Price     float64
	Quantity  int
}

type OrderInfo struct {
	OrderID        string    `json:"order_id"`
	Symbol         string    `json:"symbol"`
	SymbolName     string    `json:"symbol_name"`
	OrderType      string    `json:"order_type"`
	Side           string    `json:"side"`
	Price          float64   `json:"price"`
	Quantity       int       `json:"quantity"`
	FilledQuantity int       `json:"filled_quantity"`
	Status         string    `json:"status"`
	Fee            float64   `json:"fee"`
	CreatedAt      time.Time `json:"created_at"`
	FilledAt       *time.Time `json:"filled_at,omitempty"`
}

type TradeInfo struct {
	TradeID    string    `json:"trade_id"`
	OrderID    string    `json:"order_id"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	Price      float64   `json:"price"`
	Quantity   int       `json:"quantity"`
	Amount     float64   `json:"amount"`
	Fee        float64   `json:"fee"`
	ProfitLoss float64   `json:"profit_loss,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *Service) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*OrderInfo, error) {
	orderID := uuid.New().String()
	now := time.Now()

	amount := req.Price * float64(req.Quantity)

	if req.Side == "BUY" {
		if err := s.accountSvc.Freeze(ctx, req.UserID, amount); err != nil {
			return nil, ErrInsufficientFunds
		}
	}

	order := &model.Order{
		OrderID:        orderID,
		UserID:         req.UserID,
		Symbol:         req.Symbol,
		SymbolName:     req.Symbol,
		OrderType:      model.OrderType(req.OrderType),
		Side:           model.OrderSide(req.Side),
		Price:          req.Price,
		Quantity:       req.Quantity,
		FilledQuantity: 0,
		Status:         model.OrderStatusPending,
		Fee:            0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repo.CreateOrder(ctx, order); err != nil {
		if req.Side == "BUY" {
			s.accountSvc.Unfreeze(ctx, req.UserID, amount)
		}
		return nil, fmt.Errorf("创建订单失败: %w", err)
	}

	if err := s.kafka.SendOrderCreated(ctx, &kafka.OrderEvent{
		OrderID:   orderID,
		UserID:    req.UserID,
		Symbol:    req.Symbol,
		Side:      req.Side,
		OrderType: req.OrderType,
		Price:     req.Price,
		Quantity:  req.Quantity,
		CreatedAt: now,
	}); err != nil {
		fmt.Printf("发送订单创建事件失败: %v\n", err)
	}

	return &OrderInfo{
		OrderID:        order.OrderID,
		Symbol:         order.Symbol,
		SymbolName:     order.SymbolName,
		OrderType:      string(order.OrderType),
		Side:           string(order.Side),
		Price:          order.Price,
		Quantity:       order.Quantity,
		FilledQuantity: order.FilledQuantity,
		Status:         string(order.Status),
		Fee:            order.Fee,
		CreatedAt:      order.CreatedAt,
	}, nil
}

func (s *Service) CancelOrder(ctx context.Context, userID int64, orderID string) error {
	order, err := s.repo.GetOrderForUpdate(ctx, orderID)
	if err != nil {
		return ErrOrderNotFound
	}

	if order.UserID != userID {
		return errors.New("无权操作此订单")
	}

	if order.Status != model.OrderStatusPending && order.Status != model.OrderStatusPartial {
		return ErrOrderCannotCancel
	}

	now := time.Now()
	order.Status = model.OrderStatusCancelled
	order.UpdatedAt = now

	if err := s.repo.UpdateOrder(ctx, order); err != nil {
		return fmt.Errorf("取消订单失败: %w", err)
	}

	if order.Side == model.OrderSideBuy {
		frozenAmount := order.Price * float64(order.Quantity-order.FilledQuantity)
		if err := s.accountSvc.Unfreeze(ctx, userID, frozenAmount); err != nil {
			return fmt.Errorf("解冻资金失败: %w", err)
		}
	}

	s.kafka.SendOrderCancelled(ctx, &kafka.OrderEvent{
		OrderID:   orderID,
		UserID:    userID,
		Symbol:    order.Symbol,
		Side:      string(order.Side),
		Quantity:  order.Quantity - order.FilledQuantity,
		UpdatedAt: now,
	})

	return nil
}

func (s *Service) ListOrders(ctx context.Context, userID int64, symbol, status string, limit, offset int) ([]*OrderInfo, int64, error) {
	orders, total, err := s.repo.ListOrders(ctx, userID, symbol, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*OrderInfo, 0, len(orders))
	for _, order := range orders {
		result = append(result, &OrderInfo{
			OrderID:        order.OrderID,
			Symbol:         order.Symbol,
			SymbolName:     order.SymbolName,
			OrderType:      string(order.OrderType),
			Side:           string(order.Side),
			Price:          order.Price,
			Quantity:       order.Quantity,
			FilledQuantity: order.FilledQuantity,
			Status:         string(order.Status),
			Fee:            order.Fee,
			CreatedAt:      order.CreatedAt,
			FilledAt:       order.FilledAt,
		})
	}

	return result, total, nil
}

func (s *Service) GetTrades(ctx context.Context, userID int64, symbol string, limit, offset int) ([]*TradeInfo, int64, error) {
	trades, total, err := s.repo.GetTrades(ctx, userID, symbol, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*TradeInfo, 0, len(trades))
	for _, trade := range trades {
		result = append(result, &TradeInfo{
			TradeID:    trade.TradeID,
			OrderID:    trade.OrderID,
			Symbol:     trade.Symbol,
			Side:       string(trade.Side),
			Price:      trade.Price,
			Quantity:   trade.Quantity,
			Amount:     trade.Amount,
			Fee:        trade.Fee,
			ProfitLoss: trade.ProfitLoss,
			CreatedAt:  trade.CreatedAt,
		})
	}

	return result, total, nil
}

func (s *Service) GetOrder(ctx context.Context, orderID string) (*OrderInfo, error) {
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}

	return &OrderInfo{
		OrderID:        order.OrderID,
		Symbol:         order.Symbol,
		SymbolName:     order.SymbolName,
		OrderType:      string(order.OrderType),
		Side:           string(order.Side),
		Price:          order.Price,
		Quantity:       order.Quantity,
		FilledQuantity: order.FilledQuantity,
		Status:         string(order.Status),
		Fee:            order.Fee,
		CreatedAt:      order.CreatedAt,
		FilledAt:       order.FilledAt,
	}, nil
}

func (s *Service) ProcessTrade(ctx context.Context, trade *kafka.TradeEvent) error {
	order, err := s.repo.GetOrder(ctx, trade.OrderID)
	if err != nil {
		return fmt.Errorf("订单不存在: %w", err)
	}

	if order.Status == model.OrderStatusFilled || order.Status == model.OrderStatusCancelled {
		return nil
	}

	fee := trade.Amount * 0.0003
	order.FilledQuantity += trade.Quantity
	order.Fee += fee
	now := time.Now()
	order.UpdatedAt = now

	if order.FilledQuantity >= order.Quantity {
		order.Status = model.OrderStatusFilled
		order.FilledAt = &now
	} else {
		order.Status = model.OrderStatusPartial
	}

	if err := s.repo.UpdateOrder(ctx, order); err != nil {
		return fmt.Errorf("更新订单失败: %w", err)
	}

	dbTrade := &model.Trade{
		TradeID:        trade.TradeID,
		OrderID:        trade.OrderID,
		CounterOrderID: trade.CounterOrderID,
		UserID:         order.UserID,
		Symbol:         order.Symbol,
		Side:           order.Side,
		Price:          trade.Price,
		Quantity:       trade.Quantity,
		Amount:         trade.Amount,
		Fee:            fee,
		CreatedAt:      now,
	}

	if err := s.repo.CreateTrade(ctx, dbTrade); err != nil {
		return fmt.Errorf("创建成交记录失败: %w", err)
	}

	return nil
}
