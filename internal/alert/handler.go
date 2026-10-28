package alert

import (
	"capstone-be/internal/middleware"
	responsehandler "capstone-be/internal/utils"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type Handler struct{ s *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }
func user(c *gin.Context) int64      { return c.MustGet(middleware.UserIDKey).(int64) }
func fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		responsehandler.ToErrorHandler(c, http.StatusForbidden, err.Error(), nil)
	case errors.Is(err, ErrInvalidRule):
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, err.Error(), nil)
	case errors.Is(err, ErrAreaNotFound):
		responsehandler.ToErrorHandler(c, http.StatusNotFound, "area not found", nil)
	default:
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "operation failed", err)
	}
}
func (h *Handler) SaveRule(c *gin.Context) {
	var r Rule
	if err := c.ShouldBindJSON(&r); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	if err := h.s.SaveRule(c.Request.Context(), user(c), r); err != nil {
		fail(c, err)
		return
	}
	c.Status(204)
}
func (h *Handler) Rules(c *gin.Context) {
	var req struct {
		SensorID int64 `json:"sensor_id"`
	}
	var id int64
	if err := c.ShouldBindJSON(&req); err == nil && req.SensorID > 0 {
		id = req.SensorID
	} else if qID, err := strconv.ParseInt(c.Query("sensor_id"), 10, 64); err == nil && qID > 0 {
		id = qID
	} else {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "sensor_id is required", nil)
		return
	}
	rows, err := h.s.Rules(c.Request.Context(), user(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	responsehandler.ToSuccessHandler(c, http.StatusOK, "Alert rules retrieved successfully", rows)
}
func (h *Handler) SensorKey(c *gin.Context) {
	var r struct {
		SensorID int64 `json:"sensor_id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&r); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	key, err := h.s.SensorKey(c.Request.Context(), user(c), r.SensorID)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	responsehandler.ToSuccessHandler(c, http.StatusOK, "Sensor key generated", gin.H{"sensor_key": key})
}
func (h *Handler) CreateArea(c *gin.Context) {
	var r struct {
		Name string `json:"name" binding:"required,min=1,max=255"`
	}
	if err := c.ShouldBindJSON(&r); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	id, err := h.s.CreateArea(c.Request.Context(), user(c), r.Name)
	if err != nil {
		fail(c, err)
		return
	}
	responsehandler.ToSuccessHandler(c, http.StatusCreated, "Area created successfully", Area{ID: id, Name: r.Name})
}
func (h *Handler) Areas(c *gin.Context) {
	rows, err := h.s.Areas(c.Request.Context(), user(c))
	if err != nil {
		fail(c, err)
		return
	}
	responsehandler.ToSuccessHandler(c, http.StatusOK, "Areas retrieved successfully", rows)
}
func (h *Handler) AreaDetail(c *gin.Context) {
	var r struct {
		ID int64 `json:"id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&r); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	area, err := h.s.AreaDetail(c.Request.Context(), user(c), r.ID)
	if err != nil {
		fail(c, err)
		return
	}
	responsehandler.ToSuccessHandler(c, http.StatusOK, "Area detail retrieved successfully", area)
}
func (h *Handler) UpdateArea(c *gin.Context) {
	var r struct {
		ID   int64  `json:"id" binding:"required,gt=0"`
		Name string `json:"name" binding:"required,min=1,max=255"`
	}
	if err := c.ShouldBindJSON(&r); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	if err := h.s.UpdateArea(c.Request.Context(), user(c), r.ID, r.Name); err != nil {
		fail(c, err)
		return
	}
	responsehandler.ToSuccessHandler(c, http.StatusOK, "Area updated successfully", Area{ID: r.ID, Name: r.Name})
}
func (h *Handler) DeleteArea(c *gin.Context) {
	var r struct {
		ID int64 `json:"id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&r); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	if err := h.s.DeleteArea(c.Request.Context(), user(c), r.ID); err != nil {
		fail(c, err)
		return
	}
	responsehandler.ToSuccessHandler[any](c, http.StatusOK, "Area deleted successfully", nil)
}
func (h *Handler) AssignArea(c *gin.Context) {
	var r struct {
		ID    int64 `json:"id" binding:"required,gt=0"`
		Owner int64 `json:"owner_id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&r); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	if err := h.s.AssignArea(c.Request.Context(), r.ID, r.Owner); err != nil {
		fail(c, err)
		return
	}
	c.Status(204)
}
