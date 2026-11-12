package infrastructure

import (
	"context"
	"sync"

	"github.com/swiftdrop/core-api/internal/identity/application"
)

// MockAuditWriter is an in-memory thread-safe AuditWriter for local development and unit tests.
type MockAuditWriter struct {
	mu      sync.Mutex
	Entries []application.AuditEntry
}

// NewMockAuditWriter constructs a new MockAuditWriter.
func NewMockAuditWriter() *MockAuditWriter {
	return &MockAuditWriter{
		Entries: make([]application.AuditEntry, 0),
	}
}

// WriteAuditLog appends the audit entry to the in-memory log.
func (w *MockAuditWriter) WriteAuditLog(_ context.Context, entry application.AuditEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Entries = append(w.Entries, entry)
	return nil
}
