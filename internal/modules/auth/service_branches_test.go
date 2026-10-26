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

var errDB = errors.New("database unavailable")

type fakeRepo struct {
	created            *User
	user               *User
	createErr, findErr error
}

func (r *fakeRepo) Create(_ context.Context, u *User) error {
	if r.createErr != nil {
		return r.createErr
	}
	u.ID = 5
	r.created = u
	return nil
}

func (r *fakeRepo) GetByEmail(context.Context, string) (*User, error) { return r.user, r.findErr }

type fakeStore struct {
	session.Store
	createErr, bindErr, rotateErr, revokeErr error
	rotateUser                               int64
	bound                                    []string
	revoked                                  []byte
	revokedSID                               string
}

func (f *fakeStore) CreateRefresh(context.Context, string, int64, []byte) error { return f.createErr }
func (f *fakeStore) BindInstallation(_ context.Context, _ string, _ int64, tok, kind, inst string) error {
	f.bound = []string{tok, kind, inst}
	return f.bindErr
}
func (f *fakeStore) Rotate(context.Context, []byte, []byte) (string, int64, error) {
	return "sid", f.rotateUser, f.rotateErr
}
func (f *fakeStore) RevokeRefresh(_ context.Context, h []byte) error {
	f.revoked = h
	return f.revokeErr
}
func (f *fakeStore) Revoke(_ context.Context, sid string, _ int64) error {
	f.revokedSID = sid
	return f.revokeErr
}

func newTokens(t *testing.T) *token.Manager {
	t.Helper()
	m, err := token.NewManager(strings.Repeat("a", 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestRegisterService(t *testing.T) {
	repo := &fakeRepo{}
	s := NewAuthService(repo, newTokens(t), &fakeStore{})
	res, err := s.Register(context.Background(), RegisterRequest{Name: "n", Email: "e@x.io", Msisdn: "08", Password: "secret1"})
	if err != nil || res.ID != 5 || res.Email != "e@x.io" || res.Msisdn != "08" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if bcrypt.CompareHashAndPassword([]byte(repo.created.Password), []byte("secret1")) != nil {
		t.Fatal("password not hashed")
	}
	if _, err := s.Register(context.Background(), RegisterRequest{Password: strings.Repeat("p", 73)}); err == nil {
		t.Fatal("over-long password accepted")
	}
	s = NewAuthService(&fakeRepo{createErr: ErrEmailAlreadyExists}, newTokens(t), &fakeStore{})
	if _, err := s.Register(context.Background(), RegisterRequest{Password: "secret1"}); !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("err=%v", err)
	}
}

func TestToAuthResponseNil(t *testing.T) {
	if toAuthResponse(nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestLoginBranches(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	user := &User{ID: 7, Password: string(hash)}
	ctx := context.Background()
	ok := LoginRequest{Email: "u@x.io", Password: "password"}

	for name, req := range map[string]LoginRequest{
		"blank token":  {DeviceToken: "   ", DeviceType: "web"},
		"long token":   {DeviceToken: strings.Repeat("t", 4097), DeviceType: "web"},
		"invalid type": {DeviceToken: "t", DeviceType: "desktop"},
	} {
		s := NewAuthService(&fakeRepo{user: user}, newTokens(t), &fakeStore{})
		if _, err := s.Login(ctx, req); !errors.Is(err, ErrInvalidDevice) {
			t.Fatalf("%s: err=%v", name, err)
		}
	}

	for _, tc := range []struct {
		name  string
		repo  *fakeRepo
		store *fakeStore
		want  error
	}{
		{"unknown email", &fakeRepo{findErr: ErrUserNotFound}, &fakeStore{}, ErrInvalidCredentials},
		{"repo failure", &fakeRepo{findErr: errDB}, &fakeStore{}, errDB},
		{"create refresh failure", &fakeRepo{user: user}, &fakeStore{createErr: errDB}, errDB},
	} {
		s := NewAuthService(tc.repo, newTokens(t), tc.store)
		if _, err := s.Login(ctx, ok); !errors.Is(err, tc.want) {
			t.Fatalf("%s: err=%v", tc.name, err)
		}
	}

	// Issue rejects a non-positive user ID.
	s := NewAuthService(&fakeRepo{user: &User{ID: 0, Password: string(hash)}}, newTokens(t), &fakeStore{})
	if _, err := s.Login(ctx, ok); err == nil {
		t.Fatal("token issued for invalid user")
	}

	// Device binding failure is non-fatal; supplied installation ID is echoed.
	store := &fakeStore{bindErr: errDB}
	s = NewAuthService(&fakeRepo{user: user}, newTokens(t), store)
	req := ok
	req.DeviceToken, req.DeviceType, req.InstallationID = "fcm", "android", "00000000-0000-4000-8000-000000000001"
	res, err := s.Login(ctx, req)
	if err != nil || res.DeviceTokenRegistered || res.InstallationID != req.InstallationID || len(res.RefreshToken) != 43 || res.ExpiresIn != 3600 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if store.bound[0] != "fcm" || store.bound[1] != "android" || store.bound[2] != req.InstallationID {
		t.Fatalf("bound=%v", store.bound)
	}
	res, err = s.Login(ctx, ok)
	if err != nil || res.InstallationID == "" {
		t.Fatalf("missing generated installation id: %+v err=%v", res, err)
	}
}

func TestRefreshLogoutService(t *testing.T) {
	ctx := context.Background()
	raw, _, _ := session.NewRefresh()
	tokens := newTokens(t)

	s := NewAuthService(&fakeRepo{}, tokens, &fakeStore{})
	if _, err := s.Refresh(ctx, "short"); !errors.Is(err, session.ErrInactive) {
		t.Fatalf("err=%v", err)
	}
	s = NewAuthService(&fakeRepo{}, tokens, &fakeStore{rotateErr: session.ErrInactive})
	if _, err := s.Refresh(ctx, raw); !errors.Is(err, session.ErrInactive) {
		t.Fatalf("err=%v", err)
	}
	s = NewAuthService(&fakeRepo{}, tokens, &fakeStore{rotateUser: 0})
	if _, err := s.Refresh(ctx, raw); err == nil {
		t.Fatal("token issued for invalid user")
	}
	s = NewAuthService(&fakeRepo{}, tokens, &fakeStore{rotateUser: 9})
	res, err := s.Refresh(ctx, raw)
	if err != nil || res.TokenType != "Bearer" || len(res.RefreshToken) != 43 || res.RefreshToken == raw {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if id, sid, err := tokens.VerifySession(res.AccessToken); err != nil || id != 9 || sid != "sid" {
		t.Fatalf("id=%d sid=%s err=%v", id, sid, err)
	}

	store := &fakeStore{revokeErr: errDB}
	s = NewAuthService(&fakeRepo{}, tokens, store)
	if err := s.LogoutRefresh(ctx, raw); !errors.Is(err, errDB) || string(store.revoked) != string(session.HashRefresh(raw)) {
		t.Fatalf("err=%v", err)
	}
	if err := s.Logout(ctx, "sid-1", 1); !errors.Is(err, errDB) || store.revokedSID != "sid-1" {
		t.Fatalf("err=%v", err)
	}
}
