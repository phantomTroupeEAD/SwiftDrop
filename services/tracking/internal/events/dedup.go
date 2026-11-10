package events

import (
	"sync"
	"time"
)

// Deduplicator provides thread-safe, bounded, in-memory duplicate detection with TTL.
// Prevents unbounded memory growth and memory leaks while handling duplicate deliveries.
type Deduplicator struct {
	mu         sync.Mutex
	entries    map[string]time.Time
	order      []string
	maxEntries int
	ttl        time.Duration
}

// NewDeduplicator constructs a Deduplicator bounded by maxEntries and time-to-live.
func NewDeduplicator(maxEntries int, ttl time.Duration) *Deduplicator {
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &Deduplicator{
		entries:    make(map[string]time.Time, maxEntries),
		order:      make([]string, 0, maxEntries),
		maxEntries: maxEntries,
		ttl:        ttl,
	}
}

// SeenOrAdd checks if key has been seen within TTL.
// If seen, it returns true.
// If not seen, it adds the key with current timestamp and returns false.
func (d *Deduplicator) SeenOrAdd(key string) bool {
	if key == "" {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()

	// Check if already present and not expired
	if seenAt, exists := d.entries[key]; exists {
		if now.Sub(seenAt) <= d.ttl {
			return true
		}
		// Expired entry; will overwrite below
	}

	// Purge oldest entries if at capacity
	if len(d.entries) >= d.maxEntries {
		d.evictOldest()
	}

	d.entries[key] = now
	d.order = append(d.order, key)
	return false
}

// Seen checks if key exists and has not expired, without modifying state.
func (d *Deduplicator) Seen(key string) bool {
	if key == "" {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	seenAt, exists := d.entries[key]
	if !exists {
		return false
	}
	return time.Since(seenAt) <= d.ttl
}

// Len returns the number of active entries in the deduplication map.
func (d *Deduplicator) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.entries)
}

// evictOldest removes expired entries or the oldest entry from the ring buffer.
func (d *Deduplicator) evictOldest() {
	now := time.Now()
	// First pass: try to evict expired entries from order slice
	cleaned := make([]string, 0, len(d.order))
	for _, k := range d.order {
		seenAt, exists := d.entries[k]
		if exists && now.Sub(seenAt) <= d.ttl {
			cleaned = append(cleaned, k)
		} else {
			delete(d.entries, k)
		}
	}
	d.order = cleaned

	// If still at or over capacity, drop the oldest entry
	for len(d.entries) >= d.maxEntries && len(d.order) > 0 {
		oldest := d.order[0]
		d.order = d.order[1:]
		delete(d.entries, oldest)
	}
}
