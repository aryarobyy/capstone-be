package auth

import (
	"capstone-be/internal/middleware"
	"capstone-be/internal/session"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeService struct {
	err        error
	logoutSID  string
	logoutUser int64
}

func (f *fakeService) Register(context.Context, RegisterRequest) (*AuthResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &AuthResponse{ID: 1}, nil
}
func (f *fakeService) Login(context.Context, LoginRequest) (*LoginResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &LoginResponse{AuthResponse: &AuthResponse{ID: 1}, AccessToken: "a"}, nil
}
func (f *fakeService) Logout(_ context.Context, sid string, user int64) error {
	f.logoutSID, f.logoutUser = sid, user
	return f.err
}
func (f *fakeService) Refresh(context.Context, string) (*RefreshResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &RefreshResponse{AccessToken: "a"}, nil
}
func (f *fakeService) LogoutRefresh(context.Context, string) error { return f.err }

func serve(svc AuthService, method, path, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	h := NewAuthHandler(svc)
	r := gin.New()
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)
	r.POST("/refresh", h.Refresh)
	r.POST("/logout-refresh", h.LogoutRefresh)
	r.POST("/logout", func(c *gin.Context) {
		c.Set(middleware.SessionIDKey, "sid")
		c.Set(middleware.UserIDKey, int64(3))
	}, h.Logout)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestHandlers(t *testing.T) {
	const (
		reg     = `{"name":"Al","email":"a@x.io","msisdn":"0812","password":"secret1"}`
		login   = `{"email":"a@x.io","password":"secret1","device_token":"t","device_type":"android"}`
		refresh = `{"refresh_token":"0123456789012345678901234567890123456789012"}`
	)
	for _, tc := range []struct {
		name, path, body string
		err              error
		want             int
	}{
		{"register ok", "/register", reg, nil, 201},
		{"register invalid", "/register", `{"email":"bad"}`, nil, 400},
		{"register conflict", "/register", reg, ErrEmailAlreadyExists, 409},
		{"register failure", "/register", reg, errDB, 500},
		{"login ok", "/login", login, nil, 200},
		{"login invalid", "/login", `{}`, nil, 400},
		{"login device mismatch", "/login", `{"email":"a@x.io","password":"p","device_token":"t"}`, nil, 400},
		{"login without device", "/login", `{"email":"a@x.io","password":"secret1"}`, nil, 400},
		{"login bad device", "/login", login, ErrInvalidDevice, 400},
		{"login bad credentials", "/login", login, ErrInvalidCredentials, 401},
		{"login failure", "/login", login, errDB, 500},
		{"refresh ok", "/refresh", refresh, nil, 200},
		{"refresh invalid", "/refresh", `{"refresh_token":"short"}`, nil, 400},
		{"refresh inactive", "/refresh", refresh, session.ErrInactive, 401},
		{"refresh failure", "/refresh", refresh, errDB, 503},
		{"logout-refresh ok", "/logout-refresh", refresh, nil, 204},
		{"logout-refresh invalid", "/logout-refresh", `not json`, nil, 400},
		{"logout-refresh failure", "/logout-refresh", refresh, errDB, 503},
		{"logout ok", "/logout", "", nil, 204},
		{"logout failure", "/logout", "", errDB, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := serve(&fakeService{err: tc.err}, http.MethodPost, tc.path, tc.body)
			if w.Code != tc.want {
				t.Fatalf("status=%d want %d body=%s", w.Code, tc.want, w.Body)
			}
			if w.Code == 200 && w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("token response cacheable")
			}
		})
	}
	svc := &fakeService{}
	serve(svc, http.MethodPost, "/logout", "")
	if svc.logoutSID != "sid" || svc.logoutUser != 3 {
		t.Fatalf("logout got sid=%q user=%d", svc.logoutSID, svc.logoutUser)
	}
}
