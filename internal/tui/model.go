package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/eyejdev/traftest/internal/engine"
	"github.com/eyejdev/traftest/internal/metrics"
)

type tickMsg time.Time

type Model struct {
	collector  *metrics.Collector
	workerPool *engine.WorkerPool
	progress   progress.Model
	width      int
	height     int
	snapshot   metrics.Snapshot
	quitting   bool
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#38BDF8")).
			Background(lipgloss.Color("#0F172A")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")).
			Bold(true)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8FAFC")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#34D399")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F87171")).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FBBF24")).
			Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#334155")).
			Padding(1, 2)
)

func NewModel(col *metrics.Collector, wp *engine.WorkerPool) Model {
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
	)

	return Model{
		collector:  col,
		workerPool: wp,
		progress:   prog,
		snapshot:   col.WindowSnapshot(),
	}
}

func (m Model) Init() tea.Cmd {
	return tickEvery()
}

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.workerPool.Stop()
			return m, tea.Quit
		case "p", " ":
			m.workerPool.TogglePause()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = msg.Width - 10
		if m.progress.Width > 60 {
			m.progress.Width = 60
		}
		return m, nil

	case tickMsg:
		m.snapshot = m.collector.WindowSnapshot()
		if m.snapshot.IsFinished {
			m.quitting = true
			return m, tea.Quit
		}
		return m, tickEvery()
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	snap := m.snapshot
	var sb strings.Builder

	// Top banner
	sb.WriteString(titleStyle.Render("⚡ GOENMA TRAFTEST"))
	sb.WriteString("  ")
	if snap.IsPaused {
		sb.WriteString(warningStyle.Render("PAUSED [Press Space to Resume]"))
	} else if snap.IsRunning {
		sb.WriteString(successStyle.Render("RUNNING [Press Space to Pause, 'q' to Quit]"))
	}
	sb.WriteString("\n\n")

	// Target info
	sb.WriteString(fmt.Sprintf("%s %s\n", headerStyle.Render("Target:"), valueStyle.Render(snap.TargetURL)))
	sb.WriteString(fmt.Sprintf("%s %s | %s %s | %s %s\n\n",
		headerStyle.Render("Workers:"), valueStyle.Render(fmt.Sprintf("%d", snap.Concurrency)),
		headerStyle.Render("Target RPS:"), valueStyle.Render(formatRPS(snap.TargetRPS)),
		headerStyle.Render("Elapsed:"), valueStyle.Render(snap.Duration.Round(time.Millisecond).String()),
	))

	// Requests Stats
	reqStats := fmt.Sprintf(
		"Total: %s  |  Success: %s  |  Errors: %s  |  Current RPS: %s  |  Avg RPS: %s",
		valueStyle.Render(fmt.Sprintf("%d", snap.TotalRequests)),
		successStyle.Render(fmt.Sprintf("%d", snap.SuccessRequests)),
		errorStyle.Render(fmt.Sprintf("%d", snap.FailedRequests)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Bold(true).Render(fmt.Sprintf("%.0f", snap.CurrentRPS)),
		valueStyle.Render(fmt.Sprintf("%.1f", snap.AverageRPS)),
	)
	sb.WriteString(boxStyle.Render(reqStats))
	sb.WriteString("\n\n")

	// Latency Breakdown
	latTable := fmt.Sprintf(
		"⏱️  LATENCY SPECTRUM\n\n"+
			"Min: %-10s  p50: %-10s  p90: %-10s\n"+
			"p95: %-10s  p99: %-10s  Max: %-10s",
		formatDur(snap.MinLatency),
		successStyle.Render(formatDur(snap.P50Latency)),
		formatDur(snap.P90Latency),
		warningStyle.Render(formatDur(snap.P95Latency)),
		errorStyle.Render(formatDur(snap.P99Latency)),
		formatDur(snap.MaxLatency),
	)
	sb.WriteString(boxStyle.Render(latTable))
	sb.WriteString("\n\n")

	// HTTP Status breakdown
	if len(snap.StatusCodes) > 0 {
		var codes []string
		for code, cnt := range snap.StatusCodes {
			var style = successStyle
			if code >= 400 && code < 500 {
				style = warningStyle
			} else if code >= 500 {
				style = errorStyle
			}
			codes = append(codes, fmt.Sprintf("%s: %d", style.Render(fmt.Sprintf("[%d]", code)), cnt))
		}
		sb.WriteString(headerStyle.Render("Status Codes: ") + strings.Join(codes, "  ") + "\n")
	}

	return sb.String()
}

func formatRPS(rps int) string {
	if rps <= 0 {
		return "Unlimited"
	}
	return fmt.Sprintf("%d req/s", rps)
}

func formatDur(d time.Duration) string {
	if d <= 0 {
		return "0ms"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
