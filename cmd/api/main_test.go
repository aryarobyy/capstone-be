package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"capstone-be/internal/testutil"
)

// baseEnv moves into a temp dir (no .env, log/ lands there) and sets valid config.
func baseEnv(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("JWT_SECRET", strings.Repeat("k", 32))
	t.Setenv("JWT_ACCESS_MINUTES", "15")
	t.Setenv("JWT_EXPIRATION_HOURS", "1")
	t.Setenv("ENV", "test")
	t.Setenv("FCM_ENABLED", "")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "1") // unreachable unless dbEnv overrides it
}

// dbEnv points DB_* at a freshly migrated test database.
func dbEnv(t *testing.T) {
	t.Helper()
	db := testutil.OpenTestDB(t)
	var name string
	if err := db.QueryRow(`SELECT current_database()`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	for _, f := range strings.Fields(os.Getenv("TEST_DB_DSN")) {
		k, v, _ := strings.Cut(f, "=")
		switch k {
		case "host":
			t.Setenv("DB_HOST", v)
		case "port":
			t.Setenv("DB_PORT", v)
		case "user":
			t.Setenv("DB_USER", v)
		case "password":
			t.Setenv("DB_PASSWORD", v)
		}
	}
	t.Setenv("DB_NAME", name)
	t.Setenv("DB_SSLMODE", "disable")
	t.Setenv("DB_TIMEZONE", "UTC")
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

func TestRunServesAndShutsDownGracefully(t *testing.T) {
	baseEnv(t)
	dbEnv(t)
	port := freePort(t)
	t.Setenv("PORT", port)
	t.Setenv("SENSOR_CALC_INTERVAL", "1h")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()

	base := "http://127.0.0.1:" + port
	// No keep-alive: a dialed-but-unused pooled conn stays StateNew, which
	// Shutdown only treats as idle after 5s, i.e. its whole timeout.
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	var resp *http.Response
	var err error
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if resp, err = client.Get(base + "/api/health"); err == nil {
			break
		}
	}
	if err != nil {
		cancel()
		t.Fatalf("server never came up: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health: got %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS middleware not wired")
	}

	for _, tc := range []struct {
		method, path string
		header       map[string]string
		want         int
	}{
		{"OPTIONS", "/api/user/list", nil, http.StatusNoContent},
		{"POST", "/api/user/list", nil, http.StatusUnauthorized},
		{"POST", "/api/database/dump", nil, http.StatusUnauthorized},
		{"POST", "/api/ingest/sensor-reading", map[string]string{"X-Sensor-Key": "short"}, http.StatusUnauthorized},
	} {
		req, _ := http.NewRequest(tc.method, base+tc.path, strings.NewReader(`{}`))
		for k, v := range tc.header {
			req.Header.Set(k, v)
		}
		r, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != tc.want {
			t.Errorf("%s %s: got %d want %d", tc.method, tc.path, r.StatusCode, tc.want)
		}
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("run did not shut down")
	}
	if _, err := os.Stat("log/app.log"); err != nil {
		t.Errorf("log file not created: %v", err)
	}
}

func TestRunProductionModeAndListenError(t *testing.T) {
	baseEnv(t)
	dbEnv(t)
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	t.Setenv("PORT", strconv.Itoa(l.Addr().(*net.TCPAddr).Port))
	t.Setenv("ENV", "production")
	t.Setenv("SENSOR_CALC_INTERVAL", "bogus")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := run(ctx); err == nil || !strings.HasPrefix(err.Error(), "Listen and serve error") {
		t.Fatalf("got %v", err)
	}
}

func TestRunStartupErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T)
		want  string
	}{
		{"logger", func(t *testing.T) { _ = os.WriteFile("log", nil, 0o644) }, "Failed to initialize logger"},
		{"config", func(t *testing.T) { t.Setenv("JWT_ACCESS_MINUTES", "x") }, "Failed to load configurations"},
		{"jwt", func(t *testing.T) { t.Setenv("JWT_SECRET", "short") }, "Invalid JWT configuration"},
		{"database", func(t *testing.T) {}, "Failed to initialize database"},
		{"fcm", func(t *testing.T) {
			dbEnv(t)
			t.Setenv("FCM_ENABLED", "true")
			t.Setenv("FIREBASE_PROJECT_ID", "")
			t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/nonexistent/creds.json")
		}, "Failed to initialize FCM"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			baseEnv(t)
			tc.setup(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if err := run(ctx); err == nil || !strings.HasPrefix(err.Error(), tc.want) {
				t.Fatalf("got %v, want prefix %q", err, tc.want)
			}
		})
	}
}
