package health

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func check(t *testing.T, db *sql.DB) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r.Group("/api"), db)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json %q: %v", w.Body.String(), err)
	}
	return w.Code, body
}

func TestHealthNotConfigured(t *testing.T) {
	code, body := check(t, nil)
	data, _ := body["data"].(map[string]any)
	if code != 200 || data["status"] != "UP" || data["database"] != "NOT_CONFIGURED" {
		t.Fatalf("got %d %v", code, body)
	}
}

func TestHealthDegraded(t *testing.T) {
	db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db.Close() // Ping fails with "sql: database is closed"
	code, body := check(t, db)
	if code != http.StatusServiceUnavailable || body["success"] != false || body["message"] != "System is degraded" {
		t.Fatalf("got %d %v", code, body)
	}
	// Current behavior: ToErrorHandler drops errs on 5xx, so the HealthResponse
	// payload (status/database) is not included in the degraded response.
	if _, ok := body["errors"]; ok {
		t.Fatalf("unexpected errors field: %v", body)
	}
}

func TestHealthUp(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	code, body := check(t, db)
	data, _ := body["data"].(map[string]any)
	if code != 200 || body["message"] != "System is healthy" || data["status"] != "UP" || data["database"] != "UP" {
		t.Fatalf("got %d %v", code, body)
	}
}
