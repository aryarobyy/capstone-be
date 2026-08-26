package sensor

import (
	"errors"
	"net/http"

	responsehandler "capstone-be/internal/utils"

	"github.com/gin-gonic/gin"
)

type SensorHandler struct {
	service SensorService
}

func NewSensorHandler(service SensorService) *SensorHandler {
	return &SensorHandler{service: service}
}

func (h *SensorHandler) Create(c *gin.Context) {
	var req CreateSensorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	model, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to create sensor", err.Error())
		return
	}
	responsehandler.ToSuccessHandler(c, http.StatusCreated, "Sensor created successfully", model)
}

func (h *SensorHandler) List(c *gin.Context) {
	var req ListSensorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	model, err := h.service.List(c.Request.Context(), req)
	if err != nil {
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to retrieve sensors", err.Error())
		return
	}
	responsehandler.ToSuccessHandler(c, http.StatusOK, "Sensors retrieved successfully", model)
}

func (h *SensorHandler) Detail(c *gin.Context) {
	var req DetailSensorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

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
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

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
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if err := h.service.Delete(c.Request.Context(), req); err != nil {
		handleSensorError(c, err, "Failed to delete sensor")
		return
	}
	responsehandler.ToSuccessHandler[any](c, http.StatusOK, "Sensor deleted successfully", nil)
}

func handleSensorError(c *gin.Context, err error, message string) {
	if errors.Is(err, ErrSensorNotFound) {
		responsehandler.ToErrorHandler(c, http.StatusNotFound, err.Error(), nil)
		return
	}
	responsehandler.ToErrorHandler(c, http.StatusInternalServerError, message, err.Error())
}
