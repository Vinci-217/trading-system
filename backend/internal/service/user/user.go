package user

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"stock_trader/internal/model"
	"stock_trader/internal/repository"
)

var (
	ErrUserAlreadyExists = errors.New("用户已存在")
	ErrUserNotFound      = errors.New("用户不存在")
	ErrInvalidPassword   = errors.New("密码错误")
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Register(ctx context.Context, user *model.User) error {
	existingUser, err := s.repo.GetUserByUsername(ctx, user.Username)
	if err == nil && existingUser != nil {
		return ErrUserAlreadyExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.PasswordHash), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}
	user.PasswordHash = string(hashedPassword)
	user.Status = model.UserStatusActive

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}

	account := &model.Account{
		UserID:        user.ID,
		CashBalance:   100000,
		FrozenBalance: 0,
		TotalAssets:   100000,
		TotalProfit:   0,
		Status:        model.AccountStatusActive,
	}
	if err := s.repo.CreateAccount(ctx, account); err != nil {
		return fmt.Errorf("创建账户失败: %w", err)
	}

	return nil
}

func (s *Service) Login(ctx context.Context, username, password string) (*model.User, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if user.Status != model.UserStatusActive {
		return nil, errors.New("账户已被禁用")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}

	return user, nil
}

func (s *Service) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	user, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *Service) ValidatePassword(ctx context.Context, userID int64, password string) (bool, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return false, nil
	}
	return true, nil
}

func (s *Service) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	valid, err := s.ValidatePassword(ctx, userID, oldPassword)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidPassword
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hashedPassword)

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}

	return nil
}
