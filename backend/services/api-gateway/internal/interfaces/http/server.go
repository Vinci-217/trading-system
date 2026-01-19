package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"stock_trader/backend/services/api-gateway/internal/domain/entity"
	"stock_trader/backend/services/api-gateway/internal/domain/service"
	"stock_trader/backend/services/api-gateway/internal/infrastructure/config"
	"stock_trader/backend/services/api-gateway/internal/infrastructure/logger"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type HTTPServer struct {
	cfg           *config.Config
	logger        *logger.Logger
	domainService *service.GatewayDomainService
}

func NewHTTPServer(cfg *config.Config, logger *logger.Logger, domainService *service.GatewayDomainService) *HTTPServer {
	return &HTTPServer{
		cfg:           cfg,
		logger:        logger,
		domainService: domainService,
	}
}

func (s *HTTPServer) RegisterRoutes(router *gin.Engine) {
	router.Use(s.loggingMiddleware())
	router.Use(s.recoveryMiddleware())
	router.Use(s.corsMiddleware())

	router.GET("/health", s.healthCheck)
	router.GET("/ready", s.readinessCheck)

	v1 := router.Group("/api/v1")
	v1.Use(s.rateLimitMiddleware())
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", s.register)
			auth.POST("/login", s.login)
		}

		users := v1.Group("/users")
		users.Use(s.authMiddleware())
		{
			users.GET("/:user_id", s.getUser)
			users.GET("/:user_id/orders", s.getUserOrders)
			users.GET("/:user_id/positions", s.getUserPositions)
			users.GET("/:user_id/balance", s.getUserBalance)
		}

		orders := v1.Group("/orders")
		orders.Use(s.authMiddleware())
		{
			orders.POST("", s.createOrder)
			orders.GET("/:order_id", s.getOrder)
			orders.POST("/:order_id/cancel", s.cancelOrder)
			orders.POST("/:order_id/confirm", s.confirmOrder)
		}

		market := v1.Group("/market")
		market.Use(s.authMiddleware())
		{
			market.GET("/quotes/:symbol", s.getQuote)
			market.GET("/quotes", s.getBatchQuotes)
			market.GET("/kline/:symbol", s.getKLine)
			market.GET("/klines/:symbol", s.getKLines)
		}

		account := v1.Group("/account")
		account.Use(s.authMiddleware())
		{
			account.GET("/:user_id", s.getAccount)
			account.POST("/deposit", s.deposit)
			account.POST("/withdraw", s.withdraw)
		}

		reconciliation := v1.Group("/reconciliation")
		reconciliation.Use(s.authMiddleware())
		{
			reconciliation.POST("/funds", s.reconcileFunds)
			reconciliation.POST("/positions", s.reconcilePositions)
			reconciliation.GET("/discrepancies", s.getDiscrepancies)
			reconciliation.POST("/full", s.runFullReconciliation)
		}
	}

	router.GET("/ws", s.handleWebSocket)
}

func (s *HTTPServer) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := uuid.New().String()

		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		duration := time.Since(start)
		s.logger.Info("HTTP请求",
			logger.String("request_id", requestID),
			logger.String("method", c.Request.Method),
			logger.String("path", path),
			logger.String("query", query),
			logger.Int("status", c.Writer.Status()),
			logger.Duration("duration", duration),
			logger.String("client_ip", c.ClientIP()))
	}
}

func (s *HTTPServer) recoveryMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		requestID, _ := c.Get("request_id")
		s.logger.Error("Panic recovered",
			logger.String("request_id", requestID.(string)),
			logger.Any("panic", recovered))

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal server error",
			"message": "An unexpected error occurred",
		})
	})
}

func (s *HTTPServer) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func (s *HTTPServer) rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		allowed, remaining := s.domainService.CheckRateLimit("default", clientIP)

		c.Header("X-RateLimit-Limit", "100")
		c.Header("X-RateLimit-Remaining", "100")

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many requests, please try again later",
			})
			return
		}

		c.Next()
	}
}

func (s *HTTPServer) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Authorization header is required",
			})
			return
		}

		tokenString := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}

		claims := &entity.JWTClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(s.cfg.JWT.Secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Invalid or expired token",
			})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

func (s *HTTPServer) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "api-gateway",
		"timestamp": time.Now().UnixMilli(),
	})
}

func (s *HTTPServer) readinessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"checks": gin.H{
			"gateway": true,
		},
	})
}

func (s *HTTPServer) register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Email    string `json:"email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "注册成功",
		"user_id": uuid.New().String(),
	})
}

func (s *HTTPServer) login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &entity.JWTClaims{
		UserID:   uuid.New().String(),
		Username: req.Username,
		Role:     "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(s.cfg.JWT.Expiration) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	tokenString, err := token.SignedString([]byte(s.cfg.JWT.Secret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "生成令牌失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   tokenString,
		"user_id": uuid.New().String(),
	})
}

func (s *HTTPServer) getUser(c *gin.Context) {
	userID := c.Param("user_id")
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"user_id":  userID,
		"username": "test_user",
	})
}

func (s *HTTPServer) getUserOrders(c *gin.Context) {
	userID := c.Param("user_id")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user_id": userID,
		"orders":  []interface{}{},
	})
}

func (s *HTTPServer) getUserPositions(c *gin.Context) {
	userID := c.Param("user_id")
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"user_id":  userID,
		"positions": []interface{}{},
	})
}

func (s *HTTPServer) getUserBalance(c *gin.Context) {
	userID := c.Param("user_id")
	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"user_id":          userID,
		"balance":          "100000.00",
		"frozen_balance":   "0.00",
		"available_balance": "100000.00",
	})
}

func (s *HTTPServer) createOrder(c *gin.Context) {
	var req struct {
		Symbol    string `json:"symbol" binding:"required"`
		OrderType string `json:"order_type" binding:"required"`
		Side      string `json:"side" binding:"required"`
		Price     string `json:"price" binding:"required"`
		Quantity  int32  `json:"quantity" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	userID, _ := c.Get("user_id")

	c.JSON(http.StatusCreated, gin.H{
		"success":    true,
		"order_id":   uuid.New().String(),
		"user_id":    userID,
		"symbol":     req.Symbol,
		"side":       req.Side,
		"quantity":   req.Quantity,
		"price":      req.Price,
		"status":     "PENDING",
		"created_at": time.Now().UnixMilli(),
	})
}

func (s *HTTPServer) getOrder(c *gin.Context) {
	orderID := c.Param("order_id")
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"order_id": orderID,
		"symbol":   "600519",
		"side":     "BUY",
		"quantity": 100,
		"price":    "1800.00",
		"status":   "PENDING",
	})
}

func (s *HTTPServer) cancelOrder(c *gin.Context) {
	orderID := c.Param("order_id")
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"order_id": orderID,
		"status":   "CANCELLED",
	})
}

func (s *HTTPServer) confirmOrder(c *gin.Context) {
	orderID := c.Param("order_id")
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"order_id": orderID,
		"status":   "CONFIRMED",
	})
}

func (s *HTTPServer) getQuote(c *gin.Context) {
	symbol := c.Param("symbol")
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"symbol":     symbol,
		"price":      "1800.00",
		"change":     "10.00",
		"change_pct": "0.56",
		"volume":     1000000,
		"timestamp":  time.Now().UnixMilli(),
	})
}

func (s *HTTPServer) getBatchQuotes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"quotes": []gin.H{
			{"symbol": "600519", "price": "1800.00"},
			{"symbol": "000001", "price": "3500.00"},
		},
	})
}

func (s *HTTPServer) getKLine(c *gin.Context) {
	symbol := c.Param("symbol")
	interval := c.DefaultQuery("interval", "1m")

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"symbol":   symbol,
		"interval": interval,
		"data": gin.H{
			"timestamp": time.Now().UnixMilli(),
			"open":      "1795.00",
			"high":      "1805.00",
			"low":       "1790.00",
			"close":     "1800.00",
			"volume":    10000,
		},
	})
}

func (s *HTTPServer) getKLines(c *gin.Context) {
	symbol := c.Param("symbol")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"symbol":  symbol,
		"data":    []interface{}{},
	})
}

func (s *HTTPServer) getAccount(c *gin.Context) {
	userID := c.Param("user_id")
	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"user_id":           userID,
		"cash_balance":      "100000.00",
		"frozen_balance":    "0.00",
		"available_balance": "100000.00",
		"total_assets":      "100000.00",
	})
}

func (s *HTTPServer) deposit(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Amount string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"transaction": uuid.New().String(),
		"new_balance": "110000.00",
	})
}

func (s *HTTPServer) withdraw(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Amount string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"transaction": uuid.New().String(),
		"new_balance": "90000.00",
	})
}

func (s *HTTPServer) reconcileFunds(c *gin.Context) {
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

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"user_id":          req.UserID,
		"expected_balance": "100000.00",
		"actual_balance":   "100000.00",
		"discrepancy":      "0.00",
		"is_consistent":    true,
	})
}

func (s *HTTPServer) reconcilePositions(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Symbol string `json:"symbol" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"user_id":           req.UserID,
		"symbol":            req.Symbol,
		"expected_position": 100,
		"actual_position":   100,
		"discrepancy":       0,
		"is_consistent":     true,
	})
}

func (s *HTTPServer) getDiscrepancies(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"discrepancies": []interface{}{},
	})
}

func (s *HTTPServer) runFullReconciliation(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"report_id":          uuid.New().String(),
		"fund_discrepancies": 0,
		"duration":           "5s",
	})
}

func (s *HTTPServer) handleWebSocket(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "请直接连接行情服务的WebSocket",
	})
}
