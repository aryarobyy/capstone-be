package alert

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"capstone-be/internal/middleware"
	"capstone-be/internal/testutil"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// router mounts the real routes behind a stub auth that sets the given user.
func router(db *sql.DB, user int64) *gin.Engine {
	r := gin.New()
	api := r.Group("/api", func(c *gin.Context) { c.Set(middleware.UserIDKey, user); c.Next() })
	RegisterRoutes(api, db)
	return r
}

func do(r http.Handler, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func closedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	return db
}

func seedUser(t *testing.T, db *sql.DB, email string, admin bool) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO users(name,email,password,is_admin) VALUES('u',$1,'x',$2) RETURNING id`, email, admin).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

const validRule = `{"sensor_id":%d,"parameter":"temperature","max_value":30,"duration_seconds":0,"cooldown_seconds":60,"max_gap_seconds":300,"enabled":true}`

func TestHandlersWithoutDB(t *testing.T) {
	r := router(closedDB(t), 1)
	cases := []struct {
		path, body string
		want       int
	}{
		{"/api/alerts/rules", `{`, 400},
		{"/api/alerts/rules", `{"sensor_id":1,"parameter":"pressure","max_value":1,"cooldown_seconds":60,"max_gap_seconds":1}`, 400},
		{"/api/alerts/rules", `{"sensor_id":1,"parameter":"temperature","cooldown_seconds":60,"max_gap_seconds":1}`, 400},                                  // no bounds
		{"/api/alerts/rules/save", `{"sensor_id":1,"parameter":"temperature","min_value":5,"max_value":5,"cooldown_seconds":60,"max_gap_seconds":1}`, 400}, // min >= max
		{"/api/alerts/rules", `{"sensor_id":1,"parameter":"temperature","max_value":5,"cooldown_seconds":60,"max_gap_seconds":1}`, 500},
		{"/api/alerts/rules/list", `{}`, 400},
		{"/api/alerts/rules/list?sensor_id=abc", ``, 400},
		{"/api/alerts/rules/list", `{"sensor_id":1}`, 500},
		{"/api/sensors/key", `{"sensor_id":0}`, 400},
		{"/api/sensors/key", `{"sensor_id":1}`, 500},
		{"/api/areas", `{"name":""}`, 400},
		{"/api/areas/create", `{"name":"` + strings.Repeat("a", 256) + `"}`, 400},
		{"/api/areas", `{"name":"x"}`, 500},
		{"/api/areas/list", ``, 500},
		{"/api/areas/detail", `{"id":0}`, 400},
		{"/api/areas/detail", `{"id":1}`, 500},
		{"/api/areas/owner", `{"id":1,"owner_id":2}`, 403},
	}
	for _, c := range cases {
		if w := do(r, c.path, c.body); w.Code != c.want {
			t.Errorf("%s %s = %d, want %d (%s)", c.path, c.body, w.Code, c.want, w.Body)
		}
	}
	// AssignArea storage failure, bypassing the admin middleware.
	g := gin.New()
	g.POST("/owner", func(c *gin.Context) { c.Set(middleware.UserIDKey, int64(1)) }, NewHandler(NewService(closedDB(t))).AssignArea)
	if w := do(g, "/owner", `{"id":1,"owner_id":2}`); w.Code != 500 || !strings.Contains(w.Body.String(), "operation failed") {
		t.Errorf("AssignArea = %d %s", w.Code, w.Body)
	}
}

func TestSaveRuleRejectsNonFinite(t *testing.T) {
	s := NewService(closedDB(t))
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		v := v
		if err := s.SaveRule(context.Background(), 1, Rule{Min: &v}); !errors.Is(err, ErrInvalidRule) {
			t.Errorf("min %v: %v", v, err)
		}
		if err := s.SaveRule(context.Background(), 1, Rule{Max: &v}); !errors.Is(err, ErrInvalidRule) {
			t.Errorf("max %v: %v", v, err)
		}
	}
}

func TestRulesFlow(t *testing.T) {
	db := testutil.OpenTestDB(t)
	r := router(db, seedUser(t, db, "u@x.io", false))
	sensor := seedSensor(t, db)

	if w := do(r, "/api/alerts/rules", strings.Replace(validRule, "%d", "999", 1)); w.Code != 403 {
		t.Fatalf("unknown sensor save = %d", w.Code)
	}
	body := strings.Replace(validRule, "%d", jsonInt(sensor), 1)
	if w := do(r, "/api/alerts/rules", body); w.Code != 204 {
		t.Fatalf("save = %d %s", w.Code, w.Body)
	}

	// Open an anomaly, then re-saving the rule resolves it and clears state.
	eval(t, db, sensor, t0, map[string]float64{"temperature": 50})
	if w := do(r, "/api/alerts/rules/save", strings.Replace(body, `"max_value":30`, `"min_value":-5,"max_value":45`, 1)); w.Code != 204 {
		t.Fatalf("resave = %d", w.Code)
	}
	if n := testutil.Count(t, db, `SELECT count(*) FROM anomalies WHERE status='NORMAL' AND resolved_at IS NOT NULL`); n != 1 {
		t.Fatalf("resolved anomalies = %d", n)
	}
	if n := testutil.Count(t, db, `SELECT count(*) FROM alert_states`); n != 0 {
		t.Fatalf("states = %d", n)
	}

	for _, req := range []struct{ path, body string }{
		{"/api/alerts/rules/list", `{"sensor_id":` + jsonInt(sensor) + `}`},
		{"/api/alerts/rules/list?sensor_id=" + jsonInt(sensor), ``},
	} {
		w := do(r, req.path, req.body)
		var rules []Rule
		if w.Code != 200 || data(w, &rules) != nil || len(rules) != 1 {
			t.Fatalf("list %s = %d %s", req.path, w.Code, w.Body)
		}
		if got := rules[0]; got.Min == nil || *got.Min != -5 || *got.Max != 45 || got.Cooldown != 60 || got.MaxGap != 300 || !got.Enabled {
			t.Fatalf("rule = %+v", got)
		}
	}
	if w := do(r, "/api/alerts/rules/list", `{"sensor_id":999}`); w.Code != 403 {
		t.Fatalf("unknown sensor list = %d", w.Code)
	}
}

func TestSensorKey(t *testing.T) {
	db := testutil.OpenTestDB(t)
	r := router(db, seedUser(t, db, "u@x.io", false))
	sensor := seedSensor(t, db)

	if w := do(r, "/api/sensors/key", `{"sensor_id":999}`); w.Code != 403 {
		t.Fatalf("unknown = %d", w.Code)
	}
	w := do(r, "/api/sensors/key", `{"sensor_id":`+jsonInt(sensor)+`}`)
	var out struct {
		Key string `json:"sensor_key"`
	}
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || data(w, &out) != nil || len(out.Key) != 43 {
		t.Fatalf("key = %d %s", w.Code, w.Body)
	}
	hash := sha256.Sum256([]byte(out.Key))
	if n := testutil.Count(t, db, `SELECT count(*) FROM sensors WHERE id=$1 AND api_key_hash=$2`, sensor, hash[:]); n != 1 {
		t.Fatal("hash not stored")
	}
}

func TestAreas(t *testing.T) {
	db := testutil.OpenTestDB(t)
	user := seedUser(t, db, "u@x.io", false)
	admin := seedUser(t, db, "a@x.io", true)
	r := router(db, user)

	var created struct{ ID int64 }
	w := do(r, "/api/areas", `{"name":"North"}`)
	if w.Code != 201 || data(w, &created) != nil || created.ID == 0 {
		t.Fatalf("create = %d %s", w.Code, w.Body)
	}
	do(router(db, admin), "/api/areas/create", `{"name":"Admin"}`)

	w = do(r, "/api/areas/list", ``)
	var areas []Area
	if w.Code != 200 || data(w, &areas) != nil || len(areas) != 1 || areas[0].Name != "North" {
		t.Fatalf("list = %d %s", w.Code, w.Body)
	}

	var detail Area
	if w := do(r, "/api/areas/detail", `{"id":`+jsonInt(created.ID)+`}`); w.Code != 200 || data(w, &detail) != nil || detail.Name != "North" {
		t.Fatalf("detail = %d %s", w.Code, w.Body)
	}
	if w := do(router(db, admin), "/api/areas/detail", `{"id":`+jsonInt(created.ID)+`}`); w.Code != 404 {
		t.Fatalf("other user's area detail = %d", w.Code)
	}

	if w := do(r, "/api/areas/owner", `{"id":1,"owner_id":2}`); w.Code != 403 {
		t.Fatalf("area owner reassignment = %d", w.Code)
	}

}

func jsonInt(v int64) string { b, _ := json.Marshal(v); return string(b) }

// data decodes the "data" field of the standard response envelope into v.
func data(w *httptest.ResponseRecorder, v any) error {
	var env struct{ Data json.RawMessage }
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		return err
	}
	return json.Unmarshal(env.Data, v)
}
