package health

type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}
