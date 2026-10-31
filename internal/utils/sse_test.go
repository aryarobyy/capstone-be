package responsehandler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamSSE_PollsUntilFetchFails(t *testing.T) {
	c, w := ctx()
	c.Request = httptestRequest()

	var afters []int64
	batches := [][]int64{{1, 2}, {}, {3}}
	StreamSSE(c, "tick", time.Millisecond, 0,
		func(_ context.Context, after int64) ([]int64, error) {
			afters = append(afters, after)
			if len(afters) > len(batches) {
				return nil, errors.New("db down") // ends the stream; the client reconnects
			}
			return batches[len(afters)-1], nil
		},
		func(v int64) int64 { return v })

	body := w.Body.String()
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") || w.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("headers = %v", w.Header())
	}
	if strings.Count(body, "event:tick") != 3 || !strings.Contains(body, ": ping\n\n") {
		t.Fatalf("body = %q", body)
	}
	// Each poll resumes after the last id sent; an empty batch keeps it.
	if want := []int64{0, 2, 2, 3}; len(afters) != len(want) || afters[1] != 2 || afters[2] != 2 || afters[3] != 3 {
		t.Fatalf("afters = %v, want %v", afters, want)
	}
}

func TestStreamSSE_StopsWhenClientLeaves(t *testing.T) {
	c, _ := ctx()
	cctx, cancel := context.WithCancel(context.Background())
	c.Request = httptestRequest().WithContext(cctx)

	calls := 0
	StreamSSE(c, "tick", time.Hour, 0,
		func(context.Context, int64) ([]int64, error) {
			calls++
			cancel()
			return nil, nil
		},
		func(v int64) int64 { return v })

	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (must not wait for the next tick)", calls)
	}
}

func httptestRequest() *http.Request {
	return httptest.NewRequest(http.MethodPost, "/stream", nil)
}
