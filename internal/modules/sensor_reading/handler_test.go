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
	listResponse *ListSensorReadingResponse
	err          error
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
					SoilMoisture: &soilMoisture,
					Temperature:  &temperature,
					Humidity:     &humidity,
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
