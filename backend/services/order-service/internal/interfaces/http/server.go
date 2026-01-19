package http

import (
	"net/http"
	"strconv"

	"stock_trader/backend/services/order-service/internal/domain/entity"
	"stock_trader/backend/services/order-service/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type HTTPServer struct {
	domainService *service.OrderDomainService
}

func NewHTTPServer(domainService *service.OrderDomainService) *HTTPServer {
	return &HTTPServer{
		domainService: domainService,
	}
}

func (s *HTTPServer) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", s.healthCheck)
	router.GET("/ready", s.readinessCheck)

	api := router.Group("/api/v1")
	{
		api.POST("/orders", s.createOrder)
		api.GET("/orders/:id", s.getOrder)
		api.GET("/users/:user_id/orders", s.getUserOrders)
		api.GET("/market/:symbol/orders", s.getMarketOrders)
		api.POST("/orders/:id/cancel", s.cancelOrder)
		api.POST("/orders/:id/confirm", s.confirmOrder)
	}
}

func (s *HTTPServer) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "order-service",
		"timestamp": 0,
	})
}

func (s *HTTPServer) readinessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (s *HTTPServer) createOrder(c *gin.Context) {
	var req struct {
		UserID        string `json:"user_id" binding:"required"`
		Symbol        string `json:"symbol" binding:"required"`
		OrderType     string `json:"order_type" binding:"required"`
		Side          string `json:"side" binding:"required"`
		Price         string `json:"price" binding:"required"`
		Quantity      int32  `json:"quantity" binding:"required"`
		ClientOrderID string `json:"client_order_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	price, err := decimal.NewFromString(req.Price)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效价格",
		})
		return
	}

	order, tccTx, err := s.domainService.CreateOrder(
		c.Request.Context(),
		req.UserID,
		req.Symbol,
		entity.OrderType(req.OrderType),
		entity.OrderSide(req.Side),
		price,
		int(req.Quantity),
		req.ClientOrderID,
	)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":     true,
		"order_id":    order.ID,
		"transaction": tccTx.TransactionID,
		"created_at":  order.CreatedAt.UnixMilli(),
	})
}

func (s *HTTPServer) getOrder(c *gin.Context) {
	orderID := c.Param("id")

	order, err := s.domainService.GetOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"order": gin.H{
			"id":              order.ID,
			"user_id":         order.UserID,
			"symbol":          order.Symbol,
			"order_type":      order.OrderType,
			"side":            order.Side,
			"price":           order.Price.String(),
			"quantity":        order.Quantity,
			"filled_quantity": order.FilledQuantity,
			"status":          order.Status,
			"fee":             order.Fee.String(),
			"created_at":      order.CreatedAt.UnixMilli(),
			"updated_at":      order.UpdatedAt.UnixMilli(),
		},
	})
}

func (s *HTTPServer) getUserOrders(c *gin.Context) {
	userID := c.Param("user_id")
	status := c.Query("status")
	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	orders, err := s.domainService.GetOrdersByUser(c.Request.Context(), userID, status, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	result := make([]gin.H, len(orders))
	for i, order := range orders {
		result[i] = gin.H{
			"id":              order.ID,
			"symbol":          order.Symbol,
			"order_type":      order.OrderType,
			"side":            order.Side,
			"price":           order.Price.String(),
			"quantity":        order.Quantity,
			"filled_quantity": order.FilledQuantity,
			"status":          order.Status,
			"fee":             order.Fee.String(),
			"created_at":      order.CreatedAt.UnixMilli(),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"orders":  result,
	})
}

func (s *HTTPServer) getMarketOrders(c *gin.Context) {
	symbol := c.Param("symbol")
	status := c.Query("status")
	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	orders, err := s.domainService.GetOrdersBySymbol(c.Request.Context(), symbol, status, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	result := make([]gin.H, len(orders))
	for i, order := range orders {
		result[i] = gin.H{
			"id":              order.ID,
			"user_id":         order.UserID,
			"price":           order.Price.String(),
			"quantity":        order.Quantity,
			"filled_quantity": order.FilledQuantity,
			"status":          order.Status,
			"fee":             order.Fee.String(),
			"created_at":      order.CreatedAt.UnixMilli(),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"orders":  result,
	})
}

func (s *HTTPServer) cancelOrder(c *gin.Context) {
	orderID := c.Param("id")

	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	err := s.domainService.CancelOrder(c.Request.Context(), orderID, req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "订单取消成功",
	})
}

func (s *HTTPServer) confirmOrder(c *gin.Context) {
	orderID := c.Param("id")

	err := s.domainService.ConfirmOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "订单确认成功",
	})
}

type OrderProto struct {
	ID             string `json:"id"`
	UserID         string `json:"user_id"`
	Symbol         string `json:"symbol"`
	OrderType      string `json:"order_type"`
	Side           string `json:"side"`
	Price          string `json:"price"`
	Quantity       int32  `json:"quantity"`
	FilledQuantity int32  `json:"filled_quantity"`
	Status         string `json:"status"`
	Fee            string `json:"fee"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
	ClientOrderID  string `json:"client_order_id"`
	Remarks        string `json:"remarks"`
}

type CreateOrderRequest struct {
	UserID        string `json:"user_id"`
	Symbol        string `json:"symbol"`
	OrderType     string `json:"order_type"`
	Side          string `json:"side"`
	Price         string `json:"price"`
	Quantity      int32  `json:"quantity"`
	ClientOrderID string `json:"client_order_id"`
}

type CreateOrderResponse struct {
	Success     bool   `json:"success"`
	OrderID     string `json:"order_id"`
	Transaction string `json:"transaction"`
	Error       string `json:"error"`
	CreatedAt   int64  `json:"created_at"`
}

type GetOrderRequest struct {
	OrderID string `json:"order_id"`
}

type GetOrderResponse struct {
	Success bool        `json:"success"`
	Order   *OrderProto `json:"order"`
	Error   string      `json:"error"`
}

type GetUserOrdersRequest struct {
	UserID string `json:"user_id"`
	Status string `json:"status"`
	Limit  int32  `json:"limit"`
}

type GetUserOrdersResponse struct {
	Success bool          `json:"success"`
	Orders  []*OrderProto `json:"orders"`
	Error   string        `json:"error"`
}

type GetMarketOrdersRequest struct {
	Symbol string `json:"symbol"`
	Status string `json:"status"`
	Limit  int32  `json:"limit"`
}

type GetMarketOrdersResponse struct {
	Success bool          `json:"success"`
	Orders  []*OrderProto `json:"orders"`
	Error   string        `json:"error"`
}

type CancelOrderRequest struct {
	OrderID string `json:"order_id"`
	UserID  string `json:"user_id"`
}

type CancelOrderResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

type ConfirmOrderRequest struct {
	OrderID string `json:"order_id"`
}

type ConfirmOrderResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}
