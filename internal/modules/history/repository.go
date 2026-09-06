package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrHistoryNotFound     = errors.New("history not found")
	ErrInvalidAnomalyStatus = errors.New("invalid anomaly status")
)

type HistoryRepository interface {
	Create(ctx context.Context, req CreateHistoryRequest) (*Anomaly, error)
	List(ctx context.Context, filter HistoryFilter) ([]Anomaly, int, error)
	Detail(ctx context.Context, req DetailHistoryRequest) (*Anomaly, error)
	Delete(ctx context.Context, req DeleteHistoryRequest) error
}

type historyRepository struct {
	db *sql.DB
}

func NewHistoryRepository(db *sql.DB) HistoryRepository {
	return &historyRepository{db: db}
}

func (r *historyRepository) Create(ctx context.Context, req CreateHistoryRequest) (*Anomaly, error) {
	query := `
		INSERT INTO anomalies (
			sensor_id,
			parameter,
			started_at,
			last_detected_at,
			accumulated_duration,
			status,
			resolved_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id,
			sensor_id,
			parameter,
			started_at,
			last_detected_at,
			accumulated_duration,
			status,
			resolved_at,
			created_at,
			updated_at
	`

	var anomaly Anomaly
	err := scanAnomaly(
		r.db.QueryRowContext(
			ctx,
			query,
			req.SensorID,
			req.Parameter,
			req.StartedAt,
			req.LastDetectedAt,
			req.AccumulatedDuration,
			req.Status,
			req.ResolvedAt,
		),
		&anomaly,
	)
	if err != nil {
		return nil, err
	}

	return &anomaly, nil
}

func (r *historyRepository) List(ctx context.Context, filter HistoryFilter) ([]Anomaly, int, error) {
	conditions := make([]string, 0, 4)
	args := make([]any, 0, 6)
	argIdx := 1

	if filter.SensorID != 0 {
		conditions = append(conditions, fmt.Sprintf("sensor_id = $%d", argIdx))
		args = append(args, filter.SensorID)
		argIdx++
	}
	if filter.Parameter != "" {
		conditions = append(conditions, fmt.Sprintf("parameter = $%d", argIdx))
		args = append(args, filter.Parameter)
		argIdx++
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.Date != "" {
		conditions = append(conditions, fmt.Sprintf("started_at::date = $%d::date", argIdx))
		args = append(args, filter.Date)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM anomalies %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT
			id,
			sensor_id,
			parameter,
			started_at,
			last_detected_at,
			accumulated_duration,
			status,
			resolved_at,
			created_at,
			updated_at
		FROM anomalies
		%s
		ORDER BY started_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	queryArgs := append(args, filter.Limit, filter.Index)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	histories := make([]Anomaly, 0)
	for rows.Next() {
		var anomaly Anomaly
		if err := scanAnomaly(rows, &anomaly); err != nil {
			return nil, 0, err
		}
		histories = append(histories, anomaly)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return histories, total, nil
}

func (r *historyRepository) Detail(ctx context.Context, req DetailHistoryRequest) (*Anomaly, error) {
	query := `
		SELECT
			id,
			sensor_id,
			parameter,
			started_at,
			last_detected_at,
			accumulated_duration,
			status,
			resolved_at,
			created_at,
			updated_at
		FROM anomalies
		WHERE id = $1
	`

	var anomaly Anomaly
	if err := scanAnomaly(r.db.QueryRowContext(ctx, query, req.ID), &anomaly); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrHistoryNotFound
		}
		return nil, err
	}

	return &anomaly, nil
}

func (r *historyRepository) Delete(ctx context.Context, req DeleteHistoryRequest) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM anomalies WHERE id = $1", req.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrHistoryNotFound
	}

	return nil
}

type anomalyScanner interface {
	Scan(dest ...any) error
}

func scanAnomaly(scanner anomalyScanner, anomaly *Anomaly) error {
	return scanner.Scan(
		&anomaly.ID,
		&anomaly.SensorID,
		&anomaly.Parameter,
		&anomaly.StartedAt,
		&anomaly.LastDetectedAt,
		&anomaly.AccumulatedDuration,
		&anomaly.Status,
		&anomaly.ResolvedAt,
		&anomaly.CreatedAt,
		&anomaly.UpdatedAt,
	)
}
