package auth

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error)
	Login(ctx context.Context, req LoginRequest) (*AuthResponse, error)
}

type authService struct {
	repo AuthRepository
}

func NewAuthService(repo AuthRepository) AuthService {
	return &authService{repo: repo}
}

func (s *authService) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	u := &User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashedPassword),
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}

	return ToAuthResponse(u), nil
}

func (s *authService) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	u, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return ToAuthResponse(u), nil
}
