package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"stock_trader/matching-service/internal/domain/entity"
	"stock_trader/matching-service/internal/domain/service"
	"stock_trader/matching-service/internal/infrastructure/config"
	"stock_trader/matching-service/internal/interfaces/websocket"
)

type Server struct {
	addr           string
	logger         Logger
	cfg            *config.Config
	wsHub          *websocket.Hub
	matchingEngine *service.MatchingEngine
	orderBookRepo  interface {
		GetTrades(ctx context.Context, symbol string, limit int) ([]*entity.Trade, error)
	}
}

type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

func NewServer(
	addr string,
	logger Logger,
	cfg *config.Config,
	wsHub *websocket.Hub,
) *Server {
	return &Server{
		addr:           addr,
		logger:         logger,
		cfg:            cfg,
		wsHub:          wsHub,
		matchingEngine: service.NewMatchingEngine(),
	}
}

func (s *Server) SetHandler(handler http.Handler) {
	s.matchingEngine = service.NewMatchingEngine()
}

func (s *Server) Start() error {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(s.loggingMiddleware())

	router.GET("/health", s.healthCheck)
	router.GET("/ready", s.readinessCheck)

	v1 := router.Group("/api/v1")
	{
		v1.POST("/orders/submit", s.submitOrder)
		v1.POST("/orders/cancel", s.cancelOrder)
		v1.GET("/market/depth/:symbol", s.getDepth)
		v1.GET("/market/trades/:symbol", s.getTrades)
		v1.GET("/market/orderbook/:symbol", s.getOrderBook)
	}

	router.GET("/ws", s.handleWebSocket)

	srv := &http.Server{
		Addr:         s.addr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return srv.ListenAndServe()
}

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "matching-service",
		"timestamp": time.Now().UnixMilli(),
	})
}

func (s *Server) readinessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

func (s *Server) submitOrder(c *gin.Context) {
	var req struct {
		OrderID   string `json:"order_id"`
		UserID    string `json:"user_id"`
		Symbol    string `json:"symbol"`
		OrderType string `json:"order_type"`
		Side      string `json:"side"`
		Price     string `json:"price"`
		Quantity  int32  `json:"quantity"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order := entity.NewOrder(
		req.OrderID,
		req.UserID,
		req.Symbol,
		entity.OrderType(req.OrderType),
		entity.OrderSide(req.Side),
		parseDecimal(req.Price),
		int(req.Quantity),
	)

	trades, err := s.matchingEngine.SubmitOrder(c.Request.Context(), order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"order_id":  order.ID,
		"trades":    len(trades),
	})
}

func (s *Server) cancelOrder(c *gin.Context) {
	var req struct {
		OrderID string `json:"order_id"`
		Symbol  string `json:"symbol"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	orderBook := s.matchingEngine.GetOrderBook(req.Symbol)
	if orderBook == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订单簿不存在"})
		return
	}

	if !s.matchingEngine.CancelOrder(orderBook, req.OrderID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "订单未找到"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "订单已取消",
	})
}

func (s *Server) getDepth(c *gin.Context) {
	symbol := c.Param("symbol")

	orderBook := s.matchingEngine.GetOrderBook(symbol)
	if orderBook == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订单簿不存在"})
		return
	}

	depth := orderBook.GetMarketDepth()

	c.JSON(http.StatusOK, gin.H{
		"symbol": symbol,
		"bids":   depth.Bids,
		"asks":   depth.Asks,
	})
}

func (s *Server) getTrades(c *gin.Context) {
	symbol := c.Param("symbol")
	limit := 100

	trades, err := s.orderBookRepo.GetTrades(c.Request.Context(), symbol, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, trades)
}

func (s *Server) getOrderBook(c *gin.Context) {
	symbol := c.Param("symbol")

	orderBook := s.matchingEngine.GetOrderBook(symbol)
	if orderBook == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订单簿不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":      symbol,
		"buy_orders":  orderBook.BuyOrders.Orders,
		"sell_orders": orderBook.SellOrders.Orders,
	})
}

func (s *Server) handleWebSocket(c *gin.Context) {
	symbol := c.Query("symbol")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.logger.Error("WebSocket升级失败", s.logger.Error(err))
		return
	}

	client := &websocket.Client{
		Symbol: symbol,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Hub:    s.wsHub,
	}

	s.wsHub.Register <- client

	go client.WritePump(s.logger)
	go client.ReadPump(s.logger)
}

func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		duration := time.Since(start)
		s.logger.Info("HTTP请求",
			s.logger.String("method", c.Request.Method),
			s.logger.String("path", path),
			s.logger.Int("status", c.Writer.Status()),
			s.logger.Duration("duration", duration))
	}
}

func parseDecimal(s string) Decimal {
	d, _ := DecimalFromString(s)
	return d
}

import (
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Decimal = decimal.Decimal

func DecimalFromString(s string) (Decimal, error) {
	return decimal.NewFromString(s)
}

func init() {
	gin.SetMode(gin.ReleaseMode)
}
