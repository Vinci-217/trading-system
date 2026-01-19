package domain

import (
	"time"
)

type User struct {
	ID        string
	Username  string
	Password  string
	Email     string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

const (
	UserStatusActive   = "ACTIVE"
	UserStatusInactive = "INACTIVE"
	UserStatusLocked   = "LOCKED"
)

func NewUser(id, username, password, email string) *User {
	return &User{
		ID:        id,
		Username:  username,
		Password:  password,
		Email:     email,
		Status:    UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (u *User) Lock() {
	u.Status = UserStatusLocked
	u.UpdatedAt = time.Now()
}

func (u *User) Activate() {
	u.Status = UserStatusActive
	u.UpdatedAt = time.Now()
}
