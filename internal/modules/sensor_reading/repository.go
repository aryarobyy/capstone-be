package sensorreading

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrSensorReadingNotFound = errors.New("sensor reading not found")
)

type SensorReadingRepository interface {
	Create(ctx context.Context, req CreateSensorReadingRequest) error
	List(ctx context.Context, filter SensorReadingFilter) ([]SensorReading, int, error)
	Detail(ctx context.Context, req DetailSensorReadingRequest) (*SensorReading, error)
	Delete(ctx context.Context, req DeleteSensorReadingRequest) error
}

type sensorReadingRepository struct {
	db *sql.DB
}

func NewSensorReadingRepository(db *sql.DB) SensorReadingRepository {
	return &sensorReadingRepository{db: db}
}

func (r *sensorReadingRepository) Create(ctx context.Context, req CreateSensorReadingRequest) error {
	query := `
		INSERT INTO sensor_readings (sensor_id, soil_moisture, temperature, humidity, recorded_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.ExecContext(ctx, query, req.SensorID, req.SoilMoisture, req.Temperature, req.Humidity, req.RecordedAt)
	if err != nil {
		return err
	}
	return nil
}

func (r *sensorReadingRepository) List(ctx context.Context, filter SensorReadingFilter) ([]SensorReading, int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.SensorID != 0 {
		conditions = append(conditions, fmt.Sprintf("sensor_id = $%d", argIdx))
		args = append(args, filter.SensorID)
		argIdx++
	}

	if filter.Date != "" {
		conditions = append(conditions, fmt.Sprintf("recorded_at::date = $%d::date", argIdx))
		args = append(args, filter.Date)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM sensor_readings %s", whereClause)
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	dataQuery := fmt.Sprintf(`
		SELECT
			id,
			sensor_id,
			soil_moisture,
			temperature,
			humidity,
			recorded_at
		FROM sensor_readings
		%s
		ORDER BY id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	queryArgs := append(args, filter.Limit, filter.Index)
	rows, err := r.db.QueryContext(ctx, dataQuery, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]SensorReading, 0)
	for rows.Next() {
		var h SensorReading
		if err := rows.Scan(&h.ID, &h.SensorID, &h.SoilMoisture, &h.Temperature, &h.Humidity, &h.RecordedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, h)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return list, totalCount, nil
}

func (r *sensorReadingRepository) Detail(ctx context.Context, req DetailSensorReadingRequest) (*SensorReading, error) {
	query := `
		SELECT id, sensor_id, soil_moisture, temperature, humidity, recorded_at
		FROM sensor_readings
		WHERE id = $1
	`
	var h SensorReading
	err := r.db.QueryRowContext(ctx, query, req.ID).Scan(
		&h.ID,
		&h.SensorID,
		&h.SoilMoisture,
		&h.Temperature,
		&h.Humidity,
		&h.RecordedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSensorReadingNotFound
		}
		return nil, err
	}
	return &h, nil
}

func (r *sensorReadingRepository) Delete(ctx context.Context, req DeleteSensorReadingRequest) error {
	query := `
		DELETE FROM sensor_readings
		WHERE id = $1
	`
	result, err := r.db.ExecContext(ctx, query, req.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrSensorReadingNotFound
	}
	return nil
}
