package auth

import (
	"errors"
	"net/http"

	responsehandler "capstone-be/internal/utils"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service AuthService
}

func NewAuthHandler(service AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.JSONError(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	res, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) {
			responsehandler.JSONError(c, http.StatusConflict, err.Error(), nil)
			return
		}
		responsehandler.JSONError(c, http.StatusInternalServerError, "Failed to register user", err.Error())
		return
	}

	responsehandler.JSONSuccess(c, http.StatusCreated, "User registered successfully", res)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.JSONError(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	res, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			responsehandler.JSONError(c, http.StatusUnauthorized, err.Error(), nil)
			return
		}
		responsehandler.JSONError(c, http.StatusInternalServerError, "Failed to login", err.Error())
		return
	}

	responsehandler.JSONSuccess(c, http.StatusOK, "Login successful", res)
}
