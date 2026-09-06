package middleware

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type slogWriter struct {
	level slog.Level
}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(p))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			slog.Log(context.Background(), w.level, line)
		}
	}
	return len(p), nil
}

type jsonOrderedHandler struct {
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
	mu    *sync.Mutex
}

func newJSONOrderedHandler(w io.Writer, level slog.Level) *jsonOrderedHandler {
	return &jsonOrderedHandler{
		w:     w,
		level: level,
		mu:    &sync.Mutex{},
	}
}

func (h *jsonOrderedHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *jsonOrderedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &jsonOrderedHandler{
		w:     h.w,
		level: h.level,
		attrs: newAttrs,
		mu:    h.mu,
	}
}

func (h *jsonOrderedHandler) WithGroup(_ string) slog.Handler {
	return h
}

func appendAttr(buf *bytes.Buffer, a slog.Attr) {
	if a.Key == "" {
		return
	}
	keyBytes, _ := json.Marshal(a.Key)
	buf.WriteString(",")
	buf.Write(keyBytes)
	buf.WriteString(":")

	val := a.Value.Resolve()
	var valBytes []byte
	var err error
	if val.Kind() == slog.KindDuration {
		valBytes, err = json.Marshal(val.Duration().String())
	} else {
		valBytes, err = json.Marshal(val.Any())
	}
	if err != nil {
		valBytes, _ = json.Marshal(val.String())
	}
	buf.Write(valBytes)
}

func (h *jsonOrderedHandler) Handle(_ context.Context, r slog.Record) error {
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	buf.WriteString(`{"msg":`)
	msgBytes, err := json.Marshal(r.Message)
	if err != nil {
		msgBytes, _ = json.Marshal(r.Message)
	}
	buf.Write(msgBytes)

	buf.WriteString(`,"level":`)
	levelBytes, _ := json.Marshal(r.Level.String())
	buf.Write(levelBytes)

	for _, a := range h.attrs {
		appendAttr(buf, a)
	}

	r.Attrs(func(a slog.Attr) bool {
		appendAttr(buf, a)
		return true
	})

	if !r.Time.IsZero() {
		buf.WriteString(`,"time":`)
		timeBytes, _ := json.Marshal(r.Time.Format(time.RFC3339Nano))
		buf.Write(timeBytes)
	}

	buf.WriteString("}\n")

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err = h.w.Write(buf.Bytes())
	return err
}

func InitLogger() (func(), error) {
	if err := os.MkdirAll("log", 0755); err != nil {
		return nil, err
	}

	logFile, err := os.OpenFile("log/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)

	handler := newJSONOrderedHandler(multiWriter, slog.LevelDebug)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	gin.DefaultWriter = &slogWriter{level: slog.LevelDebug}
	gin.DefaultErrorWriter = &slogWriter{level: slog.LevelError}

	cleanup := func() {
		_ = logFile.Close()
	}

	return cleanup, nil
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		t := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(t)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		attrs := []slog.Attr{
			slog.Int("status", statusCode),
			slog.String("method", method),
			slog.String("path", path),
			slog.String("client_ip", clientIP),
			slog.Duration("latency", latency),
			slog.Float64("latency_ms", float64(latency.Microseconds())/1000.0),
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("error", c.Errors.String()))
		}

		switch {
		case statusCode >= 500:
			slog.LogAttrs(c.Request.Context(), slog.LevelError, "HTTP request", attrs...)
		case statusCode >= 400:
			slog.LogAttrs(c.Request.Context(), slog.LevelWarn, "HTTP request", attrs...)
		default:
			slog.LogAttrs(c.Request.Context(), slog.LevelInfo, "HTTP request", attrs...)
		}
	}
}
