package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"sync"
	"time"
)

// RateLimit bounds both request rates and memory. Use a shared limiter at the proxy for multiple replicas.
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	type bucket struct {
		count int
		until time.Time
	}
	entries := map[string]bucket{}
	var mu sync.Mutex
	return func(c *gin.Context) {
		key := c.ClientIP()
		now := time.Now()
		mu.Lock()
		b := entries[key]
		if !now.Before(b.until) {
			b = bucket{until: now.Add(window)}
		}
		if len(entries) >= 10000 {
			for k, v := range entries {
				if !now.Before(v.until) {
					delete(entries, k)
				}
			}
		}
		if b.count >= limit || len(entries) >= 10000 {
			mu.Unlock()
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(429, gin.H{"error": "too many requests"})
			return
		}
		b.count++
		entries[key] = b
		mu.Unlock()
		c.Next()
	}
}
func BodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) { c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536); c.Next() }
}
