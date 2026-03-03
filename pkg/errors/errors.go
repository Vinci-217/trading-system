package errors

import "fmt"

type Code int

const (
	CodeSuccess         Code = 0
	CodeUnknownError    Code = 10000
	CodeInvalidParam    Code = 10001
	CodeUnauthorized    Code = 10002
	CodeForbidden       Code = 10003
	CodeNotFound        Code = 10004
	CodeRateLimitExceed Code = 10005
	
	CodeAccountNotFound      Code = 20001
	CodeInsufficientBalance  Code = 20002
	CodeInsufficientPosition Code = 20003
	CodeAccountExists        Code = 20004
	CodeFundFreezeFailed     Code = 20005
	
	CodeOrderNotFound      Code = 30001
	CodeOrderInvalidStatus Code = 30002
	CodeOrderCannotCancel  Code = 30003
	CodeDuplicateOrder     Code = 30004
	CodeOrderProcessing    Code = 30005
	
	CodeTradeNotFound Code = 40001
	
	CodeMatchingFailed Code = 50001
	
	CodeSettlementFailed Code = 60001
	
	CodeDiscrepancyNotFound Code = 70001
)

type Error struct {
	Code    Code
	Message string
}

func NewError(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func (e *Error) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

var (
	ErrInvalidParam       = NewError(CodeInvalidParam, "参数错误")
	ErrUnauthorized       = NewError(CodeUnauthorized, "未授权")
	ErrForbidden          = NewError(CodeForbidden, "禁止访问")
	ErrNotFound           = NewError(CodeNotFound, "资源不存在")
	ErrRateLimitExceed    = NewError(CodeRateLimitExceed, "请求频率超限")
	
	ErrAccountNotFound      = NewError(CodeAccountNotFound, "账户不存在")
	ErrInsufficientBalance  = NewError(CodeInsufficientBalance, "余额不足")
	ErrInsufficientPosition = NewError(CodeInsufficientPosition, "持仓不足")
	ErrAccountExists        = NewError(CodeAccountExists, "账户已存在")
	ErrFundFreezeFailed     = NewError(CodeFundFreezeFailed, "资金冻结失败")
	
	ErrOrderNotFound      = NewError(CodeOrderNotFound, "订单不存在")
	ErrOrderInvalidStatus = NewError(CodeOrderInvalidStatus, "订单状态无效")
	ErrOrderCannotCancel  = NewError(CodeOrderCannotCancel, "订单无法取消")
	ErrDuplicateOrder     = NewError(CodeDuplicateOrder, "重复订单")
	ErrOrderProcessing    = NewError(CodeOrderProcessing, "订单处理中")
	
	ErrTradeNotFound = NewError(CodeTradeNotFound, "成交记录不存在")
	
	ErrMatchingFailed = NewError(CodeMatchingFailed, "撮合失败")
	
	ErrSettlementFailed = NewError(CodeSettlementFailed, "结算失败")
	
	ErrDiscrepancyNotFound = NewError(CodeDiscrepancyNotFound, "差异记录不存在")
)
