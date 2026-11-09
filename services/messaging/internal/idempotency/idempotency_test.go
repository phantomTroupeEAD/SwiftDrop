package idempotency

import (
	"context"
	"testing"
)

func TestMemoryStore_Idempotency(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	consumer := "notifier"
	eventID := "0192e2b3-7c8a-7890-a123-000000000001"

	// Initially unseen
	seen, err := store.Seen(ctx, consumer, eventID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seen {
		t.Fatalf("expected event to be unseen initially")
	}

	// Mark done
	if err := store.MarkDone(ctx, nil, consumer, eventID); err != nil {
		t.Fatalf("failed to mark done: %v", err)
	}

	// Now seen
	seen, err = store.Seen(ctx, consumer, eventID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !seen {
		t.Fatalf("expected event to be marked as seen")
	}

	// Different consumer for same event is not seen
	seenOther, err := store.Seen(ctx, "webhook:ep-1", eventID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenOther {
		t.Fatalf("expected other consumer to be unseen")
	}
}
