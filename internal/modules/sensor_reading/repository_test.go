package sensorreading

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

func TestSensorReadingRepository_ClosedDBErrors(t *testing.T) {
	repo := NewSensorReadingRepository(closedDB(t))
	ctx := context.Background()
	if err := repo.Create(ctx, CreateSensorReadingRequest{SensorID: 1}); err == nil {
		t.Error("create: expected error")
	}
	if _, _, err := repo.List(ctx, SensorReadingFilter{Limit: 10}); err == nil {
		t.Error("list: expected error")
	}
	if _, err := repo.Detail(ctx, DetailSensorReadingRequest{ID: 1}); err == nil || errors.Is(err, ErrSensorReadingNotFound) {
		t.Errorf("detail: expected raw error, got %v", err)
	}
	if err := repo.Delete(ctx, DeleteSensorReadingRequest{ID: 1}); err == nil || errors.Is(err, ErrSensorReadingNotFound) {
		t.Errorf("delete: expected raw error, got %v", err)
	}
	for _, args := range [][2]int64{{1, 0}, {0, 1}, {0, 0}} {
		if _, err := repo.CalculateSummary(ctx, args[0], args[1], 10); err == nil {
			t.Errorf("summary%v: expected error", args)
		}
	}
}

func seedSensors(t *testing.T, db *sql.DB) {
	t.Helper()
	testutil.Exec(t, db, `INSERT INTO users(id,name,email,password) VALUES (1,'a','a@x.io','x'),(2,'b','b@x.io','x')`)
	testutil.Exec(t, db, `INSERT INTO sensors(id,name,code,area_id,owner_id) VALUES (1,'s1','C1',5,1),(2,'s2','C2',0,1),(3,'s3','C3',0,2)`)
}

func TestSensorReadingRepository_CreateListDetailDelete(t *testing.T) {
	db := testutil.OpenTestDB(t)
	seedSensors(t, db)
	repo := NewSensorReadingRepository(db)
	ctx := context.Background()

	if err := repo.Create(ctx, CreateSensorReadingRequest{SensorID: 404, RecordedAt: time.Now()}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown sensor: want ErrNoRows, got %v", err)
	}

	day1 := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	for i, r := range []CreateSensorReadingRequest{
		{SensorID: 1, SoilMoisture: 40, Temperature: 25, Humidity: 60, RecordedAt: day1},
		{SensorID: 1, SoilMoisture: 41, Temperature: 26, Humidity: 61, RecordedAt: day1.Add(time.Minute)},
		{SensorID: 1, SoilMoisture: 42, Temperature: 27, Humidity: 62, RecordedAt: day2},
		{SensorID: 2, SoilMoisture: 50, Temperature: 20, Humidity: 70, RecordedAt: day1},
	} {
		if err := repo.Create(ctx, r); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	// Duplicate (same sensor + recorded_at) is a silent no-op.
	if err := repo.Create(ctx, CreateSensorReadingRequest{SensorID: 1, SoilMoisture: 99, RecordedAt: day1}); err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if n := testutil.Count(t, db, `SELECT count(*) FROM sensor_readings`); n != 4 {
		t.Fatalf("expected 4 readings, got %d", n)
	}

	cases := []struct {
		name      string
		f         SensorReadingFilter
		total, ln int
	}{
		{"all", SensorReadingFilter{Limit: 10}, 4, 4},
		{"sensor", SensorReadingFilter{SensorID: 1, Limit: 10}, 3, 3},
		{"date", SensorReadingFilter{Date: "2026-09-01", Limit: 10}, 3, 3},
		{"sensor+date", SensorReadingFilter{SensorID: 1, Date: "2026-09-02", Limit: 10}, 1, 1},
		{"page", SensorReadingFilter{Limit: 2, Index: 3}, 4, 1},
		{"none", SensorReadingFilter{SensorID: 3, Limit: 10}, 0, 0},
	}
	for _, tc := range cases {
		list, total, err := repo.List(ctx, tc.f)
		if err != nil || total != tc.total || len(list) != tc.ln {
			t.Errorf("%s: total=%d len=%d err=%v", tc.name, total, len(list), err)
		}
	}
	list, _, _ := repo.List(ctx, SensorReadingFilter{Limit: 10})
	if list[0].ID < list[1].ID {
		t.Error("expected ORDER BY id DESC")
	}

	got, err := repo.Detail(ctx, DetailSensorReadingRequest{ID: list[0].ID})
	if err != nil || got.SensorID != 2 || got.SoilMoisture != 50 || !got.RecordedAt.Equal(day1) {
		t.Fatalf("detail = %+v, %v", got, err)
	}
	if _, err := repo.Detail(ctx, DetailSensorReadingRequest{ID: 99999}); !errors.Is(err, ErrSensorReadingNotFound) {
		t.Fatalf("detail missing: %v", err)
	}
	if err := repo.Delete(ctx, DeleteSensorReadingRequest{ID: list[0].ID}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.Delete(ctx, DeleteSensorReadingRequest{ID: list[0].ID}); !errors.Is(err, ErrSensorReadingNotFound) {
		t.Fatalf("delete again: %v", err)
	}
}

func TestSensorReadingRepository_CreateEvaluatesAlerts(t *testing.T) {
	db := testutil.OpenTestDB(t)
	seedSensors(t, db)
	testutil.Exec(t, db, `INSERT INTO alert_rules(sensor_id,parameter,min_value,duration_seconds) VALUES (1,'soil_moisture',20,0)`)
	repo := NewSensorReadingRepository(db)

	if err := repo.Create(context.Background(), CreateSensorReadingRequest{SensorID: 1, SoilMoisture: 5, RecordedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if n := testutil.Count(t, db, `SELECT count(*) FROM anomalies WHERE sensor_id=1 AND status='WARNING'`); n != 1 {
		t.Fatalf("expected 1 anomaly from alert evaluation, got %d", n)
	}
}

func TestSensorReadingRepository_CalculateSummary(t *testing.T) {
	db := testutil.OpenTestDB(t)
	seedSensors(t, db)
	testutil.Exec(t, db, `INSERT INTO sensors(id,name,code,owner_id) VALUES (4,'s4','C4',2),(5,'s5','C5',2)`)
	now := time.Now()
	// s1: two recent optimal readings. s2: none (INACTIVE). s3: latest dry -> WARNING.
	// s4: optimal values but open anomaly -> ANOMALY. s5: old reading only, outside window.
	testutil.Exec(t, db, `INSERT INTO sensor_readings(sensor_id,soil_moisture,temperature,humidity,recorded_at) VALUES
		(1,40.123,25,60,$1),(1,50,27,70,$2),
		(3,10,25,60,$1),
		(4,40,25,60,$1),
		(5,40,25,60,$3)`, now.Add(-2*time.Minute), now.Add(-time.Minute), now.Add(-2*time.Hour))
	testutil.Exec(t, db, `INSERT INTO anomalies(sensor_id,parameter,status) VALUES (4,'temperature','WARNING')`)
	repo := NewSensorReadingRepository(db)
	ctx := context.Background()

	all, err := repo.CalculateSummary(ctx, 0, 0, 0) // window defaults to 10
	if err != nil || len(all) != 5 {
		t.Fatalf("all: len=%d err=%v", len(all), err)
	}
	want := map[int64]string{1: "OPTIMAL", 2: "INACTIVE", 3: "WARNING", 4: "ANOMALY", 5: "OPTIMAL"}
	for _, it := range all {
		if it.Status != want[it.SensorID] {
			t.Errorf("sensor %d status=%s want %s", it.SensorID, it.Status, want[it.SensorID])
		}
	}
	s1 := all[0]
	if s1.SensorName != "s1" || s1.SensorCode != "C1" || s1.AreaID != 5 || s1.TotalSamples != 2 ||
		s1.Average.SoilMoisture != 45.06 || s1.Min.SoilMoisture != 40.12 || s1.Max.Temperature != 27 || s1.Latest.SoilMoisture != 50 {
		t.Errorf("s1 aggregates wrong: %+v", s1)
	}
	if all[4].TotalSamples != 0 || all[4].Latest.RecordedAt.IsZero() {
		t.Errorf("s5: old reading must be outside window but still latest: %+v", all[4])
	}

	if own, err := repo.CalculateSummary(ctx, 0, 1, 10); err != nil || len(own) != 2 {
		t.Errorf("owner 1: len=%d err=%v", len(own), err)
	}
	if one, err := repo.CalculateSummary(ctx, 3, 2, 10); err != nil || len(one) != 1 || one[0].SensorID != 3 {
		t.Errorf("sensor 3 owner 2: %+v %v", one, err)
	}
	if one, err := repo.CalculateSummary(ctx, 3, 0, 10); err != nil || len(one) != 1 {
		t.Errorf("sensor 3 no owner: %+v %v", one, err)
	}
	if none, err := repo.CalculateSummary(ctx, 3, 1, 10); err != nil || len(none) != 0 {
		t.Errorf("owner mismatch: %+v %v", none, err)
	}
	if none, err := repo.CalculateSummary(ctx, 404, 0, 10); err != nil || none == nil || len(none) != 0 {
		t.Errorf("missing sensor: %+v %v", none, err)
	}
}

// A CRITICAL/FATAL unresolved anomaly does not flag the sensor as ANOMALY:
// the query only matches status IN ('WARNING','ANOMALY'), and 'ANOMALY' is not a valid AnomalyStatus.
func TestSensorReadingRepository_CalculateSummary_CriticalAnomalyIgnored(t *testing.T) {
	db := testutil.OpenTestDB(t)
	seedSensors(t, db)
	testutil.Exec(t, db, `INSERT INTO sensor_readings(sensor_id,soil_moisture,temperature,humidity) VALUES (1,40,25,60)`)
	testutil.Exec(t, db, `INSERT INTO anomalies(sensor_id,parameter,status) VALUES (1,'temperature','CRITICAL')`)
	got, err := NewSensorReadingRepository(db).CalculateSummary(context.Background(), 1, 0, 10)
	if err != nil || len(got) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
	if got[0].Status != "OPTIMAL" { // documents current behavior
		t.Fatalf("status = %s", got[0].Status)
	}
}

// sensors.code is nullable but scanned into a string, so one sensor with NULL code
// breaks the summary for everyone (worker + list endpoints). Documents current behavior.
func TestSensorReadingRepository_CalculateSummary_NullCodeFails(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.Exec(t, db, `INSERT INTO users(id,name,email,password) VALUES (1,'a','a@x.io','x')`)
	testutil.Exec(t, db, `INSERT INTO sensors(id,name,owner_id) VALUES (1,'nocode',1)`)
	repo := NewSensorReadingRepository(db)
	ctx := context.Background()
	for _, args := range [][2]int64{{1, 0}, {0, 1}, {0, 0}} {
		if _, err := repo.CalculateSummary(ctx, args[0], args[1], 10); err == nil {
			t.Errorf("summary%v: expected scan error for NULL code", args)
		}
	}
}
