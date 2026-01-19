package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stock_trader/backend/services/api-gateway/internal/domain/service"
	"stock_trader/backend/services/api-gateway/internal/infrastructure/config"
	"stock_trader/backend/services/api-gateway/internal/infrastructure/logger"
	"stock_trader/backend/services/api-gateway/internal/interfaces/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type Application struct {
	cfg           *config.Config
	logger        *logger.Logger
	domainService *service.GatewayDomainService
	redis         *redis.Client
	httpServer    *http.Server
}

func NewApplication(cfg *config.Config) *Application {
	return &Application{
		cfg: cfg,
	}
}

func (a *Application) Initialize() error {
	a.logger = logger.NewLogger("api-gateway")
	a.logger.Info("初始化API网关...")

	a.redis = redis.NewClient(&redis.Options{
		Addr:     a.cfg.Redis.Addr,
		Password: a.cfg.Redis.Password,
		DB:       a.cfg.Redis.DB,
		PoolSize: a.cfg.Redis.PoolSize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("连接Redis失败: %w", err)
	}

	a.domainService = service.NewGatewayDomainService(a.logger)
	a.domainService.Initialize()

	a.logger.Info("API网关初始化完成")
	return nil
}

func (a *Application) SetupHTTPServer() error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	httpServer := http.NewHTTPServer(a.cfg, a.logger, a.domainService)
	httpServer.RegisterRoutes(router)

	a.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", a.cfg.App.HTTPPort),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		a.logger.Info("API网关启动",
			logger.Int("port", a.cfg.App.HTTPPort))
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("HTTP服务器错误", logger.Error(err))
		}
	}()

	return nil
}

func (a *Application) Start() error {
	if err := a.Initialize(); err != nil {
		return err
	}

	if err := a.SetupHTTPServer(); err != nil {
		return err
	}

	return nil
}

func (a *Application) Stop() {
	a.logger.Info("停止API网关...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if a.httpServer != nil {
		a.httpServer.Shutdown(ctx)
	}

	if a.redis != nil {
		a.redis.Close()
	}

	a.logger.Info("API网关已停止")
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
			log.Fatalf("启动API网关失败: %v", err)
		}
	}()

	<-sigChan
	app.Stop()
}
