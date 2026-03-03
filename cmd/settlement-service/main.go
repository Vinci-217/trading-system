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

	"github.com/stock-trading-system/internal/infrastructure/cache"
	"github.com/stock-trading-system/internal/infrastructure/config"
	"github.com/stock-trading-system/internal/infrastructure/database"
	"github.com/stock-trading-system/internal/infrastructure/idgen"
	"github.com/stock-trading-system/internal/infrastructure/mq"
	"github.com/stock-trading-system/internal/service/account"
	settlementSvc "github.com/stock-trading-system/internal/service/settlement"
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
	log.Info("starting settlement service", "version", cfg.Server.Version)

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

	idgen := idgen.NewIDGenerator(redis.Client(), "05")
	accountService := account.NewAccountService(db, redis, idgen, log)
	settlementService := settlementSvc.NewSettlementService(db, idgen, kafkaProducer, accountService, log)

	go startHTTPServer(cfg, log, settlementService)
	go startGRPCServer(cfg, log, settlementService)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down settlement service...")
}

func startHTTPServer(cfg *config.Config, log *logger.Logger, settlementService *settlementSvc.SettlementService) {
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
		v1.POST("/settle", settleTradeHandler(settlementService, log))
		v1.GET("/settlement/:trade_id", getSettlementHandler(settlementService, log))
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.HTTP.Addr, 8085)
	log.Info("starting HTTP server", "addr", addr)
	if err := router.Run(addr); err != nil {
		log.Errorw("HTTP server error", "error", err)
	}
}

func startGRPCServer(cfg *config.Config, log *logger.Logger, settlementService *settlementSvc.SettlementService) {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Server.GRPC.Addr, 50055))
	if err != nil {
		log.Errorw("failed to listen", "error", err)
		return
	}

	grpcServer := grpc.NewServer()
	log.Info("starting gRPC server", "port", 50055)
	if err := grpcServer.Serve(lis); err != nil {
		log.Errorw("gRPC server error", "error", err)
	}
}

func settleTradeHandler(s *settlementSvc.SettlementService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			TradeID     string `json:"trade_id" binding:"required"`
			BuyOrderID  string `json:"buy_order_id" binding:"required"`
			SellOrderID string `json:"sell_order_id" binding:"required"`
			BuyUserID   string `json:"buy_user_id" binding:"required"`
			SellUserID  string `json:"sell_user_id" binding:"required"`
			Symbol      string `json:"symbol" binding:"required"`
			Price       string `json:"price" binding:"required"`
			Quantity    int    `json:"quantity" binding:"required,min=1"`
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

		result, err := s.SettleTrade(c.Request.Context(), &settlementSvc.SettlementRequest{
			TradeID:     req.TradeID,
			BuyOrderID:  req.BuyOrderID,
			SellOrderID: req.SellOrderID,
			BuyUserID:   req.BuyUserID,
			SellUserID:  req.SellUserID,
			Symbol:      req.Symbol,
			Price:       price,
			Quantity:    req.Quantity,
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeSettlementFailed, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "result": result})
	}
}

func getSettlementHandler(s *settlementSvc.SettlementService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		tradeID := c.Param("trade_id")

		settlement, err := s.GetSettlementStatus(c.Request.Context(), tradeID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"code": errors.CodeNotFound, "message": "结算记录不存在"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "settlement": settlement})
	}
}
