package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrHistoryNotFound      = errors.New("history not found")
	ErrInvalidAnomalyStatus = errors.New("invalid anomaly status")
)

type HistoryRepository interface {
	Create(ctx context.Context, req CreateHistoryRequest) (*Anomaly, error)
	List(ctx context.Context, filter HistoryFilter) ([]Anomaly, int, error)
	Newest(ctx context.Context, req NewestHistoryRequest) (*Anomaly, error)
	Detail(ctx context.Context, req DetailHistoryRequest) (*Anomaly, error)
	Delete(ctx context.Context, req DeleteHistoryRequest) error
	Since(ctx context.Context, filter HistoryFilter, afterID int64) ([]Anomaly, error)
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

// historyWhere builds the WHERE clause shared by List and Since.
func historyWhere(filter HistoryFilter) (string, []any) {
	conditions := make([]string, 0, 5)
	args := make([]any, 0, 7)
	add := func(cond string, arg any) {
		args = append(args, arg)
		conditions = append(conditions, fmt.Sprintf(cond, len(args)))
	}

	if filter.OwnerID != 0 {
		add("sensor_id IN (SELECT id FROM sensors WHERE owner_id = $%d)", filter.OwnerID)
	}
	if filter.SensorID != 0 {
		add("sensor_id = $%d", filter.SensorID)
	}
	if filter.Parameter != "" {
		add("parameter = $%d", filter.Parameter)
	}
	if filter.Status != "" {
		add("status = $%d", filter.Status)
	}
	if filter.Date != "" {
		add("started_at::date = $%d::date", filter.Date)
	}
	if filter.Active {
		// Records are append-only, so "ongoing" means the evaluator still points at it.
		conditions = append(conditions, "id IN (SELECT anomaly_id FROM alert_states WHERE anomaly_id IS NOT NULL)")
	}

	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func (r *historyRepository) List(ctx context.Context, filter HistoryFilter) ([]Anomaly, int, error) {
	whereClause, args := historyWhere(filter)
	argIdx := len(args) + 1

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

func (r *historyRepository) Newest(ctx context.Context, req NewestHistoryRequest) (*Anomaly, error) {
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
		WHERE ($1::bigint = 0 OR sensor_id = $1::bigint)
			AND ($2::text = '' OR parameter = $2::text)
			AND ($3::bigint = 0 OR sensor_id IN (SELECT id FROM sensors WHERE owner_id = $3::bigint))
		ORDER BY started_at DESC, id DESC
		LIMIT 1
	`

	var anomaly Anomaly
	if err := scanAnomaly(r.db.QueryRowContext(ctx, query, req.SensorID, req.Parameter, req.OwnerID), &anomaly); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrHistoryNotFound
		}
		return nil, err
	}

	return &anomaly, nil
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
			AND ($2::bigint = 0 OR sensor_id IN (SELECT id FROM sensors WHERE owner_id = $2::bigint))
	`

	var anomaly Anomaly
	if err := scanAnomaly(r.db.QueryRowContext(ctx, query, req.ID, req.OwnerID), &anomaly); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrHistoryNotFound
		}
		return nil, err
	}

	return &anomaly, nil
}

func (r *historyRepository) Delete(ctx context.Context, req DeleteHistoryRequest) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM anomalies WHERE id = $1 AND ($2::bigint = 0 OR sensor_id IN (SELECT id FROM sensors WHERE owner_id = $2::bigint))", req.ID, req.OwnerID)
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

func (r *historyRepository) Since(ctx context.Context, filter HistoryFilter, afterID int64) ([]Anomaly, error) {
	whereClause, args := historyWhere(filter)
	args = append(args, afterID, filter.Limit)
	idCond := fmt.Sprintf("id > $%d", len(args)-1)
	if whereClause == "" {
		whereClause = "WHERE " + idCond
	} else {
		whereClause += " AND " + idCond
	}

	query := fmt.Sprintf(`
		SELECT * FROM (
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
			ORDER BY id DESC
			LIMIT $%d
		) latest ORDER BY id
	`, whereClause, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	histories := make([]Anomaly, 0)
	for rows.Next() {
		var anomaly Anomaly
		if err := scanAnomaly(rows, &anomaly); err != nil {
			return nil, err
		}
		histories = append(histories, anomaly)
	}
	return histories, rows.Err()
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
