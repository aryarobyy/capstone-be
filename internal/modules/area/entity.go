package area

import "time"

type Area struct {
	ID        int64     `json:"id"`
	FarmID    int64     `json:"farm_id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
