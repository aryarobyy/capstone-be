package auth

import (
	"capstone-be/internal/session"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"capstone-be/internal/testutil"

	"github.com/gin-gonic/gin"
)

func TestAuthRepository(t *testing.T) {
	db := testutil.OpenTestDB(t)
	repo := NewAuthRepository(db)
	ctx := context.Background()

	u := &User{Name: "Al", Email: "al@x.io", Msisdn: "0812", Password: "hash"}
	if err := repo.Create(ctx, u); err != nil || u.ID == 0 || u.CreatedAt.IsZero() {
		t.Fatalf("u=%+v err=%v", u, err)
	}
	if err := repo.Create(ctx, &User{Name: "B", Email: "al@x.io", Password: "h"}); !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("duplicate email: %v", err)
	}
	// Non-unique constraint errors are passed through unchanged.
	if err := repo.Create(ctx, &User{Name: strings.Repeat("n", 101), Email: "c@x.io", Password: "h"}); err == nil || errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("long name: %v", err)
	}
	got, err := repo.GetByEmail(ctx, "al@x.io")
	if err != nil || got.ID != u.ID || got.Msisdn != "0812" || got.Password != "hash" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	testutil.Exec(t, db, `INSERT INTO users(name,email,password) VALUES('N','null@x.io','h')`)
	if got, err := repo.GetByEmail(ctx, "null@x.io"); err != nil || got.Msisdn != "" {
		t.Fatalf("null msisdn: got=%+v err=%v", got, err)
	}
	if _, err := repo.GetByEmail(ctx, "missing@x.io"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("missing: %v", err)
	}
	db.Close()
	if _, err := repo.GetByEmail(ctx, "al@x.io"); err == nil || errors.Is(err, ErrUserNotFound) {
		t.Fatalf("closed db: %v", err)
	}
}

// End-to-end through RegisterRoutes with the real repository and session store.
func TestAuthRoutesFlow(t *testing.T) {
	db := testutil.OpenTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r.Group(""), db, newTokens(t), session.NewRepository(db))
	call := func(path, body, bearer string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		r.ServeHTTP(w, req)
		return w
	}
	type tokens struct {
		Data struct {
			AccessToken           string `json:"access_token"`
			RefreshToken          string `json:"refresh_token"`
			DeviceTokenRegistered bool   `json:"device_token_registered"`
		} `json:"data"`
	}
	decode := func(w *httptest.ResponseRecorder) tokens {
		var v tokens
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}

	if w := call("/auth/register", `{"name":"Al","email":"al@x.io","msisdn":"0812","password":"secret1"}`, ""); w.Code != 201 {
		t.Fatalf("register %d %s", w.Code, w.Body)
	}
	if w := call("/auth/register", `{"name":"Al","email":"al@x.io","msisdn":"0812","password":"secret1"}`, ""); w.Code != 409 {
		t.Fatalf("duplicate register %d", w.Code)
	}
	if w := call("/auth/login", `{"email":"AL@X.io","password":"secret1"}`, ""); w.Code != 200 || decode(w).Data.DeviceTokenRegistered {
		t.Fatalf("case-insensitive login without device token %d %s", w.Code, w.Body)
	}
	if w := call("/auth/login", `{"email":"al@x.io","password":"wrong","device_token":"fcm","device_type":"web"}`, ""); w.Code != 401 {
		t.Fatalf("bad login %d", w.Code)
	}
	w := call("/auth/login", `{"email":"al@x.io","password":"secret1","device_token":"fcm","device_type":"web"}`, "")
	if w.Code != 200 {
		t.Fatalf("login %d %s", w.Code, w.Body)
	}
	login := decode(w)
	if !login.Data.DeviceTokenRegistered || testutil.Count(t, db, `SELECT count(*) FROM device_tokens WHERE token='fcm'`) != 1 {
		t.Fatal("device not registered")
	}

	w = call("/auth/refresh", `{"refresh_token":"`+login.Data.RefreshToken+`"}`, "")
	if w.Code != 200 {
		t.Fatalf("refresh %d %s", w.Code, w.Body)
	}
	rotated := decode(w)
	if w := call("/auth/refresh", `{"refresh_token":"`+login.Data.RefreshToken+`"}`, ""); w.Code != 401 {
		t.Fatalf("reused refresh %d", w.Code)
	}
	if w := call("/auth/logout", "", rotated.Data.AccessToken); w.Code != 204 {
		t.Fatalf("logout %d %s", w.Code, w.Body)
	}
	if w := call("/auth/logout", "", rotated.Data.AccessToken); w.Code != 401 {
		t.Fatalf("logout after revoke %d", w.Code)
	}

	second := decode(call("/auth/login", `{"email":"al@x.io","password":"secret1","device_token":"fcm","device_type":"web"}`, ""))
	if w := call("/auth/logout-refresh", `{"refresh_token":"`+second.Data.RefreshToken+`"}`, ""); w.Code != 204 {
		t.Fatalf("logout-refresh %d", w.Code)
	}
	if w := call("/auth/refresh", `{"refresh_token":"`+second.Data.RefreshToken+`"}`, ""); w.Code != 401 {
		t.Fatalf("refresh after logout-refresh %d", w.Code)
	}
}
