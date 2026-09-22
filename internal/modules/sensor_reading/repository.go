package sensorreading

import (
	"capstone-be/internal/alert"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var (
	ErrSensorReadingNotFound = errors.New("sensor reading not found")
)

type SensorReadingRepository interface {
	Create(ctx context.Context, req CreateSensorReadingRequest) error
	List(ctx context.Context, filter SensorReadingFilter) ([]SensorReading, int, error)
	Detail(ctx context.Context, req DetailSensorReadingRequest) (*SensorReading, error)
	Delete(ctx context.Context, req DeleteSensorReadingRequest) error
	CalculateSummary(ctx context.Context, sensorID int64, ownerID int64, windowMinutes int) ([]SensorSummaryItem, error)
}

type sensorReadingRepository struct {
	db *sql.DB
}

func NewSensorReadingRepository(db *sql.DB) SensorReadingRepository {
	return &sensorReadingRepository{db: db}
}

func (r *sensorReadingRepository) Create(ctx context.Context, req CreateSensorReadingRequest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM sensors WHERE id=$1 FOR UPDATE`, req.SensorID).Scan(&id); err != nil {
		return err
	}
	var duplicate bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sensor_readings WHERE sensor_id=$1 AND recorded_at=$2)`, req.SensorID, req.RecordedAt).Scan(&duplicate); err != nil {
		return err
	}
	if duplicate {
		return tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO sensor_readings(sensor_id,soil_moisture,temperature,humidity,recorded_at) VALUES($1,$2,$3,$4,$5)`, req.SensorID, req.SoilMoisture, req.Temperature, req.Humidity, req.RecordedAt); err != nil {
		return err
	}
	if err = alert.EvaluateTx(ctx, tx, req.SensorID, req.RecordedAt, map[string]float64{"soil_moisture": req.SoilMoisture, "temperature": req.Temperature, "humidity": req.Humidity}); err != nil {
		return err
	}
	return tx.Commit()
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

func (r *sensorReadingRepository) CalculateSummary(ctx context.Context, sensorID int64, ownerID int64, windowMinutes int) ([]SensorSummaryItem, error) {
	if windowMinutes <= 0 {
		windowMinutes = 10
	}

	type sensorInfo struct {
		id      int64
		name    string
		code    string
		areaID  int64
		ownerID int64
	}

	var sensors []sensorInfo

	if sensorID > 0 {
		query := `SELECT id, name, code, COALESCE(area_id, 0), owner_id FROM sensors WHERE id = $1`
		var s sensorInfo
		err := r.db.QueryRowContext(ctx, query, sensorID).Scan(&s.id, &s.name, &s.code, &s.areaID, &s.ownerID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return []SensorSummaryItem{}, nil
			}
			return nil, err
		}
		if ownerID > 0 && s.ownerID != ownerID {
			return []SensorSummaryItem{}, nil
		}
		sensors = append(sensors, s)
	} else if ownerID > 0 {
		query := `SELECT id, name, code, COALESCE(area_id, 0), owner_id FROM sensors WHERE owner_id = $1 ORDER BY id ASC`
		rows, err := r.db.QueryContext(ctx, query, ownerID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var s sensorInfo
			if err := rows.Scan(&s.id, &s.name, &s.code, &s.areaID, &s.ownerID); err != nil {
				return nil, err
			}
			sensors = append(sensors, s)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	} else {
		query := `SELECT id, name, code, COALESCE(area_id, 0), owner_id FROM sensors ORDER BY id ASC`
		rows, err := r.db.QueryContext(ctx, query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var s sensorInfo
			if err := rows.Scan(&s.id, &s.name, &s.code, &s.areaID, &s.ownerID); err != nil {
				return nil, err
			}
			sensors = append(sensors, s)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	result := make([]SensorSummaryItem, 0, len(sensors))

	for _, s := range sensors {
		item := SensorSummaryItem{
			SensorID:     s.id,
			SensorName:   s.name,
			SensorCode:   s.code,
			AreaID:       s.areaID,
			CalculatedAt: now,
			Status:       "INACTIVE",
		}

		aggQuery := `
			SELECT 
				COUNT(*),
				COALESCE(AVG(soil_moisture), 0),
				COALESCE(AVG(temperature), 0),
				COALESCE(AVG(humidity), 0),
				COALESCE(MIN(soil_moisture), 0),
				COALESCE(MIN(temperature), 0),
				COALESCE(MIN(humidity), 0),
				COALESCE(MAX(soil_moisture), 0),
				COALESCE(MAX(temperature), 0),
				COALESCE(MAX(humidity), 0)
			FROM sensor_readings
			WHERE sensor_id = $1 AND recorded_at >= NOW() - ($2 || ' minutes')::INTERVAL
		`
		var totalSamples int
		var avgSoil, avgTemp, avgHum float64
		var minSoil, minTemp, minHum float64
		var maxSoil, maxTemp, maxHum float64

		err := r.db.QueryRowContext(ctx, aggQuery, s.id, windowMinutes).Scan(
			&totalSamples,
			&avgSoil, &avgTemp, &avgHum,
			&minSoil, &minTemp, &minHum,
			&maxSoil, &maxTemp, &maxHum,
		)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}

		item.TotalSamples = totalSamples
		item.Average = TelemetryValues{
			SoilMoisture: roundFloat(avgSoil, 2),
			Temperature:  roundFloat(avgTemp, 2),
			Humidity:     roundFloat(avgHum, 2),
		}
		item.Min = TelemetryValues{
			SoilMoisture: roundFloat(minSoil, 2),
			Temperature:  roundFloat(minTemp, 2),
			Humidity:     roundFloat(minHum, 2),
		}
		item.Max = TelemetryValues{
			SoilMoisture: roundFloat(maxSoil, 2),
			Temperature:  roundFloat(maxTemp, 2),
			Humidity:     roundFloat(maxHum, 2),
		}

		latestQuery := `
			SELECT soil_moisture, temperature, humidity, recorded_at
			FROM sensor_readings
			WHERE sensor_id = $1
			ORDER BY recorded_at DESC, id DESC
			LIMIT 1
		`
		var latest LatestTelemetry
		err = r.db.QueryRowContext(ctx, latestQuery, s.id).Scan(
			&latest.SoilMoisture,
			&latest.Temperature,
			&latest.Humidity,
			&latest.RecordedAt,
		)
		if err == nil {
			item.Latest = latest
		}

		if totalSamples > 0 || !latest.RecordedAt.IsZero() {
			var activeAnomalies int
			anomalyQuery := `
				SELECT COUNT(*) 
				FROM anomalies 
				WHERE sensor_id = $1 AND status IN ('WARNING', 'ANOMALY') AND resolved_at IS NULL
			`
			_ = r.db.QueryRowContext(ctx, anomalyQuery, s.id).Scan(&activeAnomalies)
			if activeAnomalies > 0 {
				item.Status = "ANOMALY"
			} else {
				if latest.SoilMoisture < 20.0 || latest.SoilMoisture > 85.0 || latest.Temperature > 38.0 || latest.Temperature < 15.0 {
					item.Status = "WARNING"
				} else {
					item.Status = "OPTIMAL"
				}
			}
		}

		result = append(result, item)
	}

	return result, nil
}

func roundFloat(val float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

