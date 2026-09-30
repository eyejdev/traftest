package metrics

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// GenerateJSONReport serializes snapshot to formatted JSON
func GenerateJSONReport(snap Snapshot) ([]byte, error) {
	return json.MarshalIndent(snap, "", "  ")
}

// GenerateMarkdownReport generates a clean GitHub-flavored markdown report
func GenerateMarkdownReport(snap Snapshot) string {
	var sb strings.Builder

	sb.WriteString("# 🚀 Goenma TrafTest - Traffic & Load Test Report\n\n")
	sb.WriteString(fmt.Sprintf("**Target URL:** `%s`  \n", snap.TargetURL))
	sb.WriteString(fmt.Sprintf("**Date:** %s  \n", snap.Timestamp.Format("2006-01-02 15:04:05 MST")))
	sb.WriteString(fmt.Sprintf("**Duration:** %v  \n", snap.Duration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("**Concurrency (Workers):** %d  \n", snap.Concurrency))
	if snap.TargetRPS > 0 {
		sb.WriteString(fmt.Sprintf("**Target RPS:** %d req/s  \n", snap.TargetRPS))
	} else {
		sb.WriteString("**Target RPS:** Max uncapped  \n")
	}

	sb.WriteString("\n## 📊 Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("| :--- | :--- |\n")
	sb.WriteString(fmt.Sprintf("| **Total Requests** | %d |\n", snap.TotalRequests))
	sb.WriteString(fmt.Sprintf("| **Successful (2xx/3xx)** | %d (%.2f%%) |\n", snap.SuccessRequests, pct(snap.SuccessRequests, snap.TotalRequests)))
	sb.WriteString(fmt.Sprintf("| **Failed / Errors** | %d (%.2f%%) |\n", snap.FailedRequests, pct(snap.FailedRequests, snap.TotalRequests)))
	sb.WriteString(fmt.Sprintf("| **Average RPS** | %.2f req/s |\n", snap.AverageRPS))

	sb.WriteString("\n## ⏱️ Latency Percentiles\n\n")
	sb.WriteString("| Percentile | Latency |\n")
	sb.WriteString("| :--- | :--- |\n")
	sb.WriteString(fmt.Sprintf("| **Min** | %v |\n", formatDuration(snap.MinLatency)))
	sb.WriteString(fmt.Sprintf("| **p50 (Median)** | %v |\n", formatDuration(snap.P50Latency)))
	sb.WriteString(fmt.Sprintf("| **p90** | %v |\n", formatDuration(snap.P90Latency)))
	sb.WriteString(fmt.Sprintf("| **p95** | %v |\n", formatDuration(snap.P95Latency)))
	sb.WriteString(fmt.Sprintf("| **p99** | %v |\n", formatDuration(snap.P99Latency)))
	sb.WriteString(fmt.Sprintf("| **Max** | %v |\n", formatDuration(snap.MaxLatency)))
	sb.WriteString(fmt.Sprintf("| **Average** | %v |\n", formatDuration(snap.AvgLatency)))

	if len(snap.StatusCodes) > 0 {
		sb.WriteString("\n## 🏷️ HTTP Status Codes\n\n")
		sb.WriteString("| Status Code | Count | Ratio |\n")
		sb.WriteString("| :--- | :--- | :--- |\n")
		for code, count := range snap.StatusCodes {
			sb.WriteString(fmt.Sprintf("| `%d` | %d | %.2f%% |\n", code, count, pct(count, snap.TotalRequests)))
		}
	}

	if len(snap.Errors) > 0 {
		sb.WriteString("\n## ⚠️ Network / Protocol Errors\n\n")
		sb.WriteString("| Error | Occurrences |\n")
		sb.WriteString("| :--- | :--- |\n")
		for errStr, count := range snap.Errors {
			sb.WriteString(fmt.Sprintf("| `%s` | %d |\n", errStr, count))
		}
	}

	return sb.String()
}

func pct(portion, total int64) float64 {
	if total == 0 {
		return 0
	}
	return (float64(portion) / float64(total)) * 100.0
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
	}
	return fmt.Sprintf("%.3fs", d.Seconds())
}
