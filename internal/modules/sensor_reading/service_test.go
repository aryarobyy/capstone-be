package sensorreading

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockSensorReadingRepository struct {
	capturedFilter SensorReadingFilter
	readings       []SensorReading
	summaryItems   []SensorSummaryItem
	detail         *SensorReading
	summaryCalls   int
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
	return m.detail, m.err
}

func (m *mockSensorReadingRepository) Delete(ctx context.Context, req DeleteSensorReadingRequest) error {
	return m.err
}

func (m *mockSensorReadingRepository) CalculateSummary(ctx context.Context, sensorID int64, ownerID int64, windowMinutes int) ([]SensorSummaryItem, error) {
	m.summaryCalls++
	return m.summaryItems, m.err
}

func (m *mockSensorReadingRepository) Since(ctx context.Context, req StreamSensorReadingRequest, afterID int64, limit int) ([]SensorActivity, error) {
	return nil, m.err
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
				SoilMoisture: soilMoisture,
				Temperature:  temperature,
				Humidity:     humidity,
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
	if item.ID != 100 || item.SensorID != 3 || item.SoilMoisture != soilMoisture || item.Temperature != temperature || item.Humidity != humidity || item.RecordedAt != now {
		t.Errorf("data mapping mismatch: %+v", item)
	}
}

func TestSensorReadingService_Summary(t *testing.T) {
	now := time.Now()
	mockRepo := &mockSensorReadingRepository{
		summaryItems: []SensorSummaryItem{
			{
				SensorID:     1,
				SensorName:   "Greenhouse Sensor 1",
				TotalSamples: 20,
				Latest: LatestTelemetry{
					SoilMoisture: 50.0,
					Temperature:  28.5,
					Humidity:     70.0,
					RecordedAt:   now,
				},
				Average: TelemetryValues{
					SoilMoisture: 49.0,
					Temperature:  28.0,
					Humidity:     69.0,
				},
				Status:       "OPTIMAL",
				CalculatedAt: now,
			},
		},
	}
	svc := NewSensorReadingService(mockRepo)

	res, err := svc.Summary(context.Background(), SensorSummaryRequest{SensorID: 1, WindowMinutes: 10}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.WindowMinutes != 10 {
		t.Errorf("expected window_minutes 10, got %d", res.WindowMinutes)
	}
	if len(res.Data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Data))
	}
	if res.Data[0].SensorID != 1 {
		t.Errorf("expected sensor_id 1, got %d", res.Data[0].SensorID)
	}
	if res.Data[0].Status != "OPTIMAL" {
		t.Errorf("expected status OPTIMAL, got %s", res.Data[0].Status)
	}
}


func TestSensorReadingService_PassThroughAndErrors(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	repo := &mockSensorReadingRepository{err: boom}
	svc := NewSensorReadingService(repo)

	if err := svc.Create(ctx, CreateSensorReadingRequest{}); err != boom {
		t.Fatalf("create err = %v", err)
	}
	if err := svc.Delete(ctx, DeleteSensorReadingRequest{ID: 1}); err != boom {
		t.Fatalf("delete err = %v", err)
	}
	if _, err := svc.List(ctx, ListSensorReadingRequest{}); err != boom {
		t.Fatalf("list err = %v", err)
	}
	if _, err := svc.Detail(ctx, DetailSensorReadingRequest{ID: 1}); err != boom {
		t.Fatalf("detail err = %v", err)
	}
	if _, err := svc.Summary(ctx, SensorSummaryRequest{SensorID: 9001, WindowMinutes: 10}, 0); err != boom {
		t.Fatalf("summary err = %v", err)
	}

	now := time.Now()
	repo.err = nil
	repo.detail = &SensorReading{ID: 4, SensorID: 2, SoilMoisture: 1, Temperature: 2, Humidity: 3, RecordedAt: now}
	res, err := svc.Detail(ctx, DetailSensorReadingRequest{ID: 4})
	if err != nil || res.ID != 4 || res.SensorID != 2 || res.Humidity != 3 || !res.RecordedAt.Equal(now) {
		t.Fatalf("detail = %+v, %v", res, err)
	}
}

func resetSummaryCache() {
	GlobalSummaryCache.mu.Lock()
	GlobalSummaryCache.items = map[int64]SensorSummaryItem{}
	GlobalSummaryCache.mu.Unlock()
}

func TestSensorReadingService_SummaryCache(t *testing.T) {
	resetSummaryCache()
	ctx := context.Background()
	repo := &mockSensorReadingRepository{summaryItems: []SensorSummaryItem{{SensorID: 9002, Status: "OPTIMAL"}}}
	svc := NewSensorReadingService(repo)

	// window <= 0 defaults to 10, misses cache, populates it.
	res, err := svc.Summary(ctx, SensorSummaryRequest{SensorID: 9002}, 0)
	if err != nil || res.WindowMinutes != 10 || repo.summaryCalls != 1 {
		t.Fatalf("res=%+v err=%v calls=%d", res, err, repo.summaryCalls)
	}
	// Second call is served from cache.
	res, _ = svc.Summary(ctx, SensorSummaryRequest{SensorID: 9002, WindowMinutes: 10}, 0)
	if repo.summaryCalls != 1 || len(res.Data) != 1 || res.Data[0].SensorID != 9002 {
		t.Fatalf("expected cache hit, calls=%d res=%+v", repo.summaryCalls, res)
	}
	// Non-default window always hits the repo and does not touch the cache.
	repo.summaryItems = []SensorSummaryItem{{SensorID: 9003}}
	if res, _ = svc.Summary(ctx, SensorSummaryRequest{SensorID: 9003, WindowMinutes: 30}, 0); repo.summaryCalls != 2 || res.WindowMinutes != 30 {
		t.Fatalf("calls=%d res=%+v", repo.summaryCalls, res)
	}
	if _, ok := GlobalSummaryCache.Get(9003); ok {
		t.Fatal("30m window must not be cached")
	}
}

func TestSensorReadingService_SinceDelegates(t *testing.T) {
	repo := &mockSensorReadingRepository{err: errors.New("boom")}
	if _, err := NewSensorReadingService(repo).Since(context.Background(), StreamSensorReadingRequest{AreaID: 5}, 1, 20); err == nil {
		t.Fatal("expected repo error")
	}
}
