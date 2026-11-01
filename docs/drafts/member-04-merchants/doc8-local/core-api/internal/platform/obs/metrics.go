// Package obs is a tiny Prometheus text-format counter registry (no dependencies).
// STAND-IN until Member 9 picks the shared metrics library.
package obs

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type Metrics struct {
	mu     sync.Mutex
	values map[string]float64
	help   map[string]string
}

func New() *Metrics {
	return &Metrics{values: map[string]float64{}, help: map[string]string{}}
}

func (m *Metrics) Describe(name, help string) {
	m.mu.Lock()
	m.help[name] = help
	m.mu.Unlock()
}

// Add increments name{labels} by v. labels are key, value pairs.
func (m *Metrics) Add(name string, v float64, labels ...string) {
	var b strings.Builder
	b.WriteString(name)
	if len(labels) > 0 {
		b.WriteByte('{')
		for i := 0; i+1 < len(labels); i += 2 {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "%s=%q", labels[i], labels[i+1])
		}
		b.WriteByte('}')
	}
	m.mu.Lock()
	m.values[b.String()] += v
	m.mu.Unlock()
}

func (m *Metrics) Inc(name string, labels ...string) { m.Add(name, 1, labels...) }

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		m.mu.Lock()
		keys := make([]string, 0, len(m.values))
		for k := range m.values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		seen := map[string]bool{}
		for _, k := range keys {
			name := k
			if i := strings.IndexByte(k, '{'); i >= 0 {
				name = k[:i]
			}
			if !seen[name] {
				seen[name] = true
				if h := m.help[name]; h != "" {
					fmt.Fprintf(w, "# HELP %s %s\n", name, h)
				}
				fmt.Fprintf(w, "# TYPE %s counter\n", name)
			}
			fmt.Fprintf(w, "%s %g\n", k, m.values[k])
		}
		m.mu.Unlock()
	})
}
