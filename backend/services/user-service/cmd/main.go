package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"stock_trader/user-service/internal/application/service"
	"stock_trader/user-service/internal/domain/entity"
	"stock_trader/user-service/internal/domain/repository"
	"stock_trader/user-service/internal/infrastructure/config"
	"stock_trader/user-service/internal/infrastructure/database"
	"stock_trader/user-service/internal/infrastructure/logger"
	"stock_trader/user-service/internal/infrastructure/security"
	"stock_trader/user-service/internal/interfaces/grpc"
	"stock_trader/user-service/internal/interfaces/http"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"google.golang.org/grpc"
)

type UserService struct {
	cfg              *config.Config
	grpcServer       *grpc.Server
	httpServer       *http.Server
	redis            *redis.Client
	userRepo         repository.UserRepository
	accountRepo      repository.AccountRepository
	authService      *security.AuthService
	userService      *service.UserApplicationService
	logger           *logger.Logger
	shutdownChan     chan struct{}
	wg               sync.WaitGroup
	cron             *cron.Cron
}

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	appLogger := logger.NewLogger(cfg.Logging.Level)

	appLogger.Info("启动用户服务",
		logger.String("grpc_port", fmt.Sprintf("%d", cfg.GRPCPort)),
		logger.String("http_port", fmt.Sprintf("%d", cfg.HTTPPort)))

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		appLogger.Error("Redis连接失败", logger.Error(err))
		os.Exit(1)
	}
	appLogger.Info("Redis连接成功")

	db, err := database.NewMySQL(cfg.Database)
	if err != nil {
		appLogger.Error("数据库连接失败", logger.Error(err))
		os.Exit(1)
	}
	defer db.Close()
	appLogger.Info("数据库连接成功")

	userRepo := repository.NewUserRepository(db)
	accountRepo := repository.NewAccountRepository(db)
	authService := security.NewAuthService(cfg.JWT.Secret, cfg.JWT.Expiration)
	userService := service.NewUserApplicationService(userRepo, accountRepo, authService, appLogger)

	svc := NewUserService(cfg, redisClient, userRepo, accountRepo, authService, userService, appLogger)

	if err := svc.Start(); err != nil {
		appLogger.Error("启动服务失败", logger.Error(err))
		os.Exit(1)
	}

	appLogger.Info("用户服务启动成功",
		logger.String("grpc", fmt.Sprintf(":%d", cfg.GRPCPort)),
		logger.String("http", fmt.Sprintf(":%d", cfg.HTTPPort)))

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	appLogger.Info("收到停止信号，正在优雅关闭...")
	svc.Stop()
	appLogger.Info("服务已停止")
}

func NewUserService(
	cfg *config.Config,
	redis *redis.Client,
	userRepo repository.UserRepository,
	accountRepo repository.AccountRepository,
	authService *security.AuthService,
	userService *service.UserApplicationService,
	appLogger *logger.Logger,
) *UserService {
	return &UserService{
		cfg:         cfg,
		redis:       redis,
		userRepo:    userRepo,
		accountRepo: accountRepo,
		authService: authService,
		userService: userService,
		logger:      appLogger,
		shutdownChan: make(chan struct{}),
	}
}

func (s *UserService) Start() error {
	if err := s.startGRPCServer(); err != nil {
		return fmt.Errorf("启动gRPC服务器失败: %w", err)
	}

	if err := s.startHTTPServer(); err != nil {
		return fmt.Errorf("启动HTTP服务器失败: %w", err)
	}

	s.startCronJobs()

	return nil
}

func (s *UserService) Stop() {
	close(s.shutdownChan)

	if s.cron != nil {
		s.cron.Stop()
	}

	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}

	s.wg.Wait()
	s.logger.Info("用户服务已停止")
}

func (s *UserService) startGRPCServer() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("监听gRPC端口失败: %w", err)
	}

	s.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor(s.logger)),
	)

	grpcServer := grpc.NewUserServiceServer(s.userService)
	RegisterUserServiceServer(s.grpcServer, grpcServer)

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Error("gRPC服务器错误", logger.Error(err))
		}
	}()

	s.logger.Info("gRPC服务器启动成功", logger.Int("port", s.cfg.GRPCPort))
	return nil
}

func (s *UserService) startHTTPServer() error {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginLoggingMiddleware(s.logger))

	router.GET("/health", healthCheck)
	router.GET("/ready", readinessCheck)

	v1 := router.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", s.handleRegister)
			auth.POST("/login", s.handleLogin)
		}

		users := v1.Group("/users")
		users.Use(jwtAuthMiddleware(s.authService, s.logger))
		{
			users.GET("/:user_id", s.handleGetUser)
			users.GET("/:user_id/accounts", s.handleGetUserAccounts)
		}
	}

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.HTTPPort),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		s.logger.Info("HTTP服务器启动成功", logger.Int("port", s.cfg.HTTPPort))
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP服务器错误", logger.Error(err))
		}
	}()

	return nil
}

func (s *UserService) startCronJobs() {
	s.cron = cron.New()
	
	s.cron.AddFunc("0 3 * * *", func() {
		s.logger.Info("执行每日清理任务")
		ctx := context.Background()
		s.userService.CleanupInactiveUsers(ctx)
	})

	s.cron.Start()
	s.logger.Info("定时任务已启动")
}

func (s *UserService) handleRegister(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.userService.Register(c.Request.Context(), req.Username, req.Password, req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (s *UserService) handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.userService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *UserService) handleGetUser(c *gin.Context) {
	userID := c.Param("user_id")

	user, err := s.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":    user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"status":     user.Status,
		"created_at": user.CreatedAt.UnixMilli(),
	})
}

func (s *UserService) handleGetUserAccounts(c *gin.Context) {
	userID := c.Param("user_id")

	accounts, err := s.accountRepo.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":  userID,
		"accounts": accounts,
	})
}

func loggingInterceptor(logger *logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		code := "OK"
		if err != nil {
			code = "ERROR"
		}

		logger.Info("gRPC请求",
			logger.String("method", info.FullMethod),
			logger.Duration("duration", duration),
			logger.String("code", code))

		return resp, err
	}
}

func ginLoggingMiddleware(logger *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		duration := time.Since(start)
		logger.Info("HTTP请求",
			logger.String("method", c.Request.Method),
			logger.String("path", path),
			logger.Int("status", c.Writer.Status()),
			logger.Duration("duration", duration))
	}
}

func jwtAuthMiddleware(authService *security.AuthService, logger *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		tokenString := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}

		claims, err := authService.ValidateToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "user-service",
		"timestamp": time.Now().UnixMilli(),
	})
}

func readinessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

import (
	"github.com/gin-gonic/gin"
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}
