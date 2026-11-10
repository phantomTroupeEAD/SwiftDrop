package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type contextKey string

const (
	correlationIDKey contextKey = "swiftdrop_correlation_id"
)

var requestIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-]{1,128}$`)

// generateUUIDv7 generates an RFC 9562 UUIDv7 identifier for tracing and correlation.
func generateUUIDv7() string {
	var raw [16]byte
	milli := uint64(time.Now().UnixMilli())
	raw[0] = byte(milli >> 40)
	raw[1] = byte(milli >> 32)
	raw[2] = byte(milli >> 24)
	raw[3] = byte(milli >> 16)
	raw[4] = byte(milli >> 8)
	raw[5] = byte(milli)

	if _, err := rand.Read(raw[6:]); err != nil {
		// Fallback timestamp representation if crypto/rand fails
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}

	raw[6] = (raw[6] & 0x0f) | 0x70 // Version 7
	raw[8] = (raw[8] & 0x3f) | 0x80 // Variant 1

	dst := make([]byte, 36)
	hex.Encode(dst[0:8], raw[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], raw[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], raw[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], raw[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], raw[10:16])

	return string(dst)
}

// RequestID extracts the X-Request-ID header, validates it against ^[a-zA-Z0-9_\-]{1,128}$,
// generates a UUIDv7 if missing or invalid, stores it in the request context, and echoes it in
// the response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if !requestIDRegex.MatchString(reqID) {
			reqID = generateUUIDv7()
		}

		ctx := context.WithValue(r.Context(), correlationIDKey, reqID)
		w.Header().Set("X-Request-ID", reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CorrelationID extracts the distributed tracing correlation ID from context.
// Returns an empty string if no correlation ID is present.
func CorrelationID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	val, ok := ctx.Value(correlationIDKey).(string)
	if !ok {
		return ""
	}
	return val
}

// WithCorrelationID sets the correlation ID in the given context.
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// Recover catches panics in subsequent HTTP handlers, logs the error without sensitive data,
// and returns a standard HTTP 500 error envelope.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					corrID := CorrelationID(r.Context())
					if logger != nil {
						logger.Error(
							"HTTP request panic recovered",
							"error", fmt.Sprintf("%v", rec),
							"correlation_id", corrID,
							"method", r.Method,
							"path", r.URL.Path,
						)
					}

					WriteError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "An unexpected internal server error occurred")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher to pass through flush operations for SSE and chunked streaming.
func (rec *statusRecorder) Flush() {
	if flusher, ok := rec.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap exposes the underlying ResponseWriter for Go 1.20+ http.ResponseController unwrapping.
func (rec *statusRecorder) Unwrap() http.ResponseWriter {
	return rec.ResponseWriter
}

// AccessLog logs HTTP request completions using structured JSON logging.
// It never logs query parameters, authorization tokens, or request bodies.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := &statusRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(recorder, r)

			if logger != nil {
				duration := time.Since(start)
				logger.Info(
					"HTTP access",
					"method", r.Method,
					"path", r.URL.Path,
					"status", recorder.statusCode,
					"duration_ms", duration.Milliseconds(),
					"correlation_id", CorrelationID(r.Context()),
				)
			}
		})
	}
}
