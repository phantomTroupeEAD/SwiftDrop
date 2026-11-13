package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// CheckFunc is a function that returns an error if a dependency is unhealthy.
type CheckFunc func(ctx context.Context) error

// Server serves health check probes and Prometheus metrics.
type Server struct {
	mu         sync.RWMutex
	checks     map[string]CheckFunc
	httpServer *http.Server
	listener   net.Listener
}

// NewServer initializes a health and metrics server.
func NewServer(port int) *Server {
	s := &Server{
		checks: make(map[string]CheckFunc),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", s.handleLive)
	mux.HandleFunc("/health/ready", s.handleReady)
	mux.Handle("/metrics", promhttp.Handler())

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	return s
}

// AddReadyCheck registers a named readiness check.
func (s *Server) AddReadyCheck(name string, check CheckFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks[name] = check
}

// Start binds to the configured port and starts listening in a background goroutine.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("health server failed to listen on %s: %w", s.httpServer.Addr, err)
	}
	s.listener = ln

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Health HTTP server exited: %v", err)
		}
	}()

	return nil
}

// Addr returns the bound address, useful if port 0 was passed for testing.
func (s *Server) Addr() net.Addr {
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

// Shutdown gracefully shuts down the health HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleLive(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type readyResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	checksCopy := make(map[string]CheckFunc, len(s.checks))
	for k, v := range s.checks {
		checksCopy[k] = v
	}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if len(checksCopy) == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(readyResponse{
			Status: "initializing",
			Checks: map[string]string{"system": "no readiness checks registered yet"},
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	results := make(map[string]string, len(checksCopy))
	allHealthy := true

	for name, check := range checksCopy {
		if err := check(ctx); err != nil {
			allHealthy = false
			results[name] = err.Error()
		} else {
			results[name] = "ok"
		}
	}

	if !allHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(readyResponse{
			Status: "degraded",
			Checks: results,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(readyResponse{
		Status: "ok",
		Checks: results,
	})
}
