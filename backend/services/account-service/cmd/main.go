package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"stock_trader/backend/services/account-service/internal/infrastructure/config"
	"stock_trader/backend/services/account-service/internal/infrastructure/database"
	"stock_trader/backend/services/account-service/internal/infrastructure/logger"
	"stock_trader/backend/services/account-service/internal/infrastructure/messaging"
	"stock_trader/backend/services/account-service/internal/infrastructure/repository"
	"stock_trader/backend/services/account-service/internal/interfaces/grpc"
	"stock_trader/backend/services/account-service/internal/interfaces/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Application struct {
	cfg           *config.Config
	db            *sql.DB
	redis         *redis.Client
	logger        *logger.Logger
	publisher     *messaging.Publisher
	accountRepo   repository.AccountRepository
	positionRepo  repository.PositionRepository
	grpcServer    *grpc.Server
	httpServer    *http.Server
	fundLocks     *sync.Map
	tccTxns       *sync.Map
	rateLimiter   *RateLimiter
	auditLogger   *AuditLogger
}

type RateLimiter struct {
	limits map[string][]time.Time
	mu     sync.RWMutex
}

type AuditLogger struct {
	redis  *redis.Client
	logger *logger.Logger
	mu     sync.Mutex
}

type FundLock struct {
	UserID        string
	OrderID       string
	TransactionID string
	Amount        string
	Status        string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	RetryCount    int
}

type TCCTransaction struct {
	TransactionID string
	UserID        string
	OrderID       string
	Amount        string
	Phase         string
	Status        string
	TryTime       time.Time
	ConfirmTime   time.Time
	CancelTime    time.Time
	RetryCount    int
	ErrorMessage  string
}

const (
	RateLimitDepositPerDay       = 1000000
	RateLimitWithdrawPerDay      = 500000
	RateLimitTransactionPerMin   = 60
	LockTimeout                  = 5 * time.Minute
)

func NewApplication(cfg *config.Config) *Application {
	return &Application{
		cfg:         cfg,
		fundLocks:   &sync.Map{},
		tccTxns:     &sync.Map{},
		rateLimiter: NewRateLimiter(),
	}
}

func (a *Application) Initialize() error {
	var err error

	a.logger = logger.NewLogger("account-service")
	a.logger.Info("初始化账户服务...")

	a.db, err = database.NewMySQL(a.cfg.Database)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	a.redis, err = a.newRedisClient()
	if err != nil {
		return fmt.Errorf("连接Redis失败: %w", err)
	}

	a.publisher = messaging.NewPublisher(a.redis, a.logger)

	a.accountRepo = repository.NewPostgresAccountRepository(a.db, a.logger)
	a.positionRepo = repository.NewPostgresPositionRepository(a.db, a.logger)

	a.auditLogger = NewAuditLogger(a.redis, a.logger)

	if err := a.createTables(); err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	a.startLockMonitor()

	a.logger.Info("账户服务初始化完成")
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
		CREATE TABLE IF NOT EXISTS accounts (
			user_id VARCHAR(36) PRIMARY KEY,
			cash_balance DECIMAL(18, 4) NOT NULL DEFAULT 0,
			frozen_balance DECIMAL(18, 4) NOT NULL DEFAULT 0,
			securities_balance DECIMAL(18, 4) NOT NULL DEFAULT 0,
			total_assets DECIMAL(18, 4) NOT NULL DEFAULT 0,
			total_deposit DECIMAL(18, 4) NOT NULL DEFAULT 0,
			total_withdrawal DECIMAL(18, 4) NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			version BIGINT NOT NULL DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS positions (
			id VARCHAR(36) PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL,
			symbol VARCHAR(20) NOT NULL,
			quantity INT NOT NULL DEFAULT 0,
			frozen_quantity INT NOT NULL DEFAULT 0,
			avg_cost DECIMAL(18, 4) NOT NULL DEFAULT 0,
			market_value DECIMAL(18, 4) NOT NULL DEFAULT 0,
			profit_loss DECIMAL(18, 4) NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			version BIGINT NOT NULL DEFAULT 0,
			UNIQUE KEY idx_user_symbol (user_id, symbol)
		);

		CREATE TABLE IF NOT EXISTS fund_locks (
			id VARCHAR(36) PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL,
			order_id VARCHAR(36) NOT NULL,
			transaction_id VARCHAR(36) NOT NULL,
			amount DECIMAL(18, 4) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'LOCKED',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL,
			UNIQUE KEY idx_order_id (order_id)
		);

		CREATE TABLE IF NOT EXISTS tcc_transactions (
			id VARCHAR(36) PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL,
			order_id VARCHAR(36) NOT NULL,
			amount DECIMAL(18, 4) NOT NULL,
			phase VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL,
			error_message TEXT,
			retry_count INT NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_fund_locks_user_id ON fund_locks(user_id);
		CREATE INDEX IF NOT EXISTS idx_fund_locks_status ON fund_locks(status);
		CREATE INDEX IF NOT EXISTS idx_fund_locks_expires_at ON fund_locks(expires_at);
		CREATE INDEX IF NOT EXISTS idx_tcc_transactions_order_id ON tcc_transactions(order_id);
		CREATE INDEX IF NOT EXISTS idx_positions_user_id ON positions(user_id);
	`

	_, err := a.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	a.logger.Info("数据库表创建成功")
	return nil
}

func (a *Application) startLockMonitor() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			a.cleanupExpiredLocks()
		}
	}()
}

func (a *Application) cleanupExpiredLocks() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()

	a.fundLocks.Range(func(key, value interface{}) bool {
		lock := value.(*FundLock)
		if now.After(lock.ExpiresAt) && lock.Status == "LOCKED" {
			a.logger.Warn("清理过期资金锁定",
				logger.String("lock_id", key.(string)),
				logger.String("user_id", lock.UserID),
				logger.String("order_id", lock.OrderID))

			a.unlockFunds(lock.UserID, lock.OrderID, lock.Amount)
			a.fundLocks.Delete(key)
		}
		return true
	})
}

func (a *Application) unlockFunds(userID string, orderID string, amount string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	account, err := a.accountRepo.GetAccountForUpdate(ctx, userID)
	if err != nil {
		a.logger.Error("获取账户失败", logger.Error(err))
		return
	}

	a.fundLocks.Range(func(key, value interface{}) bool {
		lock := value.(*FundLock)
		if lock.OrderID == orderID && lock.UserID == userID {
			account.FrozenBalance = account.FrozenBalance.Sub(lock.Amount)
			a.fundLocks.Delete(key)
		}
		return true
	})

	if account.FrozenBalance.LessThan(decimal.Zero) {
		account.FrozenBalance = decimal.Zero
	}

	account.Version++
	if err := a.accountRepo.UpdateAccount(ctx, account); err != nil {
		a.logger.Error("更新账户失败", logger.Error(err))
	}
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

	grpcServer := grpc.NewGRPCServer(a, a.logger)
	RegisterAccountServiceServer(a.grpcServer, grpcServer)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(a.grpcServer, healthServer)
	healthServer.SetServingStatus("account-service", grpc_health_v1.HealthCheckResponse_SERVING)

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

	router.GET("/health", a.healthCheck)
	router.GET("/ready", a.readinessCheck)

	api := router.Group("/api/v1")
	{
		api.GET("/accounts/:user_id", a.getAccount)
		api.GET("/accounts/:user_id/positions", a.getPositions)
		api.POST("/accounts/deposit", a.deposit)
		api.POST("/accounts/withdraw", a.withdraw)
		api.POST("/accounts/reconcile", a.reconcile)
	}

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

func (a *Application) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "account-service",
		"timestamp": time.Now().UnixMilli(),
	})
}

func (a *Application) readinessCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := a.db.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error":  "database not available",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
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
	a.logger.Info("停止账户服务...")

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

	a.logger.Info("账户服务已停止")
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
			log.Fatalf("启动账户服务失败: %v", err)
		}
	}()

	<-sigChan
	app.Stop()
}

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)
