package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"stock_trader/internal/matching"
	"stock_trader/internal/model"
	pb "stock_trader/api"
)

type Server struct {
	pb.UnimplementedUserServiceServer
	pb.UnimplementedAccountServiceServer
	pb.UnimplementedOrderServiceServer
	pb.UnimplementedTradeServiceServer
	pb.UnimplementedMarketServiceServer

	userSvc         *user.Service
	accountSvc      *account.Service
	orderSvc        *order.Service
	marketSvc       *market.Service
	reconSvc        *reconciliation.Service
	logger          *zap.Logger
	jwtSecret       []byte
}

func NewServer(userSvc *user.Service, accountSvc *account.Service, orderSvc *order.Service, marketSvc *market.Service, reconSvc *reconciliation.Service) *Server {
	return &Server{
		userSvc:    userSvc,
		accountSvc: accountSvc,
		orderSvc:   orderSvc,
		marketSvc:  marketSvc,
		reconSvc:   reconSvc,
		logger:     nil,
		jwtSecret:  []byte("stock-trader-secret-key"),
	}
}

func (s *Server) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=3,max=50"`
		Password string `json:"password" binding:"required,min=6"`
		Email    string `json:"email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := &model.User{
		Username:     req.Username,
		PasswordHash: req.Password,
		Email:        req.Email,
		Status:       model.UserStatusActive,
	}

	if err := s.userSvc.Register(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "注册成功",
		"user_id": user.ID,
	})
}

func (s *Server) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := s.userSvc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      tokenString,
		"user_id":    user.ID,
		"username":   user.Username,
		"expires_at": time.Now().Add(24 * time.Hour),
	})
}

func (s *Server) CreateOrder(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		Symbol    string  `json:"symbol" binding:"required"`
		OrderType string  `json:"order_type" binding:"required,oneof=MARKET LIMIT STOP"`
		Side      string  `json:"side" binding:"required,oneof=BUY SELL"`
		Price     float64 `json:"price"`
		Quantity  int     `json:"quantity" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	orderReq := &order.CreateOrderRequest{
		UserID:    userID.(int64),
		Symbol:    req.Symbol,
		OrderType: req.OrderType,
		Side:      req.Side,
		Price:     req.Price,
		Quantity:  req.Quantity,
	}

	result, err := s.orderSvc.CreateOrder(c.Request.Context(), orderReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Server) CancelOrder(c *gin.Context) {
	userID, _ := c.Get("user_id")
	orderID := c.Param("id")

	if err := s.orderSvc.CancelOrder(c.Request.Context(), userID.(int64), orderID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "撤单成功"})
}

func (s *Server) ListOrders(c *gin.Context) {
	userID, _ := c.Get("user_id")
	symbol := c.Query("symbol")
	status := c.Query("status")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	orders, total, err := s.orderSvc.ListOrders(c.Request.Context(), userID.(int64), symbol, status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
		"total":  total,
	})
}

func (s *Server) GetAccount(c *gin.Context) {
	userID, _ := c.Get("user_id")

	account, err := s.accountSvc.GetAccount(c.Request.Context(), userID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, account)
}

func (s *Server) GetPositions(c *gin.Context) {
	userID, _ := c.Get("user_id")

	positions, err := s.accountSvc.GetPositions(c.Request.Context(), userID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"positions": positions})
}

func (s *Server) GetTrades(c *gin.Context) {
	userID, _ := c.Get("user_id")
	symbol := c.Query("symbol")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	trades, total, err := s.orderSvc.GetTrades(c.Request.Context(), userID.(int64), symbol, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trades": trades,
		"total":  total,
	})
}

func (s *Server) GetQuote(c *gin.Context) {
	symbol := c.Param("symbol")

	quote, err := s.marketSvc.GetQuote(c.Request.Context(), symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, quote)
}

func (s *Server) GetKLine(c *gin.Context) {
	symbol := c.Param("symbol")
	period := c.DefaultQuery("period", "1d")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	klines, err := s.marketSvc.GetKLine(c.Request.Context(), symbol, period, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"klines": klines})
}

func (s *Server) GetReconciliationReport(c *gin.Context) {
	startTime, _ := time.Parse("2006-01-02", c.Query("start_date"))
	endTime, _ := time.Parse("2006-01-02", c.Query("end_date"))

	reports, err := s.reconSvc.GetReports(c.Request.Context(), startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reports": reports})
}

func (s *Server) GetReconciliationIssues(c *gin.Context) {
	reportID := c.Query("report_id")
	status := c.Query("status")

	issues, err := s.reconSvc.GetIssues(c.Request.Context(), reportID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"issues": issues})
}

func (s *Server) ExportOrders(c *gin.Context) {
	userID, _ := c.Get("user_id")
	startTime, _ := time.Parse("2006-01-02", c.DefaultQuery("start_date", "2024-01-01"))
	endTime, _ := time.Parse("2006-01-02", c.DefaultQuery("end_date", "2024-12-31"))

	orders, _, err := s.orderSvc.ListOrders(c.Request.Context(), userID.(int64), "", "", 10000, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("orders_%d_%s.csv", userID.(int64), time.Now().Format("20060102"))
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{"订单号", "股票代码", "股票名称", "订单类型", "方向", "价格", "数量", "成交数量", "状态", "创建时间", "成交时间"})
	for _, order := range orders {
		writer.Write([]string{
			order.OrderID,
			order.Symbol,
			order.SymbolName,
			order.OrderType,
			order.Side,
			fmt.Sprintf("%.2f", order.Price),
			fmt.Sprintf("%d", order.Quantity),
			fmt.Sprintf("%d", order.FilledQuantity),
			order.Status,
			order.CreatedAt.Format("2006-01-02 15:04:05"),
			order.FilledAt.Format("2006-01-02 15:04:05"),
		})
	}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte("stock-trader-secret-key"), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		userID, ok := claims["user_id"].(float64)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user_id in token"})
			c.Abort()
			return
		}

		c.Set("user_id", int64(userID))
		c.Next()
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var req struct {
			Symbol   string `json:"symbol"`
			Action   string `json:"action"`
		}
		if err := json.Unmarshal(message, &req); err != nil {
			continue
		}

		if req.Action == "subscribe" {
			go s.subscribeQuote(conn, req.Symbol)
		}
	}
}

func (s *Server) subscribeQuote(conn *websocket.Conn, symbol string) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			quote, err := s.marketSvc.GetQuote(context.Background(), symbol)
			if err != nil {
				continue
			}

			data, _ := json.Marshal(quote)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}
}

func (s *Server) mustEmbedUnimplementedUserServer() {}
func (s *Server) mustEmbedUnimplementedAccountServer() {}
func (s *Server) mustEmbedUnimplementedOrderServer() {}
func (s *Server) mustEmbedUnimplementedTradeServer() {}
func (s *Server) mustEmbedUnimplementedMarketServer() {}
