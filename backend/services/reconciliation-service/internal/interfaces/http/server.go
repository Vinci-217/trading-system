package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"stock_trader/backend/services/reconciliation-service/internal/domain/service"
	"stock_trader/backend/services/reconciliation-service/internal/infrastructure/config"
	"stock_trader/backend/services/reconciliation-service/internal/infrastructure/logger"

	"github.com/gin-gonic/gin"
)

type HTTPServer struct {
	cfg           *config.Config
	logger        *logger.Logger
	domainService *service.ReconciliationDomainService
}

func NewHTTPServer(cfg *config.Config, logger *logger.Logger, domainService *service.ReconciliationDomainService) *HTTPServer {
	return &HTTPServer{
		cfg:           cfg,
		logger:        logger,
		domainService: domainService,
	}
}

func (s *HTTPServer) RegisterRoutes(router *gin.Engine) {
	router.Use(s.loggingMiddleware())
	router.Use(gin.Recovery())

	router.GET("/health", s.healthCheck)
	router.GET("/ready", s.readinessCheck)

	api := router.Group("/api/v1")
	{
		reconcile := api.Group("/reconcile")
		{
			reconcile.POST("/funds", s.reconcileFunds)
			reconcile.POST("/funds/all", s.reconcileAllFunds)
			reconcile.POST("/positions", s.reconcilePositions)
			reconcile.POST("/trades", s.reconcileTrades)
			reconcile.GET("/discrepancies", s.getDiscrepancies)
			reconcile.POST("/discrepancies/:id/fix", s.fixDiscrepancy)
			reconcile.POST("/full", s.runFullReconciliation)
		}
	}
}

func (s *HTTPServer) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		duration := time.Since(start)
		s.logger.Info("HTTP请求",
			logger.String("method", c.Request.Method),
			logger.String("path", path),
			logger.Int("status", c.Writer.Status()),
			logger.Duration("duration", duration))
	}
}

func (s *HTTPServer) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "reconciliation-service",
		"timestamp": time.Now().UnixMilli(),
	})
}

func (s *HTTPServer) readinessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
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

	result, err := s.domainService.ReconcileFunds(c.Request.Context(), req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"user_id":           result.UserID,
		"expected_balance":  result.ExpectedBalance.String(),
		"actual_balance":    result.ActualBalance.String(),
		"frozen_balance":    result.FrozenBalance.String(),
		"discrepancy":       result.Discrepancy.String(),
		"pending_orders":    result.PendingOrders,
		"pending_trades":    result.PendingTrades,
		"is_consistent":     result.IsConsistent,
	})
}

func (s *HTTPServer) reconcileAllFunds(c *gin.Context) {
	result, err := s.domainService.RunFullReconciliation(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"report_id":          result.ID,
		"checked_accounts":   result.CheckedAccounts,
		"discrepancies_found": result.DiscrepanciesFound,
		"duration":           result.EndTime.Sub(result.StartTime).String(),
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

	result, err := s.domainService.ReconcilePositions(c.Request.Context(), req.UserID, req.Symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"user_id":           result.UserID,
		"symbol":            result.Symbol,
		"expected_position": result.ExpectedPosition,
		"actual_position":   result.ActualPosition,
		"discrepancy":       result.Discrepancy,
		"pending_orders":    result.PendingOrders,
		"is_consistent":     result.IsConsistent,
	})
}

func (s *HTTPServer) reconcileTrades(c *gin.Context) {
	var req struct {
		UserID    string    `json:"user_id" binding:"required"`
		Symbol    string    `json:"symbol" binding:"required"`
		StartTime time.Time `json:"start_time"`
		EndTime   time.Time `json:"end_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	if req.StartTime.IsZero() {
		req.StartTime = time.Now().Add(-24 * time.Hour)
	}
	if req.EndTime.IsZero() {
		req.EndTime = time.Now()
	}

	result, err := s.domainService.ReconcileTrades(c.Request.Context(), req.UserID, req.Symbol, req.StartTime, req.EndTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"user_id":            result.UserID,
		"symbol":             result.Symbol,
		"trade_count":        result.TradeCount,
		"order_count":        result.OrderCount,
		"trade_volume":       result.TradeVolume,
		"order_volume":       result.OrderVolume,
		"volume_discrepancy": result.VolumeDiscrepancy,
		"amount_discrepancy": result.AmountDiscrepancy.String(),
		"is_consistent":      result.IsConsistent,
	})
}

func (s *HTTPServer) getDiscrepancies(c *gin.Context) {
	status := c.Query("status")
	discrepancyType := c.Query("type")
	limit := 100
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}

	discrepancies, err := s.domainService.GetDiscrepancies(c.Request.Context(), status, discrepancyType, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	result := make([]gin.H, len(discrepancies))
	for i, d := range discrepancies {
		result[i] = gin.H{
			"id":             d.ID,
			"type":           d.Type,
			"user_id":        d.UserID,
			"symbol":         d.Symbol,
			"expected_value": d.ExpectedValue.String(),
			"actual_value":   d.ActualValue.String(),
			"difference":     d.Difference.String(),
			"detected_at":    d.DetectedAt.UnixMilli(),
			"status":         d.Status,
			"resolution":     d.Resolution,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"discrepancies": result,
	})
}

func (s *HTTPServer) fixDiscrepancy(c *gin.Context) {
	discrepancyID := c.Param("id")

	var req struct {
		FixType    string `json:"fix_type" binding:"required"`
		Notes      string `json:"notes"`
		ExecutedBy string `json:"executed_by" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	fixRecord, err := s.domainService.FixDiscrepancy(c.Request.Context(), discrepancyID, req.FixType, req.Notes, req.ExecutedBy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "不一致已修复",
		"fix_id":    fixRecord.ID,
		"fix_type":  fixRecord.FixType,
		"executed_by": fixRecord.ExecutedBy,
		"executed_at": fixRecord.ExecutedAt.UnixMilli(),
	})
}

func (s *HTTPServer) runFullReconciliation(c *gin.Context) {
	report, err := s.domainService.RunFullReconciliation(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"report_id":          report.ID,
		"report_type":        report.ReportType,
		"scope":              report.Scope,
		"checked_accounts":   report.CheckedAccounts,
		"discrepancies_found": report.DiscrepanciesFound,
		"discrepancies_resolved": report.DiscrepanciesResolved,
		"status":             report.Status,
		"start_time":         report.StartTime.UnixMilli(),
		"end_time":           report.EndTime.UnixMilli(),
		"duration":           report.EndTime.Sub(report.StartTime).String(),
	})
}
