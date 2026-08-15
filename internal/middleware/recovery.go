package middleware

import (
	"errors"
	"log"
	"net/http"
	"runtime/debug"

	responsehandler "capstone-be/internal/utils"

	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[PANIC RECOVERY] err: %v\nstack:\n%s", err, string(debug.Stack()))

				responsehandler.JSONError(
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
