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
