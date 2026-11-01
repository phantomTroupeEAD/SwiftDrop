package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestHealthServer(t *testing.T) {
	srv := NewServer(0) // Port 0 for random free port
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	addr := srv.Addr().String()

	t.Run("GET /health/live returns 200", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://%s/health/live", addr))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", resp.StatusCode)
		}

		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if body["status"] != "ok" {
			t.Errorf("expected status ok, got %v", body["status"])
		}
	})

	t.Run("GET /health/ready initially returns 200 with empty checks", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://%s/health/ready", addr))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /health/ready with failing check returns 503", func(t *testing.T) {
		srv.AddReadyCheck("db", func(ctx context.Context) error {
			return errors.New("connection refused")
		})

		resp, err := http.Get(fmt.Sprintf("http://%s/health/ready", addr))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("expected 503 Service Unavailable, got %d", resp.StatusCode)
		}

		var body readyResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if body.Status != "degraded" {
			t.Errorf("expected status degraded, got %v", body.Status)
		}
		if body.Checks["db"] != "connection refused" {
			t.Errorf("expected check error message, got %v", body.Checks["db"])
		}
	})

	t.Run("GET /health/ready with passing check returns 200", func(t *testing.T) {
		srv.AddReadyCheck("db", func(ctx context.Context) error {
			return nil
		})

		resp, err := http.Get(fmt.Sprintf("http://%s/health/ready", addr))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /metrics returns prometheus text format", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://%s/metrics", addr))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", resp.StatusCode)
		}

		content, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		if len(content) == 0 {
			t.Errorf("expected non-empty metrics output")
		}
	})
}
