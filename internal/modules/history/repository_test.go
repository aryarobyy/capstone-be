package history

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"capstone-be/internal/testutil"
)

// closedDB returns a *sql.DB that fails every call; needs no server.
func closedDB(t *testing.T) *sql.DB {
	db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	return db
}

func TestHistoryRepository_ClosedDBErrors(t *testing.T) {
	repo := NewHistoryRepository(closedDB(t))
	ctx := context.Background()
	if _, err := repo.Create(ctx, CreateHistoryRequest{}); err == nil {
		t.Error("create: expected error")
	}
	if _, _, err := repo.List(ctx, HistoryFilter{Limit: 10}); err == nil {
		t.Error("list: expected error")
	}
	if _, err := repo.Newest(ctx, NewestHistoryRequest{}); err == nil || errors.Is(err, ErrHistoryNotFound) {
		t.Errorf("newest: expected raw error, got %v", err)
	}
	if _, err := repo.Detail(ctx, DetailHistoryRequest{ID: 1}); err == nil || errors.Is(err, ErrHistoryNotFound) {
		t.Errorf("detail: expected raw error, got %v", err)
	}
	if err := repo.Delete(ctx, DeleteHistoryRequest{ID: 1}); err == nil || errors.Is(err, ErrHistoryNotFound) {
		t.Errorf("delete: expected raw error, got %v", err)
	}
}

func TestHistoryRepository_CRUD(t *testing.T) {
	db := testutil.OpenTestDB(t)
	repo := NewHistoryRepository(db)
	ctx := context.Background()

	if _, err := repo.Newest(ctx, NewestHistoryRequest{}); !errors.Is(err, ErrHistoryNotFound) {
		t.Fatalf("newest on empty table: %v", err)
	}

	d1 := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	resolved := d2.Add(time.Hour)
	var ids []int64
	for _, r := range []CreateHistoryRequest{
		{SensorID: 1, Parameter: "ph", StartedAt: d1, LastDetectedAt: d1, Status: AnomalyStatusWarning},
		{SensorID: 1, Parameter: "temperature", StartedAt: d2, LastDetectedAt: d2.Add(5 * time.Minute), AccumulatedDuration: 300, Status: AnomalyStatusCritical},
		{SensorID: 2, Parameter: "ph", StartedAt: d2.Add(time.Minute), LastDetectedAt: d2, Status: AnomalyStatusNormal, ResolvedAt: &resolved},
	} {
		a, err := repo.Create(ctx, r)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if a.ID == 0 || a.SensorID != r.SensorID || a.Status != r.Status || a.AccumulatedDuration != r.AccumulatedDuration || a.CreatedAt.IsZero() {
			t.Fatalf("create returned %+v", a)
		}
		ids = append(ids, a.ID)
	}

	cases := []struct {
		name      string
		f         HistoryFilter
		total     int
		firstID   int64
		resultLen int
	}{
		{"all ordered by started_at desc", HistoryFilter{Limit: 10}, 3, ids[2], 3},
		{"sensor", HistoryFilter{SensorID: 1, Limit: 10}, 2, ids[1], 2},
		{"parameter", HistoryFilter{Parameter: "ph", Limit: 10}, 2, ids[2], 2},
		{"status", HistoryFilter{Status: "CRITICAL", Limit: 10}, 1, ids[1], 1},
		{"date", HistoryFilter{Date: "2026-09-01", Limit: 10}, 1, ids[0], 1},
		{"all filters", HistoryFilter{SensorID: 1, Parameter: "ph", Status: "WARNING", Date: "2026-09-01", Limit: 10}, 1, ids[0], 1},
		{"pagination offset", HistoryFilter{Limit: 1, Index: 1}, 3, ids[1], 1},
	}
	for _, tc := range cases {
		list, total, err := repo.List(ctx, tc.f)
		if err != nil || total != tc.total || len(list) != tc.resultLen || list[0].ID != tc.firstID {
			t.Errorf("%s: total=%d len=%d err=%v list=%+v", tc.name, total, len(list), err, list)
		}
	}
	if list, total, err := repo.List(ctx, HistoryFilter{SensorID: 99, Limit: 10}); err != nil || total != 0 || list == nil || len(list) != 0 {
		t.Errorf("empty list: %v %d %v", list, total, err)
	}

	newest := []struct {
		req  NewestHistoryRequest
		want int64
	}{
		{NewestHistoryRequest{}, ids[2]},
		{NewestHistoryRequest{SensorID: 1}, ids[1]},
		{NewestHistoryRequest{Parameter: "ph", SensorID: 1}, ids[0]},
	}
	for _, tc := range newest {
		if a, err := repo.Newest(ctx, tc.req); err != nil || a.ID != tc.want {
			t.Errorf("newest %+v: %+v %v", tc.req, a, err)
		}
	}
	if _, err := repo.Newest(ctx, NewestHistoryRequest{SensorID: 2, Parameter: "temperature"}); !errors.Is(err, ErrHistoryNotFound) {
		t.Errorf("newest no match: %v", err)
	}

	a, err := repo.Detail(ctx, DetailHistoryRequest{ID: ids[2]})
	if err != nil || a.ResolvedAt == nil || !a.ResolvedAt.Equal(resolved) || a.Parameter != "ph" {
		t.Fatalf("detail = %+v, %v", a, err)
	}
	if _, err := repo.Detail(ctx, DetailHistoryRequest{ID: 99999}); !errors.Is(err, ErrHistoryNotFound) {
		t.Fatalf("detail missing: %v", err)
	}
	if err := repo.Delete(ctx, DeleteHistoryRequest{ID: ids[0]}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.Delete(ctx, DeleteHistoryRequest{ID: ids[0]}); !errors.Is(err, ErrHistoryNotFound) {
		t.Fatalf("delete again: %v", err)
	}
	if n := testutil.Count(t, db, `SELECT count(*) FROM anomalies`); n != 2 {
		t.Fatalf("expected 2 rows left, got %d", n)
	}
}
