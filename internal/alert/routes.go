package alert

import (
	"capstone-be/internal/middleware"
	"database/sql"
	"github.com/gin-gonic/gin"
)

// api must already have JWT/session authentication applied.
func RegisterRoutes(api *gin.RouterGroup, db *sql.DB) {
	h := NewHandler(NewService(db))
	api.POST("/alerts/rules", h.SaveRule)
	api.POST("/alerts/rules/save", h.SaveRule)
	api.POST("/alerts/rules/list", h.Rules)
	api.POST("/sensors/key", h.SensorKey)
	api.POST("/areas", h.CreateArea)
	api.POST("/areas/create", h.CreateArea)
	api.POST("/areas/list", h.Areas)
	api.POST("/areas/owner", middleware.AdminOnly(db), h.AssignArea)
}
