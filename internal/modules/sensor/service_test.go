package sensor

import (
	"context"
	"errors"
	"testing"
	"time"

	"capstone-be/internal/modules/area"
)

type mockSensorRepository struct {
	capturedFilter SensorFilter
	sensors        []Sensor
	total          int
	err            error
	detailSensor   *Sensor
}

func (m *mockSensorRepository) Create(ctx context.Context, req CreateSensorRequest) (int64, error) {
	return 1, m.err
}

func (m *mockSensorRepository) Update(ctx context.Context, req UpdateSensorRequest) error {
	return m.err
}

func (m *mockSensorRepository) Delete(ctx context.Context, req DeleteSensorRequest) error {
	return m.err
}

func (m *mockSensorRepository) List(ctx context.Context, filter SensorFilter) ([]Sensor, int, error) {
	m.capturedFilter = filter
	return m.sensors, m.total, m.err
}

func (m *mockSensorRepository) Detail(ctx context.Context, req DetailSensorRequest) (*Sensor, error) {
	if m.detailSensor != nil {
		return m.detailSensor, m.err
	}
	return &Sensor{ID: req.ID, Name: "Test Sensor"}, m.err
}

type mockAreaRepository struct {
	existsMap map[int64]bool
	err       error
}

func (m *mockAreaRepository) Exists(ctx context.Context, id int64) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	if m.existsMap == nil {
		return true, nil
	}
	return m.existsMap[id], nil
}

func (m *mockAreaRepository) FindByID(ctx context.Context, id int64) (*area.Area, error) {
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
			mockSensors:   []Sensor{{ID: 1, Name: "Sensor 1"}},
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
			mockSensors:   []Sensor{{ID: 6, Name: "Sensor 6"}},
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
			mockAreaRepo := &mockAreaRepository{}
			svc := NewSensorService(mockRepo, mockAreaRepo)

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
				Code:        "SM-01",
				Description: "Area A sensor",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}
	mockAreaRepo := &mockAreaRepository{}
	svc := NewSensorService(mockRepo, mockAreaRepo)

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

func TestSensorService_Create_AreaValidation(t *testing.T) {
	mockRepo := &mockSensorRepository{}
	mockAreaRepo := &mockAreaRepository{
		existsMap: map[int64]bool{
			1: true,
			2: false,
		},
	}
	svc := NewSensorService(mockRepo, mockAreaRepo)

	t.Run("Create with AreaID == 0 succeeds without checking area", func(t *testing.T) {
		res, err := svc.Create(context.Background(), CreateSensorRequest{
			AreaID: 0,
			Name:   "Sensor Without Area",
			Code:   "S-00",
		})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if res == nil {
			t.Fatalf("expected non-nil response")
		}
	})

	t.Run("Create with existing AreaID succeeds", func(t *testing.T) {
		res, err := svc.Create(context.Background(), CreateSensorRequest{
			AreaID: 1,
			Name:   "Sensor With Valid Area",
			Code:   "S-01",
		})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if res == nil {
			t.Fatalf("expected non-nil response")
		}
	})

	t.Run("Create with non-existent AreaID returns ErrAreaNotFound", func(t *testing.T) {
		_, err := svc.Create(context.Background(), CreateSensorRequest{
			AreaID: 2,
			Name:   "Sensor With Invalid Area",
			Code:   "S-02",
		})
		if !errors.Is(err, ErrAreaNotFound) {
			t.Fatalf("expected ErrAreaNotFound, got %v", err)
		}
	})
}

func TestSensorService_Update_AreaValidation(t *testing.T) {
	mockRepo := &mockSensorRepository{}
	mockAreaRepo := &mockAreaRepository{
		existsMap: map[int64]bool{
			1: true,
			2: false,
		},
	}
	svc := NewSensorService(mockRepo, mockAreaRepo)

	t.Run("Update with non-existent AreaID returns ErrAreaNotFound", func(t *testing.T) {
		invalidAreaID := int64(2)
		_, err := svc.Update(context.Background(), UpdateSensorRequest{
			ID:     1,
			AreaID: &invalidAreaID,
		})
		if !errors.Is(err, ErrAreaNotFound) {
			t.Fatalf("expected ErrAreaNotFound, got %v", err)
		}
	})

	t.Run("Update with existing AreaID succeeds", func(t *testing.T) {
		validAreaID := int64(1)
		res, err := svc.Update(context.Background(), UpdateSensorRequest{
			ID:     1,
			AreaID: &validAreaID,
		})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if res == nil {
			t.Fatalf("expected non-nil response")
		}
	})
}

