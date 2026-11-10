package infrastructure

import (
	"errors"
	"regexp"
	"testing"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/pricing/application"
)

var uuidv7FormatRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestUUIDv7QuoteIDGenerator_ImplementsInterface(t *testing.T) {
	var gen application.QuoteIDGenerator = NewUUIDv7QuoteIDGenerator()
	if gen == nil {
		t.Fatal("expected non-nil QuoteIDGenerator")
	}
}

func TestUUIDv7QuoteIDGenerator_GeneratesValidUniqueUUIDv7s(t *testing.T) {
	gen := NewUUIDv7QuoteIDGenerator()
	const sampleSize = 1000
	seen := make(map[string]struct{}, sampleSize)

	for i := 0; i < sampleSize; i++ {
		id, err := gen.NewID()
		if err != nil {
			t.Fatalf("NewID() iteration %d returned error: %v", i, err)
		}
		if !events.IsValidUUID(id) {
			t.Fatalf("NewID() returned invalid UUID: %q", id)
		}
		if !uuidv7FormatRegex.MatchString(id) {
			t.Fatalf("NewID() returned non-UUIDv7 format: %q", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("NewID() generated duplicate UUIDv7 %q on iteration %d", id, i)
		}
		seen[id] = struct{}{}
	}
}

func TestUUIDv7QuoteIDGenerator_ZeroValueUsesDefaultGenerator(t *testing.T) {
	var gen UUIDv7QuoteIDGenerator
	id, err := gen.NewID()
	if err != nil {
		t.Fatalf("zero-value NewID() returned error: %v", err)
	}
	if !uuidv7FormatRegex.MatchString(id) {
		t.Fatalf("zero-value NewID() returned invalid UUIDv7: %q", id)
	}
}

func TestUUIDv7QuoteIDGenerator_ErrorPropagationAndGuards(t *testing.T) {
	t.Run("propagates underlying generator error", func(t *testing.T) {
		rootErr := errors.New("entropy source exhausted")
		gen := &UUIDv7QuoteIDGenerator{
			generate: func() (string, error) {
				return "", rootErr
			},
		}

		_, err := gen.NewID()
		if err == nil || !errors.Is(err, rootErr) {
			t.Fatalf("expected error wrapping %v, got %v", rootErr, err)
		}
	})

	t.Run("rejects invalid UUID output from generator", func(t *testing.T) {
		gen := &UUIDv7QuoteIDGenerator{
			generate: func() (string, error) {
				return "quote-test-001", nil
			},
		}

		_, err := gen.NewID()
		if err == nil {
			t.Fatal("expected error for non-UUID id output, got nil")
		}
	})

	t.Run("rejects nil receiver", func(t *testing.T) {
		var gen *UUIDv7QuoteIDGenerator
		_, err := gen.NewID()
		if err == nil {
			t.Fatal("expected error for nil generator receiver, got nil")
		}
	})
}

