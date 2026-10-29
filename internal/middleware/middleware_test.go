package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"capstone-be/internal/testutil"
	"capstone-be/internal/token"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func serve(r *gin.Engine, method, path, body string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range header {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// closedDB returns a *sql.DB whose every query fails ("sql: database is closed").
func closedDB(t *testing.T) *sql.DB {
	db := getTestDB(t)
	db.Close()
	return db
}

func asUser(id int64) gin.HandlerFunc {
	return func(c *gin.Context) { c.Set(UserIDKey, id); c.Next() }
}

func ok(c *gin.Context) { c.Status(http.StatusOK) }

// captureSlog routes the default slog logger into a buffer for the test.
func captureSlog(t *testing.T) *bytes.Buffer {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	buf := new(bytes.Buffer)
	slog.SetDefault(slog.New(newJSONOrderedHandler(buf, slog.LevelDebug)))
	return buf
}

func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.Any("/x", ok)
	w := serve(r, http.MethodOptions, "/x", "", nil)
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "*" || !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "X-Sensor-Key") {
		t.Fatalf("preflight: %d %v", w.Code, w.Header())
	}
	w = serve(r, http.MethodGet, "/x", "", nil)
	if w.Code != http.StatusOK || w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatalf("get: %d %v", w.Code, w.Header())
	}
}

func TestBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimit())
	r.POST("/x", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusOK)
	})
	if w := serve(r, "POST", "/x", strings.Repeat("a", 65536), nil); w.Code != 200 {
		t.Fatalf("at limit: %d", w.Code)
	}
	if w := serve(r, "POST", "/x", strings.Repeat("a", 65537), nil); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("over limit: %d", w.Code)
	}
}

func TestRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(0, 0)
	r := gin.New()
	r.Use(rateLimit(2, 50*time.Millisecond, func() time.Time { return now }))
	r.GET("/x", ok)
	for i, want := range []int{200, 200, 429} {
		if w := serve(r, "GET", "/x", "", nil); w.Code != want {
			t.Fatalf("request %d: got %d want %d", i, w.Code, want)
		} else if want == 429 && w.Header().Get("Retry-After") != "60" {
			t.Fatal("missing Retry-After")
		}
	}
	now = now.Add(50 * time.Millisecond)
	if w := serve(r, "GET", "/x", "", nil); w.Code != 200 {
		t.Fatalf("after window: %d", w.Code)
	}
}

func TestRateLimitTableFullAndEviction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(0, 0)
	r := gin.New()
	r.Use(rateLimit(1, 200*time.Millisecond, func() time.Time { return now }))
	r.GET("/x", ok)
	hit := func(i int) int {
		req := httptest.NewRequest("GET", "/x", nil)
		req.RemoteAddr = fmt.Sprintf("10.%d.%d.%d:1", i>>16&255, i>>8&255, i&255)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	for i := 0; i < 10000; i++ {
		if code := hit(i); code != 200 {
			t.Fatalf("ip %d: %d", i, code)
		}
	}
	if code := hit(10000); code != 429 { // table full, nothing expired yet
		t.Fatalf("full table: %d", code)
	}
	now = now.Add(200 * time.Millisecond)
	if code := hit(10001); code != 200 { // expired entries evicted
		t.Fatalf("after eviction: %d", code)
	}
}

func TestRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logs := captureSlog(t)
	r := gin.New()
	r.Use(Recovery())
	r.GET("/boom", func(c *gin.Context) { panic("kaboom") })
	r.GET("/fine", ok)
	w := serve(r, "GET", "/boom", "", nil)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != 500 || body["success"] != false || body["message"] != "Internal Server Error" {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(logs.String(), "panic recovered") || !strings.Contains(logs.String(), "kaboom") {
		t.Fatalf("panic not logged: %s", logs.String())
	}
	if w := serve(r, "GET", "/fine", "", nil); w.Code != 200 {
		t.Fatalf("fine: %d", w.Code)
	}
}

func TestLoggerLevels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logs := captureSlog(t)
	r := gin.New()
	r.Use(Logger())
	r.GET("/s/:code", func(c *gin.Context) {
		var code int
		fmt.Sscan(c.Param("code"), &code)
		if code >= 500 {
			_ = c.Error(fmt.Errorf("handler failed"))
		}
		c.Status(code)
	})
	for code, level := range map[int]string{200: "INFO", 404: "WARN", 500: "ERROR"} {
		logs.Reset()
		serve(r, "GET", fmt.Sprintf("/s/%d?q=1", code), "", nil)
		var entry map[string]any
		if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
			t.Fatalf("%d: bad log %q", code, logs.String())
		}
		if entry["level"] != level || entry["status"] != float64(code) || entry["path"] != fmt.Sprintf("/s/%d?q=1", code) || entry["method"] != "GET" {
			t.Errorf("%d: %v", code, entry)
		}
		if _, hasErr := entry["error"]; hasErr != (code == 500) {
			t.Errorf("%d: error attr presence wrong: %v", code, entry)
		}
	}
}

func TestSlogWriterAndHandlerExtras(t *testing.T) {
	logs := captureSlog(t)
	w := &slogWriter{level: slog.LevelWarn}
	in := []byte("first line\n\n   \nsecond line\n")
	if n, err := w.Write(in); n != len(in) || err != nil {
		t.Fatalf("Write = %d, %v", n, err)
	}
	if got := strings.Count(logs.String(), `"level":"WARN"`); got != 2 {
		t.Fatalf("want 2 WARN lines, got %d: %s", got, logs.String())
	}

	buf := new(bytes.Buffer)
	h := newJSONOrderedHandler(buf, slog.LevelInfo)
	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug should be disabled at info level")
	}
	logger := slog.New(h).With("svc", "api").WithGroup("ignored")
	logger.Info("hello", slog.Any("fn", func() {}), slog.Attr{})
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("bad json %q", buf.String())
	}
	if entry["svc"] != "api" || entry["msg"] != "hello" {
		t.Fatalf("got %v", entry)
	}
	if s, _ := entry["fn"].(string); !strings.HasPrefix(s, "0x") { // unmarshalable value falls back to String()
		t.Fatalf("fn fallback: %v", entry["fn"])
	}
	// A record with zero time omits the time key.
	buf.Reset()
	_ = h.Handle(context.Background(), slog.NewRecord(time.Time{}, slog.LevelInfo, "no time", 0))
	if strings.Contains(buf.String(), `"time"`) {
		t.Fatalf("unexpected time: %s", buf.String())
	}
}

func TestInitLogger(t *testing.T) {
	prevLog, prevOut, prevErr := slog.Default(), gin.DefaultWriter, gin.DefaultErrorWriter
	t.Cleanup(func() { slog.SetDefault(prevLog); gin.DefaultWriter, gin.DefaultErrorWriter = prevOut, prevErr })

	t.Chdir(t.TempDir())
	cleanup, err := InitLogger()
	if err != nil {
		t.Fatal(err)
	}
	slog.Info("to file")
	cleanup()
	b, err := os.ReadFile("log/app.log")
	if err != nil || !strings.Contains(string(b), `"msg":"to file"`) {
		t.Fatalf("log file: %q %v", b, err)
	}
	if _, ok := gin.DefaultWriter.(*slogWriter); !ok {
		t.Error("gin.DefaultWriter not replaced")
	}

	t.Run("log is a file", func(t *testing.T) {
		t.Chdir(t.TempDir())
		_ = os.WriteFile("log", nil, 0o644)
		if _, err := InitLogger(); err == nil {
			t.Fatal("expected MkdirAll error")
		}
	})
	t.Run("app.log is a dir", func(t *testing.T) {
		t.Chdir(t.TempDir())
		_ = os.MkdirAll("log/app.log", 0o755)
		if _, err := InitLogger(); err == nil {
			t.Fatal("expected OpenFile error")
		}
	})
}

func TestContextHelpers(t *testing.T) {
	var nilCtx context.Context
	if _, ok := GetUserIDFromContext(nilCtx); ok {
		t.Error("nil ctx user")
	}
	if _, ok := GetTokenFromContext(nilCtx); ok {
		t.Error("nil ctx token")
	}
	if _, ok := GetUserIDFromContext(context.Background()); ok {
		t.Error("empty ctx user")
	}
	if _, ok := GetTokenFromContext(context.Background()); ok {
		t.Error("empty ctx token")
	}
	ctx := context.WithValue(WithUserID(context.Background(), 9), TokenKey, "tok")
	if id, ok := GetUserIDFromContext(ctx); !ok || id != 9 {
		t.Errorf("user = %d %v", id, ok)
	}
	if tok, ok := GetTokenFromContext(ctx); !ok || tok != "tok" {
		t.Errorf("token = %q %v", tok, ok)
	}
}

func TestJWTAuthRejectsNonUUIDSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens, _ := token.NewManager(strings.Repeat("a", 32), 1)
	access, _, _ := tokens.Issue(7, "not-a-uuid")
	r := gin.New()
	r.GET("/", JWTAuth(tokens, &activeSession{}), func(c *gin.Context) { t.Error("reached handler") })
	if w := serve(r, "GET", "/", "", map[string]string{"Authorization": "Bearer " + access}); w.Code != 401 || w.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("got %d", w.Code)
	}
}

func TestUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", asUser(1), Unavailable(), ok)
	if w := serve(r, "GET", "/", "", nil); w.Code != 403 {
		t.Fatalf("got %d, want 403", w.Code)
	}
}

func TestResourceAccessBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	defer db.Close()
	router := func(db *sql.DB) *gin.Engine {
		r := gin.New()
		r.Use(asUser(42), ResourceAccess(db))
		r.POST("/*any", func(c *gin.Context) {
			b, _ := io.ReadAll(c.Request.Body)
			c.String(200, "%v|%s", c.GetBool("is_admin"), b)
		})
		return r
	}

	r := router(db)
	for _, tc := range []struct {
		path, body string
		want       int
	}{
		{"/api/user", `{}`, 403},              // one segment
		{"/api/user/detail/extra", `{}`, 403}, // three segments
		{"/api/user/detail", `not json`, 400},
		{"/api/user/detail", strings.Repeat(" ", 16385) + `{}`, 400}, // over 16 KiB
		{"/api/area/detail", `{"id":1}`, 403},                        // unknown resource
		{"/api/history/detail", `{"id":1}`, 200},
	} {
		if w := serve(r, "POST", tc.path, tc.body, nil); w.Code != tc.want {
			t.Errorf("%s: got %d want %d (%s)", tc.path, w.Code, tc.want, w.Body.String())
		}
	}
	// Body is restored for downstream handlers.
	if w := serve(r, "POST", "/api/user/detail", `{"id":42}`, nil); w.Body.String() != `false|{"id":42}` {
		t.Fatalf("body not restored: %s", w.Body.String())
	}
}

func TestSensorKeyNoDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/", SensorKey(closedDB(t)), ok)
	for key, want := range map[string]int{"": 401, "short": 401, strings.Repeat("k", 257): 401, strings.Repeat("k", 32): 503} {
		if w := serve(r, "POST", "/", `{}`, map[string]string{"X-Sensor-Key": key}); w.Code != want {
			t.Errorf("key len %d: got %d want %d", len(key), w.Code, want)
		}
	}
}

func TestSensorKeyAgainstDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.OpenTestDB(t)
	key := strings.Repeat("s", 40)
	hash := sha256.Sum256([]byte(key))
	var sensorID int64
	if err := db.QueryRow(`INSERT INTO sensors (name, api_key_hash) VALUES ('s1', $1) RETURNING id`, hash[:]).Scan(&sensorID); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.POST("/ingest", SensorKey(db), func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		c.String(201, "%s", b)
	})
	good := fmt.Sprintf(`{"sensor_id":%d}`, sensorID)
	for _, tc := range []struct {
		name, key, body string
		want            int
	}{
		{"valid", key, good, 201},
		{"unknown key", strings.Repeat("x", 40), good, 401},
		{"other sensor", key, fmt.Sprintf(`{"sensor_id":%d}`, sensorID+1), 403},
		{"bad json", key, `{`, 400},
		{"too large", key, strings.Repeat(" ", 16385) + good, 400},
	} {
		w := serve(r, "POST", "/ingest", tc.body, map[string]string{"X-Sensor-Key": tc.key})
		if w.Code != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, w.Code, tc.want)
		}
		if tc.want == 201 && w.Body.String() != good {
			t.Errorf("body not restored: %q", w.Body.String())
		}
	}
}

// ownsArea/ownsSensor are currently unused by ResourceAccess; cover their queries directly.
func TestOwnershipQueries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.OpenTestDB(t)
	var owner, other, area, sensor int64
	_ = db.QueryRow(`INSERT INTO users (name,email,password) VALUES ('a','a@x.io','p') RETURNING id`).Scan(&owner)
	_ = db.QueryRow(`INSERT INTO users (name,email,password) VALUES ('b','b@x.io','p') RETURNING id`).Scan(&other)
	if err := db.QueryRow(`INSERT INTO areas (name, owner_id) VALUES ('field', $1) RETURNING id`, owner).Scan(&area); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO sensors (name, owner_id) VALUES ('s', $1) RETURNING id`, owner).Scan(&sensor); err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	for _, tc := range []struct {
		fn   func(*gin.Context, *sql.DB, int64, int64) (bool, error)
		user int64
		id   int64
		want bool
	}{{ownsArea, owner, area, true}, {ownsArea, other, area, false}, {ownsSensor, owner, sensor, true}, {ownsSensor, other, sensor, false}} {
		if got, err := tc.fn(c, db, tc.user, tc.id); err != nil || got != tc.want {
			t.Errorf("user %d id %d: got %v %v want %v", tc.user, tc.id, got, err, tc.want)
		}
	}
}
