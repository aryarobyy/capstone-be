package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	responsehandler "capstone-be/internal/utils"

	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					slog.Any("error", err),
					slog.String("stack", string(debug.Stack())),
				)

				responsehandler.ToErrorHandler(
					c,
					http.StatusInternalServerError,
					"Internal Server Error",
					errors.New("an unexpected error occurred on the server"),
				)
				c.Abort()
			}
		}()
		c.Next()
	}
}
