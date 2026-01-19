package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"stock_trader/matching-service/internal/infrastructure/config"
	"stock_trader/matching-service/internal/infrastructure/database"
	"stock_trader/matching-service/internal/infrastructure/messaging"
	"stock_trader/matching-service/internal/infrastructure/repository"
	"stock_trader/matching-service/internal/interfaces/grpc"
	"stock_trader/matching-service/internal/interfaces/http"
	"stock_trader/matching-service/internal/interfaces/websocket"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

type MatchingService struct {
	cfg              *config.Config
	grpcServer       *grpc.Server
	httpServer       *http.Server
	redis            *redis.Client
	orderBookRepo    repository.OrderBookRepository
	tradePublisher   messaging.TradePublisher
	marketPublisher  messaging.MarketPublisher
	logger           *Logger
	shutdownChan     chan struct{}
	wg               sync.WaitGroup
}

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	logger := NewLogger(cfg.Logging.Level)

	logger.Info("启动撮合服务",
		logger.String("grpc_port", fmt.Sprintf("%d", cfg.GRPCPort)),
		logger.String("http_port", fmt.Sprintf("%d", cfg.HTTPPort)))

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Error("Redis连接失败", logger.Error(err))
		os.Exit(1)
	}
	logger.Info("Redis连接成功")

	db, err := database.NewMySQL(cfg.Database)
	if err != nil {
		logger.Error("数据库连接失败", logger.Error(err))
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("数据库连接成功")

	orderBookRepo := repository.NewOrderBookRepository(db)
	tradePublisher := messaging.NewTradePublisher(redisClient)
	marketPublisher := messaging.NewMarketPublisher(redisClient)

	service := NewMatchingService(cfg, redisClient, orderBookRepo, tradePublisher, marketPublisher, logger)

	if err := service.Start(); err != nil {
		logger.Error("启动服务失败", logger.Error(err))
		os.Exit(1)
	}

	logger.Info("撮合服务启动成功",
		logger.String("grpc", fmt.Sprintf(":%d", cfg.GRPCPort)),
		logger.String("http", fmt.Sprintf(":%d", cfg.HTTPPort)))

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("收到停止信号，正在优雅关闭...")
	service.Stop()
	logger.Info("服务已停止")
}

func NewMatchingService(
	cfg *config.Config,
	redis *redis.Client,
	orderBookRepo repository.OrderBookRepository,
	tradePublisher messaging.TradePublisher,
	marketPublisher messaging.MarketPublisher,
	logger *Logger,
) *MatchingService {
	return &MatchingService{
		cfg:             cfg,
		redis:           redis,
		orderBookRepo:   orderBookRepo,
		tradePublisher:  tradePublisher,
		marketPublisher: marketPublisher,
		logger:          logger,
		shutdownChan:    make(chan struct{}),
	}
}

func (s *MatchingService) Start() error {
	if err := s.startGRPCServer(); err != nil {
		return fmt.Errorf("启动gRPC服务器失败: %w", err)
	}

	if err := s.startHTTPServer(); err != nil {
		return fmt.Errorf("启动HTTP服务器失败: %w", err)
	}

	s.wg.Add(1)
	go s.orderProcessingLoop()

	return nil
}

func (s *MatchingService) Stop() {
	close(s.shutdownChan)
	s.wg.Wait()

	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}

	s.logger.Info("撮合服务已停止")
}

func (s *MatchingService) startGRPCServer() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("监听gRPC端口失败: %w", err)
	}

	s.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor(s.logger)),
		grpc.MaxRecvMsgSize(10*1024*1024),
		grpc.MaxSendMsgSize(10*1024*1024),
	)

	grpcServer := grpc.NewMatchingServiceServer(s)
	RegisterMatchingServiceServer(s.grpcServer, grpcServer)

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Error("gRPC服务器错误", logger.Error(err))
		}
	}()

	s.logger.Info("gRPC服务器启动成功", logger.Int("port", s.cfg.GRPCPort))
	return nil
}

func (s *MatchingService) startHTTPServer() error {
	wsHub := websocket.NewHub()
	go wsHub.Run()

	httpServer := http.NewServer(
		fmt.Sprintf(":%d", s.cfg.HTTPPort),
		s.logger,
		s.cfg,
		wsHub,
	)

	handler := http.NewRouter(httpServer, s.logger, wsHub)
	httpServer.SetHandler(handler)

	go func() {
		s.logger.Info("HTTP服务器启动成功", logger.Int("port", s.cfg.HTTPPort))
		if err := httpServer.Start(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP服务器错误", logger.Error(err))
		}
	}()

	s.httpServer = httpServer
	return nil
}

func (s *MatchingService) orderProcessingLoop() {
	defer s.wg.Done()

	interval := time.Duration(s.cfg.MatchingIntervalMs) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdownChan:
			return
		case <-ticker.C:
			s.processAllSymbols()
		}
	}
}

func (s *MatchingService) processAllSymbols() {
	for _, symbol := range s.cfg.Symbols {
		orderBook, err := s.orderBookRepo.GetOrCreate(context.Background(), symbol)
		if err != nil {
			s.logger.Error("获取订单簿失败",
				logger.Error(err),
				logger.String("symbol", symbol))
			continue
		}

		if orderBook.HasOrders() {
			trades := s.matchOrders(orderBook)
			s.publishTrades(context.Background(), symbol, trades)
			s.updateAndPublishDepth(context.Background(), orderBook)
		}
	}
}

func (s *MatchingService) matchOrders(orderBook *OrderBook) []*Trade {
	var trades []*Trade

	for {
		buyOrder := orderBook.GetBestBuyOrder()
		sellOrder := orderBook.GetBestSellOrder()

		if buyOrder == nil || sellOrder == nil {
			break
		}

		if buyOrder.Price.LessThan(sellOrder.Price) {
			break
		}

		tradePrice := sellOrder.Price
		tradeQty := min(buyOrder.Quantity, sellOrder.Quantity)

		trade := NewTrade(
			GenerateTradeID(),
			orderBook.Symbol,
			buyOrder,
			sellOrder,
			tradePrice,
			tradeQty,
		)
		trades = append(trades, trade)

		buyOrder.FilledQuantity += tradeQty
		sellOrder.FilledQuantity += tradeQty

		orderBook.LastPrice = tradePrice
		orderBook.TotalVolume += int64(tradeQty)
		orderBook.TotalValue = orderBook.TotalValue.Add(tradePrice.Mul(decimal.NewFromInt(int64(tradeQty))))

		if buyOrder.IsFilled() {
			orderBook.RemoveBuyOrder(buyOrder.ID)
		}

		if sellOrder.IsFilled() {
			orderBook.RemoveSellOrder(sellOrder.ID)
		}
	}

	if len(trades) > 0 {
		s.orderBookRepo.SaveOrderBook(context.Background(), orderBook)
	}

	return trades
}

func (s *MatchingService) publishTrades(ctx context.Context, symbol string, trades []*Trade) {
	for _, trade := range trades {
		if err := s.tradePublisher.PublishTrade(ctx, trade); err != nil {
			s.logger.Error("发布交易失败",
				logger.Error(err),
				logger.String("trade_id", trade.ID))
		}
	}
}

func (s *MatchingService) updateAndPublishDepth(ctx context.Context, orderBook *OrderBook) {
	depth := orderBook.GetMarketDepth()

	if err := s.marketPublisher.PublishDepth(ctx, orderBook.Symbol, depth); err != nil {
		s.logger.Error("发布行情深度失败",
			logger.Error(err),
			logger.String("symbol", orderBook.Symbol))
	}
}

func loggingInterceptor(logger *Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		code := "OK"
		if err != nil {
			code = "ERROR"
		}

		logger.Info("gRPC请求",
			logger.String("method", info.FullMethod),
			logger.Duration("duration", duration),
			logger.String("code", code))

		if duration > 5*time.Second {
			logger.Warn("慢请求检测",
				logger.String("method", info.FullMethod),
				logger.Duration("duration", duration))
		}

		return resp, err
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
