package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"

	pb "stock_trader/matching-service/internal/interfaces/grpc/pb"
	"stock_trader/matching-service/internal/domain/entity"
	"stock_trader/matching-service/internal/domain/service"
)

type MatchingServiceServer struct {
	pb.UnimplementedMatchingServiceServer
	matchingEngine *service.MatchingEngine
	orderBookRepo  interface {
		SaveOrderBook(ctx context.Context, orderBook *entity.OrderBook) error
		SaveTrade(ctx context.Context, trade *entity.Trade) error
		GetTrades(ctx context.Context, symbol string, limit int) ([]*entity.Trade, error)
	}
	tradePublisher  interface {
		PublishTrade(ctx context.Context, trade *entity.Trade) error
	}
	marketPublisher interface {
		PublishDepth(ctx context.Context, symbol string, depth *entity.MarketDepth) error
	}
	logger interface {
		Info(msg string, args ...interface{})
		Error(msg string, args ...interface{})
		Warn(msg string, args ...interface{})
	}
}

func NewMatchingServiceServer(
	matchingEngine *service.MatchingEngine,
	orderBookRepo interface {
		SaveOrderBook(ctx context.Context, orderBook *entity.OrderBook) error
		SaveTrade(ctx context.Context, trade *entity.Trade) error
		GetTrades(ctx context.Context, symbol string, limit int) ([]*entity.Trade, error)
	},
	tradePublisher interface {
		PublishTrade(ctx context.Context, trade *entity.Trade) error
	},
	marketPublisher interface {
		PublishDepth(ctx context.Context, symbol string, depth *entity.MarketDepth) error
	},
	logger interface {
		Info(msg string, args ...interface{})
		Error(msg string, args ...interface{})
		Warn(msg string, args ...interface{})
	},
) *MatchingServiceServer {
	return &MatchingServiceServer{
		matchingEngine:  matchingEngine,
		orderBookRepo:   orderBookRepo,
		tradePublisher:  tradePublisher,
		marketPublisher: marketPublisher,
		logger:          logger,
	}
}

func (s *MatchingServiceServer) SubmitOrder(ctx context.Context, req *pb.SubmitOrderRequest) (*pb.SubmitOrderResponse, error) {
	startTime := time.Now()

	price, err := parseDecimal(req.Price)
	if err != nil {
		return &pb.SubmitOrderResponse{
			Success: false,
			Message: "无效的价格格式",
		}, nil
	}

	if req.Quantity <= 0 {
		return &pb.SubmitOrderResponse{
			Success: false,
			Message: "数量必须为正数",
		}, nil
	}

	orderID := req.OrderId
	if orderID == "" {
		orderID = generateOrderID()
	}

	order := entity.NewOrder(
		orderID,
		req.UserId,
		req.Symbol,
		entity.OrderType(req.OrderType),
		entity.OrderSide(req.Side),
		price,
		int(req.Quantity),
	)

	trades, err := s.matchingEngine.SubmitOrder(ctx, order)
	if err != nil {
		s.logger.Error("订单提交失败", s.logger.Error(err))
		return &pb.SubmitOrderResponse{
			Success: false,
			Message: "订单处理失败",
		}, nil
	}

	for _, trade := range trades {
		s.orderBookRepo.SaveTrade(ctx, trade)
		s.tradePublisher.PublishTrade(ctx, trade)
	}

	orderBook := s.matchingEngine.GetOrCreateOrderBook(req.Symbol)
	depth := orderBook.GetMarketDepth()
	s.marketPublisher.PublishDepth(ctx, req.Symbol, depth)

	s.logger.Info("订单处理完成",
		s.logger.String("order_id", orderID),
		s.logger.String("symbol", req.Symbol),
		s.logger.Int("trades", len(trades)),
		s.logger.Duration("latency", time.Since(startTime)))

	return &pb.SubmitOrderResponse{
		Success: true,
		Message: "订单处理成功",
		OrderId: orderID,
	}, nil
}

func (s *MatchingServiceServer) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	orderBook := s.matchingEngine.GetOrderBook(req.Symbol)
	if orderBook == nil {
		return &pb.CancelOrderResponse{
			Success: false,
			Message: "订单簿不存在",
		}, nil
	}

	if !s.matchingEngine.CancelOrder(orderBook, req.OrderId) {
		return &pb.CancelOrderResponse{
			Success: false,
			Message: "订单未找到",
		}, nil
	}

	return &pb.CancelOrderResponse{
		Success: true,
		Message: "订单已取消",
	}, nil
}

func (s *MatchingServiceServer) GetMarketDepth(ctx context.Context, req *pb.GetDepthRequest) (*pb.GetDepthResponse, error) {
	orderBook := s.matchingEngine.GetOrderBook(req.Symbol)
	if orderBook == nil {
		return &pb.GetDepthResponse{
			Success: false,
			Message: "订单簿不存在",
		}, nil
	}

	depth := orderBook.GetMarketDepth()

	return &pb.GetDepthResponse{
		Success: true,
		Symbol:  req.Symbol,
		Bids:    convertDepthLevels(depth.Bids),
		Asks:    convertDepthLevels(depth.Asks),
	}, nil
}

func (s *MatchingServiceServer) GetTrades(ctx context.Context, req *pb.GetTradesRequest) (*pb.GetTradesResponse, error) {
	trades, err := s.orderBookRepo.GetTrades(ctx, req.Symbol, int(req.Limit))
	if err != nil {
		return &pb.GetTradesResponse{
			Success: false,
			Message: "获取交易失败",
		}, nil
	}

	pbTrades := make([]*pb.Trade, len(trades))
	for i, trade := range trades {
		pbTrades[i] = &pb.Trade{
			Id:          trade.ID,
			Symbol:      trade.Symbol,
			BuyOrderId:  trade.BuyOrderID,
			SellOrderId: trade.SellOrderID,
			BuyUserId:   trade.BuyUserID,
			SellUserId:  trade.SellUserID,
			Price:       trade.Price.String(),
			Quantity:    int32(trade.Quantity),
			Timestamp:   trade.Timestamp.UnixMilli(),
		}
	}

	return &pb.GetTradesResponse{
		Success: true,
		Trades:  pbTrades,
	}, nil
}

func parseDecimal(s string) (Decimal, error) {
	return DecimalFromString(s)
}

func generateOrderID() string {
	return time.Now().Format("20060102150405") + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}

func convertDepthLevels(levels []entity.DepthLevel) []*pb.DepthLevel {
	result := make([]*pb.DepthLevel, len(levels))
	for i, level := range levels {
		result[i] = &pb.DepthLevel{
			Price:    level.Price.String(),
			Quantity: int32(level.Quantity),
			Orders:   int32(level.Orders),
		}
	}
	return result
}

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *MatchingServiceServer) mustEmbedUnimplementedMatchingServiceServer() {}

type Decimal = decimal.Decimal

func DecimalFromString(s string) (Decimal, error) {
	return decimal.NewFromString(s)
}
