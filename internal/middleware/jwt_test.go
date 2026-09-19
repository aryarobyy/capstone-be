package middleware

import (
	"capstone-be/internal/session"
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"capstone-be/internal/token"
	"github.com/gin-gonic/gin"
)

func TestJWTAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens, _ := token.NewManager(strings.Repeat("a", 32), 1)
	access, _, _ := tokens.Issue(7, "00000000-0000-4000-8000-000000000001")
	router := gin.New()
	router.GET("/private", JWTAuth(tokens, &activeSession{}), func(c *gin.Context) {
		if c.MustGet(UserIDKey).(int64) != 7 {
			t.Error("wrong user ID")
		}
		c.Status(204)
	})
	for _, tc := range []struct {
		header string
		status int
	}{{"", 401}, {"Basic credentials", 401}, {"Bearer invalid", 401}, {"Bearer " + access, 204}} {
		req := httptest.NewRequest("GET", "/private", nil)
		req.Header.Set("Authorization", tc.header)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != tc.status {
			t.Errorf("got %d, want %d", response.Code, tc.status)
		}
	}
}

type activeSession struct {
	session.Store
	err error
}

func (s *activeSession) Active(context.Context, string, int64) error { return s.err }
func TestJWTSessionRevocation(t *testing.T) {
	tokens, _ := token.NewManager(strings.Repeat("a", 32), 1)
	access, _, _ := tokens.Issue(7, "00000000-0000-4000-8000-000000000001")
	for _, tc := range []struct {
		err    error
		status int
	}{{session.ErrInactive, 401}, {errors.New("db down"), 503}} {
		r := gin.New()
		r.GET("/", JWTAuth(tokens, &activeSession{err: tc.err}), func(c *gin.Context) { t.Error("inactive session reached handler") })
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("got %d want %d", w.Code, tc.status)
		}
	}
}
