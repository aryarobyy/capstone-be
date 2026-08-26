package user

import (
	"database/sql"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, db *sql.DB) {
	repo := NewUserRepository(db)
	service := NewUserService(repo)
	handler := NewUserHandler(service)
	userGroup := router.Group("/user")
	{
		userGroup.POST("/list", handler.List)
		userGroup.POST("/detail", handler.Detail)
		userGroup.POST("/update", handler.Update)
		userGroup.POST("/delete", handler.Delete)
	}
}
