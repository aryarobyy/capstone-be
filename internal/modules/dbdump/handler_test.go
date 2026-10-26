package dbdump

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"capstone-be/config"
	"capstone-be/internal/testutil"

	"github.com/gin-gonic/gin"
)

func TestDelayedWriter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	startedCount := 0
	dw := &delayedWriter{
		w: c.Writer,
		onStart: func() {
			startedCount++
			c.Header("Content-Type", "application/sql")
			c.Writer.WriteHeader(http.StatusOK)
		},
	}

	if startedCount != 0 {
		t.Fatalf("expected startedCount to be 0 before write, got %d", startedCount)
	}

	data := []byte("-- test sql dump chunk")
	n, err := dw.Write(data)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected written %d bytes, got %d", len(data), n)
	}
	if startedCount != 1 {
		t.Fatalf("expected startedCount to be 1 after first write, got %d", startedCount)
	}

	// Second write should not trigger onStart again
	data2 := []byte("\nSELECT 1;")
	_, err = dw.Write(data2)
	if err != nil {
		t.Fatalf("unexpected second write error: %v", err)
	}
	if startedCount != 1 {
		t.Fatalf("expected startedCount to remain 1 after second write, got %d", startedCount)
	}

	if rec.Header().Get("Content-Type") != "application/sql" {
		t.Fatalf("expected Content-Type header to be application/sql, got %s", rec.Header().Get("Content-Type"))
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	expectedBody := string(data) + string(data2)
	if rec.Body.String() != expectedBody {
		t.Fatalf("expected body %q, got %q", expectedBody, rec.Body.String())
	}
}

func TestFindPgDump(t *testing.T) {
	path, err := findPgDump()
	if err != nil {
		t.Skipf("pg_dump not available in test environment: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path to pg_dump")
	}
}

func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group("/database")
	cfg := &config.Config{
		DBHost:     "127.0.0.1",
		DBPort:     "5432",
		DBUser:     "postgres",
		DBPassword: "password",
		DBName:     "test_db",
	}

	RegisterRoutes(group, cfg)

	routes := r.Routes()
	expectedRoutes := map[string]string{
		"POST /database/dump": "",
	}

	for _, rt := range routes {
		key := rt.Method + " " + rt.Path
		delete(expectedRoutes, key)
	}

	if len(expectedRoutes) > 0 {
		var missing []string
		for k := range expectedRoutes {
			missing = append(missing, k)
		}
		t.Fatalf("missing routes: %v", missing)
	}
}

func TestHandlerDumpFailConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	cfg := &config.Config{
		DBHost:     "127.0.0.1",
		DBPort:     "59999", // Unreachable port
		DBUser:     "invalid_user",
		DBPassword: "invalid_password",
		DBName:     "nonexistent_db",
	}

	handler := NewHandler(cfg)
	r.POST("/dump", handler.Dump)

	req := httptest.NewRequest("POST", "/dump", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Since DB is unreachable, pg_dump should fail and return 500 JSON error
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d. Body: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("failed to execute database dump")) {
		t.Fatalf("expected error JSON, got %s", rec.Body.String())
	}
}

// testConfig points at the TEST_DB_DSN server (never the developer's .env DB).
func testConfig(t *testing.T) *config.Config {
	t.Helper()
	db := testutil.OpenTestDB(t)
	cfg := &config.Config{DBSSLMode: "disable"}
	if err := db.QueryRow(`SELECT current_database()`).Scan(&cfg.DBName); err != nil {
		t.Fatal(err)
	}
	for _, f := range strings.Fields(os.Getenv("TEST_DB_DSN")) {
		k, v, _ := strings.Cut(f, "=")
		switch k {
		case "host":
			cfg.DBHost = v
		case "port":
			cfg.DBPort = v
		case "user":
			cfg.DBUser = v
		case "password":
			cfg.DBPassword = v
		}
	}
	return cfg
}

func TestHandlerDumpSuccess(t *testing.T) {
	cfg := testConfig(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := NewHandler(cfg)
	r.POST("/dump", handler.Dump)

	for _, q := range []string{"schema_only=true", "data_only=true&table=users", "clean=true"} {
		req := httptest.NewRequest("POST", "/dump?"+q, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d: %s", q, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Content-Type") != "application/sql" {
			t.Fatalf("expected application/sql, got %s", rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Header().Get("Content-Disposition"), "backup_"+cfg.DBName) {
			t.Fatalf("bad Content-Disposition %q", rec.Header().Get("Content-Disposition"))
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte("PostgreSQL database dump")) {
			t.Fatalf("expected PostgreSQL database dump in output")
		}
	}
}

func TestFindPgDumpFallback(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // not on PATH: only the hard-coded candidates remain
	p, err := findPgDump()
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			t.Fatalf("unexpected error %v", err)
		}
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/dump", NewHandler(&config.Config{}).Dump)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("POST", "/dump", nil))
		if rec.Code != http.StatusInternalServerError || !bytes.Contains(rec.Body.Bytes(), []byte("database dump tool unavailable")) {
			t.Fatalf("got %d %s", rec.Code, rec.Body.String())
		}
		return
	}
	if !strings.HasSuffix(p, "/pg_dump") {
		t.Fatalf("unexpected path %s", p)
	}
}
