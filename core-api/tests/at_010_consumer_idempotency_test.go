package tests

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockIdempotencyStore provides thread-safe deduplication storage simulating
// the PostgreSQL processed_events table (SEC-011 / FR-EVT-005).
type mockIdempotencyStore struct {
	mu        sync.Mutex
	processed map[string]time.Time
}

func newMockIdempotencyStore() *mockIdempotencyStore {
	return &mockIdempotencyStore{
		processed: make(map[string]time.Time),
	}
}

// TryAcquire attempts to mark (consumerName + eventID) as processed.
// Returns true if acquired (first time), false if already processed (duplicate).
func (s *mockIdempotencyStore) TryAcquire(ctx context.Context, consumerName, eventID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := consumerName + ":" + eventID
	if _, exists := s.processed[key]; exists {
		return false
	}
	s.processed[key] = time.Now().UTC()
	return true
}

func (s *mockIdempotencyStore) IsProcessed(consumerName, eventID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.processed[consumerName+":"+eventID]
	return exists
}

// mockConsumerHandler simulates a background event worker consuming from RabbitMQ.
type mockConsumerHandler struct {
	store          *mockIdempotencyStore
	consumerName   string
	dispatchesDone int32
	emailsSent     int32
}

func newMockConsumerHandler(store *mockIdempotencyStore, name string) *mockConsumerHandler {
	return &mockConsumerHandler{
		store:        store,
		consumerName: name,
	}
}

// HandleEvent processes an event with strict idempotency gating.
// Returns whether the event was processed (true) or discarded as a duplicate (false).
func (h *mockConsumerHandler) HandleEvent(ctx context.Context, eventID, eventType, parcelID string) bool {
	// Step 1: Idempotency gate check
	if !h.store.TryAcquire(ctx, h.consumerName, eventID) {
		// Already processed: safely ACK and discard without re-running side effects!
		return false
	}

	// Step 2: Execute side effects (e.g., dispatch merchant webhook, send customer email)
	atomic.AddInt32(&h.dispatchesDone, 1)
	if eventType == "parcel.delivered.v1" {
		atomic.AddInt32(&h.emailsSent, 1)
	}
	return true
}

func TestAcceptance_AT010_ConsumerIdempotency(t *testing.T) {
	ctx := context.Background()
	store := newMockIdempotencyStore()
	handler := newMockConsumerHandler(store, "webhook_dispatcher")

	const eventID = "0192e2b3-7c8a-7890-a123-000000000099"
	const eventType = "parcel.delivered.v1"
	const parcelID = "018f4a7c-2001-7000-8000-000000000001"

	t.Run("1. Initial Delivery is Processed and Recorded in Idempotency Ledger", func(t *testing.T) {
		processed := handler.HandleEvent(ctx, eventID, eventType, parcelID)
		require.True(t, processed, "initial message delivery must be processed")

		assert.Equal(t, int32(1), atomic.LoadInt32(&handler.dispatchesDone))
		assert.Equal(t, int32(1), atomic.LoadInt32(&handler.emailsSent))
		assert.True(t, store.IsProcessed("webhook_dispatcher", eventID))
	})

	t.Run("2. Duplicate Delivery (Replay) is Safely Discarded with Zero Side Effects", func(t *testing.T) {
		// Replaying the exact same event envelope with identical event_id
		processed := handler.HandleEvent(ctx, eventID, eventType, parcelID)
		assert.False(t, processed, "duplicate delivery must be detected and not re-processed")

		// INVARIANT: Side effects count must remain strictly 1 (no duplicate webhooks or emails!)
		assert.Equal(t, int32(1), atomic.LoadInt32(&handler.dispatchesDone), "duplicate event must not cause duplicate dispatch")
		assert.Equal(t, int32(1), atomic.LoadInt32(&handler.emailsSent), "duplicate event must not cause duplicate customer email")
	})

	t.Run("3. Distinct Event ID is Processed Independently", func(t *testing.T) {
		const distinctEventID = "0192e2b3-7c8a-7890-a123-000000000100"
		processed := handler.HandleEvent(ctx, distinctEventID, eventType, parcelID)
		require.True(t, processed, "distinct event ID must be processed")

		assert.Equal(t, int32(2), atomic.LoadInt32(&handler.dispatchesDone))
		assert.Equal(t, int32(2), atomic.LoadInt32(&handler.emailsSent))
	})

	t.Run("4. Concurrent Delivery Race is Serialized Without Duplication", func(t *testing.T) {
		const raceEventID = "0192e2b3-7c8a-7890-a123-000000000200"
		var successCount int32
		var duplicateCount int32

		var wg sync.WaitGroup
		concurrency := 10

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if handler.HandleEvent(ctx, raceEventID, eventType, parcelID) {
					atomic.AddInt32(&successCount, 1)
				} else {
					atomic.AddInt32(&duplicateCount, 1)
				}
			}()
		}
		wg.Wait()

		// INVARIANT: Exactly 1 goroutine succeeds; exactly 9 are safely dropped as duplicates!
		assert.Equal(t, int32(1), successCount, "exactly one concurrent delivery must process")
		assert.Equal(t, int32(9), duplicateCount, "nine concurrent deliveries must be deduplicated")
	})

	t.Run("5. Distinct Consumers Maintain Independent Idempotency Scopes", func(t *testing.T) {
		// A different consumer worker (e.g. tracking_projection_worker) must be able to process the same event
		trackingHandler := newMockConsumerHandler(store, "tracking_projection_worker")
		processed := trackingHandler.HandleEvent(ctx, eventID, eventType, parcelID)

		assert.True(t, processed, "independent consumer queue must process its own copy of the event")
		assert.Equal(t, int32(1), atomic.LoadInt32(&trackingHandler.dispatchesDone))
	})
}
