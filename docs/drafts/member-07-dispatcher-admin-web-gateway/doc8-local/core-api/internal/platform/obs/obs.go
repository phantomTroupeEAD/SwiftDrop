// Package obs gives the service JSON logs, Prometheus-format metrics and the
// request middleware (correlation ID, access log, panic recovery).
// It uses only the standard library (AGENTS.md: no new dependencies without agreement).
package obs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"swiftdrop.local/m7/core-api/internal/platform/web"
)

// NewLogger writes one JSON object per line to w (runbook 2.1 format).
func NewLogger(w io.Writer, level string) *slog.Logger {
	var l slog.Level
	_ = l.UnmarshalText([]byte(level))
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: l, ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
		switch a.Key {
		case slog.TimeKey:
			a.Key = "timestamp"
		case slog.MessageKey:
			a.Key = "msg"
		}
		return a
	}})
	return slog.New(h).With("service", "core-api", "module", "admin")
}

// Metrics is a tiny registry. Label sets are fixed by the code, never by user input,
// so the number of series stays small.
type Metrics struct {
	mu       sync.Mutex
	counters map[string]map[string]float64 // name -> labels -> value
	help     map[string]string
	hist     map[string]*histogram
	gauges   map[string]func() float64
}

type histogram struct {
	buckets []float64
	counts  map[string][]uint64
	sum     map[string]float64
	n       map[string]uint64
}

func NewMetrics() *Metrics {
	return &Metrics{counters: map[string]map[string]float64{}, help: map[string]string{}, hist: map[string]*histogram{}, gauges: map[string]func() float64{}}
}

func labels(kv ...string) string {
	parts := make([]string, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		parts = append(parts, fmt.Sprintf(`%s=%q`, kv[i], kv[i+1]))
	}
	return strings.Join(parts, ",")
}

func (m *Metrics) Inc(name, help string, kv ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counters[name] == nil {
		m.counters[name] = map[string]float64{}
		m.help[name] = help
	}
	m.counters[name][labels(kv...)]++
}

func (m *Metrics) Observe(name, help string, v float64, kv ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := m.hist[name]
	if h == nil {
		h = &histogram{buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			counts: map[string][]uint64{}, sum: map[string]float64{}, n: map[string]uint64{}}
		m.hist[name] = h
		m.help[name] = help
	}
	l := labels(kv...)
	if h.counts[l] == nil {
		h.counts[l] = make([]uint64, len(h.buckets))
	}
	for i, b := range h.buckets {
		if v <= b {
			h.counts[l][i]++
		}
	}
	h.sum[l] += v
	h.n[l]++
}

func (m *Metrics) Gauge(name, help string, f func() float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = f
	m.help[name] = help
}

// ServeHTTP writes the Prometheus text format. Mounted at /metrics, which the
// gateway never routes (Doc 6 GW-09); Prometheus scrapes the pod directly.
func (m *Metrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	for _, name := range sortedKeys(m.counters) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n", name, m.help[name], name)
		for _, l := range sortedKeys(m.counters[name]) {
			fmt.Fprintf(w, "%s{%s} %g\n", name, l, m.counters[name][l])
		}
	}
	for _, name := range sortedKeys(m.hist) {
		h := m.hist[name]
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s histogram\n", name, m.help[name], name)
		for _, l := range sortedKeys(h.counts) {
			for i, b := range h.buckets {
				fmt.Fprintf(w, "%s_bucket{%s,le=\"%g\"} %d\n", name, l, b, h.counts[l][i])
			}
			fmt.Fprintf(w, "%s_bucket{%s,le=\"+Inf\"} %d\n%s_sum{%s} %g\n%s_count{%s} %d\n", name, l, h.n[l], name, l, h.sum[l], name, l, h.n[l])
		}
	}
	for _, name := range sortedKeys(m.gauges) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %g\n", name, m.help[name], name, name, m.gauges[name]())
	}
}

func sortedKeys[V any](m map[string]V) []string {
	k := make([]string, 0, len(m))
	for x := range m {
		k = append(k, x)
	}
	sort.Strings(k)
	return k
}

type routeKey struct{}

// Route wraps a handler so the access log and metrics know the route pattern
// (for example "PATCH /api/v1/admin/zones/{id}") instead of the raw path.
func Route(pattern string, h http.Handler) (string, http.Handler) {
	return pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := r.Context().Value(routeKey{}).(*string); ok {
			*p = pattern
		}
		h.ServeHTTP(w, r)
	})
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(s int) { r.status = s; r.ResponseWriter.WriteHeader(s) }

// Middleware adds X-Request-ID, recovers panics, writes one access log line and
// records metrics. It must sit inside the auth middleware so it can log the actor.
func Middleware(log *slog.Logger, m *Metrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := web.CorrelationID(r)
		r.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		rec := &recorder{ResponseWriter: w, status: 200}
		route := "unmatched"
		r = r.WithContext(context.WithValue(r.Context(), routeKey{}, &route))
		defer func() {
			if p := recover(); p != nil {
				log.Error("panic", "request_id", id, "panic", fmt.Sprint(p))
				web.WriteError(rec, r, fmt.Errorf("panic"))
			}
			d := time.Since(start)
			status := fmt.Sprint(rec.status)
			m.Inc("http_requests_total", "HTTP requests by route and status.", "route", route, "status", status)
			m.Observe("http_request_duration_seconds", "HTTP request time.", d.Seconds(), "route", route)
			lvl := slog.LevelInfo
			if rec.status >= 500 {
				lvl = slog.LevelError
			} else if r.URL.Path == "/livez" || r.URL.Path == "/readyz" || r.URL.Path == "/metrics" {
				lvl = slog.LevelDebug // probes every few seconds would flood the logs
			}
			attrs := []any{"request_id", id, "method", r.Method, "route", route, "status", rec.status, "duration_ms", float64(d.Microseconds()) / 1000}
			if p, ok := web.PrincipalFrom(r.Context()); ok {
				attrs = append(attrs, "actor_id", p.UserID, "actor_role", p.Role)
			}
			if rec.status == 401 || rec.status == 403 {
				attrs = append(attrs, "security_event", "access_denied")
			}
			log.Log(r.Context(), lvl, "request", attrs...)
		}()
		next.ServeHTTP(rec, r)
	})
}
