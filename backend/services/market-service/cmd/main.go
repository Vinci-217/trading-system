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

	"stock_trader/backend/services/market-service/internal/domain/entity"
	"stock_trader/backend/services/market-service/internal/domain/service"
	"stock_trader/backend/services/market-service/internal/infrastructure/config"
	"stock_trader/backend/services/market-service/internal/infrastructure/logger"
	"stock_trader/backend/services/market-service/internal/infrastructure/messaging"
	"stock_trader/backend/services/market-service/internal/interfaces/grpc"
	"stock_trader/backend/services/market-service/internal/interfaces/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Application struct {
	cfg           *config.Config
	redis         *redis.Client
	logger        *logger.Logger
	publisher     *messaging.Publisher
	domainService *service.MarketDomainService
	grpcServer    *grpc.Server
	httpServer    *http.Server
	wsHub         *WSHub
}

type WSHub struct {
	clients   map[string]*WSClient
	broadcast chan []byte
	register  chan *WSClient
	unregister chan *WSClient
	mu        sync.RWMutex
}

type WSClient struct {
	ID     string
	Symbol string
	Conn   *websocket.Conn
	Send   chan []byte
}

type WSMessage struct {
	Type   string
	Symbol string
	Data   interface{}
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients:   make(map[string]*WSClient),
		broadcast: make(chan []byte, 1000),
		register:  make(chan *WSClient),
		unregister: make(chan *WSClient),
	}
}

func (hub *WSHub) run() {
	for {
		select {
		case client := <-hub.register:
			hub.mu.Lock()
			hub.clients[client.ID] = client
			hub.mu.Unlock()

		case client := <-hub.unregister:
			hub.mu.Lock()
			if _, ok := hub.clients[client.ID]; ok {
				delete(hub.clients, client.ID)
				close(client.Send)
			}
			hub.mu.Unlock()

		case message := <-hub.broadcast:
			hub.mu.RLock()
			for _, client := range hub.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(hub.clients, client.ID)
				}
			}
			hub.mu.RUnlock()
		}
	}
}

func NewApplication(cfg *config.Config) *Application {
	return &Application{
		cfg:   cfg,
		wsHub: NewWSHub(),
	}
}

func (a *Application) Initialize() error {
	var err error

	a.logger = logger.NewLogger("market-service")
	a.logger.Info("初始化行情服务...")

	a.redis, err = a.newRedisClient()
	if err != nil {
		return fmt.Errorf("连接Redis失败: %w", err)
	}

	a.publisher = messaging.NewPublisher(a.redis, a.logger)

	a.domainService = service.NewMarketDomainService(a.logger)

	symbols := make([]entity.Symbol, 0, len(a.cfg.Symbols))
	for _, sc := range a.cfg.Symbols {
		symbols = append(symbols, entity.NewSymbol(sc.Symbol, sc.Name, decimal.NewFromFloat(sc.BasePrice)))
	}

	a.domainService.Initialize(symbols)

	go a.startMarketDataUpdater()

	go a.wsHub.run()

	a.logger.Info("行情服务初始化完成")
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

func (a *Application) startMarketDataUpdater() {
	interval := a.cfg.MarketData.GetUpdateInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		a.domainService.UpdateMarketData()
		a.publishQuotes()
	}
}

func (a *Application) publishQuotes() {
	ctx := context.Background()
	quotes := a.domainService.GetAllQuotes()

	for _, quote := range quotes {
		a.publisher.PublishQuote(ctx, quote)

		data, _ := json.Marshal(quote)
		msg := &WSMessage{
			Type:   "quote",
			Symbol: quote.Symbol,
			Data:   data,
		}
		msgData, _ := json.Marshal(msg)
		a.wsHub.broadcast <- msgData
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

	grpcServer := grpc.NewGRPCServer(a.domainService, a.logger)
	RegisterMarketServiceServer(a.grpcServer, grpcServer)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(a.grpcServer, healthServer)
	healthServer.SetServingStatus("market-service", grpc_health_v1.HealthCheckResponse_SERVING)

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

	httpServer := http.NewHTTPServer(a.domainService, a.publisher, a.logger)
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
	a.logger.Info("停止行情服务...")

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

	a.logger.Info("行情服务已停止")
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
			log.Fatalf("启动行情服务失败: %v", err)
		}
	}()

	<-sigChan
	app.Stop()
}

import (
	"encoding/json"

	"github.com/gorilla/websocket"

	"github.com/shopspring/decimal"
)
