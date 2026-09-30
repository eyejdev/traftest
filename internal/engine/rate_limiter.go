package engine

import (
	"context"
	"time"
)

// RateLimiter controls global requests per second
type RateLimiter struct {
	ticker *time.Ticker
	ch     chan struct{}
	rps    int
	ctx    context.Context
	cancel context.CancelFunc
}

func NewRateLimiter(rps int) *RateLimiter {
	if rps <= 0 {
		return nil // No rate limiting
	}

	ctx, cancel := context.WithCancel(context.Background())
	interval := time.Second / time.Duration(rps)
	if interval <= 0 {
		interval = time.Nanosecond
	}

	rl := &RateLimiter{
		ticker: time.NewTicker(interval),
		ch:     make(chan struct{}, rps),
		rps:    rps,
		ctx:    ctx,
		cancel: cancel,
	}

	go rl.run()
	return rl
}

func (rl *RateLimiter) run() {
	defer rl.ticker.Stop()
	for {
		select {
		case <-rl.ctx.Done():
			return
		case <-rl.ticker.C:
			select {
			case rl.ch <- struct{}{}:
			default:
				// Buffer full, skip to avoid blocking ticker
			}
		}
	}
}

func (rl *RateLimiter) Wait(ctx context.Context) bool {
	if rl == nil {
		return true
	}
	select {
	case <-ctx.Done():
		return false
	case <-rl.ch:
		return true
	}
}

func (rl *RateLimiter) Stop() {
	if rl != nil && rl.cancel != nil {
		rl.cancel()
	}
}
