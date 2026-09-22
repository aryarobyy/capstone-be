package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"capstone-be/internal/session"
	"capstone-be/internal/token"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	UserIDKey    = "user_id"
	SessionIDKey = "session_id"
	TokenKey     = "token"
)

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
		c.Set(TokenKey, fields[1])

		ctx := context.WithValue(c.Request.Context(), UserIDKey, id)
		ctx = context.WithValue(ctx, TokenKey, fields[1])
		ctx = context.WithValue(ctx, SessionIDKey, sid)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func rejectJWT(c *gin.Context) {
	c.Header("WWW-Authenticate", "Bearer")
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "valid access token required"})
}

func GetUserIDFromContext(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}
	if val, ok := ctx.Value(UserIDKey).(int64); ok {
		return val, true
	}
	return 0, false
}

func GetTokenFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	if val, ok := ctx.Value(TokenKey).(string); ok {
		return val, true
	}
	return "", false
}

func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}
