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
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"

	"github.com/stock-trading-system/internal/domain/matching"
	"github.com/stock-trading-system/internal/infrastructure/cache"
	"github.com/stock-trading-system/internal/infrastructure/config"
	"github.com/stock-trading-system/internal/infrastructure/database"
	"github.com/stock-trading-system/internal/infrastructure/mq"
	"github.com/stock-trading-system/internal/service/market"
	"github.com/stock-trading-system/pkg/errors"
	"github.com/stock-trading-system/pkg/logger"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	log := logger.NewLogger(cfg.Log.Level)
	log.Info("starting market service", "version", cfg.Server.Version)

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

	kafkaProducer, err := mq.NewKafkaProducer(cfg.Kafka.Brokers)
	if err != nil {
		log.Errorw("failed to create kafka producer", "error", err)
		panic(err)
	}
	defer kafkaProducer.Close()
	log.Info("kafka producer created")

	matchingEngine := matching.NewMatchingEngine()
	marketService := market.NewMarketService(db, matchingEngine, log)

	go startHTTPServer(cfg, log, marketService)
	go startGRPCServer(cfg, log)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down market service...")
}

func startHTTPServer(cfg *config.Config, log *logger.Logger, marketService *market.MarketService) {
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
		v1.GET("/quotes/:symbol", getQuoteHandler(marketService, log))
		v1.GET("/quotes", getQuotesHandler(marketService, log))
		v1.GET("/depth/:symbol", getDepthHandler(marketService, log))
		v1.GET("/ws/quotes/:symbol", wsQuoteHandler(marketService, log))
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.HTTP.Addr, 8083)
	log.Info("starting HTTP server", "addr", addr)
	if err := router.Run(addr); err != nil {
		log.Errorw("HTTP server error", "error", err)
	}
}

func startGRPCServer(cfg *config.Config, log *logger.Logger) {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Server.GRPC.Addr, 50053))
	if err != nil {
		log.Errorw("failed to listen", "error", err)
		return
	}

	grpcServer := grpc.NewServer()
	log.Info("starting gRPC server", "port", 50053)
	if err := grpcServer.Serve(lis); err != nil {
		log.Errorw("gRPC server error", "error", err)
	}
}

func getQuoteHandler(marketService *market.MarketService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		symbol := c.Param("symbol")

		quote, err := marketService.GetQuote(c.Request.Context(), symbol)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		if quote == nil {
			c.JSON(http.StatusNotFound, gin.H{"code": errors.CodeInvalidParam, "message": "证券不存在"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "quote": quote})
	}
}

func getQuotesHandler(marketService *market.MarketService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		quotes, err := marketService.GetAllQuotes(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "quotes": quotes})
	}
}

func getDepthHandler(marketService *market.MarketService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		symbol := c.Param("symbol")
		level := 5
		if l := c.Query("level"); l != "" {
			fmt.Sscanf(l, "%d", &level)
		}

		depth, err := marketService.GetMarketDepth(c.Request.Context(), symbol, level)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "depth": depth})
	}
}

func wsQuoteHandler(marketService *market.MarketService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		symbol := c.Param("symbol")

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Errorw("websocket upgrade failed", "error", err)
			return
		}
		defer conn.Close()

		log.Info("websocket client connected", "symbol", symbol)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		quoteChan := make(chan *market.Quote, 10)
		go marketService.SubscribeQuote(ctx, symbol, quoteChan)

		for {
			select {
			case quote := <-quoteChan:
				if err := conn.WriteJSON(quote); err != nil {
					log.Errorw("websocket write failed", "symbol", symbol, "error", err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}
}
