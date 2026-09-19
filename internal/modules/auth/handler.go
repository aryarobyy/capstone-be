package auth

import (
	"capstone-be/internal/middleware"
	"capstone-be/internal/session"
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
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	res, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) {
			responsehandler.ToErrorHandler(c, http.StatusConflict, err.Error(), nil)
			return
		}
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to register user", err.Error())
		return
	}

	responsehandler.ToSuccessHandler(c, http.StatusCreated, "User registered successfully", res)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	res, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidDevice) {
			responsehandler.ToErrorHandler(c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if errors.Is(err, ErrInvalidCredentials) {
			responsehandler.ToErrorHandler(c, http.StatusUnauthorized, err.Error(), nil)
			return
		}
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to login", nil)
		return
	}

	c.Header("Cache-Control", "no-store")
	responsehandler.ToSuccessHandler(c, http.StatusOK, "Login successful", res)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if err := h.service.Logout(c.Request.Context(), c.GetString(middleware.SessionIDKey), c.MustGet(middleware.UserIDKey).(int64)); err != nil {
		responsehandler.ToErrorHandler(c, http.StatusInternalServerError, "Failed to logout", nil)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"error": "valid refresh_token required"})
		return
	}
	res, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, session.ErrInactive) {
			c.JSON(401, gin.H{"error": "refresh token expired, revoked, or already used"})
		} else {
			c.JSON(503, gin.H{"error": "refresh unavailable"})
		}
		return
	}
	c.Header("Cache-Control", "no-store")
	responsehandler.ToSuccessHandler(c, 200, "Token refreshed", res)
}
func (h *AuthHandler) LogoutRefresh(c *gin.Context) {
	var req RefreshRequest
	if c.ShouldBindJSON(&req) != nil {
		c.Status(400)
		return
	}
	if err := h.service.LogoutRefresh(c.Request.Context(), req.RefreshToken); err != nil {
		c.Status(503)
		return
	}
	c.Status(204)
}
