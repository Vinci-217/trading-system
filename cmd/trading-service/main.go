package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"github.com/stock-trading-system/internal/domain/matching"
	"github.com/stock-trading-system/internal/domain/order"
	"github.com/stock-trading-system/internal/infrastructure/cache"
	"github.com/stock-trading-system/internal/infrastructure/config"
	"github.com/stock-trading-system/internal/infrastructure/database"
	"github.com/stock-trading-system/internal/infrastructure/idgen"
	"github.com/stock-trading-system/internal/infrastructure/mq"
	"github.com/stock-trading-system/internal/service/account"
	"github.com/stock-trading-system/internal/service/trading"
	"github.com/stock-trading-system/pkg/errors"
	"github.com/stock-trading-system/pkg/logger"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	log := logger.NewLogger(cfg.Log.Level)
	log.Info("starting trading service", "version", cfg.Server.Version)

	db, err := database.NewMySQL(&database.DBConfig{
		DSN:             cfg.Database.DSN,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: time.Duration(cfg.Database.ConnMaxLifetime) * time.Second,
	})
	if err != nil {
		log.Errorw("failed to connect database", "error", err)
		panic(err)
	}
	log.Info("database connected")

	redis, err := cache.NewRedisClient(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.Redis.PoolSize,
		cfg.Redis.MinIdleConns,
	)
	if err != nil {
		log.Errorw("failed to connect redis", "error", err)
		panic(err)
	}
	log.Info("redis connected")

	kafkaProducer, err := mq.NewKafkaProducer(cfg.Kafka.Brokers)
	if err != nil {
		log.Errorw("failed to create kafka producer", "error", err)
		panic(err)
	}
	defer kafkaProducer.Close()
	log.Info("kafka producer created")

	idgen := idgen.NewIDGenerator(redis.Client(), "01")
	matchingEngine := matching.NewMatchingEngine()
	accountService := account.NewAccountService(db, redis, idgen, log)
	tradingService := trading.NewTradingService(db, redis, idgen, kafkaProducer, accountService, matchingEngine, log)

	loadPendingOrders(db, matchingEngine, log)

	go startHTTPServer(cfg, log, tradingService, accountService)
	go startGRPCServer(cfg, log, tradingService, accountService)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down trading service...")
}

func startHTTPServer(cfg *config.Config, log *logger.Logger, tradingService *trading.TradingService, accountService *account.AccountService) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	v1 := router.Group("/api/v1")
	{
		orders := v1.Group("/orders")
		{
			orders.POST("", createOrderHandler(tradingService, log))
			orders.GET("/:order_id", getOrderHandler(tradingService, log))
			orders.POST("/:order_id/cancel", cancelOrderHandler(tradingService, log))
		}

		accounts := v1.Group("/accounts")
		{
			accounts.GET("/:user_id", getAccountHandler(accountService, log))
			accounts.POST("/deposit", depositHandler(accountService, log))
			accounts.POST("/withdraw", withdrawHandler(accountService, log))
			accounts.GET("/:user_id/positions", getPositionsHandler(accountService, log))
		}
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.HTTP.Addr, 8082)
	log.Info("starting HTTP server", "addr", addr)
	if err := router.Run(addr); err != nil {
		log.Errorw("HTTP server error", "error", err)
	}
}

func startGRPCServer(cfg *config.Config, log *logger.Logger, tradingService *trading.TradingService, accountService *account.AccountService) {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Server.GRPC.Addr, 50052))
	if err != nil {
		log.Errorw("failed to listen", "error", err)
		return
	}

	grpcServer := grpc.NewServer()
	log.Info("starting gRPC server", "port", 50052)
	if err := grpcServer.Serve(lis); err != nil {
		log.Errorw("gRPC server error", "error", err)
	}
}

func createOrderHandler(s *trading.TradingService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			UserID        string `json:"user_id" binding:"required"`
			Symbol        string `json:"symbol" binding:"required"`
			Side          int    `json:"side" binding:"required"`
			OrderType     int    `json:"order_type" binding:"required"`
			Price         string `json:"price" binding:"required"`
			Quantity      int    `json:"quantity" binding:"required,min=1"`
			ClientOrderID string `json:"client_order_id"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": errors.CodeInvalidParam, "message": "参数错误"})
			return
		}

		price, err := decimal.NewFromString(req.Price)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": errors.CodeInvalidParam, "message": "价格格式错误"})
			return
		}

		orderReq := &trading.CreateOrderRequest{
			UserID:        req.UserID,
			Symbol:        req.Symbol,
			Side:          req.Side,
			OrderType:     req.OrderType,
			Price:         price,
			Quantity:      req.Quantity,
			ClientOrderID: req.ClientOrderID,
		}

		ord, trades, err := s.CreateOrder(c.Request.Context(), orderReq)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"code":    errors.CodeSuccess,
			"message": "success",
			"order":   ord,
			"trades":  trades,
		})
	}
}

func getOrderHandler(s *trading.TradingService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		orderID := c.Param("order_id")
		userID := c.Query("user_id")

		ord, err := s.GetOrder(c.Request.Context(), userID, orderID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": errors.CodeOrderNotFound, "message": "订单不存在"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "order": ord})
	}
}

func cancelOrderHandler(s *trading.TradingService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		orderID := c.Param("order_id")
		userID := c.Query("user_id")

		ord, err := s.CancelOrder(c.Request.Context(), userID, orderID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": errors.CodeOrderCannotCancel, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "order": ord})
	}
}

func getAccountHandler(s *account.AccountService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("user_id")

		acc, err := s.GetAccount(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": errors.CodeAccountNotFound, "message": "账户不存在"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "account": acc})
	}
}

func depositHandler(s *account.AccountService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			UserID string `json:"user_id" binding:"required"`
			Amount string `json:"amount" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": errors.CodeInvalidParam, "message": "参数错误"})
			return
		}

		amount, err := decimal.NewFromString(req.Amount)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": errors.CodeInvalidParam, "message": "金额格式错误"})
			return
		}

		if err := s.Deposit(c.Request.Context(), req.UserID, amount); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "message": "入金成功"})
	}
}

func withdrawHandler(s *account.AccountService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			UserID string `json:"user_id" binding:"required"`
			Amount string `json:"amount" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": errors.CodeInvalidParam, "message": "参数错误"})
			return
		}

		amount, err := decimal.NewFromString(req.Amount)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": errors.CodeInvalidParam, "message": "金额格式错误"})
			return
		}

		if err := s.Withdraw(c.Request.Context(), req.UserID, amount); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "message": "出金成功"})
	}
}

func getPositionsHandler(s *account.AccountService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("user_id")

		positions, err := s.GetPositions(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "positions": positions})
	}
}

func loadPendingOrders(db *gorm.DB, engine *matching.MatchingEngine, log *logger.Logger) {
	var orders []*order.Order
	result := db.Where("status IN ?", []int{order.OrderStatusPending, order.OrderStatusPartial}).Find(&orders)
	if result.Error != nil {
		log.Errorw("failed to load pending orders", "error", result.Error)
		return
	}

	log.Info("pending orders found", "count", len(orders))
	
	loadedCount := 0
	for _, o := range orders {
		if o.Price == nil {
			p := decimal.NewFromFloat(0)
			o.Price = &p
		}
		if o.AvgFilledPrice == nil {
			p := decimal.NewFromFloat(0)
			o.AvgFilledPrice = &p
		}
		if o.Fee == nil {
			p := decimal.NewFromFloat(0)
			o.Fee = &p
		}
		if o.RemainingQuantity() <= 0 {
			continue
		}
		orderBook := engine.GetOrCreateOrderBook(o.Symbol)
		orderBook.AddOrder(o)
		loadedCount++
	}
	log.Info("pending orders loaded to matching engine", "count", loadedCount)
}
