package trading

import (
	"context"
	"fmt"

	"github.com/stock-trading-system/internal/domain/matching"
	"github.com/stock-trading-system/internal/domain/order"
	"github.com/stock-trading-system/internal/domain/trade"
	"github.com/stock-trading-system/internal/infrastructure/cache"
	"github.com/stock-trading-system/internal/infrastructure/idgen"
	"github.com/stock-trading-system/internal/infrastructure/mq"
	"github.com/stock-trading-system/internal/service/account"
	"github.com/stock-trading-system/pkg/errors"
	"github.com/stock-trading-system/pkg/logger"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type TradingService struct {
	db             *gorm.DB
	redis          *cache.RedisClient
	idgen          *idgen.IDGenerator
	kafkaProducer  *mq.KafkaProducer
	accountService *account.AccountService
	matchingEngine *matching.MatchingEngine
	logger         *logger.Logger
}

func NewTradingService(
	db *gorm.DB,
	redis *cache.RedisClient,
	idgen *idgen.IDGenerator,
	kafkaProducer *mq.KafkaProducer,
	accountService *account.AccountService,
	matchingEngine *matching.MatchingEngine,
	logger *logger.Logger,
) *TradingService {
	return &TradingService{
		db:             db,
		redis:          redis,
		idgen:          idgen,
		kafkaProducer:  kafkaProducer,
		accountService: accountService,
		matchingEngine: matchingEngine,
		logger:         logger,
	}
}

type CreateOrderRequest struct {
	UserID        string
	Symbol        string
	Side          int
	OrderType     int
	Price         decimal.Decimal
	Quantity      int
	ClientOrderID string
}

func (s *TradingService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*order.Order, []*trade.Trade, error) {
	if req.ClientOrderID != "" {
		idempotencyKey := fmt.Sprintf("order:%s:%s", req.UserID, req.ClientOrderID)
		ok, err := s.redis.SetNX(ctx, idempotencyKey, "processing", 30)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			var existingOrder order.Order
			err := s.db.WithContext(ctx).
				Where("user_id = ? AND client_order_id = ?", req.UserID, req.ClientOrderID).
				First(&existingOrder).Error
			if err == nil {
				return &existingOrder, nil, nil
			}
			return nil, nil, errors.ErrOrderProcessing
		}
		defer func() {
			s.redis.Del(ctx, idempotencyKey)
		}()
	}

	orderID, err := s.idgen.GenerateOrderID(ctx, req.UserID)
	if err != nil {
		return nil, nil, err
	}

	o := order.NewOrder(orderID, req.UserID, req.Symbol, req.Side, req.OrderType, req.Price, req.Quantity)
	if req.ClientOrderID != "" {
		o.ClientOrderID = req.ClientOrderID
	}

	if req.Side == order.OrderSideBuy {
		freezeAmount := req.Price.Mul(decimal.NewFromInt(int64(req.Quantity)))
		if err := s.accountService.FreezeFunds(ctx, req.UserID, orderID, freezeAmount); err != nil {
			o.Reject("资金冻结失败: " + err.Error())
			s.db.WithContext(ctx).Create(o)
			return nil, nil, err
		}
	}

	if req.Side == order.OrderSideSell {
		pos, err := s.accountService.GetPosition(ctx, req.UserID, req.Symbol)
		if err != nil || pos.AvailableQuantity() < req.Quantity {
			o.Reject("持仓不足")
			s.db.WithContext(ctx).Create(o)
			return nil, nil, errors.ErrInsufficientPosition
		}
	}

	o.SetPending()
	if err := s.db.WithContext(ctx).Create(o).Error; err != nil {
		return nil, nil, err
	}

	trades, err := s.matchingEngine.SubmitOrder(ctx, o)
	if err != nil {
		return nil, nil, err
	}

	if len(trades) > 0 {
		for _, t := range trades {
			if err := s.db.WithContext(ctx).Create(t).Error; err != nil {
				s.logger.Errorw("failed to save trade", "error", err)
			}
		}

		if err := s.db.WithContext(ctx).Save(o).Error; err != nil {
			s.logger.Errorw("failed to update order", "error", err)
		}

		s.kafkaProducer.SendMessage(ctx, "order.matched", o.OrderID, map[string]interface{}{
			"order_id": o.OrderID,
			"trades":   len(trades),
		})
	}

	s.kafkaProducer.SendMessage(ctx, "order.created", o.OrderID, map[string]interface{}{
		"order_id": o.OrderID,
		"user_id":  o.UserID,
		"symbol":   o.Symbol,
		"side":     order.GetOrderSideName(o.Side),
		"price":    (*o.Price).String(),
		"quantity": o.Quantity,
	})

	return o, trades, nil
}

func (s *TradingService) CancelOrder(ctx context.Context, userID, orderID string) (*order.Order, error) {
	lockKey := fmt.Sprintf("lock:order:%s", orderID)
	ok, err := s.redis.SetNX(ctx, lockKey, "1", 10)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.ErrOrderProcessing
	}
	defer s.redis.Del(ctx, lockKey)

	var o order.Order
	err = s.db.WithContext(ctx).Where("order_id = ? AND user_id = ?", orderID, userID).First(&o).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrOrderNotFound
		}
		return nil, err
	}

	if !o.CanCancel() {
		return nil, errors.ErrOrderCannotCancel
	}

	if o.Side == order.OrderSideBuy && o.FilledQuantity < o.Quantity {
		unfilledQty := o.Quantity - o.FilledQuantity
		unfreezeAmount := (*o.Price).Mul(decimal.NewFromInt(int64(unfilledQty)))
		if err := s.accountService.UnfreezeFunds(ctx, userID, orderID, unfreezeAmount); err != nil {
			s.logger.Errorw("failed to unfreeze funds", "error", err)
		}
	}

	o.Cancel("用户撤单")
	if err := s.db.WithContext(ctx).Save(&o).Error; err != nil {
		return nil, err
	}

	s.matchingEngine.CancelOrder(o.Symbol, orderID)

	s.kafkaProducer.SendMessage(ctx, "order.cancelled", orderID, map[string]interface{}{
		"order_id": orderID,
		"user_id":  userID,
	})

	return &o, nil
}

func (s *TradingService) GetOrder(ctx context.Context, userID, orderID string) (*order.Order, error) {
	var o order.Order
	err := s.db.WithContext(ctx).Where("order_id = ? AND user_id = ?", orderID, userID).First(&o).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrOrderNotFound
		}
		return nil, err
	}
	return &o, nil
}

func (s *TradingService) GetOrders(ctx context.Context, userID string, symbol string, status int, page, pageSize int) ([]*order.Order, int64, error) {
	var orders []*order.Order
	var total int64

	query := s.db.WithContext(ctx).Model(&order.Order{}).Where("user_id = ?", userID)
	if symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}
	if status != 0 {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&orders).Error
	if err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (s *TradingService) GetOrderByClientID(ctx context.Context, userID, clientOrderID string) (*order.Order, error) {
	var o order.Order
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND client_order_id = ?", userID, clientOrderID).
		First(&o).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrOrderNotFound
		}
		return nil, err
	}
	return &o, nil
}
