package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestJSONOrderedHandler(t *testing.T) {
	buf := new(bytes.Buffer)
	handler := newJSONOrderedHandler(buf, slog.LevelDebug)
	logger := slog.New(handler)

	logger.Info("Database connection successfully established")

	output := strings.TrimSpace(buf.String())
	t.Logf("Output: %s", output)

	if !strings.HasPrefix(output, `{"msg":"Database connection successfully established"`) {
		t.Fatalf("Expected output to start with msg, got: %s", output)
	}

	if !strings.HasSuffix(output, `}`) {
		t.Fatalf("Expected valid json end, got: %s", output)
	}

	if !strings.Contains(output, `"time":`) {
		t.Fatalf("Expected time in output, got: %s", output)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	buf.Reset()
	logger.Info("HTTP request",
		slog.Int("status", 200),
		slog.String("method", "GET"),
		slog.String("path", "/api/health"),
		slog.Duration("latency", 1500*time.Microsecond),
	)

	outputWithAttrs := strings.TrimSpace(buf.String())
	t.Logf("Output with attrs: %s", outputWithAttrs)

	if !strings.HasPrefix(outputWithAttrs, `{"msg":"HTTP request"`) {
		t.Fatalf("Expected output to start with msg, got: %s", outputWithAttrs)
	}

	lastCommaIdx := strings.LastIndex(outputWithAttrs, ",")
	if lastCommaIdx == -1 {
		t.Fatalf("Expected commas in JSON: %s", outputWithAttrs)
	}
	lastPart := outputWithAttrs[lastCommaIdx+1:]
	if !strings.HasPrefix(lastPart, `"time":`) {
		t.Fatalf("Expected last key to be 'time', got: %s", lastPart)
	}

	var parsedAttrs map[string]interface{}
	if err := json.Unmarshal([]byte(outputWithAttrs), &parsedAttrs); err != nil {
		t.Fatalf("Output with attrs is not valid JSON: %v", err)
	}

	if parsedAttrs["msg"] != "HTTP request" {
		t.Fatalf("Expected msg 'HTTP request', got %v", parsedAttrs["msg"])
	}
	if parsedAttrs["status"] != float64(200) {
		t.Fatalf("Expected status 200, got %v", parsedAttrs["status"])
	}
	if parsedAttrs["latency"] != "1.5ms" {
		t.Fatalf("Expected latency '1.5ms', got %v", parsedAttrs["latency"])
	}
}
