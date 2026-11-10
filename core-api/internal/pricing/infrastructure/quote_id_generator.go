package infrastructure

import (
	"errors"
	"fmt"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/pricing/application"
)

// Ensure UUIDv7QuoteIDGenerator implements application.QuoteIDGenerator.
var _ application.QuoteIDGenerator = (*UUIDv7QuoteIDGenerator)(nil)

// UUIDv7QuoteIDGenerator implements application.QuoteIDGenerator using RFC 9562 UUIDv7 values.
type UUIDv7QuoteIDGenerator struct {
	generate func() (string, error)
}

// NewUUIDv7QuoteIDGenerator constructs a production QuoteIDGenerator backed by events.GenerateUUIDv7.
func NewUUIDv7QuoteIDGenerator() *UUIDv7QuoteIDGenerator {
	return &UUIDv7QuoteIDGenerator{
		generate: events.GenerateUUIDv7,
	}
}

// NewID generates a unique RFC 9562 UUIDv7 string suitable for PostgreSQL UUID primary keys.
func (g *UUIDv7QuoteIDGenerator) NewID() (string, error) {
	if g == nil {
		return "", errors.New("pricing: nil quote id generator")
	}

	gen := g.generate
	if gen == nil {
		gen = events.GenerateUUIDv7
	}

	id, err := gen()
	if err != nil {
		return "", fmt.Errorf("pricing: generate quote UUIDv7: %w", err)
	}

	if !events.IsValidUUID(id) {
		return "", fmt.Errorf("pricing: generated quote id %q is not a valid UUID", id)
	}

	return id, nil
}

