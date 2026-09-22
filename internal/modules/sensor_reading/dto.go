package sensorreading

import "time"

type CreateSensorReadingRequest struct {
	SensorID     int64     `json:"sensor_id" binding:"required"`
	SoilMoisture float64   `json:"soil_moisture"`
	Temperature  float64   `json:"temperature"`
	Humidity     float64   `json:"humidity"`
	RecordedAt   time.Time `json:"recorded_at" binding:"required"`
}

type ListSensorReadingRequest struct {
	SensorID int64  `json:"sensor_id"`
	Date     string `json:"date"`
	Limit    int    `json:"limit"`
	Index    int    `json:"index"`
}

type SensorReadingResponse struct {
	ID           int64     `json:"id"`
	SensorID     int64     `json:"sensor_id"`
	SoilMoisture float64   `json:"soil_moisture"`
	Temperature  float64   `json:"temperature"`
	Humidity     float64   `json:"humidity"`
	RecordedAt   time.Time `json:"recorded_at"`
}

type ListSensorReadingResponse struct {
	Data  []SensorReadingResponse `json:"data"`
	Total int                     `json:"total"`
	Limit int                     `json:"limit"`
	Index int                     `json:"index"`
}

type DetailSensorReadingRequest struct {
	ID int64 `json:"id" binding:"required"`
}

type DeleteSensorReadingRequest struct {
	ID int64 `json:"id" binding:"required"`
}

type SensorReadingFilter struct {
	SensorID int64
	Date     string
	Limit    int
	Index    int
}

type SensorSummaryRequest struct {
	SensorID      int64 `json:"sensor_id"`
	WindowMinutes int   `json:"window_minutes"`
}

type TelemetryValues struct {
	SoilMoisture float64 `json:"soil_moisture"`
	Temperature  float64 `json:"temperature"`
	Humidity     float64 `json:"humidity"`
}

type LatestTelemetry struct {
	SoilMoisture float64   `json:"soil_moisture"`
	Temperature  float64   `json:"temperature"`
	Humidity     float64   `json:"humidity"`
	RecordedAt   time.Time `json:"recorded_at"`
}

type SensorSummaryItem struct {
	SensorID     int64           `json:"sensor_id"`
	SensorName   string          `json:"sensor_name,omitempty"`
	SensorCode   string          `json:"sensor_code,omitempty"`
	AreaID       int64           `json:"area_id,omitempty"`
	TotalSamples int             `json:"total_samples"`
	Latest       LatestTelemetry `json:"latest"`
	Average      TelemetryValues `json:"average"`
	Min          TelemetryValues `json:"min"`
	Max          TelemetryValues `json:"max"`
	Status       string          `json:"status"`
	CalculatedAt time.Time       `json:"calculated_at"`
}

type SensorSummaryResponse struct {
	WindowMinutes int                 `json:"window_minutes"`
	Data          []SensorSummaryItem `json:"data"`
}
