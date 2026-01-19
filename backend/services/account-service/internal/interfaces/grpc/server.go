package grpc

import (
	"context"

	"stock_trader/backend/services/account-service/internal/domain/entity"
	"stock_trader/backend/services/account-service/internal/domain/service"
	"stock_trader/backend/services/account-service/internal/infrastructure/logger"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	domainService *service.AccountDomainService
	logger        *logger.Logger
}

func NewGRPCServer(domainService *service.AccountDomainService, logger *logger.Logger) *GRPCServer {
	return &GRPCServer{
		domainService: domainService,
		logger:        logger,
	}
}

func (s *GRPCServer) CreateAccount(ctx context.Context, req *CreateAccountRequest) (*CreateAccountResponse, error) {
	account, err := s.domainService.CreateAccount(ctx, req.UserID)
	if err != nil {
		s.logger.Error("创建账户失败", logger.Error(err))
		return &CreateAccountResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &CreateAccountResponse{
		Success: true,
		Message: "账户创建成功",
		Account: &AccountProto{
			UserID: account.UserID,
		},
	}, nil
}

func (s *GRPCServer) GetAccount(ctx context.Context, req *GetAccountRequest) (*GetAccountResponse, error) {
	account, err := s.domainService.GetAccount(ctx, req.UserID)
	if err != nil {
		s.logger.Error("获取账户失败", logger.Error(err))
		return &GetAccountResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &GetAccountResponse{
		Success: true,
		Account: s.toAccountProto(account),
	}, nil
}

func (s *GRPCServer) Deposit(ctx context.Context, req *DepositRequest) (*DepositResponse, error) {
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "无效金额")
	}

	account, err := s.domainService.Deposit(ctx, req.UserID, amount)
	if err != nil {
		s.logger.Error("存款失败", logger.Error(err))
		return &DepositResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &DepositResponse{
		Success:    true,
		Message:    "存款成功",
		NewBalance: account.CashBalance.String(),
	}, nil
}

func (s *GRPCServer) Withdraw(ctx context.Context, req *WithdrawRequest) (*WithdrawResponse, error) {
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "无效金额")
	}

	account, err := s.domainService.Withdraw(ctx, req.UserID, amount)
	if err != nil {
		s.logger.Error("取款失败", logger.Error(err))
		return &WithdrawResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &WithdrawResponse{
		Success:    true,
		Message:    "取款成功",
		NewBalance: account.CashBalance.String(),
	}, nil
}

func (s *GRPCServer) LockFunds(ctx context.Context, req *LockFundsRequest) (*LockFundsResponse, error) {
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "无效金额")
	}

	fundLock, tccTx, err := s.domainService.LockFunds(ctx, req.UserID, req.OrderID, req.TransactionID, amount, 5*60*1000000000)
	if err != nil {
		s.logger.Error("锁定资金失败", logger.Error(err))
		return &LockFundsResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &LockFundsResponse{
		Success:      true,
		Message:      "资金锁定成功",
		LockID:       fundLock.ID,
		TransactionID: tccTx.ID,
		ExpiresAt:    fundLock.ExpiresAt.UnixMilli(),
	}, nil
}

func (s *GRPCServer) UnlockFunds(ctx context.Context, req *UnlockFundsRequest) (*UnlockFundsResponse, error) {
	amount, _ := decimal.NewFromString("0")

	if err := s.domainService.UnlockFunds(ctx, req.UserID, req.OrderID, amount); err != nil {
		s.logger.Error("解锁资金失败", logger.Error(err))
		return &UnlockFundsResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &UnlockFundsResponse{
		Success: true,
		Message: "资金解锁成功",
	}, nil
}

func (s *GRPCServer) ConfirmLock(ctx context.Context, req *ConfirmLockRequest) (*ConfirmLockResponse, error) {
	amount, _ := decimal.NewFromString("0")

	if err := s.domainService.ConfirmLock(ctx, req.UserID, amount); err != nil {
		s.logger.Error("确认锁定失败", logger.Error(err))
		return &ConfirmLockResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &ConfirmLockResponse{
		Success: true,
		Message: "锁定确认成功",
	}, nil
}

func (s *GRPCServer) SettleTrade(ctx context.Context, req *SettleTradeRequest) (*SettleTradeResponse, error) {
	price, err := decimal.NewFromString(req.Price)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "无效价格")
	}

	_, err = s.domainService.SettleTrade(ctx, req.UserID, req.Symbol, req.Side, int(req.Quantity), price)
	if err != nil {
		s.logger.Error("交易结算失败", logger.Error(err))
		return &SettleTradeResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &SettleTradeResponse{
		Success: true,
		Message: "交易结算成功",
	}, nil
}

func (s *GRPCServer) GetPositions(ctx context.Context, req *GetPositionsRequest) (*GetPositionsResponse, error) {
	positions, err := s.domainService.GetPositions(ctx, req.UserID)
	if err != nil {
		s.logger.Error("获取持仓列表失败", logger.Error(err))
		return &GetPositionsResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	protos := make([]*PositionProto, len(positions))
	for i, pos := range positions {
		protos[i] = &PositionProto{
			Symbol:     pos.Symbol,
			Quantity:   int32(pos.Quantity),
			AvgCost:    pos.AvgCost.String(),
			MarketValue: pos.MarketValue.String(),
			ProfitLoss: pos.ProfitLoss.String(),
		}
	}

	return &GetPositionsResponse{
		Success:   true,
		Positions: protos,
	}, nil
}

func (s *GRPCServer) Reconcile(ctx context.Context, req *ReconcileRequest) (*ReconcileResponse, error) {
	discrepancies, err := s.domainService.Reconcile(ctx)
	if err != nil {
		s.logger.Error("对账失败", logger.Error(err))
		return &ReconcileResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &ReconcileResponse{
		Success:       true,
		Message:       "对账完成",
		TotalAccounts: int32(len(discrepancies)),
		Discrepancies: discrepancies,
	}, nil
}

func (s *GRPCServer) toAccountProto(account *entity.Account) *AccountProto {
	return &AccountProto{
		UserID:           account.UserID,
		CashBalance:      account.CashBalance.String(),
		FrozenBalance:    account.FrozenBalance.String(),
		AvailableBalance: account.AvailableBalance().String(),
		TotalAssets:      account.TotalAssets.String(),
	}
}
