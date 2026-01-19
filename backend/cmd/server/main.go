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
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"stock_trader/internal/cache"
	"stock_trader/internal/config"
	"stock_trader/internal/kafka"
	"stock_trader/internal/repository"
	"stock_trader/internal/server"
	"stock_trader/internal/service/account"
	"stock_trader/internal/service/market"
	"stock_trader/internal/service/order"
	"stock_trader/internal/service/reconciliation"
	"stock_trader/internal/service/user"
	pb "stock_trader/api"
)

var (
	configPath = flag.String("config", "config.yaml", "配置文件路径")
	port       = flag.Int("port", 50051, "gRPC服务端口")
	httpPort   = flag.Int("http-port", 8080, "HTTP服务端口")
)

func main() {
	flag.Parse()

	logger, err := initLogger()
	if err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer logger.Sync()

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal("加载配置失败", zap.Error(err))
	}

	db, err := initDatabase(cfg)
	if err != nil {
		logger.Fatal("初始化数据库失败", zap.Error(err))
	}

	redisClient, err := initRedis(cfg)
	if err != nil {
		logger.Fatal("初始化Redis失败", zap.Error(err))
	}
	defer redisClient.Close()

	kafkaProducer, err := initKafka(cfg)
	if err != nil {
		logger.Fatal("初始化Kafka失败", zap.Error(err))
	}
	defer kafkaProducer.Close()

	repo := repository.NewRepository(db)
	redisCache := cache.NewRedisCache(redisClient)
	kafkaClient := kafka.NewClient(kafkaProducer)

	userSvc := user.NewService(repo, logger)
	accountSvc := account.NewService(repo, redisCache, logger)
	orderSvc := order.NewService(repo, accountSvc, kafkaClient, logger)
	marketSvc := market.NewService(redisCache, kafkaClient, logger)
	reconSvc := reconciliation.NewService(repo, logger)

	grpcServer := grpc.NewServer()
	s := server.NewServer(userSvc, accountSvc, orderSvc, marketSvc, reconSvc)
	pb.RegisterUserServiceServer(grpcServer, s)
	pb.RegisterAccountServiceServer(grpcServer, s)
	pb.RegisterOrderServiceServer(grpcServer, s)
	pb.RegisterTradeServiceServer(grpcServer, s)
	pb.RegisterMarketServiceServer(grpcServer, s)
	reflection.Register(grpcServer)

	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
		if err != nil {
			logger.Fatal("监听gRPC端口失败", zap.Error(err))
		}
		logger.Info("gRPC服务启动", zap.Int("port", *port))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("gRPC服务异常", zap.Error(err))
		}
	}()

	router := gin.Default()
	setupRouter(router, s, cfg)
	go func() {
		logger.Info("HTTP服务启动", zap.Int("port", *httpPort))
		if err := router.Run(fmt.Sprintf(":%d", *httpPort)); err != nil {
			logger.Fatal("HTTP服务异常", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("正在关闭服务...")

	grpcServer.GracefulStop()
	logger.Info("服务已关闭")
}

func initLogger() (*zap.Logger, error) {
	return zap.NewProduction()
}

func initDatabase(cfg *config.Config) (*repository.DB, error) {
	return repository.NewDB(cfg.Database)
}

func initRedis(cfg *config.Config) (*redis.Client, error) {
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
}

func initKafka(cfg *config.Config) (kafka.Producer, error) {
	return kafka.NewProducer(cfg.Kafka.Brokers)
}

func setupRouter(router *gin.Engine, s *server.Server, cfg *config.Config) {
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.POST("/api/v1/register", s.Register)
	router.POST("/api/v1/login", s.Login)

	api := router.Group("/api/v1")
	api.Use(server.AuthMiddleware())
	{
		api.POST("/orders", s.CreateOrder)
		api.DELETE("/orders/:id", s.CancelOrder)
		api.GET("/orders", s.ListOrders)
		api.GET("/account", s.GetAccount)
		api.GET("/positions", s.GetPositions)
		api.GET("/trades", s.GetTrades)
		api.GET("/quote/:symbol", s.GetQuote)
		api.GET("/kline/:symbol", s.GetKLine)
		api.GET("/reconciliation", s.GetReconciliationReport)
		api.GET("/reconciliation/issues", s.GetReconciliationIssues)
		api.POST("/export/orders", s.ExportOrders)
	}

	router.POST("/ws/quote", func(c *gin.Context) {
		marketSvc := market.NewService(cache.NewRedisCache(nil), nil, nil)
		marketSvc.HandleWebSocket(c.Writer, c.Request)
	})
}
