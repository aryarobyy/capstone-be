package sensorreading

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"capstone-be/internal/middleware"

	"github.com/gin-gonic/gin"
)

type mockSensorReadingService struct {
	listResponse    *ListSensorReadingResponse
	summaryResponse *SensorSummaryResponse
	detailResponse  *SensorReadingResponse
	ownerID         int64
	summaryReq      SensorSummaryRequest
	err             error
}

func (m *mockSensorReadingService) Create(ctx context.Context, req CreateSensorReadingRequest) error {
	return m.err
}

func (m *mockSensorReadingService) List(ctx context.Context, req ListSensorReadingRequest) (*ListSensorReadingResponse, error) {
	return m.listResponse, m.err
}

func (m *mockSensorReadingService) Detail(ctx context.Context, req DetailSensorReadingRequest) (*SensorReadingResponse, error) {
	return m.detailResponse, m.err
}

func (m *mockSensorReadingService) Delete(ctx context.Context, req DeleteSensorReadingRequest) error {
	return m.err
}

func (m *mockSensorReadingService) Summary(ctx context.Context, req SensorSummaryRequest, ownerID int64) (*SensorSummaryResponse, error) {
	m.ownerID, m.summaryReq = ownerID, req
	return m.summaryResponse, m.err
}

func TestSensorReadingHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now()
	soilMoisture := 48.5
	temperature := 27.0
	humidity := 70.5
	mockSvc := &mockSensorReadingService{
		listResponse: &ListSensorReadingResponse{
			Data: []SensorReadingResponse{
				{
					ID:           1,
					SensorID:     3,
					SoilMoisture: soilMoisture,
					Temperature:  temperature,
					Humidity:     humidity,
					RecordedAt:   now,
				},
			},
			Total: 45,
			Limit: 10,
			Index: 10,
		},
	}

	handler := NewSensorReadingHandler(mockSvc)

	router := gin.New()
	router.POST("/api/sensor-readings/list", handler.List)

	body, _ := json.Marshal(ListSensorReadingRequest{
		Limit: 10,
		Index: 10,
	})

	req, err := http.NewRequest(http.MethodPost, "/api/sensor-readings/list", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool                      `json:"success"`
		Message string                    `json:"message"`
		Data    ListSensorReadingResponse `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success to be true")
	}
	if resp.Data.Total != 45 {
		t.Errorf("expected total 45, got %d", resp.Data.Total)
	}
	if resp.Data.Limit != 10 {
		t.Errorf("expected limit 10, got %d", resp.Data.Limit)
	}
	if resp.Data.Index != 10 {
		t.Errorf("expected index 10, got %d", resp.Data.Index)
	}
	if len(resp.Data.Data) != 1 {
		t.Errorf("expected 1 item, got %d", len(resp.Data.Data))
	}
}

func TestSensorReadingHandler_Summary(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now()
	mockSvc := &mockSensorReadingService{
		summaryResponse: &SensorSummaryResponse{
			WindowMinutes: 10,
			Data: []SensorSummaryItem{
				{
					SensorID:     1,
					SensorName:   "Moisture GH1",
					TotalSamples: 15,
					Latest: LatestTelemetry{
						SoilMoisture: 45.0,
						Temperature:  28.0,
						Humidity:     70.0,
						RecordedAt:   now,
					},
					Average: TelemetryValues{
						SoilMoisture: 44.5,
						Temperature:  27.8,
						Humidity:     69.5,
					},
					Status:       "OPTIMAL",
					CalculatedAt: now,
				},
			},
		},
	}

	handler := NewSensorReadingHandler(mockSvc)
	router := gin.New()
	router.POST("/api/sensor-reading/summary", handler.Summary)

	body, _ := json.Marshal(SensorSummaryRequest{SensorID: 1, WindowMinutes: 10})
	req, _ := http.NewRequest(http.MethodPost, "/api/sensor-reading/summary", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Success bool                  `json:"success"`
		Message string                `json:"message"`
		Data    SensorSummaryResponse `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success to be true")
	}
	if len(resp.Data.Data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Data.Data))
	}
	if resp.Data.Data[0].SensorID != 1 {
		t.Errorf("expected sensor_id 1, got %d", resp.Data.Data[0].SensorID)
	}
	if resp.Data.Data[0].Status != "OPTIMAL" {
		t.Errorf("expected status OPTIMAL, got %s", resp.Data.Data[0].Status)
	}
}


func serve(t *testing.T, h gin.HandlerFunc, body string, pre ...gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/x", append(pre, h)...)
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestSensorReadingHandler_StatusCodes(t *testing.T) {
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	past := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	boom := errors.New("boom")
	cases := []struct {
		name string
		svc  *mockSensorReadingService
		pick func(*SensorReadingHandler) gin.HandlerFunc
		body string
		want int
	}{
		{"create bad json", &mockSensorReadingService{}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Create }, `{`, 400},
		{"create negative sensor", &mockSensorReadingService{}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Create }, `{"sensor_id":-1,"recorded_at":"` + past + `"}`, 400},
		{"create future", &mockSensorReadingService{}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Create }, `{"sensor_id":1,"recorded_at":"` + future + `"}`, 400},
		{"create svc error", &mockSensorReadingService{err: boom}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Create }, `{"sensor_id":1,"recorded_at":"` + past + `"}`, 500},
		{"create ok", &mockSensorReadingService{}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Create }, `{"sensor_id":1,"recorded_at":"` + past + `"}`, 201},
		{"list bad json", &mockSensorReadingService{}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.List }, `{`, 400},
		{"list svc error", &mockSensorReadingService{err: boom}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.List }, `{}`, 500},
		{"detail bad json", &mockSensorReadingService{}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Detail }, `{}`, 400},
		{"detail not found", &mockSensorReadingService{err: ErrSensorReadingNotFound}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Detail }, `{"id":1}`, 404},
		{"detail svc error", &mockSensorReadingService{err: boom}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Detail }, `{"id":1}`, 500},
		{"detail ok", &mockSensorReadingService{detailResponse: &SensorReadingResponse{ID: 1}}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Detail }, `{"id":1}`, 200},
		{"delete bad json", &mockSensorReadingService{}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Delete }, `{}`, 400},
		{"delete not found", &mockSensorReadingService{err: ErrSensorReadingNotFound}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Delete }, `{"id":1}`, 404},
		{"delete svc error", &mockSensorReadingService{err: boom}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Delete }, `{"id":1}`, 500},
		{"delete ok", &mockSensorReadingService{}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Delete }, `{"id":1}`, 200},
		{"summary svc error", &mockSensorReadingService{err: boom}, func(h *SensorReadingHandler) gin.HandlerFunc { return h.Summary }, `{}`, 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := serve(t, tc.pick(NewSensorReadingHandler(tc.svc)), tc.body)
			if w.Code != tc.want {
				t.Fatalf("want %d, got %d: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestSensorReadingHandler_SummaryOwnerAndDefaults(t *testing.T) {
	svc := &mockSensorReadingService{summaryResponse: &SensorSummaryResponse{}}
	h := NewSensorReadingHandler(svc)

	// Invalid body falls back to the default 10 minute window; no user -> owner 0.
	if w := serve(t, h.Summary, `not json`); w.Code != 200 || svc.summaryReq.WindowMinutes != 10 || svc.ownerID != 0 {
		t.Fatalf("got %d req=%+v owner=%d", w.Code, svc.summaryReq, svc.ownerID)
	}
	// Owner from request context.
	fromCtx := func(c *gin.Context) { c.Request = c.Request.WithContext(middleware.WithUserID(c.Request.Context(), 7)) }
	if serve(t, h.Summary, `{}`, fromCtx); svc.ownerID != 7 {
		t.Fatalf("owner from ctx = %d", svc.ownerID)
	}
	// Owner from gin keys.
	fromKeys := func(c *gin.Context) { c.Set(middleware.UserIDKey, int64(9)) }
	if serve(t, h.Summary, `{}`, fromKeys); svc.ownerID != 9 {
		t.Fatalf("owner from keys = %d", svc.ownerID)
	}
	// Wrong type in gin keys is ignored.
	badKey := func(c *gin.Context) { c.Set(middleware.UserIDKey, "9") }
	if serve(t, h.Summary, `{}`, badKey); svc.ownerID != 0 {
		t.Fatalf("owner from bad key = %d", svc.ownerID)
	}
}

func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r.Group("/api"), r.Group("/device"), nil)
	got := map[string]bool{}
	for _, ri := range r.Routes() {
		got[ri.Method+" "+ri.Path] = true
	}
	for _, p := range []string{"/device/sensor-reading/data/list", "/api/sensor-reading/create", "/api/sensor-reading/list", "/api/sensor-reading/detail", "/api/sensor-reading/delete", "/api/sensor-reading/summary"} {
		if !got["POST "+p] {
			t.Errorf("missing route POST %s", p)
		}
	}
}
