package user

import (
	"capstone-be/internal/middleware"
	"database/sql"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, db *sql.DB) {
	repo := NewUserRepository(db)
	service := NewUserService(repo)
	handler := NewUserHandler(service)
	userGroup := router.Group("/user")
	{
		userGroup.POST("/list", middleware.AdminOnly(db), handler.List)
		userGroup.POST("/detail", handler.Detail)
		userGroup.POST("/update", handler.Update)
		userGroup.POST("/delete", handler.Delete)
	}
}
