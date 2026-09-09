package sensorreading

import "time"

type SensorReading struct {
	ID           int64     `json:"id"`
	SensorID     int64     `json:"sensor_id"`
	SoilMoisture float64   `json:"soil_moisture"`
	Temperature  float64   `json:"temperature"`
	Humidity     float64   `json:"humidity"`
	RecordedAt   time.Time `json:"recorded_at"`
}
