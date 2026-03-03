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

	"github.com/stock-trading-system/internal/domain/account"
	"github.com/stock-trading-system/internal/infrastructure/cache"
	"github.com/stock-trading-system/internal/infrastructure/config"
	"github.com/stock-trading-system/internal/infrastructure/database"
	"github.com/stock-trading-system/internal/infrastructure/idgen"
	"github.com/stock-trading-system/internal/service/reconcile"
	"github.com/stock-trading-system/pkg/errors"
	"github.com/stock-trading-system/pkg/logger"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	log := logger.NewLogger(cfg.Log.Level)
	log.Info("starting reconcile service", "version", cfg.Server.Version)

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

	_ = idgen.NewIDGenerator(redis.Client(), "06")

	reconcileService := reconcile.NewReconcileService(db, log)

	go startHTTPServer(cfg, log, reconcileService)
	go startGRPCServer(cfg, log)

	go startScheduledReconciliation(cfg, log, reconcileService)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down reconcile service...")
}

func startHTTPServer(cfg *config.Config, log *logger.Logger, reconcileService *reconcile.ReconcileService) {
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
		v1.POST("/reconcile/account/:user_id", reconcileAccountHandler(reconcileService, log))
		v1.POST("/reconcile/position/:user_id/:symbol", reconcilePositionHandler(reconcileService, log))
		v1.POST("/reconcile/positions", reconcileAllPositionsHandler(reconcileService, log))
		v1.POST("/reconcile/daily/:date", dailyReconcileHandler(reconcileService, log))
		v1.GET("/discrepancies", getDiscrepanciesHandler(reconcileService, log))
		v1.POST("/discrepancies/:id/resolve", resolveDiscrepancyHandler(reconcileService, log))
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.HTTP.Addr, 8086)
	log.Info("starting HTTP server", "addr", addr)
	if err := router.Run(addr); err != nil {
		log.Errorw("HTTP server error", "error", err)
	}
}

func startGRPCServer(cfg *config.Config, log *logger.Logger) {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Server.GRPC.Addr, 50056))
	if err != nil {
		log.Errorw("failed to listen", "error", err)
		return
	}

	grpcServer := grpc.NewServer()
	log.Info("starting gRPC server", "port", 50056)
	if err := grpcServer.Serve(lis); err != nil {
		log.Errorw("gRPC server error", "error", err)
	}
}

func startScheduledReconciliation(cfg *config.Config, log *logger.Logger, reconcileService *reconcile.ReconcileService) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		log.Info("starting scheduled reconciliation")
		ctx := context.Background()
		date := time.Now().Format("2006-01-02")
		_, err := reconcileService.DailyReconcile(ctx, date)
		if err != nil {
			log.Errorw("scheduled reconciliation failed", "error", err)
		}
	}
}

func reconcileAccountHandler(reconcileService *reconcile.ReconcileService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("user_id")

		result, err := reconcileService.ReconcileAccount(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "result": result})
	}
}

func reconcilePositionHandler(reconcileService *reconcile.ReconcileService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("user_id")
		symbol := c.Param("symbol")

		result, err := reconcileService.ReconcilePosition(c.Request.Context(), userID, symbol)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "result": result})
	}
}

func reconcileAllPositionsHandler(reconcileService *reconcile.ReconcileService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		results, err := reconcileService.ReconcileAllPositions(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "results": results})
	}
}

func dailyReconcileHandler(reconcileService *reconcile.ReconcileService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		date := c.Param("date")

		report, err := reconcileService.DailyReconcile(c.Request.Context(), date)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "report": report})
	}
}

func getDiscrepanciesHandler(reconcileService *reconcile.ReconcileService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Query("user_id")
		startDateStr := c.Query("start_date")
		endDateStr := c.Query("end_date")

		var startDate, endDate time.Time
		if startDateStr != "" {
			startDate, _ = time.Parse("2006-01-02", startDateStr)
		}
		if endDateStr != "" {
			endDate, _ = time.Parse("2006-01-02", endDateStr)
		}

		discrepancies, err := reconcileService.GetDiscrepancies(c.Request.Context(), userID, startDate, endDate)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "discrepancies": discrepancies})
	}
}

func resolveDiscrepancyHandler(reconcileService *reconcile.ReconcileService, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var id uint
		fmt.Sscanf(c.Param("id"), "%d", &id)

		resolution := c.PostForm("resolution")
		if resolution == "" {
			resolution = "手动修复"
		}

		err := reconcileService.ResolveDiscrepancy(c.Request.Context(), id, resolution)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": errors.CodeUnknownError, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": errors.CodeSuccess, "message": "差异已解决"})
	}
}

func init() {
	_ = account.Discrepancy{}
}
