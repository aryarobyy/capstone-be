package notification

import (
	"capstone-be/internal/middleware"
	"database/sql"
	"errors"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes expects api to already have JWT/session authentication applied.
func RegisterRoutes(api *gin.RouterGroup, s Service) {
	api.POST("/notifications/test", func(c *gin.Context) {
		var p struct {
			Title string            `json:"title"`
			Body  string            `json:"body"`
			Data  map[string]string `json:"data"`
		}
		_ = c.ShouldBindJSON(&p)
		if p.Title == "" {
			p.Title = "Test notifikasi"
		}
		if p.Body == "" {
			p.Body = "Halo dari capstone-be"
		}
		res, err := s.SendNotification(c.Request.Context(), c.MustGet(middleware.UserIDKey).(int64), p.Title, p.Body, p.Data)
		if errors.Is(err, ErrInvalidPayload) {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"error": "operation failed"})
			return
		}
		c.JSON(202, res)
	})
	api.POST("/notifications/status", func(c *gin.Context) {
		var r struct {
			ID int64 `json:"id" binding:"required,gt=0"`
		}
		if c.ShouldBindJSON(&r) != nil {
			c.Status(400)
			return
		}
		res, err := s.Status(c.Request.Context(), c.MustGet(middleware.UserIDKey).(int64), r.ID)
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(404, gin.H{"error": "notification not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"error": "operation failed"})
			return
		}
		c.JSON(200, res)
	})
}
