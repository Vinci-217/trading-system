package grpc

import (
	"context"

	"stock_trader/backend/services/order-service/internal/domain/entity"
	"stock_trader/backend/services/order-service/internal/domain/service"
	"stock_trader/backend/services/order-service/internal/infrastructure/logger"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	domainService *service.OrderDomainService
	logger        *logger.Logger
}

func NewGRPCServer(domainService *service.OrderDomainService, logger *logger.Logger) *GRPCServer {
	return &GRPCServer{
		domainService: domainService,
		logger:        logger,
	}
}

func (s *GRPCServer) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	price, err := decimal.NewFromString(req.Price)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "无效价格")
	}

	order, tccTx, err := s.domainService.CreateOrder(
		ctx,
		req.UserID,
		req.Symbol,
		entity.OrderType(req.OrderType),
		entity.OrderSide(req.Side),
		price,
		int(req.Quantity),
		req.ClientOrderID,
	)

	if err != nil {
		s.logger.Error("创建订单失败", logger.Error(err))
		return &CreateOrderResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &CreateOrderResponse{
		Success:     true,
		OrderID:     order.ID,
		Transaction: tccTx.TransactionID,
		CreatedAt:   order.CreatedAt.UnixMilli(),
	}, nil
}

func (s *GRPCServer) GetOrder(ctx context.Context, req *GetOrderRequest) (*GetOrderResponse, error) {
	order, err := s.domainService.GetOrder(ctx, req.OrderID)
	if err != nil {
		s.logger.Error("获取订单失败", logger.Error(err))
		return &GetOrderResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &GetOrderResponse{
		Success: true,
		Order:   toOrderProto(order),
	}, nil
}

func (s *GRPCServer) GetUserOrders(ctx context.Context, req *GetUserOrdersRequest) (*GetUserOrdersResponse, error) {
	orders, err := s.domainService.GetOrdersByUser(ctx, req.UserID, req.Status, int(req.Limit))
	if err != nil {
		s.logger.Error("获取用户订单列表失败", logger.Error(err))
		return &GetUserOrdersResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	protos := make([]*OrderProto, len(orders))
	for i, order := range orders {
		protos[i] = toOrderProto(order)
	}

	return &GetUserOrdersResponse{
		Success: true,
		Orders:  protos,
	}, nil
}

func (s *GRPCServer) GetMarketOrders(ctx context.Context, req *GetMarketOrdersRequest) (*GetMarketOrdersResponse, error) {
	orders, err := s.domainService.GetOrdersBySymbol(ctx, req.Symbol, req.Status, int(req.Limit))
	if err != nil {
		s.logger.Error("获取市场订单列表失败", logger.Error(err))
		return &GetMarketOrdersResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	protos := make([]*OrderProto, len(orders))
	for i, order := range orders {
		protos[i] = toOrderProto(order)
	}

	return &GetMarketOrdersResponse{
		Success: true,
		Orders:  protos,
	}, nil
}

func (s *GRPCServer) CancelOrder(ctx context.Context, req *CancelOrderRequest) (*CancelOrderResponse, error) {
	err := s.domainService.CancelOrder(ctx, req.OrderID, req.UserID)
	if err != nil {
		s.logger.Error("取消订单失败", logger.Error(err))
		return &CancelOrderResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &CancelOrderResponse{
		Success: true,
		Message: "订单取消成功",
	}, nil
}

func (s *GRPCServer) ConfirmOrder(ctx context.Context, req *ConfirmOrderRequest) (*ConfirmOrderResponse, error) {
	err := s.domainService.ConfirmOrder(ctx, req.OrderID)
	if err != nil {
		s.logger.Error("确认订单失败", logger.Error(err))
		return &ConfirmOrderResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ConfirmOrderResponse{
		Success: true,
	}, nil
}

func toOrderProto(order *entity.Order) *OrderProto {
	return &OrderProto{
		ID:             order.ID,
		UserID:         order.UserID,
		Symbol:         order.Symbol,
		OrderType:      string(order.OrderType),
		Side:           string(order.Side),
		Price:          order.Price.String(),
		Quantity:       int32(order.Quantity),
		FilledQuantity: int32(order.FilledQuantity),
		Status:         string(order.Status),
		Fee:            order.Fee.String(),
		CreatedAt:      order.CreatedAt.UnixMilli(),
		UpdatedAt:      order.UpdatedAt.UnixMilli(),
		ClientOrderID:  order.ClientOrderID,
		Remarks:        order.Remarks,
	}
}
