package auth

import (
	"capstone-be/internal/session"
	"context"
	"errors"
	"github.com/google/uuid"
	"log/slog"
	"strings"

	"capstone-be/internal/token"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidDevice      = errors.New("device_token and valid device_type must be supplied together")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	Logout(context.Context, string, int64) error
	Refresh(context.Context, string) (*RefreshResponse, error)
	LogoutRefresh(context.Context, string) error
}

type authService struct {
	tokens   *token.Manager
	sessions session.Store
	repo     AuthRepository
}

func NewAuthService(repo AuthRepository, tokens *token.Manager, sessions session.Store) AuthService {
	return &authService{repo: repo, tokens: tokens, sessions: sessions}
}

func toAuthResponse(u *User) *AuthResponse {
	if u == nil {
		return nil
	}
	return &AuthResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Msisdn:    u.Msisdn,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func (s *authService) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	u := &User{
		Name:     req.Name,
		Email:    req.Email,
		Msisdn:   req.Msisdn,
		Password: string(hashedPassword),
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}

	return toAuthResponse(u), nil
}

func (s *authService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	if (req.DeviceToken == "") != (req.DeviceType == "") || (req.DeviceToken != "" && (strings.TrimSpace(req.DeviceToken) == "" || len(req.DeviceToken) > 4096 || (req.DeviceType != "android" && req.DeviceType != "ios" && req.DeviceType != "web"))) {
		return nil, ErrInvalidDevice
	}
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

	sid := uuid.NewString()
	accessToken, expiresIn, err := s.tokens.Issue(u.ID, sid)
	if err != nil {
		return nil, err
	}
	refresh, hash, err := session.NewRefresh()
	if err != nil {
		return nil, err
	}
	if err := s.sessions.CreateRefresh(ctx, sid, u.ID, hash); err != nil {
		return nil, err
	}
	installation := req.InstallationID
	if installation == "" {
		installation = uuid.NewString()
	}
	registered := false
	if req.DeviceToken != "" {
		if err := s.sessions.BindInstallation(ctx, sid, u.ID, req.DeviceToken, req.DeviceType, installation); err != nil {
			slog.WarnContext(ctx, "device registration deferred", "user_id", u.ID)
		} else {
			registered = true
		}
	}

	return &LoginResponse{RefreshToken: refresh, InstallationID: installation, AuthResponse: toAuthResponse(u), AccessToken: accessToken, TokenType: "Bearer", ExpiresIn: expiresIn, DeviceTokenRegistered: registered}, nil
}

func (s *authService) Logout(ctx context.Context, sid string, user int64) error {
	return s.sessions.Revoke(ctx, sid, user)
}

func (s *authService) Refresh(ctx context.Context, raw string) (*RefreshResponse, error) {
	if len(raw) != 43 {
		return nil, session.ErrInactive
	}
	next, hash, err := session.NewRefresh()
	if err != nil {
		return nil, err
	}
	sid, user, err := s.sessions.Rotate(ctx, session.HashRefresh(raw), hash)
	if err != nil {
		return nil, err
	}
	access, expiry, err := s.tokens.Issue(user, sid)
	if err != nil {
		return nil, err
	}
	return &RefreshResponse{AccessToken: access, RefreshToken: next, TokenType: "Bearer", ExpiresIn: expiry}, nil
}
func (s *authService) LogoutRefresh(ctx context.Context, raw string) error {
	return s.sessions.RevokeRefresh(ctx, session.HashRefresh(raw))
}
