package metrics

import (
	"fmt"
	"testing"
	"time"
)

func TestPercentileCalculation(t *testing.T) {
	durations := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	p50 := percentile(durations, 50)
	if p50 != 50*time.Millisecond {
		t.Errorf("Expected p50 to be 50ms, got %v", p50)
	}

	p90 := percentile(durations, 90)
	if p90 != 90*time.Millisecond {
		t.Errorf("Expected p90 to be 90ms, got %v", p90)
	}

	p99 := percentile(durations, 99)
	if p99 != 100*time.Millisecond {
		t.Errorf("Expected p99 to be 100ms, got %v", p99)
	}
}

func TestCollectorRecordAndSnapshots(t *testing.T) {
	c := NewCollector("http://example.local", 5, 50)
	c.Start()

	// Record successes
	for i := 0; i < 10; i++ {
		c.Record(RequestResult{
			Timestamp:  time.Now(),
			Duration:   time.Duration(10+i) * time.Millisecond,
			StatusCode: 200,
		})
	}

	// Record an error
	c.Record(RequestResult{
		Timestamp: time.Now(),
		Duration:  5 * time.Millisecond,
		Err:       fmt.Errorf("connection timeout"),
	})

	snap := c.GlobalSnapshot()
	if snap.TotalRequests != 11 {
		t.Errorf("Expected 11 total requests, got %d", snap.TotalRequests)
	}
	if snap.SuccessRequests != 10 {
		t.Errorf("Expected 10 success requests, got %d", snap.SuccessRequests)
	}
	if snap.FailedRequests != 1 {
		t.Errorf("Expected 1 failed request, got %d", snap.FailedRequests)
	}
	if snap.StatusCodes[200] != 10 {
		t.Errorf("Expected 10 200s, got %d", snap.StatusCodes[200])
	}
	if snap.Errors["connection timeout"] != 1 {
		t.Errorf("Expected 1 connection timeout error, got %d", snap.Errors["connection timeout"])
	}
}

func TestReportGeneration(t *testing.T) {
	snap := Snapshot{
		TargetURL:       "http://127.0.0.1:8080/api",
		Duration:        5 * time.Second,
		TotalRequests:   100,
		SuccessRequests: 98,
		FailedRequests:  2,
		AverageRPS:      20.0,
		P50Latency:      10 * time.Millisecond,
		P95Latency:      25 * time.Millisecond,
		P99Latency:      40 * time.Millisecond,
		StatusCodes:     map[int]int64{200: 98},
	}

	md := GenerateMarkdownReport(snap)
	if md == "" {
		t.Fatal("Expected markdown report, got empty string")
	}

	jsonBytes, err := GenerateJSONReport(snap)
	if err != nil || len(jsonBytes) == 0 {
		t.Fatalf("Expected valid JSON report, got error: %v", err)
	}
}
