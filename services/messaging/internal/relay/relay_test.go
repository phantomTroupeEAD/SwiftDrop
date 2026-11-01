package relay

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/swiftdrop/messaging/internal/config"
)

type mockOutboxRepo struct {
	mu           sync.Mutex
	records      map[string]*mockRecord
	pendingCount int64
	oldestAge    float64
}

type mockRecord struct {
	EventRecord
	Status        string
	LastError     string
	NextAttemptAt time.Time
	PublishedAt   time.Time
}

func newMockOutboxRepo() *mockOutboxRepo {
	return &mockOutboxRepo{
		records: make(map[string]*mockRecord),
	}
}

func (m *mockOutboxRepo) addRecord(rec EventRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[rec.ID] = &mockRecord{
		EventRecord: rec,
		Status:      "PENDING",
	}
}

func (m *mockOutboxRepo) ClaimBatch(ctx context.Context, batchSize int, leaseDuration time.Duration) ([]EventRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []EventRecord
	now := time.Now()

	for _, rec := range m.records {
		if rec.Status == "PENDING" && (rec.NextAttemptAt.IsZero() || rec.NextAttemptAt.Before(now)) {
			result = append(result, rec.EventRecord)
			rec.NextAttemptAt = now.Add(leaseDuration)
			if len(result) >= batchSize {
				break
			}
		}
	}
	return result, nil
}

func (m *mockOutboxRepo) MarkPublished(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.records[id]
	if !ok {
		return errors.New("not found")
	}
	rec.Status = "PUBLISHED"
	rec.PublishedAt = time.Now()
	rec.LastError = ""
	return nil
}

func (m *mockOutboxRepo) RecordFailure(ctx context.Context, id string, errStr string, nextAttemptAt time.Time, finalFailure bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.records[id]
	if !ok {
		return errors.New("not found")
	}
	rec.AttemptCount++
	rec.LastError = errStr
	rec.NextAttemptAt = nextAttemptAt
	if finalFailure {
		rec.Status = "FAILED"
	} else {
		rec.Status = "PENDING"
	}
	return nil
}

func (m *mockOutboxRepo) GetOutboxStats(ctx context.Context) (int64, float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pendingCount, m.oldestAge, nil
}

type mockPublisher struct {
	mu        sync.Mutex
	published []amqp.Publishing
	failErr   error
}

func (p *mockPublisher) Publish(ctx context.Context, exchange, routingKey string, msg amqp.Publishing) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.failErr != nil {
		return p.failErr
	}
	p.published = append(p.published, msg)
	return nil
}

func TestRelay_PollAndPublish_Success(t *testing.T) {
	repo := newMockOutboxRepo()
	pub := &mockPublisher{}

	cfg := &config.Config{
		OutboxBatchSize:    10,
		OutboxLeaseSeconds: 60,
	}

	relay := NewRelay(repo, pub, cfg, nil)
	relay.SetRNG(rand.New(rand.NewSource(1)))

	testRecord := EventRecord{
		ID:               "rec-1",
		EventID:          "0192e2b3-7c8a-7890-a123-456789abcdef",
		EventType:        "parcel.booked.v1",
		AggregateType:    "PARCEL",
		AggregateID:      "0192e2b3-7c8a-7890-a123-000000000001",
		AggregateVersion: 1,
		SchemaVersion:    "1.0.0",
		OccurredAt:       time.Now(),
		CorrelationID:    "corr-123",
		Producer:         "core-api",
		Payload:          json.RawMessage(`{"parcel_id":"0192e2b3-7c8a-7890-a123-000000000001"}`),
		AttemptCount:     0,
	}
	repo.addRecord(testRecord)

	count, err := relay.PollAndPublish(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 record processed, got %d", count)
	}

	// Verify publisher received message with correct headers & properties
	pub.mu.Lock()
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(pub.published))
	}
	msg := pub.published[0]
	pub.mu.Unlock()

	if msg.MessageId != testRecord.EventID {
		t.Errorf("expected MessageId %s, got %s", testRecord.EventID, msg.MessageId)
	}
	if msg.DeliveryMode != amqp.Persistent {
		t.Errorf("expected Persistent delivery mode, got %d", msg.DeliveryMode)
	}
	if msg.Headers["event_id"] != testRecord.EventID {
		t.Errorf("expected header event_id %s, got %v", testRecord.EventID, msg.Headers["event_id"])
	}
	if msg.Headers["correlation_id"] != testRecord.CorrelationID {
		t.Errorf("expected header correlation_id %s, got %v", testRecord.CorrelationID, msg.Headers["correlation_id"])
	}

	// Verify repo updated to PUBLISHED
	repo.mu.Lock()
	saved := repo.records["rec-1"]
	repo.mu.Unlock()

	if saved.Status != "PUBLISHED" {
		t.Errorf("expected status PUBLISHED, got %s", saved.Status)
	}
}

func TestRelay_PollAndPublish_FailureAndBackoff(t *testing.T) {
	repo := newMockOutboxRepo()
	pub := &mockPublisher{
		failErr: errors.New("amqp broker: connection refused"),
	}

	cfg := &config.Config{
		OutboxBatchSize:    10,
		OutboxLeaseSeconds: 60,
	}

	relay := NewRelay(repo, pub, cfg, nil)
	relay.SetRNG(rand.New(rand.NewSource(42)))

	testRecord := EventRecord{
		ID:           "rec-fail",
		EventID:      "0192e2b3-7c8a-7890-a123-456789abcdef",
		EventType:    "parcel.booked.v1",
		AttemptCount: 0,
		Payload:      json.RawMessage(`{}`),
	}
	repo.addRecord(testRecord)

	count, err := relay.PollAndPublish(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 record processed, got %d", count)
	}

	repo.mu.Lock()
	saved := repo.records["rec-fail"]
	repo.mu.Unlock()

	if saved.Status != "PENDING" {
		t.Errorf("expected status PENDING after 1st failure, got %s", saved.Status)
	}
	if saved.AttemptCount != 1 {
		t.Errorf("expected attempt_count 1, got %d", saved.AttemptCount)
	}
	if saved.LastError == "" {
		t.Errorf("expected non-empty last_error")
	}
	if saved.NextAttemptAt.Before(time.Now()) {
		t.Errorf("expected next_attempt_at in the future, got %v", saved.NextAttemptAt)
	}
}

func TestRelay_PollAndPublish_MaxRetriesReached(t *testing.T) {
	repo := newMockOutboxRepo()
	pub := &mockPublisher{
		failErr: errors.New("nacked"),
	}

	cfg := &config.Config{
		OutboxBatchSize:    10,
		OutboxLeaseSeconds: 60,
	}

	relay := NewRelay(repo, pub, cfg, nil)
	relay.SetRNG(rand.New(rand.NewSource(42)))

	testRecord := EventRecord{
		ID:           "rec-max",
		EventID:      "0192e2b3-7c8a-7890-a123-456789abcdef",
		EventType:    "parcel.booked.v1",
		AttemptCount: 4, // 5th failure will happen now
		Payload:      json.RawMessage(`{}`),
	}
	repo.addRecord(testRecord)

	count, err := relay.PollAndPublish(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 record processed, got %d", count)
	}

	repo.mu.Lock()
	saved := repo.records["rec-max"]
	repo.mu.Unlock()

	if saved.Status != "FAILED" {
		t.Errorf("expected status FAILED after 5th failure, got %s", saved.Status)
	}
	if saved.AttemptCount != 5 {
		t.Errorf("expected attempt_count 5, got %d", saved.AttemptCount)
	}
}
