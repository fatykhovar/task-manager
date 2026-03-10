package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fatykhovar/task-manager/internal/config"
	"github.com/fatykhovar/task-manager/internal/model"
	"github.com/fatykhovar/task-manager/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
)

type AuthService struct {
	users  *repository.UserRepository
	jwtCfg config.JWTConfig
}

func NewAuthService(users *repository.UserRepository, jwtCfg config.JWTConfig) *AuthService {
	return &AuthService{users: users, jwtCfg: jwtCfg}
}

func (s *AuthService) Register(ctx context.Context, username, email, password string) (*model.User, error) {
	existing, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUserAlreadyExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u := &model.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, *model.User, error) {
	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return "", nil, err
	}
	if u == nil {
		return "", nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}

	token, err := s.generateToken(u.ID)
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
}

func (s *AuthService) ValidateToken(tokenStr string) (int64, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.jwtCfg.Secret), nil
	})
	if err != nil || !token.Valid {
		return 0, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("invalid claims")
	}

	sub, ok := claims["sub"].(float64)
	if !ok {
		return 0, errors.New("invalid sub claim")
	}
	return int64(sub), nil
}

func (s *AuthService) generateToken(userID int64) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(s.jwtCfg.Expiration).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtCfg.Secret))
}
