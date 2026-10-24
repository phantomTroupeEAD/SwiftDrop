// Package web holds small HTTP helpers: request IDs, access logs, panic recovery,
// JSON bodies and the standard error envelope (docs/contracts/api-conventions.md §2.3).
// STAND-IN for the shared platform package.
package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

type ctxKey int

const (
	keyRequestID ctxKey = iota
	keyPrincipal
	keyLogFields
	keyMeta
)

// RequestMeta is client info for audit rows (SRS SEC-015). Behind the gateway the
// client IP comes from the trusted proxy header set by Member 7's Traefik config.
type RequestMeta struct{ ClientIP, UserAgent string }

func Meta(ctx context.Context) RequestMeta {
	m, _ := ctx.Value(keyMeta).(RequestMeta)
	return m
}

func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return ""
}

// Principal is the authenticated JWT caller (from Member 1's middleware).
type Principal struct {
	UserID   string
	Role     string
	TenantID string
}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, keyPrincipal, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(keyPrincipal).(Principal)
	return p, ok
}

func RequestID(ctx context.Context) string {
	s, _ := ctx.Value(keyRequestID).(string)
	return s
}

// LogFields lets handlers attach allow-listed fields (Doc 7 §4) to the access log.
type LogFields struct{ MerchantID, KeyPrefix, ErrorCode string }

func Fields(ctx context.Context) *LogFields {
	if f, ok := ctx.Value(keyLogFields).(*LogFields); ok {
		return f
	}
	return &LogFields{}
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }

// Middleware adds request ID, panic recovery and one structured access-log line per request.
// The log never contains headers, query strings or bodies.
// routeOf returns the matched route pattern (low-cardinality label; Go 1.22 has no r.Pattern).
func Middleware(log *slog.Logger, routeOf func(*http.Request) string, onDone func(route string, status int, d time.Duration), next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 64 || strings.ContainsAny(id, "\r\n") {
			id = newID()
		}
		lf := &LogFields{}
		ua := r.UserAgent()
		if len(ua) > 200 {
			ua = ua[:200]
		}
		ctx := context.WithValue(context.WithValue(r.Context(), keyRequestID, id), keyLogFields, lf)
		ctx = context.WithValue(ctx, keyMeta, RequestMeta{ClientIP: clientIP(r), UserAgent: ua})
		w.Header().Set("X-Request-ID", id)
		sw := &statusWriter{ResponseWriter: w, status: 200}
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic", "request_id", id, "panic", rec)
				Error(sw, r.WithContext(ctx), http.StatusInternalServerError, "INTERNAL_ERROR", "Unexpected error.", nil)
			}
			route := routeOf(r)
			if route == "" {
				route = "unmatched"
			}
			d := time.Since(start)
			attrs := []any{"request_id", id, "method", r.Method, "route", route, "status", sw.status,
				"duration_ms", float64(d.Microseconds()) / 1000}
			if lf.MerchantID != "" {
				attrs = append(attrs, "merchant_id", lf.MerchantID)
			}
			if lf.KeyPrefix != "" {
				attrs = append(attrs, "key_prefix", lf.KeyPrefix)
			}
			if lf.ErrorCode != "" {
				attrs = append(attrs, "error_code", lf.ErrorCode)
			}
			log.Info("http_request", attrs...)
			if onDone != nil {
				onDone(route, sw.status, d)
			}
		}()
		next.ServeHTTP(sw, r.WithContext(ctx))
	})
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Data(w http.ResponseWriter, status int, v any) { JSON(w, status, map[string]any{"data": v}) }

type Detail struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

// Error writes the standard envelope with correlation_id; never a stack trace or SQL text.
func Error(w http.ResponseWriter, r *http.Request, status int, code, msg string, details []Detail) {
	Fields(r.Context()).ErrorCode = code
	body := map[string]any{
		"code":           code,
		"message":        msg,
		"correlation_id": RequestID(r.Context()),
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}
	if len(details) > 0 {
		body["details"] = details
	}
	JSON(w, status, map[string]any{"error": body})
}

const MaxBody = 64 << 10 // Doc 7 §6.1

var ErrBadJSON = errors.New("invalid JSON body")

// Decode reads a JSON body with a size limit. strict rejects unknown fields.
func Decode(r *http.Request, dst any, strict bool) error {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		return ErrBadJSON
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, MaxBody+1))
	if strict {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(dst); err != nil {
		return ErrBadJSON
	}
	if dec.More() {
		return ErrBadJSON
	}
	return nil
}
