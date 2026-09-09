package sensor

import (
	"database/sql"

	"capstone-be/internal/modules/area"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, db *sql.DB) {
	areaRepo := area.NewAreaRepository(db)
	repo := NewSensorRepository(db)
	service := NewSensorService(repo, areaRepo)
	handler := NewSensorHandler(service)

	sensorGroup := router.Group("/sensor")
	{
		sensorGroup.POST("/create", handler.Create)
		sensorGroup.POST("/list", handler.List)
		sensorGroup.POST("/detail", handler.Detail)
		sensorGroup.POST("/update", handler.Update)
		sensorGroup.POST("/delete", handler.Delete)
	}
}
