package errors

import (
	"errors"
	"fmt"
	"net/http"
)

type Code string

const (
	// 通用错误码
	CodeSuccess       Code = "SUCCESS"
	CodeInternalError Code = "INTERNAL_ERROR"
	CodeInvalidParam  Code = "INVALID_PARAM"
	CodeNotFound      Code = "NOT_FOUND"
	CodeConflict      Code = "CONFLICT"
	CodeUnauthorized  Code = "UNAUTHORIZED"
	CodeForbidden     Code = "FORBIDDEN"
	CodeTooManyReq    Code = "TOO_MANY_REQUESTS"

	// 业务错误码
	CodeUserAlreadyExists    Code = "USER_ALREADY_EXISTS"
	CodeUserNotFound         Code = "USER_NOT_FOUND"
	CodeInvalidPassword      Code = "INVALID_PASSWORD"
	CodeAccountNotFound      Code = "ACCOUNT_NOT_FOUND"
	CodeInsufficientBalance  Code = "INSUFFICIENT_BALANCE"
	CodeInsufficientShares   Code = "INSUFFICIENT_SHARES"
	CodeOrderNotFound        Code = "ORDER_NOT_FOUND"
	CodeOrderAlreadyFilled   Code = "ORDER_ALREADY_FILLED"
	CodeOrderCannotCancel    Code = "ORDER_CANNOT_CANCEL"
	CodeInvalidOrderStatus   Code = "INVALID_ORDER_STATUS"
	CodeInvalidPrice         Code = "INVALID_PRICE"
	CodeInvalidQuantity      Code = "INVALID_QUANTITY"
	CodeStockSuspended       Code = "STOCK_SUSPENDED"
	CodeStockDelisted        Code = "STOCK_DELISTED"
	CodePriceLimitExceeded   Code = "PRICE_LIMIT_EXCEEDED"
	CodeRiskControlBlocked   Code = "RISK_CONTROL_BLOCKED"
	CodeDuplicateOrder       Code = "DUPLICATE_ORDER"
	CodeConcurrentModify     Code = "CONCURRENT_MODIFY"
	CodeTCCFailed            Code = "TCC_FAILED"
	CodeReconciliationIssue  Code = "RECONCILIATION_ISSUE"
)

type AppError struct {
	Code    Code     `json:"code"`
	Message string   `json:"message"`
	Details string   `json:"details,omitempty"`
	Causes  []string `json:"causes,omitempty"`
	Status  int      `json:"-"`
}

func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return errors.New(e.Message)
}

func NewAppError(code Code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  http.StatusInternalServerError,
	}
}

func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

func (e *AppError) WithCauses(causes ...string) *AppError {
	e.Causes = causes
	return e
}

func (e *AppError) WithStatus(status int) *AppError {
	e.Status = status
	return e
}

// 预定义错误
var (
	ErrSuccess            = NewAppError(CodeSuccess, "操作成功").WithStatus(http.StatusOK)
	ErrInternalError      = NewAppError(CodeInternalError, "内部错误")
	ErrInvalidParam       = NewAppError(CodeInvalidParam, "参数错误").WithStatus(http.StatusBadRequest)
	ErrNotFound           = NewAppError(CodeNotFound, "资源不存在").WithStatus(http.StatusNotFound)
	ErrConflict           = NewAppError(CodeConflict, "资源冲突").WithStatus(http.StatusConflict)
	ErrUnauthorized       = NewAppError(CodeUnauthorized, "未授权").WithStatus(http.StatusUnauthorized)
	ErrForbidden          = NewAppError(CodeForbidden, "禁止访问").WithStatus(http.StatusForbidden)
	ErrTooManyRequests    = NewAppError(CodeTooManyReq, "请求过于频繁").WithStatus(http.StatusTooManyRequests)

	ErrUserAlreadyExists  = NewAppError(CodeUserAlreadyExists, "用户已存在").WithStatus(http.StatusConflict)
	ErrUserNotFound       = NewAppError(CodeUserNotFound, "用户不存在").WithStatus(http.StatusNotFound)
	ErrInvalidPassword    = NewAppError(CodeInvalidPassword, "密码错误").WithStatus(http.StatusUnauthorized)
	ErrAccountNotFound    = NewAppError(CodeAccountNotFound, "账户不存在").WithStatus(http.StatusNotFound)

	ErrInsufficientBalance = NewAppError(CodeInsufficientBalance, "可用余额不足").WithStatus(http.StatusBadRequest)
	ErrInsufficientShares  = NewAppError(CodeInsufficientShares, "可用股份不足").WithStatus(http.StatusBadRequest)

	ErrOrderNotFound       = NewAppError(CodeOrderNotFound, "订单不存在").WithStatus(http.StatusNotFound)
	ErrOrderAlreadyFilled  = NewAppError(CodeOrderAlreadyFilled, "订单已完全成交").WithStatus(http.StatusBadRequest)
	ErrOrderCannotCancel   = NewAppError(CodeOrderCannotCancel, "订单无法取消").WithStatus(http.StatusBadRequest)
	ErrInvalidOrderStatus  = NewAppError(CodeInvalidOrderStatus, "无效的订单状态").WithStatus(http.StatusBadRequest)
	ErrDuplicateOrder      = NewAppError(CodeDuplicateOrder, "重复订单").WithStatus(http.StatusConflict)

	ErrInvalidPrice        = NewAppError(CodeInvalidPrice, "无效价格").WithStatus(http.StatusBadRequest)
	ErrInvalidQuantity     = NewAppError(CodeInvalidQuantity, "无效数量").WithStatus(http.StatusBadRequest)

	ErrStockSuspended      = NewAppError(CodeStockSuspended, "股票已停牌").WithStatus(http.StatusBadRequest)
	ErrStockDelisted       = NewAppError(CodeStockDelisted, "股票已退市").WithStatus(http.StatusBadRequest)
	ErrPriceLimitExceeded  = NewAppError(CodePriceLimitExceeded, "价格超出涨跌停范围").WithStatus(http.StatusBadRequest)
	ErrRiskControlBlocked  = NewAppError(CodeRiskControlBlocked, "风控拦截").WithStatus(http.StatusForbidden)

	ErrConcurrentModify    = NewAppError(CodeConcurrentModify, "并发修改冲突").WithStatus(http.StatusConflict)
	ErrTCCFailed           = NewAppError(CodeTCCFailed, "分布式事务失败").WithStatus(http.StatusInternalServerError)
)

// WrapError 包装错误
func WrapError(err error, code Code, message string) *AppError {
	if err == nil {
		return nil
	}
	appErr, ok := err.(*AppError)
	if ok {
		return appErr
	}
	return NewAppError(code, message).WithDetails(err.Error())
}

// IsCode 判断是否是指定错误码
func IsCode(err error, code Code) bool {
	if err == nil {
		return false
	}
	appErr, ok := err.(*AppError)
	return ok && appErr.Code == code
}
