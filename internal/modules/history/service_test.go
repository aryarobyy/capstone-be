package history

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockHistoryRepository struct {
	filter        HistoryFilter
	histories     []Anomaly
	total         int
	err           error
	createdReq    CreateHistoryRequest
	createdResult *Anomaly
}

func (m *mockHistoryRepository) Since(_ context.Context, filter HistoryFilter, _ int64) ([]Anomaly, error) {
	m.filter = filter
	return m.histories, m.err
}

func (m *mockHistoryRepository) Create(_ context.Context, req CreateHistoryRequest) (*Anomaly, error) {
	m.createdReq = req
	if m.err != nil {
		return nil, m.err
	}
	if m.createdResult != nil {
		return m.createdResult, nil
	}
	return &Anomaly{
		ID:                  1,
		SensorID:            req.SensorID,
		Parameter:           req.Parameter,
		StartedAt:           req.StartedAt,
		LastDetectedAt:      req.LastDetectedAt,
		AccumulatedDuration: req.AccumulatedDuration,
		Status:              req.Status,
		ResolvedAt:          req.ResolvedAt,
	}, nil
}

func (m *mockHistoryRepository) List(_ context.Context, filter HistoryFilter) ([]Anomaly, int, error) {
	m.filter = filter
	return m.histories, m.total, m.err
}

func (m *mockHistoryRepository) Newest(_ context.Context, _ NewestHistoryRequest) (*Anomaly, error) {
	if len(m.histories) == 0 {
		return nil, m.err
	}
	return &m.histories[0], m.err
}

func (m *mockHistoryRepository) Detail(_ context.Context, _ DetailHistoryRequest) (*Anomaly, error) {
	if len(m.histories) == 0 {
		return nil, m.err
	}
	return &m.histories[0], m.err
}

func (m *mockHistoryRepository) Delete(_ context.Context, _ DeleteHistoryRequest) error {
	return m.err
}

func TestHistoryServiceList(t *testing.T) {
	startedAt := time.Now()
	repo := &mockHistoryRepository{
		total: 1,
		histories: []Anomaly{
			{
				ID:                  10,
				SensorID:            3,
				Parameter:           "temperature",
				StartedAt:           startedAt,
				LastDetectedAt:      startedAt,
				AccumulatedDuration: 300,
				Status:              AnomalyStatusWarning,
			},
		},
	}
	service := NewHistoryService(repo)

	response, err := service.List(context.Background(), ListHistoryRequest{
		SensorID:  3,
		Parameter: "temperature",
		Status:    "WARNING",
		Date:      "2026-09-06",
		Limit:     200,
		Index:     -1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.filter.Limit != 100 || repo.filter.Index != 0 {
		t.Fatalf("unexpected pagination filter: %+v", repo.filter)
	}
	if repo.filter.SensorID != 3 || repo.filter.Parameter != "temperature" || repo.filter.Status != "WARNING" {
		t.Fatalf("unexpected history filter: %+v", repo.filter)
	}
	if response.Total != 1 || len(response.Data) != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Data[0].AccumulatedDuration != 300 || response.Data[0].Status != AnomalyStatusWarning {
		t.Fatalf("unexpected data mapping: %+v", response.Data[0])
	}
}

func TestHistoryServiceListReturnsEmptySlice(t *testing.T) {
	service := NewHistoryService(&mockHistoryRepository{})

	response, err := service.List(context.Background(), ListHistoryRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response.Data == nil {
		t.Fatal("expected initialized data slice")
	}
}

func TestHistoryServiceCreate_Success(t *testing.T) {
	repo := &mockHistoryRepository{}
	service := NewHistoryService(repo)

	now := time.Now()
	req := CreateHistoryRequest{
		SensorID:            5,
		Parameter:           "humidity",
		StartedAt:           now,
		LastDetectedAt:      now.Add(time.Minute),
		AccumulatedDuration: 60,
		Status:              AnomalyStatusCritical,
	}

	res, err := service.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.ID != 1 || res.SensorID != 5 || res.Parameter != "humidity" || res.Status != AnomalyStatusCritical {
		t.Fatalf("unexpected response: %+v", res)
	}
	if repo.createdReq.SensorID != 5 || repo.createdReq.Status != AnomalyStatusCritical {
		t.Fatalf("unexpected repo created req: %+v", repo.createdReq)
	}
}

func TestHistoryServiceCreate_DefaultsLastDetectedAt(t *testing.T) {
	repo := &mockHistoryRepository{}
	service := NewHistoryService(repo)

	now := time.Now()
	req := CreateHistoryRequest{
		SensorID:  5,
		Parameter: "soil_moisture",
		StartedAt: now,
		Status:    AnomalyStatusFatal,
	}

	res, err := service.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.LastDetectedAt.Equal(now) {
		t.Fatalf("expected LastDetectedAt to equal StartedAt, got %v vs %v", res.LastDetectedAt, now)
	}
	if !repo.createdReq.LastDetectedAt.Equal(now) {
		t.Fatalf("expected repo created req LastDetectedAt to equal StartedAt")
	}
}

func TestHistoryServiceCreate_InvalidStatus(t *testing.T) {
	repo := &mockHistoryRepository{}
	service := NewHistoryService(repo)

	req := CreateHistoryRequest{
		SensorID:  5,
		Parameter: "soil_moisture",
		StartedAt: time.Now(),
		Status:    "UNKNOWN_STATUS",
	}

	_, err := service.Create(context.Background(), req)
	if err != ErrInvalidAnomalyStatus {
		t.Fatalf("expected ErrInvalidAnomalyStatus, got %v", err)
	}
}

func TestHistoryService_ErrorsAndPassThrough(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	repo := &mockHistoryRepository{err: boom}
	svc := NewHistoryService(repo)

	if _, err := svc.Create(ctx, CreateHistoryRequest{Status: AnomalyStatusWarning}); err != boom {
		t.Errorf("create err = %v", err)
	}
	if _, err := svc.List(ctx, ListHistoryRequest{}); err != boom {
		t.Errorf("list err = %v", err)
	}
	if _, err := svc.Newest(ctx, NewestHistoryRequest{}); err != boom {
		t.Errorf("newest err = %v", err)
	}
	if _, err := svc.Detail(ctx, DetailHistoryRequest{ID: 1}); err != boom {
		t.Errorf("detail err = %v", err)
	}
	if err := svc.Delete(ctx, DeleteHistoryRequest{ID: 1}); err != boom {
		t.Errorf("delete err = %v", err)
	}

	resolved := time.Now()
	repo.err = nil
	repo.histories = []Anomaly{{ID: 8, SensorID: 2, Parameter: "ph", Status: AnomalyStatusNormal, ResolvedAt: &resolved}}
	if res, err := svc.Newest(ctx, NewestHistoryRequest{SensorID: 2}); err != nil || res.ID != 8 || res.ResolvedAt != &resolved {
		t.Errorf("newest = %+v, %v", res, err)
	}
	if res, err := svc.Detail(ctx, DetailHistoryRequest{ID: 8}); err != nil || res.ID != 8 || res.Parameter != "ph" {
		t.Errorf("detail = %+v, %v", res, err)
	}
	if err := svc.Delete(ctx, DeleteHistoryRequest{ID: 8}); err != nil {
		t.Errorf("delete err = %v", err)
	}

	// Limit <= 0 defaults to 10.
	if res, _ := svc.List(ctx, ListHistoryRequest{Limit: 0, Index: 5}); res.Limit != 10 || res.Index != 5 || repo.filter.Limit != 10 {
		t.Errorf("list defaults = %+v filter=%+v", res, repo.filter)
	}
}

func TestAnomalyStatus_IsValid(t *testing.T) {
	for _, s := range []AnomalyStatus{AnomalyStatusFatal, AnomalyStatusCritical, AnomalyStatusWarning, AnomalyStatusNormal} {
		if !s.IsValid() {
			t.Errorf("%s should be valid", s)
		}
	}
	if AnomalyStatus("warning").IsValid() || AnomalyStatus("").IsValid() {
		t.Error("lowercase/empty must be invalid")
	}
}

func TestHistoryService_Since(t *testing.T) {
	now := time.Now()
	repo := &mockHistoryRepository{histories: []Anomaly{{ID: 7, SensorID: 9, Parameter: "soil_moisture", Status: AnomalyStatusWarning, StartedAt: now}}}
	svc := NewHistoryService(repo)

	got, err := svc.Since(context.Background(), StreamHistoryRequest{OwnerID: 3, SensorID: 9, Parameter: "soil_moisture", Status: "WARNING", Date: "2026-10-31", AfterID: 99}, 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	wantFilter := HistoryFilter{OwnerID: 3, SensorID: 9, Parameter: "soil_moisture", Status: "WARNING", Date: "2026-10-31", Limit: 20}
	if repo.filter != wantFilter {
		t.Fatalf("filter = %+v, want %+v", repo.filter, wantFilter)
	}
	if len(got) != 1 || got[0].ID != 7 || got[0].Parameter != "soil_moisture" || !got[0].StartedAt.Equal(now) {
		t.Fatalf("got %+v", got)
	}

	repo.err = errors.New("boom")
	if _, err := svc.Since(context.Background(), StreamHistoryRequest{}, 0, 1); err == nil {
		t.Fatal("expected repo error")
	}
}
