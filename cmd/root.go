package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eyejdev/traftest/internal/engine"
	"github.com/eyejdev/traftest/internal/metrics"
	"github.com/eyejdev/traftest/internal/server"
	"github.com/eyejdev/traftest/internal/tui"
	"github.com/spf13/cobra"
)

var (
	targetURL     string
	method        string
	concurrency   int
	totalRequests int64
	durationStr   string
	rps           int
	timeoutStr    string
	runCLI        bool
	runWeb        bool
	webPort       int
	openBrowser   bool
	outputReport  string

	// Advanced features
	rawHeaders    []string
	bodyData      string
	bodyFile      string
	maxP95Str     string
	maxErrorPct   float64
)

var RootCmd = &cobra.Command{
	Use:   "traftest [flags] [url]",
	Short: "Goenma TrafTest is a high-performance local traffic simulation & load testing sandbox",
	Long: `⚡ Goenma TrafTest (traftest) is an open-source, friction-free tool to benchmark, simulate traffic,
and load-test local or development endpoints with real-time TUI and interactive Web UI.
Part of the GOENMA ecosystem.`,
	Args: cobra.ArbitraryArgs,
	RunE: runLoadTest,
}

func init() {
	RootCmd.Flags().StringVarP(&targetURL, "url", "u", "http://127.0.0.1:8080", "Target URL to benchmark")
	RootCmd.Flags().StringVarP(&method, "method", "X", "GET", "HTTP method (GET, POST, PUT, DELETE, PATCH, etc.)")
	RootCmd.Flags().StringArrayVarP(&rawHeaders, "header", "H", []string{}, "Custom HTTP headers (e.g. -H 'Authorization: Bearer token' -H 'Content-Type: application/json')")
	RootCmd.Flags().StringVarP(&bodyData, "body", "b", "", "HTTP request body raw string (e.g. -b '{\"user\":\"alice\"}')")
	RootCmd.Flags().StringVar(&bodyFile, "body-file", "", "Path to file containing request body (e.g. --body-file payload.json)")
	RootCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 10, "Number of concurrent workers")
	RootCmd.Flags().Int64VarP(&totalRequests, "requests", "n", 0, "Total number of requests to send (0 for unlimited/duration-based)")
	RootCmd.Flags().StringVarP(&durationStr, "duration", "d", "0s", "Duration of test (e.g. 10s, 1m, 30s)")
	RootCmd.Flags().IntVarP(&rps, "rps", "r", 0, "Rate limit in requests per second (0 for unlimited)")
	RootCmd.Flags().StringVarP(&timeoutStr, "timeout", "t", "10s", "Individual request timeout (e.g. 5s, 500ms)")
	RootCmd.Flags().BoolVar(&runCLI, "cli", false, "Force terminal TUI mode (Bubble Tea)")
	RootCmd.Flags().BoolVar(&runWeb, "web", false, "Run in Web UI mode (default if --cli is omitted)")
	RootCmd.Flags().IntVarP(&webPort, "port", "p", 9090, "Port for the local Web UI server")
	RootCmd.Flags().BoolVar(&openBrowser, "open", true, "Automatically open web browser in web mode")
	RootCmd.Flags().StringVarP(&outputReport, "output", "o", "", "Export report to file upon completion (e.g. report.md, report.json)")
	RootCmd.Flags().StringVar(&maxP95Str, "max-p95", "", "Fail test (exit 1) if p95 latency exceeds threshold (e.g. 250ms, 1s)")
	RootCmd.Flags().Float64Var(&maxErrorPct, "max-errors", -1, "Fail test (exit 1) if error rate percentage exceeds threshold (e.g. 5.0 for 5%)")
}

func runLoadTest(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		targetURL = args[0]
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return fmt.Errorf("invalid duration format: %w", err)
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return fmt.Errorf("invalid timeout format: %w", err)
	}

	// Parse Headers
	headersMap := make(map[string]string)
	for _, h := range rawHeaders {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			headersMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	// Process Body
	var bodyBytes []byte
	if bodyFile != "" {
		data, err := os.ReadFile(bodyFile)
		if err != nil {
			return fmt.Errorf("failed to read body file: %w", err)
		}
		bodyBytes = data
	} else if bodyData != "" {
		bodyBytes = []byte(bodyData)
	}

	// Default Content-Type if JSON payload detected and not specified
	if len(bodyBytes) > 0 && headersMap["Content-Type"] == "" {
		if strings.HasPrefix(strings.TrimSpace(string(bodyBytes)), "{") || strings.HasPrefix(strings.TrimSpace(string(bodyBytes)), "[") {
			headersMap["Content-Type"] = "application/json"
		}
	}

	cfg := engine.Config{
		URL:           targetURL,
		Method:        strings.ToUpper(method),
		Headers:       headersMap,
		Body:          bodyBytes,
		Concurrency:   concurrency,
		TotalRequests: totalRequests,
		Duration:      duration,
		RPS:           rps,
		Timeout:       timeout,
	}

	collector := metrics.NewCollector(targetURL, concurrency, rps)
	pool := engine.NewWorkerPool(cfg, collector)

	// Intercept SIGINT/SIGTERM for clean shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		pool.Stop()
		cancel()
	}()

	pool.Start(ctx)

	// Determine execution mode (CLI vs Web)
	if runCLI {
		// Run Interactive Bubble Tea TUI
		p := tea.NewProgram(tui.NewModel(collector, pool))
		if _, err := p.Run(); err != nil {
			return err
		}
	} else {
		// Default or --web mode: Start local HTTP Server with SSE Dashboard
		srv := server.NewServer(webPort, collector, pool)
		
		serverErrChan := make(chan error, 1)
		go func() {
			serverErrChan <- srv.Start(openBrowser)
		}()

		// Wait for either:
		// 1. All requests / duration finished (pool.Done())
		// 2. Ctrl+C (ctx.Done())
		// 3. Server critical error
		select {
		case <-pool.Done():
		case <-ctx.Done():
		case err := <-serverErrChan:
			if err != nil && err != http.ErrServerClosed {
				return err
			}
		}

		// Allow brief moment for browser UI to receive final snapshot, then shutdown server
		time.Sleep(500 * time.Millisecond)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutdownCancel()
		_ = srv.Stop(shutdownCtx)
	}

	// Final Summary after run
	finalSnap := collector.GlobalSnapshot()
	fmt.Println("\n" + metrics.GenerateMarkdownReport(finalSnap))

	// Save report if requested
	if outputReport != "" {
		if err := saveReport(outputReport, finalSnap); err != nil {
			fmt.Printf("⚠️  Error saving report: %v\n", err)
		} else {
			fmt.Printf("✅ Report successfully saved to %s\n", outputReport)
		}
	}

	// Evaluate CI/CD thresholds if configured
	if maxP95Str != "" {
		maxP95, err := time.ParseDuration(maxP95Str)
		if err == nil && finalSnap.P95Latency > maxP95 {
			return fmt.Errorf("🚨 THRESHOLD FAILED: p95 latency (%v) exceeded allowed limit (%v)", finalSnap.P95Latency, maxP95)
		}
	}

	if maxErrorPct >= 0 {
		actualErrPct := 0.0
		if finalSnap.TotalRequests > 0 {
			actualErrPct = (float64(finalSnap.FailedRequests) / float64(finalSnap.TotalRequests)) * 100.0
		}
		if actualErrPct > maxErrorPct {
			return fmt.Errorf("🚨 THRESHOLD FAILED: error rate (%.2f%%) exceeded allowed limit (%.2f%%)", actualErrPct, maxErrorPct)
		}
	}

	return nil
}

func saveReport(path string, snap metrics.Snapshot) error {
	if len(path) > 5 && path[len(path)-5:] == ".json" {
		data, err := metrics.GenerateJSONReport(snap)
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, 0644)
	}

	md := metrics.GenerateMarkdownReport(snap)
	return os.WriteFile(path, []byte(md), 0644)
}
