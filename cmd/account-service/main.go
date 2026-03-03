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
	accountSvc "github.com/stock-trading-system/internal/service/account"
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
	log.Info("starting account service", "version", cfg.Server.Version)

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

	idgen := idgen.NewIDGenerator(redis.Client(), "02")
	accountService := accountSvc.NewAccountService(db, redis, idgen, log)

	go startHTTPServer(cfg, log, accountService)
	go startGRPCServer(cfg, log, accountService)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down account service...")
}

func startHTTPServer(cfg *config.Config, log *logger.Logger, accountService *accountSvc.AccountService) {
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
		accounts := v1.Group("/accounts")
		{
			accounts.GET("/:user_id", getAccountHandler(accountService, log))
			accounts.POST("", createAccountHandler(accountService, log))
			accounts.POST("/deposit", depositHandler(accountService, log))
			accounts.POST("/withdraw", withdrawHandler(accountService, log))
			accounts.GET("/:user_id/positions", getPositionsHandler(accountService, log))
			accounts.GET("/:user_id/positions/:symbol", getPositionHandler(accountService, log))
		}
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.HTTP.Addr, 8081)
	log.Info("starting HTTP server", "addr", addr)
	if err := router.Run(addr); err != nil {
		log.Errorw("HTTP server error", "error", err)
	}
}

func startGRPCServer(cfg *config.Config, log *logger.Logger, accountService *accountSvc.AccountService) {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Server.GRPC.Addr, 50051))
	if err != nil {
		log.Errorw("failed to listen", "error", err)
		return
	}

	grpcServer := grpc.NewServer()
	log.Info("starting gRPC server", "port", 50051)
	if err := grpcServer.Serve(lis); err != nil {
		log.Errorw("gRPC server error", "error", err)
	}
}

func getAccountHandler(s *accountSvc.AccountService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("user_id")

		acc, err := s.GetAccount(c.Request.Context(), userID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"code": errors.CodeAccountNotFound, "message": "账户不存在"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "account": acc})
	}
}

func createAccountHandler(s *accountSvc.AccountService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			UserID string `json:"user_id" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": errors.CodeInvalidParam, "message": "参数错误"})
			return
		}

		acc, err := s.CreateAccount(c.Request.Context(), req.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"code": errors.CodeSuccess, "account": acc})
	}
}

func depositHandler(s *accountSvc.AccountService, log *logger.Logger) gin.HandlerFunc {
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

func withdrawHandler(s *accountSvc.AccountService, log *logger.Logger) gin.HandlerFunc {
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

func getPositionsHandler(s *accountSvc.AccountService, log *logger.Logger) gin.HandlerFunc {
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

func getPositionHandler(s *accountSvc.AccountService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("user_id")
		symbol := c.Param("symbol")

		pos, err := s.GetPosition(c.Request.Context(), userID, symbol)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": errors.CodeInsufficientPosition, "message": "持仓不存在"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "position": pos})
	}
}
