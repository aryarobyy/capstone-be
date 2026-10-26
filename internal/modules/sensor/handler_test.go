package sensor

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"capstone-be/internal/middleware"

	"github.com/gin-gonic/gin"
)

type mockSensorService struct {
	err       error
	gotOwner  int64
	gotCreate CreateSensorRequest
}

func (m *mockSensorService) Create(ctx context.Context, req CreateSensorRequest) (*SensorResponse, error) {
	m.gotOwner, m.gotCreate = req.OwnerID, req
	if m.err != nil {
		return nil, m.err
	}
	return &SensorResponse{ID: 1, Name: req.Name, OwnerID: req.OwnerID}, nil
}
func (m *mockSensorService) Update(ctx context.Context, req UpdateSensorRequest) (*SensorResponse, error) {
	m.gotOwner = req.OwnerID
	if m.err != nil {
		return nil, m.err
	}
	return &SensorResponse{ID: req.ID}, nil
}
func (m *mockSensorService) Delete(ctx context.Context, req DeleteSensorRequest) error {
	m.gotOwner = req.OwnerID
	return m.err
}
func (m *mockSensorService) List(ctx context.Context, req ListSensorRequest) (*ListSensorResponse, error) {
	m.gotOwner = req.OwnerID
	if m.err != nil {
		return nil, m.err
	}
	return &ListSensorResponse{Data: []ListSensorData{{ID: 1}}, Total: 1, Limit: 10}, nil
}
func (m *mockSensorService) Detail(ctx context.Context, req DetailSensorRequest) (*SensorResponse, error) {
	m.gotOwner = req.OwnerID
	if m.err != nil {
		return nil, m.err
	}
	return &SensorResponse{ID: req.ID}, nil
}

// serve runs one request through a router; setUser mimics middleware.JWTAuth.
func serve(t *testing.T, svc SensorService, path, body string, setUser func(*gin.Context)) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if setUser != nil {
		r.Use(func(c *gin.Context) { setUser(c); c.Next() })
	}
	h := NewSensorHandler(svc)
	g := r.Group("/sensor")
	g.POST("/create", h.Create)
	g.POST("/list", h.List)
	g.POST("/detail", h.Detail)
	g.POST("/update", h.Update)
	g.POST("/delete", h.Delete)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func asUser(id int64) func(*gin.Context) {
	return func(c *gin.Context) {
		c.Set(middleware.UserIDKey, id)
		c.Request = c.Request.WithContext(middleware.WithUserID(c.Request.Context(), id))
	}
}

func TestSensorHandler_StatusCodes(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name string
		path string
		body string
		err  error
		want int
	}{
		{"create ok", "/sensor/create", `{"name":"n","code":"c"}`, nil, http.StatusCreated},
		{"create bad json", "/sensor/create", `{`, nil, http.StatusBadRequest},
		{"create missing field", "/sensor/create", `{"name":"n"}`, nil, http.StatusBadRequest},
		{"create area not found", "/sensor/create", `{"name":"n","code":"c","area_id":9}`, ErrAreaNotFound, http.StatusBadRequest},
		{"create internal", "/sensor/create", `{"name":"n","code":"c"}`, boom, http.StatusInternalServerError},

		{"list ok", "/sensor/list", `{}`, nil, http.StatusOK},
		{"list empty body", "/sensor/list", ``, nil, http.StatusBadRequest},
		{"list internal", "/sensor/list", `{}`, boom, http.StatusInternalServerError},

		{"detail ok", "/sensor/detail", `{"id":1}`, nil, http.StatusOK},
		{"detail missing id", "/sensor/detail", `{}`, nil, http.StatusBadRequest},
		{"detail not found", "/sensor/detail", `{"id":1}`, ErrSensorNotFound, http.StatusNotFound},
		{"detail internal", "/sensor/detail", `{"id":1}`, boom, http.StatusInternalServerError},

		{"update ok", "/sensor/update", `{"id":1,"name":"x"}`, nil, http.StatusOK},
		{"update wrong type", "/sensor/update", `{"id":"one"}`, nil, http.StatusBadRequest},
		{"update not found", "/sensor/update", `{"id":1}`, ErrSensorNotFound, http.StatusNotFound},
		{"update area not found", "/sensor/update", `{"id":1,"area_id":2}`, ErrAreaNotFound, http.StatusBadRequest},
		{"update internal", "/sensor/update", `{"id":1}`, boom, http.StatusInternalServerError},

		{"delete ok", "/sensor/delete", `{"id":1}`, nil, http.StatusOK},
		{"delete missing id", "/sensor/delete", `{}`, nil, http.StatusBadRequest},
		{"delete not found", "/sensor/delete", `{"id":1}`, ErrSensorNotFound, http.StatusNotFound},
		{"delete internal", "/sensor/delete", `{"id":1}`, boom, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockSensorService{err: tt.err}
			w := serve(t, svc, tt.path, tt.body, asUser(7))
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d, body %s", w.Code, tt.want, w.Body)
			}
			var resp struct {
				Success bool `json:"success"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("bad json: %v", err)
			}
			if resp.Success != (tt.want < 300) {
				t.Fatalf("success = %v for status %d", resp.Success, w.Code)
			}
			if tt.want < 300 && svc.gotOwner != 7 {
				t.Fatalf("owner = %d, want 7", svc.gotOwner)
			}
		})
	}
}

func TestSensorHandler_CreateResponseBody(t *testing.T) {
	svc := &mockSensorService{}
	w := serve(t, svc, "/sensor/create", `{"name":"Soil","code":"S1","area_id":3,"description":"d"}`, asUser(4))
	var resp struct {
		Data SensorResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Name != "Soil" || resp.Data.OwnerID != 4 {
		t.Fatalf("unexpected body %s", w.Body)
	}
	if svc.gotCreate.AreaID == nil || *svc.gotCreate.AreaID != 3 || svc.gotCreate.Description == nil || *svc.gotCreate.Description != "d" {
		t.Fatalf("unexpected request %+v", svc.gotCreate)
	}
}

func TestGetUserID(t *testing.T) {
	cases := []struct {
		name    string
		setUser func(*gin.Context)
		want    int64
	}{
		{"request context", func(c *gin.Context) {
			c.Request = c.Request.WithContext(middleware.WithUserID(c.Request.Context(), 11))
		}, 11},
		{"gin keys fallback", func(c *gin.Context) { c.Set(middleware.UserIDKey, int64(12)) }, 12},
		{"gin keys wrong type", func(c *gin.Context) { c.Set(middleware.UserIDKey, "12") }, 0},
		{"anonymous", nil, 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockSensorService{}
			w := serve(t, svc, "/sensor/list", `{}`, tt.setUser)
			if w.Code != http.StatusOK || svc.gotOwner != tt.want {
				t.Fatalf("status %d owner %d, want %d", w.Code, svc.gotOwner, tt.want)
			}
		})
	}
}
