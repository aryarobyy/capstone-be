package history

import (
	"context"
	"errors"
	"net/http"
	"time"

	"capstone-be/internal/middleware"
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
		responsehandler.ToValidationError(c, err)
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
		responsehandler.ToValidationError(c, err)
		return
	}
	req.OwnerID = middleware.OwnerScope(c)

	response, err := h.service.List(c.Request.Context(), req)
	if err != nil {
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to retrieve history", err)
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusOK, "History retrieved successfully", response)
}

func (h *HistoryHandler) Newest(c *gin.Context) {
	var req NewestHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	req.OwnerID = middleware.OwnerScope(c)

	response, err := h.service.Newest(c.Request.Context(), req)
	if err != nil {
		handleHistoryError(c, err, "Failed to retrieve newest history")
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusOK, "Newest history retrieved successfully", response)
}

func (h *HistoryHandler) Detail(c *gin.Context) {
	var req DetailHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	req.OwnerID = middleware.OwnerScope(c)

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
		responsehandler.ToValidationError(c, err)
		return
	}
	req.OwnerID = middleware.OwnerScope(c)

	if err := h.service.Delete(c.Request.Context(), req); err != nil {
		handleHistoryError(c, err, "Failed to delete history")
		return
	}

	responsehandler.ToSuccessHandler[any](c, http.StatusOK, "History deleted successfully", nil)
}

const historyStreamPoll = 2 * time.Second

func (h *HistoryHandler) Stream(c *gin.Context) {
	var req StreamHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToValidationError(c, err)
		return
	}
	req.OwnerID = middleware.OwnerScope(c)

	after := req.AfterID
	if after <= 0 {
		latest, err := h.service.Since(c.Request.Context(), req, 0, 1)
		if err != nil {
			responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to open history stream", err)
			return
		}
		if len(latest) > 0 {
			after = latest[0].ID
		}
	}

	responsehandler.StreamSSE(c, "anomaly", historyStreamPoll, after,
		func(ctx context.Context, after int64) ([]HistoryResponse, error) {
			return h.service.Since(ctx, req, after, 20)
		},
		func(a HistoryResponse) int64 { return a.ID })
}

func handleHistoryError(c *gin.Context, err error, message string) {
	if errors.Is(err, ErrHistoryNotFound) {
		responsehandler.ToErrorHandler(c, http.StatusNotFound, "History record not found.", nil)
		return
	}
	if errors.Is(err, ErrInvalidAnomalyStatus) {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid anomaly status.", nil)
		return
	}
	responsehandler.ToErrorHandler(c, http.StatusInternalServerError, message, err)
}
