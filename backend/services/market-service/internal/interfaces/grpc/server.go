package grpc

import (
	"context"

	"stock_trader/backend/services/market-service/internal/domain/entity"
	"stock_trader/backend/services/market-service/internal/domain/service"
	"stock_trader/backend/services/market-service/internal/infrastructure/logger"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	domainService *service.MarketDomainService
	logger        *logger.Logger
}

func NewGRPCServer(domainService *service.MarketDomainService, logger *logger.Logger) *GRPCServer {
	return &GRPCServer{
		domainService: domainService,
		logger:        logger,
	}
}

func (s *GRPCServer) GetQuote(ctx context.Context, req *QuoteRequest) (*QuoteResponse, error) {
	quote, err := s.domainService.GetQuote(ctx, req.Symbol)
	if err != nil {
		s.logger.Error("获取行情失败", logger.Error(err))
		return &QuoteResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &QuoteResponse{
		Success:   true,
		Symbol:    quote.Symbol,
		Price:     quote.Price.String(),
		Change:    quote.Change.String(),
		ChangePct: quote.ChangePct.String(),
		Open:      quote.Open.String(),
		High:      quote.High.String(),
		Low:       quote.Low.String(),
		Close:     quote.Close.String(),
		Volume:    quote.Volume,
		Amount:    quote.Amount.String(),
		BidPrice:  quote.BidPrice.String(),
		AskPrice:  quote.AskPrice.String(),
		BidVolume: quote.BidVolume,
		AskVolume: quote.AskVolume,
		Timestamp:  quote.Timestamp.UnixMilli(),
	}, nil
}

func (s *GRPCServer) GetBatchQuotes(ctx context.Context, req *BatchQuoteRequest) (*BatchQuoteResponse, error) {
	quotes, err := s.domainService.GetBatchQuotes(ctx, req.Symbols)
	if err != nil {
		s.logger.Error("批量获取行情失败", logger.Error(err))
		return &BatchQuoteResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	protos := make([]*QuoteProto, len(quotes))
	for i, quote := range quotes {
		protos[i] = &QuoteProto{
			Symbol:    quote.Symbol,
			Price:     quote.Price.String(),
			Change:    quote.Change.String(),
			ChangePct: quote.ChangePct.String(),
			Volume:    quote.Volume,
			Timestamp: quote.Timestamp.UnixMilli(),
		}
	}

	return &BatchQuoteResponse{
		Success: true,
		Quotes:  protos,
	}, nil
}

func (s *GRPCServer) GetKLine(ctx context.Context, req *KLineRequest) (*KLineResponse, error) {
	kline, err := s.domainService.GetKLine(ctx, req.Symbol, req.Interval)
	if err != nil {
		s.logger.Error("获取K线失败", logger.Error(err))
		return &KLineResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &KLineResponse{
		Success:  true,
		Symbol:   kline.Symbol,
		Interval: kline.Interval,
		Data: &KLineData{
			Timestamp: kline.StartTime.UnixMilli(),
			Open:      kline.Open.String(),
			High:      kline.High.String(),
			Low:       kline.Low.String(),
			Close:     kline.Close.String(),
			Volume:    kline.Volume,
			Amount:    kline.Amount.String(),
		},
	}, nil
}

func (s *GRPCServer) GetKLines(ctx context.Context, req *KLinesRequest) (*KLinesResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	klines, err := s.domainService.GetKLines(ctx, req.Symbol, req.Interval, limit)
	if err != nil {
		s.logger.Error("获取K线历史失败", logger.Error(err))
		return &KLinesResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	data := make([]*KLineData, len(klines))
	for i, kline := range klines {
		data[i] = &KLineData{
			Timestamp: kline.StartTime.UnixMilli(),
			Open:      kline.Open.String(),
			High:      kline.High.String(),
			Low:       kline.Low.String(),
			Close:     kline.Close.String(),
			Volume:    kline.Volume,
			Amount:    kline.Amount.String(),
		}
	}

	return &KLinesResponse{
		Success:  true,
		Symbol:   req.Symbol,
		Interval: req.Interval,
		Data:     data,
	}, nil
}

type QuoteRequest struct {
	Symbol string `json:"symbol"`
}

type QuoteResponse struct {
	Success    bool   `json:"success"`
	Symbol     string `json:"symbol"`
	Price      string `json:"price"`
	Change     string `json:"change"`
	ChangePct  string `json:"change_pct"`
	Open       string `json:"open"`
	High       string `json:"high"`
	Low        string `json:"low"`
	Close      string `json:"close"`
	Volume     int64  `json:"volume"`
	Amount     string `json:"amount"`
	BidPrice   string `json:"bid_price"`
	AskPrice   string `json:"ask_price"`
	BidVolume  int64  `json:"bid_volume"`
	AskVolume  int64  `json:"ask_volume"`
	Timestamp  int64  `json:"timestamp"`
	Message    string `json:"message,omitempty"`
}

type QuoteProto struct {
	Symbol    string `json:"symbol"`
	Price     string `json:"price"`
	Change    string `json:"change"`
	ChangePct string `json:"change_pct"`
	Volume    int64  `json:"volume"`
	Timestamp int64  `json:"timestamp"`
}

type BatchQuoteRequest struct {
	Symbols []string `json:"symbols"`
}

type BatchQuoteResponse struct {
	Success bool         `json:"success"`
	Quotes  []*QuoteProto `json:"quotes,omitempty"`
	Message string       `json:"message,omitempty"`
}

type KLineRequest struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`
}

type KLineResponse struct {
	Success  bool       `json:"success"`
	Symbol   string     `json:"symbol"`
	Interval string     `json:"interval"`
	Data     *KLineData `json:"data,omitempty"`
	Message  string     `json:"message,omitempty"`
}

type KLineData struct {
	Timestamp int64  `json:"timestamp"`
	Open      string `json:"open"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Close     string `json:"close"`
	Volume    int64  `json:"volume"`
	Amount    string `json:"amount"`
}

type KLinesRequest struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`
	Limit    int32  `json:"limit"`
}

type KLinesResponse struct {
	Success  bool         `json:"success"`
	Symbol   string       `json:"symbol"`
	Interval string       `json:"interval"`
	Data     []*KLineData `json:"data,omitempty"`
	Message  string       `json:"message,omitempty"`
}
