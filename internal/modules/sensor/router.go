package sensor

import (
	"database/sql"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, db *sql.DB) {
	repo := NewSensorRepository(db)
	service := NewSensorService(repo)
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
