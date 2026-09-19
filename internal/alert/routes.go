package alert

import (
	"capstone-be/internal/middleware"
	"database/sql"
	"github.com/gin-gonic/gin"
)

// api must already have JWT/session authentication applied.
func RegisterRoutes(api *gin.RouterGroup, db *sql.DB) {
	h := NewHandler(NewService(db))
	api.PUT("/alerts/rules", h.SaveRule)
	api.GET("/alerts/rules", h.Rules)
	api.POST("/sensors/key", h.SensorKey)
	api.POST("/areas", h.CreateArea)
	api.GET("/areas", h.Areas)
	api.PATCH("/areas/owner", middleware.AdminOnly(db), h.AssignArea)
}
