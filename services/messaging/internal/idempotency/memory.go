package idempotency

import (
	"context"
	"database/sql"
	"sync"
)

// MemoryStore provides an in-memory thread-safe implementation of Store for tests.
type MemoryStore struct {
	mu   sync.RWMutex
	seen map[string]bool
}

// NewMemoryStore initializes an in-memory idempotency store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{seen: make(map[string]bool)}
}

func (m *MemoryStore) key(consumerName, eventID string) string {
	return consumerName + ":" + eventID
}

// Seen checks if consumer and eventID combination has been recorded.
func (m *MemoryStore) Seen(_ context.Context, consumerName, eventID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.seen[m.key(consumerName, eventID)], nil
}

// MarkDone records consumer and eventID combination as processed.
func (m *MemoryStore) MarkDone(_ context.Context, _ *sql.Tx, consumerName, eventID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seen[m.key(consumerName, eventID)] = true
	return nil
}
