package health

import (
	"database/sql"
	"net/http"

	responsehandler "capstone-be/internal/utils"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	db *sql.DB
}

func NewHealthHandler(db *sql.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Check(c *gin.Context) {
	status := "UP"
	dbStatus := "UP"

	if h.db != nil {
		if err := h.db.Ping(); err != nil {
			dbStatus = "DOWN"
			status = "DEGRADED"
		}
	} else {
		dbStatus = "NOT_CONFIGURED"
	}

	data := gin.H{
		"status":   status,
		"database": dbStatus,
	}

	statusCode := http.StatusOK
	if status == "DEGRADED" {
		statusCode = http.StatusServiceUnavailable
		responsehandler.JSONError(c, statusCode, "System is degraded", data)
		return
	}

	responsehandler.JSONSuccess(c, statusCode, "System is healthy", data)
}
