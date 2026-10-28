package notification

import (
	"capstone-be/internal/middleware"
	responsehandler "capstone-be/internal/utils"
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes expects api to already have JWT/session authentication applied.
func RegisterRoutes(api *gin.RouterGroup, s Service) {
	user := func(c *gin.Context) int64 { return c.MustGet(middleware.UserIDKey).(int64) }
	api.POST("/notifications/list", func(c *gin.Context) {
		var r struct {
			BeforeID int64 `json:"before_id" binding:"gte=0"`
		}
		if err := c.ShouldBindJSON(&r); err != nil {
			responsehandler.ToValidationError(c, err)
			return
		}
		rows, err := s.List(c.Request.Context(), user(c), r.BeforeID)
		if err != nil {
			responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to retrieve notifications", err)
			return
		}
		responsehandler.ToSuccessHandler(c, http.StatusOK, "Notifications retrieved successfully", rows)
	})
	api.POST("/notifications/read", func(c *gin.Context) {
		var r struct {
			ID int64 `json:"id" binding:"required,gt=0"`
		}
		if err := c.ShouldBindJSON(&r); err != nil {
			responsehandler.ToValidationError(c, err)
			return
		}
		err := s.MarkRead(c.Request.Context(), user(c), r.ID)
		if errors.Is(err, sql.ErrNoRows) {
			responsehandler.ToErrorHandler(c, http.StatusNotFound, "Notification not found.", nil)
			return
		}
		if err != nil {
			responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to mark notification as read", err)
			return
		}
		responsehandler.ToSuccessHandler[any](c, http.StatusOK, "Notification marked as read", nil)
	})
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
		res, err := s.SendNotification(c.Request.Context(), user(c), p.Title, p.Body, p.Data)
		if errors.Is(err, ErrInvalidPayload) {
			responsehandler.ToErrorHandler(c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if err != nil {
			responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to send notification", err)
			return
		}
		responsehandler.ToSuccessHandler(c, http.StatusAccepted, "Notification queued", res)
	})
	api.POST("/notifications/status", func(c *gin.Context) {
		var r struct {
			ID int64 `json:"id" binding:"required,gt=0"`
		}
		if err := c.ShouldBindJSON(&r); err != nil {
			responsehandler.ToValidationError(c, err)
			return
		}
		res, err := s.Status(c.Request.Context(), user(c), r.ID)
		if errors.Is(err, sql.ErrNoRows) {
			responsehandler.ToErrorHandler(c, http.StatusNotFound, "Notification not found.", nil)
			return
		}
		if err != nil {
			responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to retrieve notification status", err)
			return
		}
		responsehandler.ToSuccessHandler(c, http.StatusOK, "Notification status retrieved successfully", res)
	})
}
