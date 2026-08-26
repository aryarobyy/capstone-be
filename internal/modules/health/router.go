package health

import (
	"database/sql"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, db *sql.DB) {
	handler := NewHealthHandler(db)

	healthGroup := router.Group("/health")
	{
		healthGroup.GET("", handler.Check)
	}
}
