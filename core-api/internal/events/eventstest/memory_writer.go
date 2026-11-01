package eventstest

import (
	"context"
	"database/sql"
	"sync"

	"github.com/swiftdrop/core-api/internal/events"
)

// InMemoryWriter provides an in-memory implementation of events.OutboxWriter for unit testing.
// It applies the exact same validation as PostgresOutboxWriter without requiring a live PostgreSQL instance.
type InMemoryWriter struct {
	mu        sync.RWMutex
	envelopes []events.AnyEnvelope
}

// NewInMemoryWriter constructs a new InMemoryWriter.
func NewInMemoryWriter() *InMemoryWriter {
	return &InMemoryWriter{
		envelopes: make([]events.AnyEnvelope, 0),
	}
}

// Ensure InMemoryWriter implements events.OutboxWriter.
var _ events.OutboxWriter = (*InMemoryWriter)(nil)

// Write validates and stores envelopes in-memory.
// In accordance with outbox contracts, tx must not be nil.
func (m *InMemoryWriter) Write(ctx context.Context, tx *sql.Tx, envelopes ...events.AnyEnvelope) error {
	if tx == nil {
		return events.ErrMissingTransaction
	}

	if len(envelopes) == 0 {
		return nil
	}

	// Validate all envelopes using canonical rules
	for _, env := range envelopes {
		if err := events.ValidateEnvelope(env); err != nil {
			return err
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.envelopes = append(m.envelopes, envelopes...)
	return nil
}

// Events returns a snapshot of all stored envelopes.
func (m *InMemoryWriter) Events() []events.AnyEnvelope {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]events.AnyEnvelope, len(m.envelopes))
	copy(res, m.envelopes)
	return res
}

// Count returns the number of captured envelopes.
func (m *InMemoryWriter) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.envelopes)
}

// Reset clears all captured envelopes.
func (m *InMemoryWriter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.envelopes = make([]events.AnyEnvelope, 0)
}

// FindByEventType filters captured envelopes by exact event type string.
func (m *InMemoryWriter) FindByEventType(eventType string) []events.AnyEnvelope {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var matches []events.AnyEnvelope
	for _, env := range m.envelopes {
		if env.GetEventType() == eventType {
			matches = append(matches, env)
		}
	}
	return matches
}

// FindByAggregateID filters captured envelopes by aggregate ID string.
func (m *InMemoryWriter) FindByAggregateID(aggregateID string) []events.AnyEnvelope {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var matches []events.AnyEnvelope
	for _, env := range m.envelopes {
		if env.GetAggregateID() == aggregateID {
			matches = append(matches, env)
		}
	}
	return matches
}
