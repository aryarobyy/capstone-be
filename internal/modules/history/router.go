package history

import (
	"database/sql"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, db *sql.DB) {
	repo := NewHistoryRepository(db)
	service := NewHistoryService(repo)
	handler := NewHistoryHandler(service)

	historyGroup := router.Group("/history")
	{
		historyGroup.POST("/create", handler.Create)
		historyGroup.POST("/list", handler.List)
		historyGroup.POST("/detail", handler.Detail)
		historyGroup.POST("/delete", handler.Delete)
	}
}
