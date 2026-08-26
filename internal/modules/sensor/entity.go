package sensor

import "time"

type Sensor struct {
	ID          int64     `json:"id"`
	AreaID      int64     `json:"area_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Code        string    `json:"code" binding:"required"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
