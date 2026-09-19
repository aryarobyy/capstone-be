package middleware

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

var (
	mockDriverOnce sync.Once
	mockState      struct {
		sync.Mutex
		isAdmin bool
		owns    bool
	}
)

type testDriver struct{}

func (d *testDriver) Open(name string) (driver.Conn, error) {
	return &testConn{}, nil
}

type testConn struct{}

func (c *testConn) Prepare(query string) (driver.Stmt, error) {
	return &testStmt{query: query}, nil
}
func (c *testConn) Close() error              { return nil }
func (c *testConn) Begin() (driver.Tx, error) { return &testTx{}, nil }

type testTx struct{}

func (t *testTx) Commit() error   { return nil }
func (t *testTx) Rollback() error { return nil }

type testStmt struct {
	query string
}

func (s *testStmt) Close() error { return nil }
func (s *testStmt) NumInput() int {
	return -1
}
func (s *testStmt) Exec(args []driver.Value) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}
func (s *testStmt) Query(args []driver.Value) (driver.Rows, error) {
	mockState.Lock()
	defer mockState.Unlock()

	// If querying is_admin
	val := mockState.owns
	if len(s.query) > 0 && (s.query == `SELECT is_admin FROM users WHERE id=$1`) {
		val = mockState.isAdmin
	}

	return &testRows{
		columns: []string{"val"},
		rows:    [][]driver.Value{{val}},
	}, nil
}

type testRows struct {
	columns []string
	rows    [][]driver.Value
	idx     int
}

func (r *testRows) Columns() []string { return r.columns }
func (r *testRows) Close() error      { return nil }
func (r *testRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	for i, v := range r.rows[r.idx] {
		dest[i] = v
	}
	r.idx++
	return nil
}

func getTestDB(t *testing.T) *sql.DB {
	mockDriverOnce.Do(func() {
		sql.Register("access_mock_driver", &testDriver{})
	})
	db, err := sql.Open("access_mock_driver", "test")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}

func TestResourceAccess_ListEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	defer db.Close()

	mockState.Lock()
	mockState.isAdmin = false
	mockState.owns = false
	mockState.Unlock()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, int64(42))
		c.Next()
	})
	r.Use(ResourceAccess(db))

	r.POST("/api/sensor/list", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.POST("/api/sensor-reading/list", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.POST("/api/history/list", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.POST("/api/user/list", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.POST("/api/sensor/detail", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	t.Run("sensor list with area_id is allowed for non-admin without owning area", func(t *testing.T) {
		body := bytes.NewBufferString(`{"area_id": 1}`)
		req := httptest.NewRequest(http.MethodPost, "/api/sensor/list", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d with body %s", w.Code, w.Body.String())
		}
	})

	t.Run("sensor list without area_id is allowed", func(t *testing.T) {
		body := bytes.NewBufferString(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/api/sensor/list", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d with body %s", w.Code, w.Body.String())
		}
	})

	t.Run("sensor-reading list is allowed", func(t *testing.T) {
		body := bytes.NewBufferString(`{"sensor_id": 1}`)
		req := httptest.NewRequest(http.MethodPost, "/api/sensor-reading/list", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d with body %s", w.Code, w.Body.String())
		}
	})

	t.Run("history list is allowed", func(t *testing.T) {
		body := bytes.NewBufferString(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/api/history/list", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d with body %s", w.Code, w.Body.String())
		}
	})

	t.Run("non-list endpoint like sensor detail is denied if not owned", func(t *testing.T) {
		body := bytes.NewBufferString(`{"id": 999}`)
		req := httptest.NewRequest(http.MethodPost, "/api/sensor/detail", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d with body %s", w.Code, w.Body.String())
		}
	})
}
