package sensor

import (
	"context"
	"testing"
	"time"
)

type mockSensorRepository struct {
	capturedFilter SensorFilter
	sensors        []Sensor
	total          int
	err            error
}

func (m *mockSensorRepository) Create(ctx context.Context, req CreateSensorRequest) (int64, error) {
	return 1, nil
}

func (m *mockSensorRepository) Update(ctx context.Context, req UpdateSensorRequest) error {
	return nil
}

func (m *mockSensorRepository) Delete(ctx context.Context, req DeleteSensorRequest) error {
	return nil
}

func (m *mockSensorRepository) List(ctx context.Context, filter SensorFilter) ([]Sensor, int, error) {
	m.capturedFilter = filter
	return m.sensors, m.total, m.err
}

func (m *mockSensorRepository) Detail(ctx context.Context, req DetailSensorRequest) (*Sensor, error) {
	return nil, nil
}

func TestSensorService_List_Pagination(t *testing.T) {
	tests := []struct {
		name          string
		req           ListSensorRequest
		mockTotal     int
		mockSensors   []Sensor
		expectedLimit int
		expectedIndex int
	}{
		{
			name:          "Default pagination when request is empty",
			req:           ListSensorRequest{},
			mockTotal:     15,
			mockSensors:   []Sensor{{ID: 1, Name: "Sensor 1", Type: "DHT22"}},
			expectedLimit: 10,
			expectedIndex: 0,
		},
		{
			name: "Pagination with explicit Limit and Index",
			req: ListSensorRequest{
				Limit: 5,
				Index: 10,
			},
			mockTotal:     15,
			mockSensors:   []Sensor{{ID: 6, Name: "Sensor 6", Type: "DHT22"}},
			expectedLimit: 5,
			expectedIndex: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockSensorRepository{
				total:   tt.mockTotal,
				sensors: tt.mockSensors,
			}
			svc := NewSensorService(mockRepo)

			res, err := svc.List(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if mockRepo.capturedFilter.Limit != tt.expectedLimit {
				t.Errorf("repo filter Limit = %d, expected %d", mockRepo.capturedFilter.Limit, tt.expectedLimit)
			}
			if mockRepo.capturedFilter.Index != tt.expectedIndex {
				t.Errorf("repo filter Index = %d, expected %d", mockRepo.capturedFilter.Index, tt.expectedIndex)
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
		})
	}
}

func TestSensorService_List_DataMapping(t *testing.T) {
	now := time.Now()
	mockRepo := &mockSensorRepository{
		total: 1,
		sensors: []Sensor{
			{
				ID:          1,
				AreaID:      2,
				Name:        "Soil Moisture #1",
				Type:        "Capacitive",
				Code:        "SM-01",
				Description: "Area A sensor",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}
	svc := NewSensorService(mockRepo)

	res, err := svc.List(context.Background(), ListSensorRequest{Limit: 10, Index: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Data))
	}
	item := res.Data[0]
	if item.ID != 1 || item.AreaID != 2 || item.Name != "Soil Moisture #1" || item.Code != "SM-01" {
		t.Errorf("data mapping mismatch: %+v", item)
	}
}
