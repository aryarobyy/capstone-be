package sensorreading

import (
	"errors"
	"net/http"

	responsehandler "capstone-be/internal/utils"

	"github.com/gin-gonic/gin"
)

type SensorReadingHandler struct {
	service SensorReadingService
}

func NewSensorReadingHandler(service SensorReadingService) *SensorReadingHandler {
	return &SensorReadingHandler{service: service}
}

func (h *SensorReadingHandler) Create(c *gin.Context) {
	var req CreateSensorReadingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if err := h.service.Create(c.Request.Context(), req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to create sensor reading", err.Error())
		return
	}

	responsehandler.ToSuccessHandler[any](c, http.StatusCreated, "Sensor reading created successfully", nil)
}

func (h *SensorReadingHandler) List(c *gin.Context) {
	var req ListSensorReadingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	res, err := h.service.List(c.Request.Context(), req)
	if err != nil {
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to retrieve sensor readings", err.Error())
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusOK, "Sensor readings retrieved successfully", res)
}

func (h *SensorReadingHandler) Detail(c *gin.Context) {
	var req DetailSensorReadingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	res, err := h.service.Detail(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrSensorReadingNotFound) {
			responsehandler.ToErrorHandler(c, http.StatusNotFound, err.Error(), nil)
			return
		}
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to retrieve sensor reading detail", err.Error())
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusOK, "Sensor reading detail retrieved successfully", res)
}

func (h *SensorReadingHandler) Delete(c *gin.Context) {
	var req DeleteSensorReadingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	err := h.service.Delete(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrSensorReadingNotFound) {
			responsehandler.ToErrorHandler(c, http.StatusNotFound, err.Error(), nil)
			return
		}
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to delete sensor reading", err.Error())
		return
	}

	responsehandler.ToSuccessHandler[any](c, http.StatusOK, "Sensor reading deleted successfully", nil)
}
