package sensorreading

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewPeriodicCalcWorker_Defaults(t *testing.T) {
	w := NewPeriodicCalcWorker(nil, 0, 0)
	if w.interval != time.Minute || w.windowMinutes != 10 {
		t.Fatalf("defaults not applied: %+v", w)
	}
	w = NewPeriodicCalcWorker(nil, time.Second, 5)
	if w.interval != time.Second || w.windowMinutes != 5 {
		t.Fatalf("explicit values not kept: %+v", w)
	}
}

// cancellingRepo cancels the worker context after n CalculateSummary calls.
type cancellingRepo struct {
	mockSensorReadingRepository
	n      int
	cancel context.CancelFunc
}

func (r *cancellingRepo) CalculateSummary(ctx context.Context, sensorID, ownerID int64, windowMinutes int) ([]SensorSummaryItem, error) {
	items, err := r.mockSensorReadingRepository.CalculateSummary(ctx, sensorID, ownerID, windowMinutes)
	if r.summaryCalls >= r.n {
		r.cancel()
	}
	return items, err
}

func TestPeriodicCalcWorker_RunTicksAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := &cancellingRepo{n: 3, cancel: cancel}
	repo.summaryItems = []SensorSummaryItem{{SensorID: 9101, Status: "OPTIMAL"}}

	done := make(chan struct{})
	go func() { NewPeriodicCalcWorker(repo, time.Millisecond, 10).Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop")
	}
	if repo.summaryCalls != 3 {
		t.Fatalf("expected initial + 2 ticks = 3 calls, got %d", repo.summaryCalls)
	}
	if item, ok := GlobalSummaryCache.Get(9101); !ok || item.Status != "OPTIMAL" {
		t.Fatalf("cache not updated: %+v %v", item, ok)
	}
}

func TestPeriodicCalcWorker_CalculateError(t *testing.T) {
	repo := &mockSensorReadingRepository{err: errors.New("db down"), summaryItems: []SensorSummaryItem{{SensorID: 9102}}}
	NewPeriodicCalcWorker(repo, time.Minute, 10).calculate(context.Background())
	if _, ok := GlobalSummaryCache.Get(9102); ok {
		t.Fatal("cache must not be updated on error")
	}
}

func TestSummaryCache(t *testing.T) {
	c := &SummaryCache{items: map[int64]SensorSummaryItem{}}
	c.Set(SensorSummaryItem{SensorID: 1, Status: "A"})
	c.SetBulk([]SensorSummaryItem{{SensorID: 1, Status: "B"}, {SensorID: 2}})
	if got, ok := c.Get(1); !ok || got.Status != "B" {
		t.Fatalf("get = %+v %v", got, ok)
	}
	if _, ok := c.Get(3); ok {
		t.Fatal("unexpected hit")
	}
	if len(c.GetAll()) != 2 {
		t.Fatalf("getall = %d", len(c.GetAll()))
	}
}
