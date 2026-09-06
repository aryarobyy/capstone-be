package history

import "time"

type CreateHistoryRequest struct {
	SensorID            int64         `json:"sensor_id" binding:"required"`
	Parameter           string        `json:"parameter" binding:"required"`
	StartedAt           time.Time     `json:"started_at" binding:"required"`
	LastDetectedAt      time.Time     `json:"last_detected_at"`
	AccumulatedDuration int64         `json:"accumulated_duration"`
	Status              AnomalyStatus `json:"status" binding:"required"`
	ResolvedAt          *time.Time    `json:"resolved_at"`
}

type ListHistoryRequest struct {
	SensorID  int64  `json:"sensor_id"`
	Parameter string `json:"parameter"`
	Status    string `json:"status"`
	Date      string `json:"date"`
	Limit     int    `json:"limit"`
	Index     int    `json:"index"`
}

type HistoryResponse struct {
	ID                  int64         `json:"id"`
	SensorID            int64         `json:"sensor_id"`
	Parameter           string        `json:"parameter"`
	StartedAt           time.Time     `json:"started_at"`
	LastDetectedAt      time.Time     `json:"last_detected_at"`
	AccumulatedDuration int64         `json:"accumulated_duration"`
	Status              AnomalyStatus `json:"status"`
	ResolvedAt          *time.Time    `json:"resolved_at"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
}

type ListHistoryResponse struct {
	Data  []HistoryResponse `json:"data"`
	Total int               `json:"total"`
	Limit int               `json:"limit"`
	Index int               `json:"index"`
}

type DetailHistoryRequest struct {
	ID int64 `json:"id" binding:"required"`
}

type DeleteHistoryRequest struct {
	ID int64 `json:"id" binding:"required"`
}

type HistoryFilter struct {
	SensorID  int64
	Parameter string
	Status    string
	Date      string
	Limit     int
	Index     int
}
