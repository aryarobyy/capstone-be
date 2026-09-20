package sensor

import (
	"errors"
	"net/http"

	"capstone-be/internal/middleware"
	responsehandler "capstone-be/internal/utils"

	"github.com/gin-gonic/gin"
)

type SensorHandler struct {
	service SensorService
}

func NewSensorHandler(service SensorService) *SensorHandler {
	return &SensorHandler{service: service}
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

func (h *SensorHandler) Create(c *gin.Context) {
	var req CreateSensorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	req.OwnerID = getUserID(c)

	model, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		handleSensorError(c, err, "Failed to create sensor")
		return
	}
	responsehandler.ToSuccessHandler(c, http.StatusCreated, "Sensor created successfully", model)
}

func (h *SensorHandler) List(c *gin.Context) {
	var req ListSensorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	req.OwnerID = getUserID(c)

	model, err := h.service.List(c.Request.Context(), req)
	if err != nil {
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to retrieve sensors", err)
		return
	}
	responsehandler.ToSuccessHandler(c, http.StatusOK, "Sensors retrieved successfully", model)
}

func (h *SensorHandler) Detail(c *gin.Context) {
	var req DetailSensorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	req.OwnerID = getUserID(c)

	model, err := h.service.Detail(c.Request.Context(), req)
	if err != nil {
		handleSensorError(c, err, "Failed to retrieve sensor")
		return
	}
	responsehandler.ToSuccessHandler(c, http.StatusOK, "Sensor retrieved successfully", model)
}

func (h *SensorHandler) Update(c *gin.Context) {
	var req UpdateSensorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	req.OwnerID = getUserID(c)

	model, err := h.service.Update(c.Request.Context(), req)
	if err != nil {
		handleSensorError(c, err, "Failed to update sensor")
		return
	}
	responsehandler.ToSuccessHandler(c, http.StatusOK, "Sensor updated successfully", model)
}

func (h *SensorHandler) Delete(c *gin.Context) {
	var req DeleteSensorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	req.OwnerID = getUserID(c)

	if err := h.service.Delete(c.Request.Context(), req); err != nil {
		handleSensorError(c, err, "Failed to delete sensor")
		return
	}
	responsehandler.ToSuccessHandler[any](c, http.StatusOK, "Sensor deleted successfully", nil)
}

func handleSensorError(c *gin.Context, err error, message string) {
	if errors.Is(err, ErrSensorNotFound) {
		responsehandler.ToErrorHandler(c, http.StatusNotFound, "Sensor not found.", nil)
		return
	}
	if errors.Is(err, ErrAreaNotFound) {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "The specified area does not exist.", nil)
		return
	}
	responsehandler.ToErrorHandler(c, http.StatusInternalServerError, message, err)
}
