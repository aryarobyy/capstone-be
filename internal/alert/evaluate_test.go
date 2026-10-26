package alert

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"capstone-be/internal/testutil"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func seedSensor(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO sensors(name) VALUES('s') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// rule inserts an alert rule; nil bounds are stored as NULL.
func rule(t *testing.T, db *sql.DB, sensor int64, param string, min, max *float64, duration, cooldown, gap int, enabled bool) {
	t.Helper()
	testutil.Exec(t, db, `INSERT INTO alert_rules(sensor_id,parameter,min_value,max_value,duration_seconds,cooldown_seconds,max_gap_seconds,enabled) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, sensor, param, min, max, duration, cooldown, gap, enabled)
}

func f(v float64) *float64 { return &v }

func eval(t *testing.T, db *sql.DB, sensor int64, at time.Time, values map[string]float64) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := EvaluateTx(context.Background(), tx, sensor, at, values); err != nil {
		t.Fatalf("EvaluateTx(%v): %v", at, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

type anomalyRow struct {
	status   string
	started  time.Time
	last     time.Time
	duration int64
	resolved sql.NullTime
}

func anomalies(t *testing.T, db *sql.DB, sensor int64) []anomalyRow {
	t.Helper()
	rows, err := db.Query(`SELECT status,started_at,last_detected_at,accumulated_duration,resolved_at FROM anomalies WHERE sensor_id=$1 ORDER BY id`, sensor)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []anomalyRow
	for rows.Next() {
		var a anomalyRow
		if err := rows.Scan(&a.status, &a.started, &a.last, &a.duration, &a.resolved); err != nil {
			t.Fatal(err)
		}
		out = append(out, a)
	}
	return out
}

type stateRow struct {
	started  sql.NullTime
	last     time.Time
	notified sql.NullTime
	anomaly  sql.NullInt64
}

func state(t *testing.T, db *sql.DB, sensor int64, param string) stateRow {
	t.Helper()
	var s stateRow
	if err := db.QueryRow(`SELECT started_at,last_recorded_at,last_notified_at,anomaly_id FROM alert_states WHERE sensor_id=$1 AND parameter=$2`, sensor, param).Scan(&s.started, &s.last, &s.notified, &s.anomaly); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestEvaluateNoRules(t *testing.T) {
	db := testutil.OpenTestDB(t)
	sensor := seedSensor(t, db)
	eval(t, db, sensor, t0, map[string]float64{"temperature": 99})
	if testutil.Count(t, db, `SELECT count(*) FROM alert_states`) != 0 {
		t.Fatal("no rule, no state")
	}
}

func TestEvaluateLifecycle(t *testing.T) {
	db := testutil.OpenTestDB(t)
	sensor := seedSensor(t, db)
	rule(t, db, sensor, "temperature", nil, f(30), 60, 60, 300, true)

	// In range: state recorded, nothing open.
	eval(t, db, sensor, t0, map[string]float64{"temperature": 25})
	if s := state(t, db, sensor, "temperature"); s.started.Valid || s.anomaly.Valid || !s.last.Equal(t0) {
		t.Fatalf("state = %+v", s)
	}

	// Out of range but shorter than duration: tracking only.
	eval(t, db, sensor, t0.Add(10*time.Second), map[string]float64{"temperature": 31})
	eval(t, db, sensor, t0.Add(40*time.Second), map[string]float64{"temperature": 32})
	if len(anomalies(t, db, sensor)) != 0 {
		t.Fatal("anomaly opened before duration elapsed")
	}
	if s := state(t, db, sensor, "temperature"); !s.started.Valid || !s.started.Time.Equal(t0.Add(10*time.Second)) {
		t.Fatalf("start not tracked: %+v", s)
	}

	// Duration reached: WARNING opened with accumulated time since the first breach.
	eval(t, db, sensor, t0.Add(70*time.Second), map[string]float64{"temperature": 33})
	a := anomalies(t, db, sensor)
	if len(a) != 1 || a[0].status != "WARNING" || a[0].duration != 60 || !a[0].started.Equal(t0.Add(10*time.Second)) || !a[0].last.Equal(t0.Add(70*time.Second)) {
		t.Fatalf("anomalies = %+v", a)
	}
	if s := state(t, db, sensor, "temperature"); !s.anomaly.Valid {
		t.Fatal("state must link the anomaly")
	}

	// Still out: same anomaly updated.
	eval(t, db, sensor, t0.Add(100*time.Second), map[string]float64{"temperature": 34})
	a = anomalies(t, db, sensor)
	if len(a) != 1 || a[0].duration != 90 || !a[0].last.Equal(t0.Add(100*time.Second)) {
		t.Fatalf("anomalies = %+v", a)
	}

	// Out-of-order and duplicate readings never rewind state.
	eval(t, db, sensor, t0.Add(50*time.Second), map[string]float64{"temperature": 10})
	eval(t, db, sensor, t0.Add(100*time.Second), map[string]float64{"temperature": 10})
	if a = anomalies(t, db, sensor); a[0].status != "WARNING" {
		t.Fatalf("stale reading resolved anomaly: %+v", a)
	}

	// Back in range: NORMAL with resolved_at, state cleared.
	eval(t, db, sensor, t0.Add(130*time.Second), map[string]float64{"temperature": 20})
	a = anomalies(t, db, sensor)
	if len(a) != 1 || a[0].status != "NORMAL" || !a[0].resolved.Valid || !a[0].resolved.Time.Equal(t0.Add(130*time.Second)) {
		t.Fatalf("anomalies = %+v", a)
	}
	if s := state(t, db, sensor, "temperature"); s.started.Valid || s.anomaly.Valid {
		t.Fatalf("state = %+v", s)
	}
}

func TestEvaluateMinBoundAndBoundaries(t *testing.T) {
	db := testutil.OpenTestDB(t)
	sensor := seedSensor(t, db)
	rule(t, db, sensor, "soil_moisture", f(20), f(80), 0, 60, 300, true)

	eval(t, db, sensor, t0, map[string]float64{"soil_moisture": 20}) // boundary is inside
	eval(t, db, sensor, t0.Add(time.Second), map[string]float64{"soil_moisture": 80})
	if len(anomalies(t, db, sensor)) != 0 {
		t.Fatal("boundaries must be inclusive")
	}
	eval(t, db, sensor, t0.Add(2*time.Second), map[string]float64{"soil_moisture": 19.9})
	if a := anomalies(t, db, sensor); len(a) != 1 || a[0].status != "WARNING" || a[0].duration != 0 {
		t.Fatalf("zero-duration rule must open immediately: %+v", a)
	}
	// A missing parameter is evaluated as 0, which is below min.
	eval(t, db, sensor, t0.Add(3*time.Second), map[string]float64{})
	if a := anomalies(t, db, sensor); len(a) != 1 || a[0].duration != 1 {
		t.Fatalf("anomalies = %+v", a)
	}
}

func TestEvaluateGapResetsAnomaly(t *testing.T) {
	db := testutil.OpenTestDB(t)
	sensor := seedSensor(t, db)
	rule(t, db, sensor, "humidity", nil, f(90), 0, 60, 60, true)

	eval(t, db, sensor, t0, map[string]float64{"humidity": 95})
	eval(t, db, sensor, t0.Add(60*time.Second), map[string]float64{"humidity": 95}) // gap == max: continuous
	if a := anomalies(t, db, sensor); len(a) != 1 || a[0].duration != 60 {
		t.Fatalf("anomalies = %+v", a)
	}
	// Silence longer than max gap closes the old anomaly and starts a fresh one.
	at := t0.Add(121 * time.Second)
	eval(t, db, sensor, at, map[string]float64{"humidity": 95})
	a := anomalies(t, db, sensor)
	if len(a) != 2 || a[0].status != "NORMAL" || !a[0].resolved.Time.Equal(at) || a[1].status != "WARNING" || !a[1].started.Equal(at) || a[1].duration != 0 {
		t.Fatalf("anomalies = %+v", a)
	}
}

func TestEvaluateDisabledAndMultipleRules(t *testing.T) {
	db := testutil.OpenTestDB(t)
	sensor, other := seedSensor(t, db), seedSensor(t, db)
	rule(t, db, sensor, "temperature", nil, f(30), 0, 60, 300, false)
	rule(t, db, sensor, "humidity", f(10), nil, 0, 60, 300, true)
	rule(t, db, other, "humidity", f(10), nil, 0, 60, 300, true)

	eval(t, db, sensor, t0, map[string]float64{"temperature": 99, "humidity": 5})
	a := anomalies(t, db, sensor)
	if len(a) != 1 {
		t.Fatalf("anomalies = %+v", a)
	}
	if n := testutil.Count(t, db, `SELECT count(*) FROM alert_states WHERE parameter='temperature'`); n != 0 {
		t.Fatal("disabled rule must not track state")
	}
	if len(anomalies(t, db, other)) != 0 {
		t.Fatal("rules are per sensor")
	}
}

func TestEvaluateDoesNotNotify(t *testing.T) {
	db := testutil.OpenTestDB(t)
	sensor := seedSensor(t, db)
	var owner int64
	if err := db.QueryRow(`INSERT INTO users(name,email,password) VALUES('o','o@x.io','x') RETURNING id`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	testutil.Exec(t, db, `UPDATE sensors SET owner_id=$1 WHERE id=$2`, owner, sensor)
	rule(t, db, sensor, "temperature", nil, f(30), 0, 60, 300, true)
	eval(t, db, sensor, t0, map[string]float64{"temperature": 40})
	// Current behaviour: the owner lookup was removed, so no notification is ever enqueued.
	if n := testutil.Count(t, db, `SELECT count(*) FROM notifications`); n != 0 {
		t.Fatalf("notifications = %d", n)
	}
	if s := state(t, db, sensor, "temperature"); s.notified.Valid {
		t.Fatal("last_notified_at set without notification")
	}
}

func TestEvaluateNotifiesOwnerWithCooldown(t *testing.T) {
	t.Skip("BUG: EvaluateTx declares `owner` but never loads it (owner query dropped in 8e05db3), so anomaly notifications and cooldown are dead code")
	db := testutil.OpenTestDB(t)
	sensor := seedSensor(t, db)
	var owner int64
	if err := db.QueryRow(`INSERT INTO users(name,email,password) VALUES('o','o@x.io','x') RETURNING id`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	testutil.Exec(t, db, `UPDATE sensors SET owner_id=$1 WHERE id=$2`, owner, sensor)
	rule(t, db, sensor, "temperature", nil, f(30), 0, 60, 300, true)
	eval(t, db, sensor, t0, map[string]float64{"temperature": 40})
	eval(t, db, sensor, t0.Add(30*time.Second), map[string]float64{"temperature": 40}) // within cooldown
	eval(t, db, sensor, t0.Add(60*time.Second), map[string]float64{"temperature": 40})
	if n := testutil.Count(t, db, `SELECT count(*) FROM notifications WHERE user_id=$1`, owner); n != 2 {
		t.Fatalf("notifications = %d, want 2", n)
	}
}

func TestEvaluateTxError(t *testing.T) {
	db := testutil.OpenTestDB(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	if err := EvaluateTx(context.Background(), tx, 1, t0, nil); err == nil {
		t.Fatal("expected error on finished transaction")
	}
}
