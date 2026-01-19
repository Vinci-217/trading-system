package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"stock_trader/backend/services/market-service/internal/domain/entity"
	"stock_trader/backend/services/market-service/internal/domain/service"
	"stock_trader/backend/services/market-service/internal/infrastructure/logger"
	"stock_trader/backend/services/market-service/internal/infrastructure/messaging"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type HTTPServer struct {
	domainService *service.MarketDomainService
	publisher     *messaging.Publisher
	logger        *logger.Logger
	upgrader      websocket.Upgrader
}

func NewHTTPServer(domainService *service.MarketDomainService, publisher *messaging.Publisher, logger *logger.Logger) *HTTPServer {
	return &HTTPServer{
		domainService: domainService,
		publisher:     publisher,
		logger:        logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (s *HTTPServer) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", s.healthCheck)
	router.GET("/ready", s.readinessCheck)

	api := router.Group("/api/v1")
	{
		api.GET("/quotes/:symbol", s.getQuote)
		api.POST("/quotes/batch", s.getBatchQuotes)
		api.GET("/kline/:symbol", s.getKLine)
		api.GET("/klines/:symbol", s.getKLines)
	}

	router.GET("/ws", s.handleWebSocket)
}

func (s *HTTPServer) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "market-service",
		"timestamp": time.Now().UnixMilli(),
	})
}

func (s *HTTPServer) readinessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (s *HTTPServer) getQuote(c *gin.Context) {
	symbol := c.Param("symbol")

	quote, err := s.domainService.GetQuote(c.Request.Context(), symbol)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"quote": gin.H{
			"symbol":     quote.Symbol,
			"price":      quote.Price.String(),
			"change":     quote.Change.String(),
			"change_pct": quote.ChangePct.String(),
			"open":       quote.Open.String(),
			"high":       quote.High.String(),
			"low":        quote.Low.String(),
			"close":      quote.Close.String(),
			"volume":     quote.Volume,
			"amount":     quote.Amount.String(),
			"bid_price":  quote.BidPrice.String(),
			"ask_price":  quote.AskPrice.String(),
			"bid_volume": quote.BidVolume,
			"ask_volume": quote.AskVolume,
			"timestamp":  quote.Timestamp.UnixMilli(),
		},
	})
}

func (s *HTTPServer) getBatchQuotes(c *gin.Context) {
	var req struct {
		Symbols []string `json:"symbols" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	quotes, err := s.domainService.GetBatchQuotes(c.Request.Context(), req.Symbols)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	result := make([]gin.H, len(quotes))
	for i, quote := range quotes {
		result[i] = gin.H{
			"symbol":     quote.Symbol,
			"price":      quote.Price.String(),
			"change":     quote.Change.String(),
			"change_pct": quote.ChangePct.String(),
			"volume":     quote.Volume,
			"timestamp":  quote.Timestamp.UnixMilli(),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"quotes":  result,
	})
}

func (s *HTTPServer) getKLine(c *gin.Context) {
	symbol := c.Param("symbol")
	interval := c.DefaultQuery("interval", "1m")

	kline, err := s.domainService.GetKLine(c.Request.Context(), symbol, interval)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"kline": gin.H{
			"symbol":    kline.Symbol,
			"interval":  kline.Interval,
			"open":      kline.Open.String(),
			"high":      kline.High.String(),
			"low":       kline.Low.String(),
			"close":     kline.Close.String(),
			"volume":    kline.Volume,
			"amount":    kline.Amount.String(),
			"timestamp": kline.StartTime.UnixMilli(),
		},
	})
}

func (s *HTTPServer) getKLines(c *gin.Context) {
	symbol := c.Param("symbol")
	interval := c.DefaultQuery("interval", "1m")
	limit := 100
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	klines, err := s.domainService.GetKLines(c.Request.Context(), symbol, interval, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	result := make([]gin.H, len(klines))
	for i, kline := range klines {
		result[i] = gin.H{
			"timestamp": kline.StartTime.UnixMilli(),
			"open":      kline.Open.String(),
			"high":      kline.High.String(),
			"low":       kline.Low.String(),
			"close":     kline.Close.String(),
			"volume":    kline.Volume,
			"amount":    kline.Amount.String(),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"symbol":  symbol,
		"data":    result,
	})
}

func (s *HTTPServer) handleWebSocket(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		symbol = "all"
	}

	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.logger.Error("WebSocket升级失败", logger.Error(err))
		return
	}

	client := &WSClient{
		ID:     "",
		Symbol: symbol,
		Conn:   conn,
		Send:   make(chan []byte, 256),
	}

	go s.writePump(client)
	go s.readPump(client)
}

func (s *HTTPServer) readPump(client *WSClient) {
	defer func() {
		client.Conn.Close()
	}()

	client.Conn.SetReadLimit(512 * 1024)
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Error("WebSocket读取错误", logger.Error(err))
			}
			break
		}
	}
}

func (s *HTTPServer) writePump(client *WSClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type WSClient struct {
	ID     string
	Symbol string
	Conn   *websocket.Conn
	Send   chan []byte
}
