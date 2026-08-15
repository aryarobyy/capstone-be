package user

import (
	"database/sql"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, db *sql.DB) {
	repo := NewUserRepository(db)
	service := NewUserService(repo)
	handler := NewUserHandler(service)

	userGroup := router.Group("/users")
	{
		userGroup.POST("", handler.Register)
		userGroup.GET("", handler.GetAll)
		userGroup.GET("/:id", handler.GetByID)
		userGroup.PUT("/:id", handler.Update)
		userGroup.DELETE("/:id", handler.Delete)
	}
}
