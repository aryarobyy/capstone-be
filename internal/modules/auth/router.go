package auth

import (
	"capstone-be/internal/middleware"
	"capstone-be/internal/session"
	"database/sql"
	"time"

	"capstone-be/internal/token"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, db *sql.DB, tokens *token.Manager, sessions session.Store) {
	repo := NewAuthRepository(db)
	service := NewAuthService(repo, tokens, sessions)
	handler := NewAuthHandler(service)

	authGroup := router.Group("/auth")
	authGroup.Use(middleware.RateLimit(30, time.Minute))
	{
		authGroup.POST("/register", handler.Register)
		authGroup.POST("/login", handler.Login)
		authGroup.POST("/refresh", handler.Refresh)
		authGroup.POST("/logout-refresh", handler.LogoutRefresh)
		authGroup.POST("/logout", middleware.JWTAuth(tokens, sessions), handler.Logout)
	}
}
