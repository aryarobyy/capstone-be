package history

import "time"

type AnomalyStatus string

const (
	AnomalyStatusFatal    AnomalyStatus = "FATAL"
	AnomalyStatusCritical AnomalyStatus = "CRITICAL"
	AnomalyStatusWarning  AnomalyStatus = "WARNING"
	AnomalyStatusNormal   AnomalyStatus = "NORMAL"
)

type Anomaly struct {
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

func (s AnomalyStatus) IsValid() bool {
	switch s {
	case AnomalyStatusFatal, AnomalyStatusCritical, AnomalyStatusWarning, AnomalyStatusNormal:
		return true
	default:
		return false
	}
}

