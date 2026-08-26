package sensor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrSensorNotFound = errors.New("sensor not found")
)

type SensorRepository interface {
	Create(ctx context.Context, req CreateSensorRequest) (int64, error)
	Update(ctx context.Context, req UpdateSensorRequest) error
	Delete(ctx context.Context, req DeleteSensorRequest) error
	List(ctx context.Context, filter SensorFilter) ([]Sensor, int, error)
	Detail(ctx context.Context, req DetailSensorRequest) (*Sensor, error)
}

type sensoriRepository struct {
	db *sql.DB
}

func NewSensorRepository(db *sql.DB) SensorRepository {
	return &sensoriRepository{db: db}
}

func (r *sensoriRepository) Create(ctx context.Context, req CreateSensorRequest) (int64, error) {
	query := `INSERT INTO sensors (area_id, name, type, description) VALUES ($1, $2, $3, $4) RETURNING id`
	var id int64
	err := r.db.QueryRowContext(ctx, query, req.AreaID, req.Name, req.Type, req.Description).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *sensoriRepository) Update(ctx context.Context, req UpdateSensorRequest) error {
	query := `UPDATE sensors SET area_id = $1, name = $2, type = $3, description = $4 WHERE id = $5`
	result, err := r.db.ExecContext(ctx, query, req.AreaID, req.Name, req.Type, req.Description, req.ID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrSensorNotFound
	}
	return nil
}

func (r *sensoriRepository) Delete(ctx context.Context, req DeleteSensorRequest) error {
	query := `DELETE FROM sensors WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, req.ID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrSensorNotFound
	}
	return nil
}

func (r *sensoriRepository) List(ctx context.Context, filter SensorFilter) ([]Sensor, int, error) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 5)
	argIndex := 1

	if filter.AreaID != 0 {
		conditions = append(conditions, fmt.Sprintf("area_id = $%d", argIndex))
		args = append(args, filter.AreaID)
		argIndex++
	}
	if filter.Name != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argIndex))
		args = append(args, "%"+filter.Name+"%")
		argIndex++
	}
	if filter.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIndex))
		args = append(args, filter.Type)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM sensors %s", whereClause)
	var count int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&count); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT id, area_id, name, type, description, created_at, updated_at
		FROM sensors
		%s
		ORDER BY id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)
	args = append(args, filter.Limit, filter.Index)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	sensors := make([]Sensor, 0)
	for rows.Next() {
		var sensor Sensor
		if err := rows.Scan(&sensor.ID, &sensor.AreaID, &sensor.Name, &sensor.Type, &sensor.Description, &sensor.CreatedAt, &sensor.UpdatedAt); err != nil {
			return nil, 0, err
		}
		sensors = append(sensors, sensor)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return sensors, count, nil
}

func (r *sensoriRepository) Detail(ctx context.Context, req DetailSensorRequest) (*Sensor, error) {
	query := `SELECT id, area_id, name, type, description, created_at, updated_at FROM sensors WHERE id = $1`
	var sensor Sensor
	err := r.db.QueryRowContext(ctx, query, req.ID).Scan(&sensor.ID, &sensor.AreaID, &sensor.Name, &sensor.Type, &sensor.Description, &sensor.CreatedAt, &sensor.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSensorNotFound
		}
		return nil, err
	}
	return &sensor, nil
}
