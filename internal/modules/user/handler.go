package user

import (
	"errors"
	"net/http"

	responsehandler "capstone-be/internal/utils"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	service UserService
}

func NewUserHandler(service UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) Detail(c *gin.Context) {
	var req UserDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	res, err := h.service.Detail(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			responsehandler.ToErrorHandler(c, http.StatusNotFound, err.Error(), nil)
			return
		}
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to retrieve user", err.Error())
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusOK, "User retrieved successfully", res)
}

func (h *UserHandler) List(c *gin.Context) {
	var req ListUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	res, err := h.service.List(c.Request.Context(), req)
	if err != nil {
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to retrieve users", err.Error())
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusOK, "Users retrieved successfully", res)
}

func (h *UserHandler) Update(c *gin.Context) {
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	res, err := h.service.Update(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			responsehandler.ToErrorHandler(c, http.StatusNotFound, err.Error(), nil)
			return
		}
		if errors.Is(err, ErrEmailAlreadyExists) {
			responsehandler.ToErrorHandler(c, http.StatusConflict, err.Error(), nil)
			return
		}
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to update user", err.Error())
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusOK, "User updated successfully", res)
}

func (h *UserHandler) Delete(c *gin.Context) {
	var req DeleteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	err := h.service.Delete(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			responsehandler.ToErrorHandler(c, http.StatusNotFound, err.Error(), nil)
			return
		}
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to delete user", err.Error())
		return
	}

	responsehandler.ToSuccessHandler[any](c, http.StatusOK, "User deleted successfully", nil)
}
