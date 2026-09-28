package dbdump

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"capstone-be/config"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	cfg *config.Config
}

func NewHandler(cfg *config.Config) *Handler {
	return &Handler{cfg: cfg}
}

// delayedWriter delays sending HTTP response headers until the first write happens.
// If pg_dump fails before writing anything (e.g. invalid credentials or DB connection error),
// we can return a proper HTTP 500 JSON error rather than a broken 200 download.
type delayedWriter struct {
	w       gin.ResponseWriter
	started bool
	onStart func()
}

func (d *delayedWriter) Write(p []byte) (int, error) {
	if !d.started {
		d.onStart()
		d.started = true
	}
	return d.w.Write(p)
}

func findPgDump() (string, error) {
	if p, err := exec.LookPath("pg_dump"); err == nil {
		return p, nil
	}
	candidates := []string{
		"/opt/homebrew/bin/pg_dump",
		"/usr/local/bin/pg_dump",
		"/usr/bin/pg_dump",
		"/usr/lib/postgresql/16/bin/pg_dump",
		"/usr/lib/postgresql/15/bin/pg_dump",
		"/usr/lib/postgresql/14/bin/pg_dump",
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("pg_dump executable not found in PATH or standard locations")
}

// Dump executes pg_dump and streams the SQL output directly to the response writer.
func (h *Handler) Dump(c *gin.Context) {
	pgDumpPath, err := findPgDump()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database dump tool unavailable",
			"details": err.Error(),
		})
		return
	}

	// Prepare pg_dump arguments
	args := []string{
		"-h", h.cfg.DBHost,
		"-p", h.cfg.DBPort,
		"-U", h.cfg.DBUser,
		"-d", h.cfg.DBName,
		"--no-owner",
		"--no-acl",
	}

	// Optional query parameters
	if c.Query("schema_only") == "true" {
		args = append(args, "--schema-only")
	} else if c.Query("data_only") == "true" {
		args = append(args, "--data-only")
	}

	if c.Query("clean") == "true" {
		args = append(args, "--clean", "--if-exists")
	}

	if table := strings.TrimSpace(c.Query("table")); table != "" {
		args = append(args, "-t", table)
	}

	cmd := exec.CommandContext(c.Request.Context(), pgDumpPath, args...)

	env := os.Environ()
	env = append(env, fmt.Sprintf("PGPASSWORD=%s", h.cfg.DBPassword))
	if h.cfg.DBSSLMode != "" {
		env = append(env, fmt.Sprintf("PGSSLMODE=%s", h.cfg.DBSSLMode))
	}
	cmd.Env = env

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("backup_%s_%s.sql", h.cfg.DBName, timestamp)

	dw := &delayedWriter{
		w: c.Writer,
		onStart: func() {
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
			c.Header("Content-Type", "application/sql")
			c.Writer.WriteHeader(http.StatusOK)
		},
	}
	cmd.Stdout = dw

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if !dw.started {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "failed to execute database dump",
				"details": strings.TrimSpace(stderr.String()),
			})
			return
		}
		log.Printf("pg_dump encountered error during streaming: %v, stderr: %s", err, stderr.String())
		return
	}

	if !dw.started {
		// Even if pg_dump produced 0 bytes without error, set headers and return 200
		dw.onStart()
	}
}
