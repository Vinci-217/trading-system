package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/stock-trading-system/internal/infrastructure/cache"
	"github.com/stock-trading-system/internal/infrastructure/config"
	"github.com/stock-trading-system/internal/infrastructure/database"
	"github.com/stock-trading-system/pkg/errors"
	"github.com/stock-trading-system/pkg/logger"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	log := logger.NewLogger(cfg.Log.Level)
	log.Info("starting API gateway", "version", cfg.Server.Version)

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

	startHTTPServer(cfg, log, db, redis)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down API gateway...")
}

func startHTTPServer(cfg *config.Config, log *logger.Logger, db interface{}, redis *cache.RedisClient) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(rateLimitMiddleware(redis))
	router.Use(loggingMiddleware(log))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    "Stock Trading System API Gateway",
			"version": cfg.Server.Version,
			"status":  "running",
		})
	})

	api := router.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			v1.Any("/accounts/*path", proxyHandler("http://account-service:8081", log))
			v1.Any("/orders/*path", proxyHandler("http://trading-service:8082", log))
			v1.Any("/quotes/*path", proxyHandler("http://market-service:8083", log))
			v1.Any("/orderbook/*path", proxyHandler("http://matching-service:8084", log))
			v1.Any("/settle/*path", proxyHandler("http://settlement-service:8085", log))
			v1.Any("/reconcile/*path", proxyHandler("http://reconcile-service:8086", log))
			v1.Any("/discrepancies/*path", proxyHandler("http://reconcile-service:8086", log))
		}
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.HTTP.Addr, cfg.Server.HTTP.Port)
	log.Info("starting HTTP server", "addr", addr)
	if err := router.Run(addr); err != nil {
		log.Errorw("HTTP server error", "error", err)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func rateLimitMiddleware(redis *cache.RedisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func loggingMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		log.Info("HTTP request",
			"method", method,
			"path", path,
			"status", status,
			"latency", latency.String(),
			"client_ip", c.ClientIP(),
		)
	}
}

func proxyHandler(target string, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code":    errors.CodeSuccess,
			"message": fmt.Sprintf("Proxy to %s%s", target, c.Param("path")),
		})
	}
}
