package http

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddleware(t *testing.T) {
	t.Run("preserves valid incoming X-Request-ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", "custom-trace-id-1234")
		rr := httptest.NewRecorder()

		var capturedID string
		handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = CorrelationID(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rr, req)

		if capturedID != "custom-trace-id-1234" {
			t.Errorf("expected custom-trace-id-1234, got %s", capturedID)
		}
		if rr.Header().Get("X-Request-ID") != "custom-trace-id-1234" {
			t.Errorf("expected X-Request-ID response header to be preserved, got %s", rr.Header().Get("X-Request-ID"))
		}
	})

	t.Run("generates UUIDv7 when X-Request-ID is missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()

		var capturedID string
		handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = CorrelationID(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rr, req)

		if capturedID == "" {
			t.Fatal("expected generated correlation ID, got empty string")
		}
		if len(capturedID) != 36 {
			t.Errorf("expected 36-char UUIDv7 string, got %s (len %d)", capturedID, len(capturedID))
		}
		if rr.Header().Get("X-Request-ID") != capturedID {
			t.Errorf("expected X-Request-ID response header to match context, got %s", rr.Header().Get("X-Request-ID"))
		}
	})

	t.Run("regenerates when X-Request-ID has invalid characters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", "invalid;header$injection\n")
		rr := httptest.NewRecorder()

		var capturedID string
		handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = CorrelationID(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rr, req)

		if capturedID == "invalid;header$injection\n" {
			t.Fatal("expected invalid header to be rejected and replaced")
		}
		if len(capturedID) != 36 {
			t.Errorf("expected 36-char UUIDv7, got %s", capturedID)
		}
	})
}

func TestRecoverMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	panickingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("database connection exploded")
	})

	handler := RequestID(Recover(logger)(panickingHandler))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rr := httptest.NewRecorder()

	// Handler should catch panic and not let it bubble up
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got %d", rr.Code)
	}

	var env ErrorEnvelope
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}

	if env.Error.Code != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected INTERNAL_SERVER_ERROR code, got %s", env.Error.Code)
	}
	if env.Error.CorrelationID == "" {
		t.Error("expected non-empty correlation ID in recovered error response")
	}

	// Verify panic was logged
	loggedOutput := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("HTTP request panic recovered")) {
		t.Errorf("expected log to contain panic recovery message, got %s", loggedOutput)
	}
}

func TestAccessLogMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	normalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	handler := RequestID(AccessLog(logger)(normalHandler))

	req := httptest.NewRequest(http.MethodGet, "/test/path", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	loggedOutput := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("HTTP access")) {
		t.Errorf("expected log to contain 'HTTP access', got %s", loggedOutput)
	}
	if !bytes.Contains(buf.Bytes(), []byte("/test/path")) {
		t.Errorf("expected log to contain '/test/path', got %s", loggedOutput)
	}
}
