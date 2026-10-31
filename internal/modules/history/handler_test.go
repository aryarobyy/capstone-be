package history

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"capstone-be/internal/middleware"

	"github.com/gin-gonic/gin"
)

type mockHistoryService struct {
	createResponse *HistoryResponse
	listResponse   *ListHistoryResponse
	detailResponse *HistoryResponse
	newestResponse *HistoryResponse
	streamReq      StreamHistoryRequest
	sinceAfter     []int64
	onSince        func()
	err            error
}

// Since serves anomaly 5 as the newest existing one and 6 as recorded later.
func (m *mockHistoryService) Since(_ context.Context, req StreamHistoryRequest, afterID int64, limit int) ([]HistoryResponse, error) {
	m.streamReq = req
	m.sinceAfter = append(m.sinceAfter, afterID)
	if m.err != nil {
		return nil, m.err
	}
	if limit == 1 {
		return []HistoryResponse{{ID: 5}}, nil
	}
	if m.onSince != nil {
		m.onSince()
	}
	return []HistoryResponse{{ID: 6, Parameter: "soil_moisture", Status: AnomalyStatusWarning}}, nil
}

func (m *mockHistoryService) Create(_ context.Context, _ CreateHistoryRequest) (*HistoryResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.createResponse, nil
}

func (m *mockHistoryService) List(_ context.Context, _ ListHistoryRequest) (*ListHistoryResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.listResponse, nil
}

func (m *mockHistoryService) Newest(_ context.Context, _ NewestHistoryRequest) (*HistoryResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.newestResponse, nil
}

func (m *mockHistoryService) Detail(_ context.Context, _ DetailHistoryRequest) (*HistoryResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.detailResponse, nil
}

func (m *mockHistoryService) Delete(_ context.Context, _ DeleteHistoryRequest) error {
	return m.err
}

func TestHistoryHandler_Stream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithCancel(context.Background())
	mockSvc := &mockHistoryService{onSince: cancel} // client leaves after the first poll
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(middleware.UserIDKey, int64(42)); c.Next() })
	r.POST("/stream", NewHistoryHandler(mockSvc).Stream)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/stream", strings.NewReader(`{"status":"WARNING"}`)).WithContext(ctx)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "event:anomaly") || !strings.Contains(body, `"id":6`) || strings.Contains(body, `"id":5`) {
		t.Fatalf("existing anomaly must be skipped, new one sent: %q", body)
	}
	if len(mockSvc.sinceAfter) != 2 || mockSvc.sinceAfter[1] != 5 {
		t.Fatalf("since calls = %v", mockSvc.sinceAfter)
	}
	if mockSvc.streamReq.Status != "WARNING" || mockSvc.streamReq.OwnerID != 42 {
		t.Fatalf("req = %+v", mockSvc.streamReq)
	}

	// Reconnect with after_id resumes there instead of skipping to the newest.
	ctx, cancel = context.WithCancel(context.Background())
	mockSvc = &mockHistoryService{onSince: cancel}
	r = gin.New()
	r.Use(func(c *gin.Context) { c.Set(middleware.UserIDKey, int64(42)); c.Next() })
	r.POST("/stream", NewHistoryHandler(mockSvc).Stream)
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/stream", strings.NewReader(`{"after_id":3}`)).WithContext(ctx))
	if len(mockSvc.sinceAfter) != 1 || mockSvc.sinceAfter[0] != 3 {
		t.Fatalf("since calls = %v", mockSvc.sinceAfter)
	}
}

func TestHistoryHandler_Create(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now()
	mockSvc := &mockHistoryService{
		createResponse: &HistoryResponse{
			ID:                  1,
			SensorID:            2,
			Parameter:           "temperature",
			StartedAt:           now,
			LastDetectedAt:      now,
			AccumulatedDuration: 0,
			Status:              AnomalyStatusFatal,
			CreatedAt:           now,
			UpdatedAt:           now,
		},
	}

	handler := NewHistoryHandler(mockSvc)
	router := gin.New()
	router.POST("/history/create", handler.Create)

	body, _ := json.Marshal(CreateHistoryRequest{
		SensorID:  2,
		Parameter: "temperature",
		StartedAt: now,
		Status:    AnomalyStatusFatal,
	})

	req, err := http.NewRequest(http.MethodPost, "/history/create", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool            `json:"success"`
		Message string          `json:"message"`
		Data    HistoryResponse `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success to be true")
	}
	if resp.Data.ID != 1 || resp.Data.SensorID != 2 {
		t.Errorf("unexpected data: %+v", resp.Data)
	}
}

func TestHistoryHandler_Create_InvalidStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockSvc := &mockHistoryService{
		err: ErrInvalidAnomalyStatus,
	}

	handler := NewHistoryHandler(mockSvc)
	router := gin.New()
	router.POST("/history/create", handler.Create)

	body, _ := json.Marshal(CreateHistoryRequest{
		SensorID:  2,
		Parameter: "temperature",
		StartedAt: time.Now(),
		Status:    "INVALID",
	})

	req, err := http.NewRequest(http.MethodPost, "/history/create", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHistoryHandler_Create_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockSvc := &mockHistoryService{}
	handler := NewHistoryHandler(mockSvc)
	router := gin.New()
	router.POST("/history/create", handler.Create)

	req, err := http.NewRequest(http.MethodPost, "/history/create", bytes.NewBufferString("{invalid-json}"))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHistoryHandler_Newest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockSvc := &mockHistoryService{newestResponse: &HistoryResponse{ID: 7, SensorID: 2}}
	router := gin.New()
	router.POST("/history/newest", NewHistoryHandler(mockSvc).Newest)

	req, _ := http.NewRequest(http.MethodPost, "/history/newest", bytes.NewBufferString(`{"sensor_id":2}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp struct {
		Data HistoryResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || w.Code != http.StatusOK || resp.Data.ID != 7 {
		t.Fatalf("expected 200 with id 7, got %d. Body: %s", w.Code, w.Body.String())
	}

	mockSvc.err = ErrHistoryNotFound
	req, _ = http.NewRequest(http.MethodPost, "/history/newest", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func serve(t *testing.T, h gin.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/x", h)
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestHistoryHandler_StatusCodes(t *testing.T) {
	boom := errors.New("boom")
	create := `{"sensor_id":1,"parameter":"ph","started_at":"2026-10-19T08:00:00Z","status":"WARNING"}`
	ok := &mockHistoryService{
		listResponse:   &ListHistoryResponse{Data: []HistoryResponse{}},
		detailResponse: &HistoryResponse{ID: 3},
		newestResponse: &HistoryResponse{ID: 4},
	}
	type pick func(*HistoryHandler) gin.HandlerFunc
	createH := pick(func(h *HistoryHandler) gin.HandlerFunc { return h.Create })
	listH := pick(func(h *HistoryHandler) gin.HandlerFunc { return h.List })
	newestH := pick(func(h *HistoryHandler) gin.HandlerFunc { return h.Newest })
	detailH := pick(func(h *HistoryHandler) gin.HandlerFunc { return h.Detail })
	deleteH := pick(func(h *HistoryHandler) gin.HandlerFunc { return h.Delete })
	cases := []struct {
		name string
		svc  *mockHistoryService
		h    pick
		body string
		want int
	}{
		{"create missing fields", ok, createH, `{}`, 400},
		{"create svc error", &mockHistoryService{err: boom}, createH, create, 500},
		{"create not found", &mockHistoryService{err: ErrHistoryNotFound}, createH, create, 404},
		{"list bad json", ok, listH, `{`, 400},
		{"list svc error", &mockHistoryService{err: boom}, listH, `{}`, 500},
		{"list ok", ok, listH, `{}`, 200},
		{"newest bad json", ok, newestH, `{`, 400},
		{"newest svc error", &mockHistoryService{err: boom}, newestH, `{}`, 500},
		{"detail missing id", ok, detailH, `{}`, 400},
		{"detail not found", &mockHistoryService{err: ErrHistoryNotFound}, detailH, `{"id":1}`, 404},
		{"detail svc error", &mockHistoryService{err: boom}, detailH, `{"id":1}`, 500},
		{"detail ok", ok, detailH, `{"id":3}`, 200},
		{"delete missing id", ok, deleteH, `{}`, 400},
		{"delete not found", &mockHistoryService{err: ErrHistoryNotFound}, deleteH, `{"id":1}`, 404},
		{"delete svc error", &mockHistoryService{err: boom}, deleteH, `{"id":1}`, 500},
		{"delete ok", ok, deleteH, `{"id":1}`, 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if w := serve(t, tc.h(NewHistoryHandler(tc.svc)), tc.body); w.Code != tc.want {
				t.Fatalf("want %d, got %d: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r.Group("/api"), nil)
	got := map[string]bool{}
	for _, ri := range r.Routes() {
		got[ri.Method+" "+ri.Path] = true
	}
	for _, p := range []string{"create", "list", "newest", "detail", "delete"} {
		if !got["POST /api/history/"+p] {
			t.Errorf("missing route POST /api/history/%s", p)
		}
	}
}

func TestHistoryHandler_StreamErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name string
		body string
		err  error
		code int
	}{
		{"invalid json", `{`, nil, http.StatusBadRequest},
		{"cannot find newest", `{}`, errors.New("db down"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.POST("/stream", NewHistoryHandler(&mockHistoryService{err: tc.err}).Stream)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/stream", strings.NewReader(tc.body)))
			if w.Code != tc.code || strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
				t.Fatalf("code=%d headers=%v", w.Code, w.Header())
			}
		})
	}
}
