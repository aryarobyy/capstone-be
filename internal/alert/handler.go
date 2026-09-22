package alert

import (
	"capstone-be/internal/middleware"
	"errors"
	"github.com/gin-gonic/gin"
	"strconv"
)

type Handler struct{ s *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }
func user(c *gin.Context) int64      { return c.MustGet(middleware.UserIDKey).(int64) }
func fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		c.JSON(403, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidRule):
		c.JSON(400, gin.H{"error": err.Error()})
	default:
		c.JSON(500, gin.H{"error": "operation failed"})
	}
}
func (h *Handler) SaveRule(c *gin.Context) {
	var r Rule
	if c.ShouldBindJSON(&r) != nil {
		c.Status(400)
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
		c.Status(400)
		return
	}
	rows, err := h.s.Rules(c.Request.Context(), user(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, rows)
}
func (h *Handler) SensorKey(c *gin.Context) {
	var r struct {
		SensorID int64 `json:"sensor_id" binding:"required,gt=0"`
	}
	if c.ShouldBindJSON(&r) != nil {
		c.Status(400)
		return
	}
	key, err := h.s.SensorKey(c.Request.Context(), user(c), r.SensorID)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"sensor_key": key})
}
func (h *Handler) CreateArea(c *gin.Context) {
	var r struct {
		Name string `json:"name" binding:"required,min=1,max=255"`
	}
	if c.ShouldBindJSON(&r) != nil {
		c.Status(400)
		return
	}
	id, err := h.s.CreateArea(c.Request.Context(), user(c), r.Name)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(201, gin.H{"id": id})
}
func (h *Handler) Areas(c *gin.Context) {
	rows, err := h.s.Areas(c.Request.Context(), user(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, rows)
}
func (h *Handler) AssignArea(c *gin.Context) {
	var r struct {
		ID    int64 `json:"id" binding:"required,gt=0"`
		Owner int64 `json:"owner_id" binding:"required,gt=0"`
	}
	if c.ShouldBindJSON(&r) != nil {
		c.Status(400)
		return
	}
	if err := h.s.AssignArea(c.Request.Context(), r.ID, r.Owner); err != nil {
		fail(c, err)
		return
	}
	c.Status(204)
}
