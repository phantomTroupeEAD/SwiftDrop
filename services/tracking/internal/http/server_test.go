package http

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/swiftdrop/tracking/internal/config"
	"github.com/swiftdrop/tracking/internal/health"
)

func TestServerStartupAndShutdown(t *testing.T) {
	cfg := &config.Config{
		HTTPAddr:            "127.0.0.1:0", // Ephemeral port for test isolation
		AppEnv:              "test",
		ShutdownTimeout:     5 * time.Second,
		ReadTimeout:         5 * time.Second,
		WriteTimeout:        5 * time.Second,
		IdleTimeout:         10 * time.Second,
		LogLevel:            "info",
		SSEHeartbeatSeconds: 15,
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	healthHandler := health.NewHandler()
	router := NewRouter(cfg, logger, healthHandler)

	srv := NewServer(cfg, router, logger)

	// Start server
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	addr := srv.Addr()
	if addr == nil {
		t.Fatal("expected non-nil server address")
	}

	baseURL := fmt.Sprintf("http://%s", addr.String())

	// Client request to /health/live
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/health/live")
	if err != nil {
		t.Fatalf("failed to GET /health/live: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK from running server, got %d", resp.StatusCode)
	}

	// Client request to /health/ready
	respReady, err := client.Get(baseURL + "/health/ready")
	if err != nil {
		t.Fatalf("failed to GET /health/ready: %v", err)
	}
	defer respReady.Body.Close()

	if respReady.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK from /health/ready, got %d", respReady.StatusCode)
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("error during graceful shutdown: %v", err)
	}

	// Verify server stopped accepting requests
	_, err = client.Get(baseURL + "/health/live")
	if err == nil {
		t.Error("expected connection error after server shutdown, but request succeeded")
	}
}
