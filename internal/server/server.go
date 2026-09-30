package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/eyejdev/traftest/internal/engine"
	"github.com/eyejdev/traftest/internal/metrics"
	"github.com/eyejdev/traftest/web"
)

// Server coordinates the local web dashboard and SSE metrics stream
type Server struct {
	port       int
	collector  *metrics.Collector
	workerPool *engine.WorkerPool
	httpServer *http.Server

	clientsMu sync.Mutex
	clients   map[chan []byte]struct{}
}

func NewServer(port int, col *metrics.Collector, wp *engine.WorkerPool) *Server {
	if port <= 0 {
		port = 8080
	}
	return &Server{
		port:       port,
		collector:  col,
		workerPool: wp,
		clients:    make(map[chan []byte]struct{}),
	}
}

func (s *Server) Start(openBrowser bool) error {
	mux := http.NewServeMux()

	// Sub-filesystem for embedded web static files
	staticFS, err := fs.Sub(web.Content, "static")
	if err == nil {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	}

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/stream", s.handleSSE)
	mux.HandleFunc("/api/pause", s.handlePause)
	mux.HandleFunc("/api/stop", s.handleStop)
	mux.HandleFunc("/api/report", s.handleReport)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.port))
	if err != nil {
		// Fallback to random available port if specified port is taken
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
		s.port = listener.Addr().(*net.TCPAddr).Port
	}

	s.httpServer = &http.Server{
		Handler: mux,
	}

	// Start SSE broadcaster loop
	go s.broadcastLoop()

	dashboardURL := fmt.Sprintf("http://127.0.0.1:%d", s.port)
	fmt.Printf("\n  🌐 Goenma TrafTest Web Dashboard running at: %s\n\n", dashboardURL)

	if openBrowser {
		go openURL(dashboardURL)
	}

	return s.httpServer.Serve(listener)
}

func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	content, err := web.Content.ReadFile("index.html")
	if err != nil {
		http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	msgChan := make(chan []byte, 10)
	s.clientsMu.Lock()
	s.clients[msgChan] = struct{}{}
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, msgChan)
		close(msgChan)
		s.clientsMu.Unlock()
	}()

	// Send initial snapshot immediately
	initSnap := s.collector.WindowSnapshot()
	data, _ := json.Marshal(initSnap)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-msgChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func (s *Server) broadcastLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		snap := s.collector.WindowSnapshot()
		data, err := json.Marshal(snap)
		if err != nil {
			continue
		}

		s.clientsMu.Lock()
		for ch := range s.clients {
			select {
			case ch <- data:
			default:
			}
		}
		s.clientsMu.Unlock()
	}
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	isPaused := s.workerPool.TogglePause()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"is_paused": isPaused})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.workerPool.Stop()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"is_stopped": true})
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	snap := s.collector.GlobalSnapshot()

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=traftest-report.json")
		data, _ := metrics.GenerateJSONReport(snap)
		w.Write(data)
		return
	}

	// Markdown format by default
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=traftest-report.md")
	md := metrics.GenerateMarkdownReport(snap)
	w.Write([]byte(md))
}

func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
