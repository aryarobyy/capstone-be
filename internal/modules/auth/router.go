package auth

import (
	"database/sql"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, db *sql.DB) {
	repo := NewAuthRepository(db)
	service := NewAuthService(repo)
	handler := NewAuthHandler(service)

	authGroup := router.Group("/auth")
	{
		authGroup.POST("/register", handler.Register)
		authGroup.POST("/login", handler.Login)
	}
}
