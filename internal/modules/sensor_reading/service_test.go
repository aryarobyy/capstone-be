package sensorreading

import (
	"context"
	"testing"
	"time"
)

type mockSensorReadingRepository struct {
	capturedFilter SensorReadingFilter
	readings       []SensorReading
	total          int
	err            error
}

func (m *mockSensorReadingRepository) Create(ctx context.Context, req CreateSensorReadingRequest) error {
	return m.err
}

func (m *mockSensorReadingRepository) List(ctx context.Context, filter SensorReadingFilter) ([]SensorReading, int, error) {
	m.capturedFilter = filter
	return m.readings, m.total, m.err
}

func (m *mockSensorReadingRepository) Detail(ctx context.Context, req DetailSensorReadingRequest) (*SensorReading, error) {
	return nil, nil
}

func (m *mockSensorReadingRepository) Delete(ctx context.Context, req DeleteSensorReadingRequest) error {
	return nil
}

func TestSensorReadingService_List_Pagination(t *testing.T) {
	tests := []struct {
		name          string
		req           ListSensorReadingRequest
		mockTotal     int
		mockReadings  []SensorReading
		expectedLimit int
		expectedIndex int
	}{
		{
			name:          "Default pagination when request is empty",
			req:           ListSensorReadingRequest{},
			mockTotal:     25,
			mockReadings:  []SensorReading{{ID: 1, SensorID: 1}},
			expectedLimit: 10,
			expectedIndex: 0,
		},
		{
			name: "Pagination with explicit Limit and Index",
			req: ListSensorReadingRequest{
				Limit: 5,
				Index: 10,
			},
			mockTotal:     25,
			mockReadings:  []SensorReading{{ID: 11, SensorID: 1}},
			expectedLimit: 5,
			expectedIndex: 10,
		},
		{
			name: "Limit capped at 100",
			req: ListSensorReadingRequest{
				Limit: 200,
				Index: 0,
			},
			mockTotal:     150,
			mockReadings:  []SensorReading{},
			expectedLimit: 100,
			expectedIndex: 0,
		},
		{
			name: "Negative index defaulted to 0",
			req: ListSensorReadingRequest{
				Limit: 10,
				Index: -5,
			},
			mockTotal:     10,
			mockReadings:  []SensorReading{},
			expectedLimit: 10,
			expectedIndex: 0,
		},
		{
			name: "Empty result returns non-nil slice in response",
			req: ListSensorReadingRequest{
				Limit: 10,
				Index: 0,
			},
			mockTotal:     0,
			mockReadings:  nil,
			expectedLimit: 10,
			expectedIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockSensorReadingRepository{
				total:    tt.mockTotal,
				readings: tt.mockReadings,
			}
			svc := NewSensorReadingService(mockRepo)

			res, err := svc.List(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if mockRepo.capturedFilter.Limit != tt.expectedLimit {
				t.Errorf("repo filter Limit = %d, expected %d", mockRepo.capturedFilter.Limit, tt.expectedLimit)
			}
			if mockRepo.capturedFilter.Index != tt.expectedIndex {
				t.Errorf("repo filter Index (offset) = %d, expected %d", mockRepo.capturedFilter.Index, tt.expectedIndex)
			}
			if res.Limit != tt.expectedLimit {
				t.Errorf("response Limit = %d, expected %d", res.Limit, tt.expectedLimit)
			}
			if res.Index != tt.expectedIndex {
				t.Errorf("response Index = %d, expected %d", res.Index, tt.expectedIndex)
			}
			if res.Total != tt.mockTotal {
				t.Errorf("response Total = %d, expected %d", res.Total, tt.mockTotal)
			}
			if res.Data == nil {
				t.Errorf("response Data is nil, expected initialized slice")
			}
		})
	}
}

func TestSensorReadingService_List_DataMapping(t *testing.T) {
	now := time.Now()
	soilMoisture := 51.5
	temperature := 28.25
	humidity := 72.0
	mockRepo := &mockSensorReadingRepository{
		total: 1,
		readings: []SensorReading{
			{
				ID:           100,
				SensorID:     3,
				SoilMoisture: &soilMoisture,
				Temperature:  &temperature,
				Humidity:     &humidity,
				RecordedAt:   now,
			},
		},
	}
	svc := NewSensorReadingService(mockRepo)

	res, err := svc.List(context.Background(), ListSensorReadingRequest{Limit: 10, Index: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Data))
	}
	item := res.Data[0]
	if item.ID != 100 || item.SensorID != 3 || item.SoilMoisture == nil || *item.SoilMoisture != soilMoisture || item.RecordedAt != now {
		t.Errorf("data mapping mismatch: %+v", item)
	}
}
