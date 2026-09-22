package sensorreading

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type mockSensorReadingService struct {
	listResponse    *ListSensorReadingResponse
	summaryResponse *SensorSummaryResponse
	err             error
}

func (m *mockSensorReadingService) Create(ctx context.Context, req CreateSensorReadingRequest) error {
	return m.err
}

func (m *mockSensorReadingService) List(ctx context.Context, req ListSensorReadingRequest) (*ListSensorReadingResponse, error) {
	return m.listResponse, m.err
}

func (m *mockSensorReadingService) Detail(ctx context.Context, req DetailSensorReadingRequest) (*SensorReadingResponse, error) {
	return nil, nil
}

func (m *mockSensorReadingService) Delete(ctx context.Context, req DeleteSensorReadingRequest) error {
	return nil
}

func (m *mockSensorReadingService) Summary(ctx context.Context, req SensorSummaryRequest, ownerID int64) (*SensorSummaryResponse, error) {
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

