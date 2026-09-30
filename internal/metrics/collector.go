package metrics

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// RequestResult holds the outcome of a single HTTP request
type RequestResult struct {
	Timestamp  time.Time
	Duration   time.Duration
	StatusCode int
	Err        error
}

// Snapshot represents an aggregated metric window (e.g. 1 second or whole run)
type Snapshot struct {
	Timestamp       time.Time     `json:"timestamp"`
	Duration        time.Duration `json:"duration"`
	TotalRequests   int64         `json:"total_requests"`
	SuccessRequests int64         `json:"success_requests"`
	FailedRequests  int64         `json:"failed_requests"`
	CurrentRPS      float64       `json:"current_rps"`
	AverageRPS      float64       `json:"average_rps"`
	MinLatency      time.Duration `json:"min_latency"`
	MaxLatency      time.Duration `json:"max_latency"`
	AvgLatency      time.Duration `json:"avg_latency"`
	P50Latency      time.Duration `json:"p50_latency"`
	P90Latency      time.Duration `json:"p90_latency"`
	P95Latency      time.Duration `json:"p95_latency"`
	P99Latency      time.Duration `json:"p99_latency"`
	StatusCodes     map[int]int64 `json:"status_codes"`
	ErrorCount      int64         `json:"error_count"`
	Errors          map[string]int64 `json:"errors"`
	IsRunning       bool          `json:"is_running"`
	IsPaused        bool          `json:"is_paused"`
	IsFinished      bool          `json:"is_finished"`
	TargetURL       string        `json:"target_url"`
	TargetRPS       int           `json:"target_rps"`
	Concurrency     int           `json:"concurrency"`
}

// Collector tracks request results in real time
type Collector struct {
	mu           sync.RWMutex
	startTime    time.Time
	endTime      time.Time
	targetURL    string
	targetRPS    int
	concurrency  int

	// Running metrics (atomic counters)
	totalRequests atomic.Int64
	errorRequests atomic.Int64

	// Status code counts
	statusMu    sync.Mutex
	statusCodes map[int]int64
	errorsMap   map[string]int64

	// Sliding window for recent 1-second snapshots
	windowMu       sync.Mutex
	windowResults  []RequestResult
	allDurations   []time.Duration
	allDurationsMu sync.Mutex

	isRunning atomic.Bool
	isPaused  atomic.Bool
	isFinished atomic.Bool
}

func NewCollector(url string, concurrency, targetRPS int) *Collector {
	return &Collector{
		targetURL:     url,
		concurrency:   concurrency,
		targetRPS:     targetRPS,
		statusCodes:   make(map[int]int64),
		errorsMap:     make(map[string]int64),
		windowResults: make([]RequestResult, 0, 5000),
		allDurations:  make([]time.Duration, 0, 50000),
	}
}

func (c *Collector) Start() {
	c.mu.Lock()
	c.startTime = time.Now()
	c.mu.Unlock()
	c.isRunning.Store(true)
	c.isPaused.Store(false)
	c.isFinished.Store(false)
}

func (c *Collector) SetPaused(paused bool) {
	c.isPaused.Store(paused)
}

func (c *Collector) Finish() {
	c.mu.Lock()
	c.endTime = time.Now()
	c.mu.Unlock()
	c.isRunning.Store(false)
	c.isPaused.Store(false)
	c.isFinished.Store(true)
}

func (c *Collector) IsRunning() bool {
	return c.isRunning.Load()
}

func (c *Collector) IsPaused() bool {
	return c.isPaused.Load()
}

func (c *Collector) Record(res RequestResult) {
	c.totalRequests.Add(1)

	c.statusMu.Lock()
	if res.Err != nil {
		c.errorRequests.Add(1)
		errStr := res.Err.Error()
		c.errorsMap[errStr]++
	} else {
		c.statusCodes[res.StatusCode]++
	}
	c.statusMu.Unlock()

	c.windowMu.Lock()
	c.windowResults = append(c.windowResults, res)
	c.windowMu.Unlock()

	c.allDurationsMu.Lock()
	c.allDurations = append(c.allDurations, res.Duration)
	c.allDurationsMu.Unlock()
}

// WindowSnapshot drains the current 1-second window and calculates current metrics
func (c *Collector) WindowSnapshot() Snapshot {
	c.windowMu.Lock()
	results := c.windowResults
	c.windowResults = make([]RequestResult, 0, len(results)+100)
	c.windowMu.Unlock()

	now := time.Now()
	c.mu.RLock()
	start := c.startTime
	c.mu.RUnlock()

	totalReq := c.totalRequests.Load()
	errReq := c.errorRequests.Load()
	successReq := totalReq - errReq

	elapsed := now.Sub(start)
	if elapsed <= 0 {
		elapsed = time.Millisecond
	}

	windowCount := int64(len(results))
	currentRPS := float64(windowCount) // Since it's queried once per second
	avgRPS := float64(totalReq) / elapsed.Seconds()

	// Calculate latencies for recent window
	var minLat, maxLat, totalLat time.Duration
	durations := make([]time.Duration, 0, len(results))
	windowErrors := make(map[string]int64)
	windowStatusCodes := make(map[int]int64)

	for i, r := range results {
		durations = append(durations, r.Duration)
		totalLat += r.Duration
		if i == 0 || r.Duration < minLat {
			minLat = r.Duration
		}
		if r.Duration > maxLat {
			maxLat = r.Duration
		}
		if r.Err != nil {
			windowErrors[r.Err.Error()]++
		} else {
			windowStatusCodes[r.StatusCode]++
		}
	}

	var avgLat, p50, p90, p95, p99 time.Duration
	if len(durations) > 0 {
		avgLat = totalLat / time.Duration(len(durations))
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		p50 = percentile(durations, 50)
		p90 = percentile(durations, 90)
		p95 = percentile(durations, 95)
		p99 = percentile(durations, 99)
	}

	c.statusMu.Lock()
	statusCodesCopy := make(map[int]int64, len(c.statusCodes))
	for k, v := range c.statusCodes {
		statusCodesCopy[k] = v
	}
	errorsCopy := make(map[string]int64, len(c.errorsMap))
	for k, v := range c.errorsMap {
		errorsCopy[k] = v
	}
	c.statusMu.Unlock()

	return Snapshot{
		Timestamp:       now,
		Duration:        elapsed,
		TotalRequests:   totalReq,
		SuccessRequests: successReq,
		FailedRequests:  errReq,
		CurrentRPS:      currentRPS,
		AverageRPS:      avgRPS,
		MinLatency:      minLat,
		MaxLatency:      maxLat,
		AvgLatency:      avgLat,
		P50Latency:      p50,
		P90Latency:      p90,
		P95Latency:      p95,
		P99Latency:      p99,
		StatusCodes:     statusCodesCopy,
		ErrorCount:      errReq,
		Errors:          errorsCopy,
		IsRunning:       c.isRunning.Load(),
		IsPaused:        c.isPaused.Load(),
		IsFinished:      c.isFinished.Load(),
		TargetURL:       c.targetURL,
		TargetRPS:       c.targetRPS,
		Concurrency:     c.concurrency,
	}
}

// GlobalSnapshot calculates overall statistics across the entire execution
func (c *Collector) GlobalSnapshot() Snapshot {
	snap := c.WindowSnapshot()

	c.allDurationsMu.Lock()
	if len(c.allDurations) > 0 {
		allD := make([]time.Duration, len(c.allDurations))
		copy(allD, c.allDurations)
		c.allDurationsMu.Unlock()

		sort.Slice(allD, func(i, j int) bool { return allD[i] < allD[j] })
		var totalLat time.Duration
		for _, d := range allD {
			totalLat += d
		}
		snap.MinLatency = allD[0]
		snap.MaxLatency = allD[len(allD)-1]
		snap.AvgLatency = totalLat / time.Duration(len(allD))
		snap.P50Latency = percentile(allD, 50)
		snap.P90Latency = percentile(allD, 90)
		snap.P95Latency = percentile(allD, 95)
		snap.P99Latency = percentile(allD, 99)
	} else {
		c.allDurationsMu.Unlock()
	}

	return snap
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	idx := int(math.Ceil(float64(len(sorted))*(p/100.0))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
