package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"github.com/stock-trading-system/internal/domain/order"
	"github.com/stock-trading-system/internal/infrastructure/cache"
	"github.com/stock-trading-system/internal/infrastructure/config"
	"github.com/stock-trading-system/internal/infrastructure/database"
	"github.com/stock-trading-system/internal/service/matching"
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
	log.Info("starting matching service", "version", cfg.Server.Version)

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
	_ = redis
	log.Info("redis connected")

	matchingService := matching.NewMatchingService(log)

	loadPendingOrders(db, matchingService, log)

	go startHTTPServer(cfg, log, matchingService)
	go startGRPCServer(cfg, log)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down matching service...")
}

func startHTTPServer(cfg *config.Config, log *logger.Logger, matchingService *matching.MatchingService) {
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
		v1.GET("/orderbook/:symbol", getOrderBookHandler(matchingService, log))
		v1.GET("/depth/:symbol", getMarketDepthHandler(matchingService, log))
		v1.GET("/lastprice/:symbol", getLastPriceHandler(matchingService, log))
		v1.GET("/spread/:symbol", getSpreadHandler(matchingService, log))
		v1.GET("/midprice/:symbol", getMidPriceHandler(matchingService, log))
		v1.GET("/snapshot/:symbol", getSnapshotHandler(matchingService, log))
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.HTTP.Addr, 8084)
	log.Info("starting HTTP server", "addr", addr)
	if err := router.Run(addr); err != nil {
		log.Errorw("HTTP server error", "error", err)
	}
}

func startGRPCServer(cfg *config.Config, log *logger.Logger) {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Server.GRPC.Addr, 50054))
	if err != nil {
		log.Errorw("failed to listen", "error", err)
		return
	}

	grpcServer := grpc.NewServer()
	log.Info("starting gRPC server", "port", 50054)
	if err := grpcServer.Serve(lis); err != nil {
		log.Errorw("gRPC server error", "error", err)
	}
}

func getOrderBookHandler(matchingService *matching.MatchingService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		symbol := c.Param("symbol")
		ob := matchingService.GetOrderBook(c.Request.Context(), symbol)
		if ob == nil {
			c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "orderbook": nil, "message": "empty orderbook"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "orderbook": ob})
	}
}

func getMarketDepthHandler(matchingService *matching.MatchingService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		symbol := c.Param("symbol")
		level := 5
		if l := c.Query("level"); l != "" {
			fmt.Sscanf(l, "%d", &level)
		}
		depth := matchingService.GetMarketDepth(c.Request.Context(), symbol, level)
		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "depth": depth})
	}
}

func getLastPriceHandler(matchingService *matching.MatchingService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		symbol := c.Param("symbol")
		price := matchingService.GetLastPrice(c.Request.Context(), symbol)
		c.JSON(http.StatusOK, gin.H{
			"code":       errors.CodeSuccess,
			"symbol":     symbol,
			"last_price": price.String(),
		})
	}
}

func getSpreadHandler(matchingService *matching.MatchingService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		symbol := c.Param("symbol")
		spread := matchingService.GetSpread(c.Request.Context(), symbol)
		c.JSON(http.StatusOK, gin.H{
			"code":    errors.CodeSuccess,
			"symbol":  symbol,
			"spread":  spread.String(),
		})
	}
}

func getMidPriceHandler(matchingService *matching.MatchingService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		symbol := c.Param("symbol")
		midPrice := matchingService.GetMidPrice(c.Request.Context(), symbol)
		c.JSON(http.StatusOK, gin.H{
			"code":      errors.CodeSuccess,
			"symbol":    symbol,
			"mid_price": midPrice.String(),
		})
	}
}

func getSnapshotHandler(matchingService *matching.MatchingService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		symbol := c.Param("symbol")
		depth := 10
		if d := c.Query("depth"); d != "" {
			fmt.Sscanf(d, "%d", &depth)
		}
		snapshot := matchingService.GetOrderBookSnapshot(c.Request.Context(), symbol, depth)
		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "snapshot": snapshot})
	}
}

func loadPendingOrders(db *gorm.DB, matchingService *matching.MatchingService, log *logger.Logger) {
	var orders []*order.Order
	result := db.Where("status IN ?", []int{order.OrderStatusPending, order.OrderStatusPartial}).Find(&orders)
	if result.Error != nil {
		log.Errorw("failed to load pending orders", "error", result.Error)
		return
	}

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
	}

	loadedCount := matchingService.LoadPendingOrders(context.Background(), orders)
	log.Info("pending orders loaded", "count", loadedCount)
}
