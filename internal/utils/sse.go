package responsehandler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// StreamSSE polls fetch every interval and sends each item as a Server-Sent Event
// named event, until the client leaves or fetch fails (the client reconnects).
// fetch receives the id of the last item sent, starting at after.
func StreamSSE[T any](c *gin.Context, event string, interval time.Duration, after int64, fetch func(ctx context.Context, after int64) ([]T, error), id func(T) int64) {
	ctx := c.Request.Context()
	rc := http.NewResponseController(c.Writer)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no") // nginx buffers proxied responses otherwise
	c.Status(http.StatusOK)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		items, err := fetch(ctx, after)
		if err != nil {
			return
		}
		_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second)) // server WriteTimeout would cut the stream
		for _, item := range items {
			c.SSEvent(event, item)
			after = id(item)
		}
		if len(items) == 0 {
			_, _ = c.Writer.WriteString(": ping\n\n") // keeps proxies from closing an idle stream
		}
		if rc.Flush() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
