package user

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"testing"

	"capstone-be/internal/testutil"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// closedDB returns a *sql.DB whose every call fails, without needing a server.
func closedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	return db
}

func TestUserRepository_ClosedDB(t *testing.T) {
	repo := NewUserRepository(closedDB(t))
	ctx := context.Background()
	if _, err := repo.Detail(ctx, UserDetailRequest{ID: 1}); err == nil || errors.Is(err, ErrUserNotFound) {
		t.Errorf("Detail: %v", err)
	}
	if _, err := repo.GetByEmail(ctx, "a@x.io"); err == nil || errors.Is(err, ErrUserNotFound) {
		t.Errorf("GetByEmail: %v", err)
	}
	if _, _, err := repo.List(ctx, 10, 0); err == nil {
		t.Error("List: expected error")
	}
	if err := repo.Update(ctx, UpdateUserRequest{ID: 1}); err == nil || errors.Is(err, ErrUserNotFound) {
		t.Errorf("Update: %v", err)
	}
	if err := repo.Delete(ctx, DeleteUserRequest{ID: 1}); err == nil || errors.Is(err, ErrUserNotFound) {
		t.Errorf("Delete: %v", err)
	}
}

func TestUserRepository_DB(t *testing.T) {
	db := testutil.OpenTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()
	testutil.Exec(t, db, `INSERT INTO users(id,name,email,password,msisdn) VALUES
		(1,'Alice','alice@x.io','h1','0811'),
		(2,'Bob','bob@x.io','h2',NULL),
		(3,'Carol','carol@x.io','h3','0833')`)

	u, err := repo.Detail(ctx, UserDetailRequest{ID: 1})
	if err != nil || u.Name != "Alice" || u.Email != "alice@x.io" || u.Msisdn != "0811" || u.Password != "h1" || u.CreatedAt.IsZero() {
		t.Fatalf("detail %+v %v", u, err)
	}
	if u, err := repo.Detail(ctx, UserDetailRequest{ID: 2}); err != nil || u.Msisdn != "" {
		t.Fatalf("NULL msisdn: %+v %v", u, err)
	}
	if _, err := repo.Detail(ctx, UserDetailRequest{ID: 99}); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("missing detail: %v", err)
	}

	if u, err := repo.GetByEmail(ctx, "bob@x.io"); err != nil || u.ID != 2 {
		t.Fatalf("by email %+v %v", u, err)
	}
	if _, err := repo.GetByEmail(ctx, "none@x.io"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("missing email: %v", err)
	}

	users, n, err := repo.List(ctx, 2, 0)
	if err != nil || n != 3 || len(users) != 2 || users[0].ID != 3 || users[1].ID != 2 {
		t.Fatalf("list page 1: n=%d users=%+v err=%v", n, users, err)
	}
	users, n, err = repo.List(ctx, 2, 2)
	if err != nil || n != 3 || len(users) != 1 || users[0].ID != 1 {
		t.Fatalf("list page 2: n=%d users=%+v err=%v", n, users, err)
	}
	users, _, err = repo.List(ctx, 10, 50)
	if err != nil || users == nil || len(users) != 0 {
		t.Fatalf("list past end: %+v %v", users, err)
	}

	// Partial update keeps empty fields unchanged.
	before, _ := repo.Detail(ctx, UserDetailRequest{ID: 1})
	if err := repo.Update(ctx, UpdateUserRequest{ID: 1, Name: "Alicia"}); err != nil {
		t.Fatal(err)
	}
	after, _ := repo.Detail(ctx, UserDetailRequest{ID: 1})
	if after.Name != "Alicia" || after.Email != "alice@x.io" || after.Msisdn != "0811" || after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("after update %+v", after)
	}
	if err := repo.Update(ctx, UpdateUserRequest{ID: 2, Email: "bobby@x.io", Msisdn: "0822"}); err != nil {
		t.Fatal(err)
	}
	if u, _ := repo.Detail(ctx, UserDetailRequest{ID: 2}); u.Email != "bobby@x.io" || u.Msisdn != "0822" || u.Name != "Bob" {
		t.Fatalf("after update %+v", u)
	}
	if err := repo.Update(ctx, UpdateUserRequest{ID: 2, Email: "carol@x.io"}); !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("duplicate email: %v", err)
	}
	if err := repo.Update(ctx, UpdateUserRequest{ID: 99, Name: "Ghost"}); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("missing update: %v", err)
	}

	// BUG (documented, not fixed): UpdateUserRequest allows msisdn up to 100
	// chars (binding max=100) but users.msisdn is VARCHAR(20), so 21..100 chars
	// pass validation and fail in the DB with a generic error (HTTP 500).
	err = repo.Update(ctx, UpdateUserRequest{ID: 3, Msisdn: strings.Repeat("9", 21)})
	if err == nil || errors.Is(err, ErrEmailAlreadyExists) || errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected raw DB error for over-long msisdn, got %v", err)
	}

	if err := repo.Delete(ctx, DeleteUserRequest{ID: 3}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, DeleteUserRequest{ID: 3}); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	if testutil.Count(t, db, `SELECT count(*) FROM users`) != 2 {
		t.Fatal("expected 2 users left")
	}
}

func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r.Group("/api"), closedDB(t))

	want := map[string]bool{"/api/user/list": false, "/api/user/detail": false, "/api/user/update": false, "/api/user/delete": false}
	for _, rt := range r.Routes() {
		if _, ok := want[rt.Path]; ok && rt.Method == http.MethodPost {
			want[rt.Path] = true
		}
	}
	for p, ok := range want {
		if !ok {
			t.Errorf("route POST %s not registered", p)
		}
	}
}

// TestRegisterRoutes_EndToEnd wires router -> handler -> service -> repository on a real DB.
func TestRegisterRoutes_EndToEnd(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.Exec(t, db, `INSERT INTO users(id,name,email,password) VALUES(1,'A','a@x.io','x'),(2,'B','b@x.io','x')`)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r.Group("/api"), db)

	steps := []struct {
		path, body string
		want       int
	}{
		{"/api/user/list", `{}`, http.StatusOK},
		{"/api/user/detail", `{"id":1}`, http.StatusOK},
		{"/api/user/detail", `{"id":99}`, http.StatusNotFound},
		{"/api/user/update", `{"id":1,"email":"b@x.io"}`, http.StatusConflict},
		{"/api/user/update", `{"id":1,"name":"Ann"}`, http.StatusOK},
		{"/api/user/delete", `{"id":2}`, http.StatusOK},
		{"/api/user/delete", `{"id":2}`, http.StatusNotFound},
	}
	for _, s := range steps {
		if w := post(r, s.path, s.body); w.Code != s.want {
			t.Errorf("%s %s = %d, want %d (%s)", s.path, s.body, w.Code, s.want, w.Body)
		}
	}
}
