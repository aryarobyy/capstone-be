package sensor

import "time"

type CreateSensorRequest struct {
	AreaID      int64  `json:"area_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Type        string `json:"type" binding:"required"`
	Description string `json:"description" binding:"required"`
}

type UpdateSensorRequest struct {
	ID          int64  `json:"id" binding:"required"`
	AreaID      int64  `json:"area_id" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Type        string `json:"type" binding:"required"`
	Description string `json:"description" binding:"required"`
}

type DetailSensorRequest struct {
	ID int64 `json:"id" binding:"required"`
}

type ListSensorRequest struct {
	AreaID int64  `json:"area_id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Limit  int    `json:"limit"`
	Index  int    `json:"index"`
}

type SensorFilter struct {
	AreaID int64
	Name   string
	Type   string
	Limit  int
	Index  int
}

type DeleteSensorRequest struct {
	ID int64 `json:"id" binding:"required"`
}

type ListSensorData struct {
	ID          int64     `json:"id"`
	AreaID      int64     `json:"area_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ListSensorResponse struct {
	Data  []ListSensorData `json:"data"`
	Total int              `json:"total"`
	Limit int              `json:"limit"`
	Index int              `json:"index"`
}

type SensorResponse struct {
	ID          int64     `json:"id"`
	AreaID      int64     `json:"area_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
