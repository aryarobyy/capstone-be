package alert

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"capstone-be/internal/notification"
)

// EvaluateTx runs while ingestion holds the sensor row lock. Out-of-order readings
// remain in history but never rewind alert state. Gaps reset the sustained duration.
func EvaluateTx(ctx context.Context, tx *sql.Tx, sensor int64, at time.Time, values map[string]float64) error {
	var owner sql.NullInt64
	rows, err := tx.QueryContext(ctx, `SELECT parameter,min_value,max_value,duration_seconds,cooldown_seconds,max_gap_seconds FROM alert_rules WHERE sensor_id=$1 AND enabled`, sensor)
	if err != nil {
		return err
	}
	type rule struct {
		parameter               string
		min, max                sql.NullFloat64
		duration, cooldown, gap int
	}
	rules := []rule{}
	for rows.Next() {
		var r rule
		if err := rows.Scan(&r.parameter, &r.min, &r.max, &r.duration, &r.cooldown, &r.gap); err != nil {
			rows.Close()
			return err
		}
		rules = append(rules, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, r := range rules {
		var start, lastNotice sql.NullTime
		var last time.Time
		var anomaly sql.NullInt64
		err := tx.QueryRowContext(ctx, `SELECT started_at,last_recorded_at,last_notified_at,anomaly_id FROM alert_states WHERE sensor_id=$1 AND parameter=$2`, sensor, r.parameter).Scan(&start, &last, &lastNotice, &anomaly)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil && !at.After(last) {
			continue
		}
		value := values[r.parameter]
		outside := (r.min.Valid && value < r.min.Float64) || (r.max.Valid && value > r.max.Float64)
		if !outside || (!last.IsZero() && at.Sub(last) > time.Duration(r.gap)*time.Second) {
			if anomaly.Valid {
				if _, err := tx.ExecContext(ctx, `UPDATE anomalies SET status='NORMAL',resolved_at=$2,updated_at=NOW() WHERE id=$1`, anomaly.Int64, at); err != nil {
					return err
				}
			}
			start = sql.NullTime{}
			anomaly = sql.NullInt64{}
		}
		if outside {
			if !start.Valid {
				start = sql.NullTime{Time: at, Valid: true}
			}
			duration := int64(at.Sub(start.Time).Seconds())
			if duration >= int64(r.duration) {
				if !anomaly.Valid {
					if err := tx.QueryRowContext(ctx, `INSERT INTO anomalies(sensor_id,parameter,started_at,last_detected_at,accumulated_duration,status) VALUES($1,$2,$3,$4,$5,'WARNING') RETURNING id`, sensor, r.parameter, start.Time, at, duration).Scan(&anomaly.Int64); err != nil {
						return err
					}
					anomaly.Valid = true
				} else {
					if _, err := tx.ExecContext(ctx, `UPDATE anomalies SET last_detected_at=$2,accumulated_duration=$3,updated_at=NOW() WHERE id=$1`, anomaly.Int64, at, duration); err != nil {
						return err
					}
				}
				if owner.Valid && (!lastNotice.Valid || at.Sub(lastNotice.Time) >= time.Duration(r.cooldown)*time.Second) {
					_, err := notification.EnqueueTx(ctx, tx, owner.Int64, notification.PushNotificationPayload{Title: "Peringatan sensor", Body: fmt.Sprintf("Sensor %d: %s = %g di luar batas selama %d detik", sensor, r.parameter, value, duration), Data: map[string]string{"sensor_id": fmt.Sprint(sensor), "parameter": r.parameter, "anomaly_id": fmt.Sprint(anomaly.Int64)}, IdempotencyKey: fmt.Sprintf("sensor:%d:%s:%d", sensor, r.parameter, at.UnixNano())})
					if err != nil {
						return err
					}
					lastNotice = sql.NullTime{Time: at, Valid: true}
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO alert_states(sensor_id,parameter,started_at,last_recorded_at,last_notified_at,anomaly_id) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(sensor_id,parameter) DO UPDATE SET started_at=EXCLUDED.started_at,last_recorded_at=EXCLUDED.last_recorded_at,last_notified_at=EXCLUDED.last_notified_at,anomaly_id=EXCLUDED.anomaly_id`, sensor, r.parameter, start, at, lastNotice, anomaly); err != nil {
			return err
		}
	}
	return nil
}
