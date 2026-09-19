package alert

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"math"
)

var ErrForbidden = errors.New("resource access denied")
var ErrInvalidRule = errors.New("invalid alert rule")

type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db} }

type Rule struct {
	SensorID  int64    `json:"sensor_id" binding:"required,gt=0"`
	Parameter string   `json:"parameter" binding:"required,oneof=temperature humidity soil_moisture"`
	Min       *float64 `json:"min_value"`
	Max       *float64 `json:"max_value"`
	Duration  int      `json:"duration_seconds" binding:"gte=0,lte=86400"`
	Cooldown  int      `json:"cooldown_seconds" binding:"gte=60,lte=604800"`
	MaxGap    int      `json:"max_gap_seconds" binding:"gte=1,lte=86400"`
	Enabled   bool     `json:"enabled"`
}

func (s *Service) SaveRule(ctx context.Context, user int64, r Rule) error {
	if r.Min == nil && r.Max == nil || r.Min != nil && (math.IsNaN(*r.Min) || math.IsInf(*r.Min, 0)) || r.Max != nil && (math.IsNaN(*r.Max) || math.IsInf(*r.Max, 0)) || r.Min != nil && r.Max != nil && *r.Min >= *r.Max {
		return ErrInvalidRule
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT s.id FROM sensors s WHERE s.id=$1 FOR UPDATE OF s`, r.SensorID).Scan(&id)
	if err == sql.ErrNoRows {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO alert_rules(sensor_id,parameter,min_value,max_value,duration_seconds,cooldown_seconds,max_gap_seconds,enabled) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(sensor_id,parameter) DO UPDATE SET min_value=EXCLUDED.min_value,max_value=EXCLUDED.max_value,duration_seconds=EXCLUDED.duration_seconds,cooldown_seconds=EXCLUDED.cooldown_seconds,max_gap_seconds=EXCLUDED.max_gap_seconds,enabled=EXCLUDED.enabled`, r.SensorID, r.Parameter, r.Min, r.Max, r.Duration, r.Cooldown, r.MaxGap, r.Enabled)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE anomalies SET status='NORMAL',resolved_at=NOW(),updated_at=NOW() WHERE id IN(SELECT anomaly_id FROM alert_states WHERE sensor_id=$1 AND parameter=$2)`, r.SensorID, r.Parameter); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM alert_states WHERE sensor_id=$1 AND parameter=$2`, r.SensorID, r.Parameter); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) Rules(ctx context.Context, user, sensor int64) ([]Rule, error) {
	var allowed bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sensors WHERE id=$1)`, sensor).Scan(&allowed); err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	rows, err := s.db.QueryContext(ctx, `SELECT sensor_id,parameter,min_value,max_value,duration_seconds,cooldown_seconds,max_gap_seconds,enabled FROM alert_rules WHERE sensor_id=$1 ORDER BY parameter`, sensor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.SensorID, &r.Parameter, &r.Min, &r.Max, &r.Duration, &r.Cooldown, &r.MaxGap, &r.Enabled); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Service) SensorKey(ctx context.Context, user, sensor int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(raw))
	result, err := s.db.ExecContext(ctx, `UPDATE sensors s SET api_key_hash=$2 WHERE s.id=$1`, sensor, hash[:])
	if err != nil {
		return "", err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	if count == 0 {
		return "", ErrForbidden
	}
	return raw, nil
}

type Area struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (s *Service) CreateArea(ctx context.Context, user int64, name string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `INSERT INTO areas(name) VALUES($1) RETURNING id`, name).Scan(&id)
	return id, err
}
func (s *Service) Areas(ctx context.Context, user int64) ([]Area, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name FROM areas ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Area{}
	for rows.Next() {
		var a Area
		if err := rows.Scan(&a.ID, &a.Name); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Assignment of legacy unowned areas is administrative and cannot be claimed by arbitrary users.
func (s *Service) AssignArea(ctx context.Context, id, owner int64) error {
	return nil
}
