package sensor

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
