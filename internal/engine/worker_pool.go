package engine

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eyejdev/traftest/internal/metrics"
)

// Config defines load test execution options
type Config struct {
	URL          string
	Method       string
	Headers      map[string]string
	Body         []byte
	Concurrency  int           // Number of parallel workers
	TotalRequests int64        // Target total requests (0 for duration-based)
	Duration     time.Duration // Target duration (0 for count-based)
	RPS          int           // Rate limit (0 for unlimited)
	Timeout      time.Duration // Per-request timeout
}

// WorkerPool manages the concurrent load test execution
type WorkerPool struct {
	cfg       Config
	client    *http.Client
	collector *metrics.Collector
	limiter   *RateLimiter

	ctx       context.Context
	cancel    context.CancelFunc
	pauseChan chan struct{}

	isPaused atomic.Bool
	isDone   atomic.Bool
	doneChan chan struct{}
	wg       sync.WaitGroup

	reqCounter atomic.Int64
}

func NewWorkerPool(cfg Config, col *metrics.Collector) *WorkerPool {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.Method == "" {
		cfg.Method = "GET"
	}

	return &WorkerPool{
		cfg:       cfg,
		client:    NewHTTPClient(cfg.Timeout, cfg.Concurrency*2),
		collector: col,
		limiter:   NewRateLimiter(cfg.RPS),
		pauseChan: make(chan struct{}),
		doneChan:  make(chan struct{}),
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	wp.ctx, wp.cancel = context.WithCancel(ctx)
	wp.collector.Start()

	if wp.cfg.Duration > 0 {
		time.AfterFunc(wp.cfg.Duration, func() {
			wp.Stop()
		})
	}

	for i := 0; i < wp.cfg.Concurrency; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}

	go func() {
		wp.wg.Wait()
		wp.Stop()
	}()
}

func (wp *WorkerPool) TogglePause() bool {
	if wp.isPaused.Load() {
		wp.isPaused.Store(false)
		wp.collector.SetPaused(false)
		return false
	} else {
		wp.isPaused.Store(true)
		wp.collector.SetPaused(true)
		return true
	}
}

func (wp *WorkerPool) Stop() {
	if wp.isDone.CompareAndSwap(false, true) {
		if wp.cancel != nil {
			wp.cancel()
		}
		if wp.limiter != nil {
			wp.limiter.Stop()
		}
		close(wp.doneChan)
		wp.collector.Finish()
	}
}

func (wp *WorkerPool) Done() <-chan struct{} {
	return wp.doneChan
}

func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
	wp.Stop()
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()

	for {
		if wp.ctx.Err() != nil {
			return
		}

		if wp.isPaused.Load() {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if wp.cfg.TotalRequests > 0 {
			curr := wp.reqCounter.Add(1)
			if curr > wp.cfg.TotalRequests {
				return
			}
		}

		if wp.limiter != nil {
			if !wp.limiter.Wait(wp.ctx) {
				return
			}
		}

		wp.executeRequest()
	}
}

func (wp *WorkerPool) executeRequest() {
	var bodyReader io.Reader
	if len(wp.cfg.Body) > 0 {
		bodyReader = bytes.NewReader(wp.cfg.Body)
	}

	req, err := http.NewRequestWithContext(wp.ctx, wp.cfg.Method, wp.cfg.URL, bodyReader)
	if err != nil {
		wp.collector.Record(metrics.RequestResult{
			Timestamp: time.Now(),
			Err:       err,
		})
		return
	}

	for k, v := range wp.cfg.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Goenma-TrafTest/1.0")
	}

	start := time.Now()
	resp, err := wp.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		// If test was stopped intentionally, ignore canceled in-flight requests
		if wp.ctx.Err() != nil {
			return
		}
		wp.collector.Record(metrics.RequestResult{
			Timestamp: start,
			Duration:  duration,
			Err:       err,
		})
		return
	}

	// Drain and close body for keep-alive reuse
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	wp.collector.Record(metrics.RequestResult{
		Timestamp:  start,
		Duration:   duration,
		StatusCode: resp.StatusCode,
	})
}
