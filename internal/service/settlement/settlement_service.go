package settlement

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stock-trading-system/internal/domain/trade"
	"github.com/stock-trading-system/internal/infrastructure/idgen"
	"github.com/stock-trading-system/internal/infrastructure/mq"
	"github.com/stock-trading-system/internal/service/account"
	"github.com/stock-trading-system/pkg/logger"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type SettlementService struct {
	db             *gorm.DB
	idgen          *idgen.IDGenerator
	kafkaProducer  *mq.KafkaProducer
	accountService *account.AccountService
	logger         *logger.Logger
}

func NewSettlementService(
	db *gorm.DB,
	idgen *idgen.IDGenerator,
	kafkaProducer *mq.KafkaProducer,
	accountService *account.AccountService,
	logger *logger.Logger,
) *SettlementService {
	return &SettlementService{
		db:             db,
		idgen:          idgen,
		kafkaProducer:  kafkaProducer,
		accountService: accountService,
		logger:         logger,
	}
}

type SettlementRequest struct {
	TradeID     string
	BuyOrderID  string
	SellOrderID string
	BuyUserID   string
	SellUserID  string
	Symbol      string
	Price       decimal.Decimal
	Quantity    int
}

type SettlementResult struct {
	TradeID               string
	BuyerFundDeducted     bool
	SellerFundCredited    bool
	BuyerPositionAdded    bool
	SellerPositionReduced bool
	BuyerNewBalance       decimal.Decimal
	SellerNewBalance      decimal.Decimal
}

func (s *SettlementService) SettleTrade(ctx context.Context, req *SettlementRequest) (*SettlementResult, error) {
	result := &SettlementResult{
		TradeID: req.TradeID,
	}

	amount := req.Price.Mul(decimal.NewFromInt(int64(req.Quantity)))

	steps := []trade.SettlementStep{
		{Name: "DeductBuyerFunds", Status: trade.StepStatusPending},
		{Name: "CreditSellerFunds", Status: trade.StepStatusPending},
		{Name: "AddBuyerPosition", Status: trade.StepStatusPending},
		{Name: "ReduceSellerPosition", Status: trade.StepStatusPending},
	}

	completedSteps := make([]int, 0)

	for i, step := range steps {
		s.logger.Info("executing settlement step",
			"trade_id", req.TradeID,
			"step", step.Name)

		var err error
		switch step.Name {
		case "DeductBuyerFunds":
			err = s.accountService.ConfirmFreeze(ctx, req.BuyUserID, req.BuyOrderID, amount)
			if err == nil {
				result.BuyerFundDeducted = true
			}
		case "CreditSellerFunds":
			err = s.accountService.CreditFunds(ctx, req.SellUserID, req.SellOrderID, amount)
			if err == nil {
				result.SellerFundCredited = true
			}
		case "AddBuyerPosition":
			err = s.accountService.AddPosition(ctx, req.BuyUserID, req.Symbol, req.Quantity, req.Price)
			if err == nil {
				result.BuyerPositionAdded = true
			}
		case "ReduceSellerPosition":
			err = s.accountService.ReducePosition(ctx, req.SellUserID, req.Symbol, req.Quantity)
			if err == nil {
				result.SellerPositionReduced = true
			}
		}

		if err != nil {
			s.logger.Errorw("settlement step failed",
				"trade_id", req.TradeID,
				"step", step.Name,
				"error", err)

			steps[i].Status = trade.StepStatusFailed
			steps[i].ErrorMessage = err.Error()

			s.compensate(ctx, req, completedSteps, steps)

			return result, fmt.Errorf("settlement step %s failed: %w", step.Name, err)
		}

		steps[i].Status = trade.StepStatusSuccess
		completedSteps = append(completedSteps, i)
	}

	stepsJSON, _ := json.Marshal(steps)
	settlement := &trade.Settlement{
		TradeID:     req.TradeID,
		BuyOrderID:  req.BuyOrderID,
		SellOrderID: req.SellOrderID,
		BuyUserID:   req.BuyUserID,
		SellUserID:  req.SellUserID,
		Symbol:      req.Symbol,
		Price:       req.Price.String(),
		Quantity:    req.Quantity,
		Status:      trade.SettlementStatusSuccess,
		Steps:       string(stepsJSON),
	}

	if err := s.db.WithContext(ctx).Create(settlement).Error; err != nil {
		s.logger.Errorw("failed to save settlement", "error", err)
	}

	s.kafkaProducer.SendMessage(ctx, "trade.settled", req.TradeID, map[string]interface{}{
		"trade_id":     req.TradeID,
		"buy_user_id":  req.BuyUserID,
		"sell_user_id": req.SellUserID,
		"symbol":       req.Symbol,
		"price":        req.Price.String(),
		"quantity":     req.Quantity,
	})

	return result, nil
}

func (s *SettlementService) compensate(ctx context.Context, req *SettlementRequest, completedSteps []int, steps []trade.SettlementStep) {
	s.logger.Warn("starting compensation",
		"trade_id", req.TradeID,
		"completed_steps", len(completedSteps))

	for i := len(completedSteps) - 1; i >= 0; i-- {
		stepIdx := completedSteps[i]
		stepName := steps[stepIdx].Name

		var err error
		switch stepName {
		case "DeductBuyerFunds":
			amount := req.Price.Mul(decimal.NewFromInt(int64(req.Quantity)))
			err = s.accountService.CreditFunds(ctx, req.BuyUserID, req.BuyOrderID, amount)
		case "CreditSellerFunds":
			amount := req.Price.Mul(decimal.NewFromInt(int64(req.Quantity)))
			err = s.accountService.ConfirmFreeze(ctx, req.SellUserID, req.SellOrderID, amount)
		case "AddBuyerPosition":
			err = s.accountService.ReducePosition(ctx, req.BuyUserID, req.Symbol, req.Quantity)
		case "ReduceSellerPosition":
			err = s.accountService.AddPosition(ctx, req.SellUserID, req.Symbol, req.Quantity, req.Price)
		}

		if err != nil {
			s.logger.Errorw("compensation step failed",
				"trade_id", req.TradeID,
				"step", stepName,
				"error", err)
			steps[stepIdx].Status = trade.StepStatusCompensate
			steps[stepIdx].ErrorMessage = "compensation failed: " + err.Error()
		} else {
			steps[stepIdx].Status = trade.StepStatusCompensate
			s.logger.Info("compensation step completed",
				"trade_id", req.TradeID,
				"step", stepName)
		}
	}
}

func (s *SettlementService) GetSettlementStatus(ctx context.Context, tradeID string) (*trade.Settlement, error) {
	var settlement trade.Settlement
	err := s.db.WithContext(ctx).Where("trade_id = ?", tradeID).First(&settlement).Error
	if err != nil {
		return nil, err
	}
	return &settlement, nil
}
