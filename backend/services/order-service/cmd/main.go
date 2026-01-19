package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"stock_trader/backend/services/order-service/internal/domain/repository"
	"stock_trader/backend/services/order-service/internal/domain/service"
	"stock_trader/backend/services/order-service/internal/infrastructure/config"
	"stock_trader/backend/services/order-service/internal/infrastructure/database"
	"stock_trader/backend/services/order-service/internal/infrastructure/logger"
	"stock_trader/backend/services/order-service/internal/infrastructure/messaging"
	"stock_trader/backend/services/order-service/internal/infrastructure/repository"
	"stock_trader/backend/services/order-service/internal/interfaces/grpc"
	"stock_trader/backend/services/order-service/internal/interfaces/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Application struct {
	cfg          *config.Config
	db           *sql.DB
	redis        *redis.Client
	logger       *logger.Logger
	publisher    *messaging.Publisher
	orderRepo    repository.OrderRepository
	tccRepo      repository.TCCTransactionRepository
	domainService *service.OrderDomainService
	grpcServer   *grpc.Server
	httpServer   *http.Server
}

func NewApplication(cfg *config.Config) *Application {
	return &Application{
		cfg: cfg,
	}
}

func (a *Application) Initialize() error {
	var err error

	a.logger = logger.NewLogger("order-service")
	a.logger.Info("初始化订单服务...")

	a.db, err = database.NewMySQL(a.cfg.Database)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	a.redis, err = a.newRedisClient()
	if err != nil {
		return fmt.Errorf("连接Redis失败: %w", err)
	}

	a.publisher = messaging.NewPublisher(a.redis, a.logger)

	a.orderRepo = repository.NewPostgresOrderRepository(a.db, a.logger)
	a.tccRepo = repository.NewPostgresTCCTransactionRepository(a.db, a.logger)

	a.domainService = service.NewOrderDomainService(a.orderRepo, a.tccRepo, a.logger)

	if err := a.createTables(); err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	a.logger.Info("订单服务初始化完成")
	return nil
}

func (a *Application) newRedisClient() (*redis.Client, error) {
	return redis.NewClient(&redis.Options{
		Addr:         a.cfg.Redis.Addr,
		Password:     a.cfg.Redis.Password,
		DB:           a.cfg.Redis.DB,
		PoolSize:     a.cfg.Redis.PoolSize,
		MinIdleConns: a.cfg.Redis.MinIdleConns,
	}), nil
}

func (a *Application) createTables() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schema := `
		CREATE TABLE IF NOT EXISTS orders (
			id VARCHAR(36) PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL,
			symbol VARCHAR(20) NOT NULL,
			order_type VARCHAR(10) NOT NULL,
			side VARCHAR(10) NOT NULL,
			price DECIMAL(18, 4) NOT NULL,
			quantity INT NOT NULL,
			filled_quantity INT DEFAULT 0,
			status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
			fee DECIMAL(18, 4) DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			client_order_id VARCHAR(36),
			remarks VARCHAR(255),
			UNIQUE KEY idx_client_order_id (client_order_id)
		);

		CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
		CREATE INDEX IF NOT EXISTS idx_orders_symbol ON orders(symbol);
		CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
		CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC);

		CREATE TABLE IF NOT EXISTS tcc_transactions (
			id VARCHAR(36) PRIMARY KEY,
			order_id VARCHAR(36) NOT NULL,
			transaction_type VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL,
			try_params TEXT,
			confirm_params TEXT,
			cancel_params TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			retry_count INT DEFAULT 0,
			error_msg TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_tcc_order_id ON tcc_transactions(order_id);
		CREATE INDEX IF NOT EXISTS idx_tcc_status ON tcc_transactions(status);
	`

	_, err := a.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	a.logger.Info("数据库表创建成功")
	return nil
}

func (a *Application) SetupGRPCServer() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", a.cfg.App.GRPCPort))
	if err != nil {
		return fmt.Errorf("监听端口失败: %w", err)
	}

	a.grpcServer = grpc.NewServer(
		grpc.MaxRecvMsgSize(10*1024*1024),
		grpc.MaxSendMsgSize(10*1024*1024),
	)

	grpcServer := grpc.NewGRPCServer(a.domainService, a.logger)
	RegisterOrderServiceServer(a.grpcServer, grpcServer)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(a.grpcServer, healthServer)
	healthServer.SetServingStatus("order-service", grpc_health_v1.HealthCheckResponse_SERVING)

	go func() {
		if err := a.grpcServer.Serve(lis); err != nil {
			a.logger.Error("gRPC服务器错误", logger.Error(err))
		}
	}()

	a.logger.Info("gRPC服务器启动", logger.Int("port", a.cfg.App.GRPCPort))
	return nil
}

func (a *Application) SetupHTTPServer() error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(a.loggingMiddleware())

	httpServer := http.NewHTTPServer(a.domainService)
	httpServer.RegisterRoutes(router)

	a.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", a.cfg.App.HTTPPort),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		a.logger.Info("HTTP服务器启动", logger.Int("port", a.cfg.App.HTTPPort))
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("HTTP服务器错误", logger.Error(err))
		}
	}()

	return nil
}

func (a *Application) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		duration := time.Since(start)
		a.logger.Info("HTTP请求",
			logger.String("method", c.Request.Method),
			logger.String("path", path),
			logger.Int("status", c.Writer.Status()),
			logger.Duration("duration", duration))
	}
}

func (a *Application) Start() error {
	if err := a.Initialize(); err != nil {
		return err
	}

	if err := a.SetupGRPCServer(); err != nil {
		return err
	}

	if err := a.SetupHTTPServer(); err != nil {
		return err
	}

	return nil
}

func (a *Application) Stop() {
	a.logger.Info("停止订单服务...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if a.httpServer != nil {
		a.httpServer.Shutdown(ctx)
	}

	if a.grpcServer != nil {
		a.grpcServer.GracefulStop()
	}

	if a.redis != nil {
		a.redis.Close()
	}

	a.logger.Info("订单服务已停止")
}

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	app := NewApplication(cfg)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := app.Start(); err != nil {
			log.Fatalf("启动订单服务失败: %v", err)
		}
	}()

	<-sigChan
	app.Stop()
}

import (
	"database/sql"

	"github.com/google/uuid"
)
