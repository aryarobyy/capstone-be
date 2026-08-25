package user

import (
	"errors"
	"net/http"
	"strconv"

	responsehandler "capstone-be/internal/utils"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	service UserService
}

func NewUserHandler(service UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		responsehandler.JSONError(c, http.StatusBadRequest, "Invalid user ID format", nil)
		return
	}

	res, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			responsehandler.JSONError(c, http.StatusNotFound, err.Error(), nil)
			return
		}
		responsehandler.JSONError(c, http.StatusInternalServerError, "Failed to retrieve user", err.Error())
		return
	}

	responsehandler.JSONSuccess(c, http.StatusOK, "User retrieved successfully", res)
}

func (h *UserHandler) GetAll(c *gin.Context) {
	res, err := h.service.GetAll(c.Request.Context())
	if err != nil {
		responsehandler.JSONError(c, http.StatusInternalServerError, "Failed to retrieve users", err.Error())
		return
	}

	responsehandler.JSONSuccess(c, http.StatusOK, "Users retrieved successfully", res)
}

func (h *UserHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		responsehandler.JSONError(c, http.StatusBadRequest, "Invalid user ID format", nil)
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.JSONError(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	res, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			responsehandler.JSONError(c, http.StatusNotFound, err.Error(), nil)
			return
		}
		if errors.Is(err, ErrEmailAlreadyExists) {
			responsehandler.JSONError(c, http.StatusConflict, err.Error(), nil)
			return
		}
		responsehandler.JSONError(c, http.StatusInternalServerError, "Failed to update user", err.Error())
		return
	}

	responsehandler.JSONSuccess(c, http.StatusOK, "User updated successfully", res)
}

func (h *UserHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		responsehandler.JSONError(c, http.StatusBadRequest, "Invalid user ID format", nil)
		return
	}

	err = h.service.Delete(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			responsehandler.JSONError(c, http.StatusNotFound, err.Error(), nil)
			return
		}
		responsehandler.JSONError(c, http.StatusInternalServerError, "Failed to delete user", err.Error())
		return
	}

	responsehandler.JSONSuccess(c, http.StatusOK, "User deleted successfully", nil)
}
