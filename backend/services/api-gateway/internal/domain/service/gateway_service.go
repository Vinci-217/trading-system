package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"stock_trader/backend/services/api-gateway/internal/domain/entity"
	"stock_trader/backend/services/api-gateway/internal/infrastructure/logger"

	"github.com/google/uuid"
	"github.com/sony/gobreaker"
)

type GatewayDomainService struct {
	routes          map[string]*entity.GatewayRoute
	circuitBreakers map[string]*gobreaker.CircuitBreaker
	rateLimiters    map[string]*RateLimiter
	logger          *logger.Logger
	mu              sync.RWMutex
}

type RateLimiter struct {
	counts  map[string][]time.Time
	window  time.Duration
	limit   int
	mu      sync.RWMutex
}

func NewRateLimiter(window time.Duration, limit int) *RateLimiter {
	return &RateLimiter{
		counts: make(map[string][]time.Time),
		window: window,
		limit:  limit,
	}
}

func (rl *RateLimiter) Allow(key string) (bool, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	requests := rl.counts[key]

	var valid []time.Time
	for _, t := range requests {
		if now.Sub(t) < rl.window {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.counts[key] = valid
		return false, 0
	}

	valid = append(valid, now)
	rl.counts[key] = valid

	remaining := rl.limit - len(valid)
	return true, remaining
}

func NewGatewayDomainService(logger *logger.Logger) *GatewayDomainService {
	return &GatewayDomainService{
		routes:          make(map[string]*entity.GatewayRoute),
		circuitBreakers: make(map[string]*gobreaker.CircuitBreaker),
		rateLimiters:    make(map[string]*RateLimiter),
		logger:          logger,
	}
}

func (s *GatewayDomainService) Initialize() {
	s.initRoutes()
	s.initCircuitBreakers()
	s.initRateLimiters()

	s.logger.Info("API网关领域服务初始化完成",
		logger.Int("routes", len(s.routes)),
		logger.Int("circuit_breakers", len(s.circuitBreakers)))
}

func (s *GatewayDomainService) initRoutes() {
	routes := []*entity.GatewayRoute{
		{Path: "/api/v1/auth/register", Method: "POST", Service: "user-service", Endpoint: "/auth/register", Timeout: 10 * time.Second, AuthRequired: false},
		{Path: "/api/v1/auth/login", Method: "POST", Service: "user-service", Endpoint: "/auth/login", Timeout: 10 * time.Second, AuthRequired: false},
		{Path: "/api/v1/users/:user_id", Method: "GET", Service: "user-service", Endpoint: "/users/:user_id", Timeout: 10 * time.Second, AuthRequired: true},
		{Path: "/api/v1/users/:user_id/orders", Method: "GET", Service: "order-service", Endpoint: "/users/:user_id/orders", Timeout: 10 * time.Second, AuthRequired: true},
		{Path: "/api/v1/users/:user_id/positions", Method: "GET", Service: "account-service", Endpoint: "/users/:user_id/positions", Timeout: 10 * time.Second, AuthRequired: true},
		{Path: "/api/v1/orders", Method: "POST", Service: "order-service", Endpoint: "/orders", Timeout: 30 * time.Second, AuthRequired: true},
		{Path: "/api/v1/orders/:order_id", Method: "GET", Service: "order-service", Endpoint: "/orders/:order_id", Timeout: 10 * time.Second, AuthRequired: true},
		{Path: "/api/v1/orders/:order_id/cancel", Method: "POST", Service: "order-service", Endpoint: "/orders/:order_id/cancel", Timeout: 30 * time.Second, AuthRequired: true},
		{Path: "/api/v1/market/quotes/:symbol", Method: "GET", Service: "market-service", Endpoint: "/quotes/:symbol", Timeout: 10 * time.Second, AuthRequired: true},
		{Path: "/api/v1/market/kline/:symbol", Method: "GET", Service: "market-service", Endpoint: "/kline/:symbol", Timeout: 10 * time.Second, AuthRequired: true},
		{Path: "/api/v1/account/:user_id", Method: "GET", Service: "account-service", Endpoint: "/accounts/:user_id", Timeout: 10 * time.Second, AuthRequired: true},
		{Path: "/api/v1/account/deposit", Method: "POST", Service: "account-service", Endpoint: "/accounts/deposit", Timeout: 30 * time.Second, AuthRequired: true},
		{Path: "/api/v1/account/withdraw", Method: "POST", Service: "account-service", Endpoint: "/accounts/withdraw", Timeout: 30 * time.Second, AuthRequired: true},
		{Path: "/api/v1/reconciliation/funds", Method: "POST", Service: "reconciliation-service", Endpoint: "/reconcile/funds", Timeout: 30 * time.Second, AuthRequired: true},
		{Path: "/api/v1/reconciliation/positions", Method: "POST", Service: "reconciliation-service", Endpoint: "/reconcile/positions", Timeout: 30 * time.Second, AuthRequired: true},
	}

	for _, route := range routes {
		key := fmt.Sprintf("%s:%s", route.Method, route.Path)
		s.routes[key] = route
	}
}

func (s *GatewayDomainService) initCircuitBreakers() {
	services := []string{"user-service", "order-service", "account-service", "market-service", "reconciliation-service"}

	for _, service := range services {
		s.circuitBreakers[service] = gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        service,
			MaxRequests: 5,
			Interval:    60 * time.Second,
			Timeout:     30 * time.Second,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
				return counts.Requests >= 3 && failureRatio >= 0.6
			},
		})
	}
}

func (s *GatewayDomainService) initRateLimiters() {
	limiters := map[string]*RateLimiter{
		"default": NewRateLimiter(1*time.Minute, 100),
		"auth":    NewRateLimiter(1*time.Minute, 20),
	}

	for key, limiter := range limiters {
		s.rateLimiters[key] = limiter
	}
}

func (s *GatewayDomainService) GetRoute(method string, path string) (*entity.GatewayRoute, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", method, path)
	route, exists := s.routes[key]
	if !exists {
		return nil, entity.ErrRouteNotFound
	}

	return route, nil
}

func (s *GatewayDomainService) CheckRateLimit(category string, clientIP string) (bool, int) {
	key := fmt.Sprintf("%s:%s", category, clientIP)
	limiter, exists := s.rateLimiters[category]
	if !exists {
		limiter = s.rateLimiters["default"]
	}

	return limiter.Allow(key)
}

func (s *GatewayDomainService) CheckCircuitBreaker(serviceName string) error {
	breaker, exists := s.circuitBreakers[serviceName]
	if !exists {
		return nil
	}

	result, err := breaker.Execute(func() (interface{}, error) {
		return nil, nil
	})

	if err == gobreaker.ErrCircuitOpen {
		return entity.ErrCircuitOpen
	}

	return nil
}

func (s *GatewayDomainService) GenerateRequestID() string {
	return uuid.New().String()
}

func (s *GatewayDomainService) GetServiceEndpoint(serviceName string) *entity.ServiceEndpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	endpoints := map[string]*entity.ServiceEndpoint{
		"user-service":           {Name: "user-service", Host: "user-service", Port: 5001, Timeout: 10},
		"order-service":          {Name: "order-service", Host: "order-service", Port: 5002, Timeout: 10},
		"account-service":        {Name: "account-service", Host: "account-service", Port: 5004, Timeout: 10},
		"market-service":         {Name: "market-service", Host: "market-service", Port: 5003, Timeout: 10},
		"reconciliation-service": {Name: "reconciliation-service", Host: "reconciliation-service", Port: 5006, Timeout: 30},
	}

	return endpoints[serviceName]
}
