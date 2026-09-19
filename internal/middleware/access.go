package middleware

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func ResourceAccess(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.MustGet(UserIDKey).(int64)
		var admin bool
		if err := db.QueryRowContext(c.Request.Context(), `SELECT is_admin FROM users WHERE id=$1`, user).Scan(&admin); err != nil {
			c.AbortWithStatusJSON(503, gin.H{"error": "authorization unavailable"})
			return
		}
		c.Set("is_admin", admin)
		if admin {
			c.Next()
			return
		}
		path := strings.TrimPrefix(c.Request.URL.Path, "/api/")
		parts := strings.Split(path, "/")
		if len(parts) != 2 {
			c.AbortWithStatus(403)
			return
		}
		if parts[1] == "list" {
			c.Next()
			return
		}
		var req struct {
			ID       int64  `json:"id"`
			SensorID int64  `json:"sensor_id"`
			AreaID   *int64 `json:"area_id"`
		}
		body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 16384))
		if err != nil || json.Unmarshal(body, &req) != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": "invalid request"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		allowed := false
		switch parts[0] {
		case "user":
			allowed = req.ID == user
		case "sensor", "sensor-reading", "history":
			allowed = true
		}
		if err != nil {
			c.AbortWithStatusJSON(503, gin.H{"error": "authorization unavailable"})
			return
		}
		if !allowed {
			c.AbortWithStatusJSON(403, gin.H{"error": "resource access denied"})
			return
		}
		c.Next()
	}
}
func ownsArea(c *gin.Context, db *sql.DB, user, id int64) (bool, error) {
	var ok bool
	err := db.QueryRowContext(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM areas WHERE id=$1)`, id).Scan(&ok)
	return ok, err
}
func ownsSensor(c *gin.Context, db *sql.DB, user, id int64) (bool, error) {
	var ok bool
	err := db.QueryRowContext(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM sensors WHERE id=$1)`, id).Scan(&ok)
	return ok, err
}

func AdminOnly(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var ok bool
		err := db.QueryRowContext(c.Request.Context(), `SELECT is_admin FROM users WHERE id=$1`, c.MustGet(UserIDKey)).Scan(&ok)
		if err != nil {
			c.AbortWithStatus(503)
			return
		}
		if !ok {
			c.AbortWithStatus(403)
			return
		}
		c.Next()
	}
}

// SensorKey authenticates machine ingestion without granting account privileges.
func SensorKey(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-Sensor-Key")
		if len(key) < 32 || len(key) > 256 {
			c.AbortWithStatus(401)
			return
		}
		hash := sha256.Sum256([]byte(key))
		var sensor int64
		err := db.QueryRowContext(c.Request.Context(), `SELECT id FROM sensors WHERE api_key_hash=$1`, hash[:]).Scan(&sensor)
		if err != nil {
			if err == sql.ErrNoRows {
				c.AbortWithStatus(401)
			} else {
				c.AbortWithStatus(503)
			}
			return
		}
		var req struct {
			SensorID int64 `json:"sensor_id"`
		}
		body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 16384))
		if err != nil || json.Unmarshal(body, &req) != nil {
			c.AbortWithStatus(400)
			return
		}
		if req.SensorID != sensor {
			c.AbortWithStatus(403)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}
