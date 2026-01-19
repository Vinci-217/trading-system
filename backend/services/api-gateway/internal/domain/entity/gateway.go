package entity

import (
	"time"
)

type GatewayRoute struct {
	Path        string            `json:"path"`
	Method      string            `json:"method"`
	Service     string            `json:"service"`
	Endpoint    string            `json:"endpoint"`
	Timeout     time.Duration     `json:"timeout"`
	RateLimit   int               `json:"rate_limit"`
	AuthRequired bool             `json:"auth_required"`
	Middleware  []string          `json:"middleware"`
}

type RateLimitRule struct {
	Window     time.Duration `json:"window"`
	MaxRequests int          `json:"max_requests"`
	BurstSize  int           `json:"burst_size"`
}

type CircuitBreakerConfig struct {
	Name         string        `json:"name"`
	MaxRequests  uint32        `json:"max_requests"`
	Interval     time.Duration `json:"interval"`
	Timeout      time.Duration `json:"timeout"`
	FailureRate  float64       `json:"failure_rate"`
}

type ServiceEndpoint struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Timeout int    `json:"timeout"`
}

type JWTClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type GatewayError struct {
	Code    string
	Message string
}

var (
	ErrRouteNotFound       = &GatewayError{Code: "ROUTE_NOT_FOUND", Message: "路由不存在"}
	ErrServiceUnavailable  = &GatewayError{Code: "SERVICE_UNAVAILABLE", Message: "服务不可用"}
	ErrRateLimitExceeded   = &GatewayError{Code: "RATE_LIMIT_EXCEEDED", Message: "超出频率限制"}
	ErrCircuitOpen         = &GatewayError{Code: "CIRCUIT_OPEN", Message: "熔断器开启"}
	ErrUnauthorized        = &GatewayError{Code: "UNAUTHORIZED", Message: "未授权访问"}
	ErrInvalidToken        = &GatewayError{Code: "INVALID_TOKEN", Message: "无效令牌"}
	ErrRequestTimeout      = &GatewayError{Code: "REQUEST_TIMEOUT", Message: "请求超时"}
)

func (e *GatewayError) Error() string {
	return e.Message
}
