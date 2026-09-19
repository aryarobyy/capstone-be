package auth

import (
	"capstone-be/internal/session"
	"context"
	"errors"
	"strings"
	"testing"

	"capstone-be/internal/token"

	"golang.org/x/crypto/bcrypt"
)

type loginRepo struct {
	AuthRepository
	user *User
}

func (r *loginRepo) GetByEmail(context.Context, string) (*User, error) { return r.user, nil }

type devicesFake struct {
	session.Store
	sid    string
	calls  int
	userID int64
	err    error
}

func (d *devicesFake) CreateRefresh(_ context.Context, sid string, id int64, _ []byte) error {
	d.calls++
	d.sid = sid
	d.userID = id
	return d.err
}

func (d *devicesFake) BindInstallation(_ context.Context, _ string, _ int64, _, _, _ string) error {
	return d.err
}
func TestLoginTokenAndDevice(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	for _, tc := range []struct {
		name, password, device, kind string
		storeErr                     error
		wantErr                      bool
		calls                        int
	}{
		{"with device", "password", "fcm", "web", nil, false, 1},
		{"without device", "password", "", "", nil, false, 1},
		{"wrong password", "wrong", "fcm", "web", nil, true, 0},
		{"missing kind", "password", "fcm", "", nil, true, 0},
		{"missing token", "password", "", "web", nil, true, 0},
		{"store failed", "password", "fcm", "web", errors.New("database unavailable"), true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens, _ := token.NewManager(strings.Repeat("a", 32), 1)
			devices := &devicesFake{err: tc.storeErr}
			s := NewAuthService(&loginRepo{user: &User{ID: 7, Password: string(hash)}}, tokens, devices)
			result, err := s.Login(context.Background(), LoginRequest{Email: "user@example.com", Password: tc.password, DeviceToken: tc.device, DeviceType: tc.kind})
			if (err != nil) != tc.wantErr || devices.calls != tc.calls {
				t.Fatalf("err=%v calls=%d", err, devices.calls)
			}
			if err == nil {
				id, sid, err := tokens.VerifySession(result.AccessToken)
				if err != nil || sid != devices.sid || id != 7 || result.TokenType != "Bearer" || result.DeviceTokenRegistered != (tc.device != "") {
					t.Fatalf("bad login response %+v err=%v", result, err)
				}
			}
		})
	}
}
