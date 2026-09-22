package sensorreading

import (
	"errors"
	"net/http"
	"time"

	"capstone-be/internal/middleware"
	responsehandler "capstone-be/internal/utils"

	"github.com/gin-gonic/gin"
)

type SensorReadingHandler struct {
	service SensorReadingService
}

func NewSensorReadingHandler(service SensorReadingService) *SensorReadingHandler {
	return &SensorReadingHandler{service: service}
}

func getUserID(c *gin.Context) int64 {
	if uid, ok := middleware.GetUserIDFromContext(c.Request.Context()); ok {
		return uid
	}
	if val, exists := c.Get(middleware.UserIDKey); exists {
		if uid, ok := val.(int64); ok {
			return uid
		}
	}
	return 0
}

func (h *SensorReadingHandler) Create(c *gin.Context) {
	var req CreateSensorReadingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}

	if req.SensorID <= 0 || req.RecordedAt.After(time.Now().Add(5*time.Minute)) {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid sensor ID or future recorded timestamp.", nil)
		return
	}
	if err := h.service.Create(c.Request.Context(), req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to create sensor reading", err)
		return
	}

	responsehandler.ToSuccessHandler[any](c, http.StatusCreated, "Sensor reading created successfully", nil)
}

func (h *SensorReadingHandler) List(c *gin.Context) {
	var req ListSensorReadingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}

	res, err := h.service.List(c.Request.Context(), req)
	if err != nil {
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to retrieve sensor readings", err)
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusOK, "Sensor readings retrieved successfully", res)
}

func (h *SensorReadingHandler) Detail(c *gin.Context) {
	var req DetailSensorReadingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}

	res, err := h.service.Detail(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrSensorReadingNotFound) {
			responsehandler.ToErrorHandler(c, http.StatusNotFound, "Sensor reading not found.", nil)
			return
		}
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to retrieve sensor reading detail", err)
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusOK, "Sensor reading detail retrieved successfully", res)
}

func (h *SensorReadingHandler) Delete(c *gin.Context) {
	var req DeleteSensorReadingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}

	err := h.service.Delete(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrSensorReadingNotFound) {
			responsehandler.ToErrorHandler(c, http.StatusNotFound, "Sensor reading not found.", nil)
			return
		}
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to delete sensor reading", err)
		return
	}

	responsehandler.ToSuccessHandler[any](c, http.StatusOK, "Sensor reading deleted successfully", nil)
}

func (h *SensorReadingHandler) Summary(c *gin.Context) {
	var req SensorSummaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = SensorSummaryRequest{WindowMinutes: 10}
	}

	ownerID := getUserID(c)
	res, err := h.service.Summary(c.Request.Context(), req, ownerID)
	if err != nil {
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to retrieve sensor reading summary", err)
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusOK, "Sensor reading summary retrieved successfully", res)
}

