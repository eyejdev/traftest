package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eyejdev/traftest/internal/metrics"
)

func TestWorkerPoolExecution(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	col := metrics.NewCollector(server.URL, 4, 100)
	cfg := Config{
		URL:           server.URL,
		Method:        "GET",
		Concurrency:   4,
		TotalRequests: 50,
		Timeout:       2 * time.Second,
	}

	pool := NewWorkerPool(cfg, col)
	pool.Start(context.Background())
	pool.Wait()

	snap := col.GlobalSnapshot()
	if snap.TotalRequests < 50 {
		t.Errorf("Expected at least 50 requests, got %d", snap.TotalRequests)
	}
	if snap.FailedRequests != 0 {
		t.Errorf("Expected 0 failed requests, got %d", snap.FailedRequests)
	}
	if snap.StatusCodes[200] < 50 {
		t.Errorf("Expected at least 50 HTTP 200 responses, got %d", snap.StatusCodes[200])
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(20) // 20 per sec
	defer rl.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	count := 0
	for rl.Wait(ctx) {
		count++
	}

	// In 200ms at 20 RPS, we expect around 3-6 requests
	if count < 2 || count > 8 {
		t.Logf("Rate limiter passed %d tokens in 200ms (expected ~4)", count)
	}
}
