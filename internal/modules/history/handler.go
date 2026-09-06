package history

import (
	"errors"
	"net/http"

	responsehandler "capstone-be/internal/utils"

	"github.com/gin-gonic/gin"
)

type HistoryHandler struct {
	service HistoryService
}

func NewHistoryHandler(service HistoryService) *HistoryHandler {
	return &HistoryHandler{service: service}
}

func (h *HistoryHandler) Create(c *gin.Context) {
	var req CreateHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	response, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		handleHistoryError(c, err, "Failed to create history")
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusCreated, "History created successfully", response)
}

func (h *HistoryHandler) List(c *gin.Context) {
	var req ListHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	response, err := h.service.List(c.Request.Context(), req)
	if err != nil {
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to retrieve history", err.Error())
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusOK, "History retrieved successfully", response)
}

func (h *HistoryHandler) Detail(c *gin.Context) {
	var req DetailHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	response, err := h.service.Detail(c.Request.Context(), req)
	if err != nil {
		handleHistoryError(c, err, "Failed to retrieve history detail")
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusOK, "History detail retrieved successfully", response)
}

func (h *HistoryHandler) Delete(c *gin.Context) {
	var req DeleteHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if err := h.service.Delete(c.Request.Context(), req); err != nil {
		handleHistoryError(c, err, "Failed to delete history")
		return
	}

	responsehandler.ToSuccessHandler[any](c, http.StatusOK, "History deleted successfully", nil)
}

func handleHistoryError(c *gin.Context, err error, message string) {
	if errors.Is(err, ErrHistoryNotFound) {
		responsehandler.ToErrorHandler(c, http.StatusNotFound, err.Error(), nil)
		return
	}
	if errors.Is(err, ErrInvalidAnomalyStatus) {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	responsehandler.ToErrorHandler(c, http.StatusInternalServerError, message, err.Error())
}
