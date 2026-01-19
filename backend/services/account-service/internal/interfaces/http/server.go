package http

import (
	"net/http"
	"strconv"

	"stock_trader/backend/services/account-service/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type HTTPServer struct {
	domainService *service.AccountDomainService
}

func NewHTTPServer(domainService *service.AccountDomainService) *HTTPServer {
	return &HTTPServer{
		domainService: domainService,
	}
}

func (s *HTTPServer) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		accounts := api.Group("/accounts")
		{
			accounts.GET("/:user_id", s.getAccount)
			accounts.GET("/:user_id/positions", s.getPositions)
			accounts.POST("/deposit", s.deposit)
			accounts.POST("/withdraw", s.withdraw)
			accounts.POST("/reconcile", s.reconcile)
		}
	}
}

func (s *HTTPServer) getAccount(c *gin.Context) {
	userID := c.Param("user_id")

	account, err := s.domainService.GetAccount(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"account": gin.H{
			"user_id":            account.UserID,
			"cash_balance":       account.CashBalance.String(),
			"frozen_balance":     account.FrozenBalance.String(),
			"available_balance":  account.AvailableBalance().String(),
			"total_assets":       account.TotalAssets.String(),
			"total_deposit":      account.TotalDeposit.String(),
			"total_withdrawal":   account.TotalWithdrawal.String(),
		},
	})
}

func (s *HTTPServer) getPositions(c *gin.Context) {
	userID := c.Param("user_id")

	positions, err := s.domainService.GetPositions(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	result := make([]gin.H, len(positions))
	for i, pos := range positions {
		result[i] = gin.H{
			"symbol":       pos.Symbol,
			"quantity":     pos.Quantity,
			"frozen_quantity": pos.FrozenQuantity,
			"avg_cost":     pos.AvgCost.String(),
			"market_value": pos.MarketValue.String(),
			"profit_loss":  pos.ProfitLoss.String(),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"positions": result,
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

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效金额",
		})
		return
	}

	account, err := s.domainService.Deposit(c.Request.Context(), req.UserID, amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "存款成功",
		"new_balance": account.CashBalance.String(),
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

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效金额",
		})
		return
	}

	account, err := s.domainService.Withdraw(c.Request.Context(), req.UserID, amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "取款成功",
		"new_balance": account.CashBalance.String(),
	})
}

func (s *HTTPServer) reconcile(c *gin.Context) {
	discrepancies, err := s.domainService.Reconcile(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        "对账完成",
		"total_accounts": len(discrepancies),
		"discrepancies":  discrepancies,
	})
}

type AccountRequest struct {
	UserID string `json:"user_id"`
}

type AccountResponse struct {
	Success bool       `json:"success"`
	Account *AccountInfo `json:"account,omitempty"`
	Message string     `json:"message,omitempty"`
}

type AccountInfo struct {
	UserID           string `json:"user_id"`
	CashBalance      string `json:"cash_balance"`
	FrozenBalance    string `json:"frozen_balance"`
	AvailableBalance string `json:"available_balance"`
	TotalAssets      string `json:"total_assets"`
}

type DepositRequest struct {
	UserID string `json:"user_id"`
	Amount string `json:"amount"`
}

type DepositResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	NewBalance  string `json:"new_balance"`
}

type WithdrawRequest struct {
	UserID string `json:"user_id"`
	Amount string `json:"amount"`
}

type WithdrawResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	NewBalance  string `json:"new_balance"`
}

type Position struct {
	Symbol         string `json:"symbol"`
	Quantity       int    `json:"quantity"`
	FrozenQuantity int    `json:"frozen_quantity"`
	AvgCost        string `json:"avg_cost"`
	MarketValue    string `json:"market_value"`
	ProfitLoss     string `json:"profit_loss"`
}

type GetPositionsResponse struct {
	Success   bool       `json:"success"`
	Positions []Position `json:"positions"`
	Message   string     `json:"message,omitempty"`
}

type ReconcileResponse struct {
	Success        bool     `json:"success"`
	TotalAccounts  int      `json:"total_accounts"`
	Discrepancies  []string `json:"discrepancies"`
	Message        string   `json:"message,omitempty"`
}

func strconvParseInt(s string, bitSize int) (int64, error) {
	val, err := strconv.ParseInt(s, 10, bitSize)
	if err != nil {
		return 0, err
	}
	return val, nil
}
