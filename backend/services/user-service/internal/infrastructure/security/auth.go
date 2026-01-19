package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthService struct {
	secret     []byte
	expiration int
}

type JWTClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func NewAuthService(secret string, expiration int) *AuthService {
	return &AuthService{
		secret:     []byte(secret),
		expiration: expiration,
	}
}

func (s *AuthService) GenerateToken(userID, username string) (string, int64, error) {
	expiresAt := time.Now().Add(time.Duration(s.expiration) * time.Second).Unix()

	claims := &JWTClaims{
		UserID:   userID,
		Username: username,
		Role:     "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Unix(expiresAt, 0)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.secret)
	if err != nil {
		return "", 0, err
	}

	return tokenString, expiresAt, nil
}

func (s *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
