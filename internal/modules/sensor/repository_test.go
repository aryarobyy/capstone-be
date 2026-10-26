package sensor

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"capstone-be/internal/middleware"
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

func TestSensorRepository_ClosedDB(t *testing.T) {
	repo := NewSensorRepository(closedDB(t))
	ctx := context.Background()
	if _, err := repo.Create(ctx, CreateSensorRequest{Name: "n"}); err == nil {
		t.Error("Create: expected error")
	}
	if err := repo.Update(ctx, UpdateSensorRequest{ID: 1}); err == nil || errors.Is(err, ErrSensorNotFound) {
		t.Errorf("Update: got %v", err)
	}
	if err := repo.Delete(ctx, DeleteSensorRequest{ID: 1}); err == nil || errors.Is(err, ErrSensorNotFound) {
		t.Errorf("Delete: got %v", err)
	}
	if _, _, err := repo.List(ctx, SensorFilter{Limit: 10}); err == nil {
		t.Error("List: expected error")
	}
	if _, err := repo.Detail(ctx, DetailSensorRequest{ID: 1}); err == nil || errors.Is(err, ErrSensorNotFound) {
		t.Errorf("Detail: got %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

func TestSensorRepository_DB(t *testing.T) {
	db := testutil.OpenTestDB(t)
	repo := NewSensorRepository(db)
	ctx := context.Background()
	testutil.Exec(t, db, `INSERT INTO users(id,name,email,password) VALUES(1,'a','a@x.io','x'),(2,'b','b@x.io','x')`)

	// Create: nil area -> 0, zero owner -> NULL.
	anonID, err := repo.Create(ctx, CreateSensorRequest{Name: "Anon", Code: "A0"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := repo.Detail(ctx, DetailSensorRequest{ID: anonID})
	if err != nil {
		t.Fatal(err)
	}
	if s.AreaID != 0 || s.OwnerID != 0 || s.Description != "" || s.Name != "Anon" || s.Code != "A0" {
		t.Fatalf("anon sensor %+v", s)
	}
	if testutil.Count(t, db, `SELECT count(*) FROM sensors WHERE id=$1 AND owner_id IS NULL`, anonID) != 1 {
		t.Fatal("expected NULL owner_id")
	}

	id1, err := repo.Create(ctx, CreateSensorRequest{OwnerID: 1, AreaID: ptr(int64(5)), Name: "Soil North", Code: "S1", Description: ptr("d1")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, CreateSensorRequest{OwnerID: 1, AreaID: ptr(int64(6)), Name: "Temp", Code: "T1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, CreateSensorRequest{OwnerID: 2, AreaID: ptr(int64(5)), Name: "soil south", Code: "S2"}); err != nil {
		t.Fatal(err)
	}
	// Unknown owner violates the users FK.
	if _, err := repo.Create(ctx, CreateSensorRequest{OwnerID: 999, Name: "x", Code: "x"}); err == nil {
		t.Fatal("expected FK error")
	}

	// Detail scoping.
	s, err = repo.Detail(ctx, DetailSensorRequest{ID: id1, OwnerID: 1})
	if err != nil || s.AreaID != 5 || s.OwnerID != 1 || s.Description != "d1" {
		t.Fatalf("detail %+v %v", s, err)
	}
	if _, err := repo.Detail(ctx, DetailSensorRequest{ID: id1, OwnerID: 2}); !errors.Is(err, ErrSensorNotFound) {
		t.Fatalf("other owner detail: %v", err)
	}
	if _, err := repo.Detail(ctx, DetailSensorRequest{ID: 9999}); !errors.Is(err, ErrSensorNotFound) {
		t.Fatalf("missing detail: %v", err)
	}

	// List filters.
	list := func(f SensorFilter) ([]Sensor, int) {
		t.Helper()
		if f.Limit == 0 {
			f.Limit = 10
		}
		items, n, err := repo.List(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		return items, n
	}
	if items, n := list(SensorFilter{}); n != 4 || len(items) != 4 || items[0].ID < items[3].ID {
		t.Fatalf("all: n=%d items=%v", n, items)
	}
	if _, n := list(SensorFilter{OwnerID: 1}); n != 2 {
		t.Fatalf("owner 1: n=%d", n)
	}
	if _, n := list(SensorFilter{AreaID: 5}); n != 2 {
		t.Fatalf("area 5: n=%d", n)
	}
	if items, n := list(SensorFilter{Name: "SOIL"}); n != 2 || len(items) != 2 {
		t.Fatalf("name ilike: n=%d", n)
	}
	if items, n := list(SensorFilter{OwnerID: 1, AreaID: 5, Name: "soil"}); n != 1 || items[0].ID != id1 {
		t.Fatalf("combined: n=%d items=%v", n, items)
	}
	if items, n := list(SensorFilter{Limit: 1, Index: 1}); n != 4 || len(items) != 1 {
		t.Fatalf("paged: n=%d len=%d", n, len(items))
	}
	if items, n := list(SensorFilter{OwnerID: 42}); n != 0 || items == nil || len(items) != 0 {
		t.Fatalf("empty: n=%d items=%v", n, items)
	}

	// Update: partial fields, owner scoping, not found.
	if err := repo.Update(ctx, UpdateSensorRequest{ID: id1, OwnerID: 1, Name: ptr("Renamed")}); err != nil {
		t.Fatal(err)
	}
	s, _ = repo.Detail(ctx, DetailSensorRequest{ID: id1})
	if s.Name != "Renamed" || s.Code != "S1" || s.AreaID != 5 || s.Description != "d1" {
		t.Fatalf("after partial update %+v", s)
	}
	if err := repo.Update(ctx, UpdateSensorRequest{ID: id1, AreaID: ptr(int64(0)), Code: ptr("C"), Description: ptr("")}); err != nil {
		t.Fatal(err)
	}
	s, _ = repo.Detail(ctx, DetailSensorRequest{ID: id1})
	if s.AreaID != 0 || s.Code != "C" || s.Description != "" {
		t.Fatalf("after full update %+v", s)
	}
	if err := repo.Update(ctx, UpdateSensorRequest{ID: id1, OwnerID: 2, Name: ptr("hijack")}); !errors.Is(err, ErrSensorNotFound) {
		t.Fatalf("other owner update: %v", err)
	}
	if err := repo.Update(ctx, UpdateSensorRequest{ID: 9999}); !errors.Is(err, ErrSensorNotFound) {
		t.Fatalf("missing update: %v", err)
	}

	// Delete: owner scoping, success, not found.
	if err := repo.Delete(ctx, DeleteSensorRequest{ID: id1, OwnerID: 2}); !errors.Is(err, ErrSensorNotFound) {
		t.Fatalf("other owner delete: %v", err)
	}
	if err := repo.Delete(ctx, DeleteSensorRequest{ID: id1, OwnerID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, DeleteSensorRequest{ID: anonID}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, DeleteSensorRequest{ID: id1}); !errors.Is(err, ErrSensorNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r.Group("/api"), closedDB(t))

	want := map[string]bool{
		"/api/sensor/create": false, "/api/sensor/list": false, "/api/sensor/detail": false,
		"/api/sensor/update": false, "/api/sensor/delete": false,
	}
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

// TestRegisterRoutes_EndToEnd wires router -> handler -> service -> repositories on a real DB.
func TestRegisterRoutes_EndToEnd(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.Exec(t, db, `INSERT INTO users(id,name,email,password) VALUES(3,'a','a@x.io','x')`)
	testutil.Exec(t, db, `INSERT INTO areas(id,name) VALUES(1,'A')`)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(3))
		c.Request = c.Request.WithContext(middleware.WithUserID(c.Request.Context(), 3))
		c.Next()
	})
	RegisterRoutes(r.Group("/api"), db)

	do := func(path, body string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w.Code
	}
	steps := []struct {
		path, body string
		want       int
	}{
		{"/api/sensor/create", `{"name":"n","code":"c","area_id":1}`, http.StatusCreated},
		{"/api/sensor/create", `{"name":"n","code":"c","area_id":77}`, http.StatusBadRequest},
		{"/api/sensor/list", `{}`, http.StatusOK},
		{"/api/sensor/detail", `{"id":999}`, http.StatusNotFound},
		{"/api/sensor/update", `{"id":999,"name":"z"}`, http.StatusNotFound},
		{"/api/sensor/delete", `{"id":999}`, http.StatusNotFound},
	}
	for _, s := range steps {
		if got := do(s.path, s.body); got != s.want {
			t.Errorf("%s %s = %d, want %d", s.path, s.body, got, s.want)
		}
	}
	if testutil.Count(t, db, `SELECT count(*) FROM sensors WHERE owner_id=3 AND area_id=1`) != 1 {
		t.Fatal("expected one sensor owned by user 3")
	}
}
