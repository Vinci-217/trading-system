package service

import (
	"context"
	"time"

	"stock_trader/user-service/internal/domain/entity"
	"stock_trader/user-service/internal/domain/repository"
	"stock_trader/user-service/internal/infrastructure/security"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserApplicationService struct {
	userRepo    repository.UserRepository
	accountRepo repository.AccountRepository
	authService *security.AuthService
	logger      Logger
}

type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
}

func NewUserApplicationService(
	userRepo repository.UserRepository,
	accountRepo repository.AccountRepository,
	authService *security.AuthService,
	logger Logger,
) *UserApplicationService {
	return &UserApplicationService{
		userRepo:    userRepo,
		accountRepo: accountRepo,
		authService: authService,
		logger:      logger,
	}
}

type RegisterResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Expires  int64  `json:"expires_at"`
}

func (s *UserApplicationService) Register(ctx context.Context, username, password, email string) (*RegisterResponse, error) {
	s注册用户", s.logger.Info("开始.logger.String("username", username))

	exists, err := s.userRepo.ExistsByUsername(ctx, username)
	if err != nil {
		s.logger.Error("检查用户名失败", s.logger.Error(err))
		return nil, err
	}
	if exists {
		return nil, ErrUsernameExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("密码加密失败", s.logger.Error(err))
		return nil, err
	}

	userID := uuid.New().String()
	user := entity.NewUser(userID, username, string(hashedPassword), email)

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("创建用户失败", s.logger.Error(err))
		return nil, err
	}

	accountID := uuid.New().String()
	account := entity.NewAccount(accountID, userID, decimal.NewFromInt(100000))
	if err := s.accountRepo.Create(ctx, account); err != nil {
		s.logger.Error("创建账户失败", s.logger.Error(err))
		return nil, err
	}

	s.logger.Info("用户注册成功", s.logger.String("user_id", userID))

	return &RegisterResponse{
		UserID:   userID,
		Username: username,
		Email:    email,
	}, nil
}

func (s *UserApplicationService) Login(ctx context.Context, username, password string) (*LoginResponse, error) {
	s.logger.Info("开始登录", s.logger.String("username", username))

	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		s.logger.Error("用户不存在", s.logger.Error(err))
		return nil, ErrInvalidCredentials
	}

	if user.Status == entity.UserStatusLocked {
		return nil, ErrAccountLocked
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		s.logger.Error("密码错误", s.logger.Error(err))
		return nil, ErrInvalidCredentials
	}

	token, expiresAt, err := s.authService.GenerateToken(user.ID, user.Username)
	if err != nil {
		s.logger.Error("生成Token失败", s.logger.Error(err))
		return nil, err
	}

	s.logger.Info("用户登录成功", s.logger.String("user_id", user.ID))

	return &LoginResponse{
		Token:    token,
		UserID:   user.ID,
		Username: user.Username,
		Expires:  expiresAt,
	}, nil
}

func (s *UserApplicationService) CleanupInactiveUsers(ctx context.Context) {
	s.logger.Info("清理不活跃用户")
}

import (
	"errors"
	"github.com/shopspring/decimal"
)

var ErrUsernameExists = errors.New("username already exists")
var ErrInvalidCredentials = errors.New("invalid username or password")
var ErrAccountLocked = errors.New("account is locked")
