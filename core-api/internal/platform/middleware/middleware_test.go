package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestID_Table(t *testing.T) {
	tests := []struct {
		name          string
		inputHeader   string
		expectSame    bool
		expectValidID bool
	}{
		{
			name:          "Valid request ID",
			inputHeader:   "valid-request-123_ABC",
			expectSame:    true,
			expectValidID: true,
		},
		{
			name:          "Empty request ID generates UUIDv7",
			inputHeader:   "",
			expectSame:    false,
			expectValidID: true,
		},
		{
			name:          "Whitespace request ID generates UUIDv7",
			inputHeader:   "   ",
			expectSame:    false,
			expectValidID: true,
		},
		{
			name:          "Invalid characters generates UUIDv7",
			inputHeader:   "req id with spaces; malicious=true",
			expectSame:    false,
			expectValidID: true,
		},
		{
			name:          "Too long (>128 chars) generates UUIDv7",
			inputHeader:   strings.Repeat("a", 129),
			expectSame:    false,
			expectValidID: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedContextID string
			handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedContextID = CorrelationID(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			if tc.inputHeader != "" {
				req.Header.Set("X-Request-ID", tc.inputHeader)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			respHeaderID := rec.Header().Get("X-Request-ID")
			if respHeaderID == "" {
				t.Fatal("expected X-Request-ID response header to be set")
			}
			if capturedContextID != respHeaderID {
				t.Fatalf("expected context correlation_id %q == response header %q", capturedContextID, respHeaderID)
			}

			if tc.expectSame && respHeaderID != tc.inputHeader {
				t.Errorf("expected header %q to be preserved, got %q", tc.inputHeader, respHeaderID)
			}
			if !tc.expectSame && respHeaderID == tc.inputHeader {
				t.Errorf("expected invalid header %q to be replaced, but it was kept", tc.inputHeader)
			}

			if tc.expectValidID && !requestIDRegex.MatchString(respHeaderID) {
				t.Errorf("result %q does not match request ID regex", respHeaderID)
			}
		})
	}
}

func TestCorrelationID_Accessor(t *testing.T) {
	if CorrelationID(nil) != "" {
		t.Error("expected empty string for nil context")
	}
	if CorrelationID(context.Background()) != "" {
		t.Error("expected empty string for empty context")
	}

	ctx := WithCorrelationID(context.Background(), "trace-xyz")
	if CorrelationID(ctx) != "trace-xyz" {
		t.Errorf("expected 'trace-xyz', got %q", CorrelationID(ctx))
	}
}

func TestRecover_Middleware(t *testing.T) {
	panickingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("database connection lost")
	})

	stack := RequestID(Recover(nil)(panickingHandler))

	req := httptest.NewRequest("GET", "/api/v1/fail", nil)
	req.Header.Set("X-Request-ID", "corr-panic-test")
	rec := httptest.NewRecorder()

	stack.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	var body standardErrorPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}

	if body.Error.Code != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected code INTERNAL_SERVER_ERROR, got %s", body.Error.Code)
	}
	if body.Error.CorrelationID != "corr-panic-test" {
		t.Errorf("expected correlation_id 'corr-panic-test', got %s", body.Error.CorrelationID)
	}
	if body.Error.Timestamp == "" {
		t.Error("expected non-empty timestamp in error response")
	}
}

func TestAccessLog_Middleware(t *testing.T) {
	handler := AccessLog(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest("POST", "/api/v1/parcels", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", rec.Code)
	}
}

func TestBodyLimit_Middleware(t *testing.T) {
	handler := BodyLimit(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Within limit
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString("12345"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 within limit, got %d", rec.Code)
	}

	// 2. Exceeds limit
	req2 := httptest.NewRequest("POST", "/test", bytes.NewBufferString("12345678901234567890"))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 exceeding limit, got %d", rec2.Code)
	}
}
