package history

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

type mockHistoryService struct {
	createResponse *HistoryResponse
	listResponse   *ListHistoryResponse
	detailResponse *HistoryResponse
	err            error
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

func (m *mockHistoryService) Detail(_ context.Context, _ DetailHistoryRequest) (*HistoryResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.detailResponse, nil
}

func (m *mockHistoryService) Delete(_ context.Context, _ DeleteHistoryRequest) error {
	return m.err
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
