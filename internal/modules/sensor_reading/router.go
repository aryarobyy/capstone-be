package sensorreading

import (
	"database/sql"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, apiGroup *gin.RouterGroup, db *sql.DB) {
	repo := NewSensorReadingRepository(db)
	service := NewSensorReadingService(repo)
	handler := NewSensorReadingHandler(service)
	sensorReadingGroup := router.Group("/sensor-reading")
	apiGroup.POST("/sensor-reading/data/list", handler.List)
	{
		sensorReadingGroup.POST("/create", handler.Create)
		sensorReadingGroup.POST("/list", handler.List)
		sensorReadingGroup.POST("/detail", handler.Detail)
		sensorReadingGroup.POST("/delete", handler.Delete)
		sensorReadingGroup.POST("/summary", handler.Summary)
	}
}
