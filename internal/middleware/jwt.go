package middleware

import (
	"capstone-be/internal/session"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"strings"

	"capstone-be/internal/token"

	"github.com/gin-gonic/gin"
)

const SessionIDKey = "session_id"

const UserIDKey = "user_id"

func JWTAuth(tokens *token.Manager, sessions session.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		fields := strings.Fields(c.GetHeader("Authorization"))
		if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
			rejectJWT(c)
			return
		}
		id, sid, err := tokens.VerifySession(fields[1])
		if err != nil {
			rejectJWT(c)
			return
		}
		if _, err := uuid.Parse(sid); err != nil {
			rejectJWT(c)
			return
		}
		if err := sessions.Active(c.Request.Context(), sid, id); err != nil {
			if errors.Is(err, session.ErrInactive) {
				rejectJWT(c)
			} else {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "session verification unavailable"})
			}
			return
		}
		c.Set(SessionIDKey, sid)
		c.Set(UserIDKey, id)
		c.Next()
	}
}
func rejectJWT(c *gin.Context) {
	c.Header("WWW-Authenticate", "Bearer")
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "valid access token required"})
}
