package events

import (
	"context"
	"errors"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/swiftdrop/tracking/internal/domain"
)

type mockAMQPDeliveryImpl struct {
	body        []byte
	deliveryTag uint64
	headers     amqp.Table
	redelivered bool
	acked       bool
	nacked      bool
	requeued    bool
}

func (m *mockAMQPDeliveryImpl) GetBody() []byte         { return m.body }
func (m *mockAMQPDeliveryImpl) GetDeliveryTag() uint64  { return m.deliveryTag }
func (m *mockAMQPDeliveryImpl) GetHeaders() amqp.Table  { return m.headers }
func (m *mockAMQPDeliveryImpl) IsRedelivered() bool     { return m.redelivered }
func (m *mockAMQPDeliveryImpl) Ack(multiple bool) error { m.acked = true; return nil }
func (m *mockAMQPDeliveryImpl) Nack(multiple, requeue bool) error {
	m.nacked = true
	m.requeued = requeue
	return nil
}

type mockPublisherImpl struct {
	published []RedisTrackingEvent
	err       error
}

func (m *mockPublisherImpl) Publish(ctx context.Context, event RedisTrackingEvent) error {
	if m.err != nil {
		return m.err
	}
	m.published = append(m.published, event)
	return nil
}

type mockRepoForConsumer struct {
	findParcelFn  func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, error)
	findHistoryFn func(ctx context.Context, historyID string) (*domain.MilestoneRecord, error)
}

func (m *mockRepoForConsumer) FindParcelByTrackingNumber(ctx context.Context, tn string) (*domain.ParcelTrackingInfo, error) {
	if m.findParcelFn != nil {
		return m.findParcelFn(ctx, tn)
	}
	return nil, nil
}
func (m *mockRepoForConsumer) FindStatusHistoryByParcelID(ctx context.Context, parcelID string) ([]domain.MilestoneRecord, error) {
	return nil, nil
}
func (m *mockRepoForConsumer) FindStatusHistoryByID(ctx context.Context, historyID string) (*domain.MilestoneRecord, error) {
	if m.findHistoryFn != nil {
		return m.findHistoryFn(ctx, historyID)
	}
	return nil, nil
}
func (m *mockRepoForConsumer) FindTrackingSummary(ctx context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
	return nil, nil, nil
}
func (m *mockRepoForConsumer) CheckReadiness(ctx context.Context) error { return nil }
func (m *mockRepoForConsumer) Ping(ctx context.Context) error           { return nil }
func (m *mockRepoForConsumer) Close() error                             { return nil }

func TestRabbitMQConsumer_ValidParcelStatusChangedEvent(t *testing.T) {
	rawJSON := `{
		"event_id": "0192e2b3-7c8a-7890-a123-000000000001",
		"event_type": "parcel.in_transit.v1",
		"aggregate_type": "parcel",
		"aggregate_id": "0192e2b3-7c8a-7890-a123-000000000010",
		"aggregate_version": 3,
		"schema_version": "1.0.0",
		"occurred_at": "2026-11-10T12:00:00Z",
		"correlation_id": "0192e2b3-7c8a-7890-a123-000000000099",
		"producer": "core-api.parcels",
		"payload": {
			"parcel_id": "0192e2b3-7c8a-7890-a123-000000000010",
			"tracking_number": "SD-20261110-000001",
			"previous_status": "PICKED_UP",
			"new_status": "IN_TRANSIT",
			"status_history_id": "0192e2b3-7c8a-7890-a123-000000000042",
			"delivery_attempts": 0,
			"occurred_at": "2026-11-10T12:00:00Z"
		}
	}`

	mockPub := &mockPublisherImpl{}
	mockRepo := &mockRepoForConsumer{
		findHistoryFn: func(ctx context.Context, historyID string) (*domain.MilestoneRecord, error) {
			if historyID == "0192e2b3-7c8a-7890-a123-000000000042" {
				return &domain.MilestoneRecord{
					ID:        "0192e2b3-7c8a-7890-a123-000000000042",
					ParcelID:  "0192e2b3-7c8a-7890-a123-000000000010",
					ToStatus:  "IN_TRANSIT",
					Timestamp: time.Now().UTC(),
				}, nil
			}
			return nil, domain.ErrTrackingNotFound
		},
	}

	consumer := NewRabbitMQConsumer("amqp://localhost", "q.tracking.updates", 10, time.Second, mockRepo, mockPub, nil)
	d := &mockAMQPDeliveryImpl{body: []byte(rawJSON), deliveryTag: 1}

	consumer.ProcessDelivery(context.Background(), d)

	if !d.acked {
		t.Errorf("expected delivery to be ACKed, but it was not")
	}
	if d.nacked {
		t.Errorf("expected delivery not to be NACKed")
	}

	if len(mockPub.published) != 1 {
		t.Fatalf("expected 1 published event to Redis, got %d", len(mockPub.published))
	}

	published := mockPub.published[0]
	if published.TrackingNumber != "SD-20261110-000001" {
		t.Errorf("expected tracking number SD-20261110-000001, got %s", published.TrackingNumber)
	}
	if published.Status != "IN_TRANSIT" {
		t.Errorf("expected status IN_TRANSIT, got %s", published.Status)
	}
	if published.StatusHistoryID != "0192e2b3-7c8a-7890-a123-000000000042" {
		t.Errorf("expected status_history_id 0192e2b3-7c8a-7890-a123-000000000042, got %s", published.StatusHistoryID)
	}
	if published.EventID != "0192e2b3-7c8a-7890-a123-000000000001" {
		t.Errorf("expected independent event_id preserved, got %s", published.EventID)
	}
}

func TestRabbitMQConsumer_ValidParcelBookedEvent(t *testing.T) {
	rawJSON := `{
		"event_id": "0192e2b3-7c8a-7890-a123-000000000002",
		"event_type": "parcel.booked.v1",
		"aggregate_type": "parcel",
		"aggregate_id": "0192e2b3-7c8a-7890-a123-000000000020",
		"aggregate_version": 1,
		"schema_version": "1.0.0",
		"occurred_at": "2026-11-10T10:00:00Z",
		"correlation_id": "0192e2b3-7c8a-7890-a123-000000000020",
		"producer": "core-api.parcels",
		"payload": {
			"parcel_id": "0192e2b3-7c8a-7890-a123-000000000020",
			"tracking_number": "SD-20261110-000002",
			"status": "CREATED",
			"created_at": "2026-11-10T10:00:00Z"
		}
	}`

	mockPub := &mockPublisherImpl{}
	tn, _ := domain.NewTrackingNumber("SD-20261110-000002")
	mockRepo := &mockRepoForConsumer{
		findParcelFn: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, error) {
			if trackingNumber == "SD-20261110-000002" {
				return &domain.ParcelTrackingInfo{
					ID:             "0192e2b3-7c8a-7890-a123-000000000020",
					TrackingNumber: tn,
					Status:         domain.StatusCreated,
				}, nil
			}
			return nil, domain.ErrTrackingNotFound
		},
	}

	consumer := NewRabbitMQConsumer("amqp://localhost", "q.tracking.updates", 10, time.Second, mockRepo, mockPub, nil)
	d := &mockAMQPDeliveryImpl{body: []byte(rawJSON), deliveryTag: 2}

	consumer.ProcessDelivery(context.Background(), d)

	if !d.acked {
		t.Errorf("expected booked event to be ACKed")
	}
	if len(mockPub.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(mockPub.published))
	}
	if mockPub.published[0].Status != "CREATED" {
		t.Errorf("expected status CREATED, got %s", mockPub.published[0].Status)
	}
}

func TestRabbitMQConsumer_DuplicateDeliveryIdempotency(t *testing.T) {
	rawJSON := `{
		"event_id": "0192e2b3-7c8a-7890-a123-000000000003",
		"event_type": "parcel.delivered.v1",
		"aggregate_type": "parcel",
		"aggregate_id": "0192e2b3-7c8a-7890-a123-000000000030",
		"aggregate_version": 5,
		"schema_version": "1.0.0",
		"occurred_at": "2026-11-10T14:00:00Z",
		"correlation_id": "corr-1",
		"producer": "core-api.parcels",
		"payload": {
			"parcel_id": "0192e2b3-7c8a-7890-a123-000000000030",
			"tracking_number": "SD-20261110-000003",
			"new_status": "DELIVERED",
			"status_history_id": "0192e2b3-7c8a-7890-a123-000000000033",
			"occurred_at": "2026-11-10T14:00:00Z"
		}
	}`

	mockPub := &mockPublisherImpl{}
	mockRepo := &mockRepoForConsumer{
		findHistoryFn: func(ctx context.Context, historyID string) (*domain.MilestoneRecord, error) {
			return &domain.MilestoneRecord{
				ID:       "0192e2b3-7c8a-7890-a123-000000000033",
				ParcelID: "0192e2b3-7c8a-7890-a123-000000000030",
				ToStatus: "DELIVERED",
			}, nil
		},
	}

	consumer := NewRabbitMQConsumer("amqp://localhost", "q.tracking.updates", 10, time.Second, mockRepo, mockPub, nil)

	// First delivery: should process and publish
	d1 := &mockAMQPDeliveryImpl{body: []byte(rawJSON), deliveryTag: 10}
	consumer.ProcessDelivery(context.Background(), d1)
	if !d1.acked {
		t.Errorf("expected first delivery to be ACKed")
	}
	if len(mockPub.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(mockPub.published))
	}

	// Second delivery (duplicate from RabbitMQ redelivery):
	// Should ACK delivery but NOT republish to Redis
	d2 := &mockAMQPDeliveryImpl{body: []byte(rawJSON), deliveryTag: 11}
	consumer.ProcessDelivery(context.Background(), d2)
	if !d2.acked {
		t.Errorf("expected duplicate delivery to be ACKed")
	}
	if len(mockPub.published) != 1 {
		t.Errorf("expected still only 1 published event after duplicate, got %d", len(mockPub.published))
	}
}

func TestRabbitMQConsumer_MalformedAndCorruptPayloadsToDLQ(t *testing.T) {
	mockPub := &mockPublisherImpl{}
	consumer := NewRabbitMQConsumer("amqp://localhost", "q.tracking.updates", 10, time.Second, nil, mockPub, nil)

	testCases := []struct {
		name string
		json string
	}{
		{
			name: "malformed JSON",
			json: "invalid-json-content",
		},
		{
			name: "invalid event_id UUID",
			json: `{"event_id":"not-a-uuid","event_type":"parcel.in_transit.v1","schema_version":"1.0.0","aggregate_type":"parcel","aggregate_id":"0192e2b3-7c8a-7890-a123-000000000010","payload":{"status_history_id":"0192e2b3-7c8a-7890-a123-000000000011"}}`,
		},
		{
			name: "unsupported schema_version",
			json: `{"event_id":"0192e2b3-7c8a-7890-a123-000000000001","event_type":"parcel.in_transit.v1","schema_version":"2.0.0","aggregate_type":"parcel","aggregate_id":"0192e2b3-7c8a-7890-a123-000000000010","payload":{}}`,
		},
		{
			name: "invalid aggregate_id UUID",
			json: `{"event_id":"0192e2b3-7c8a-7890-a123-000000000001","event_type":"parcel.in_transit.v1","schema_version":"1.0.0","aggregate_type":"parcel","aggregate_id":"not-uuid","payload":{}}`,
		},
		{
			name: "missing status_history_id in transition event",
			json: `{
				"event_id": "0192e2b3-7c8a-7890-a123-000000000001",
				"event_type": "parcel.in_transit.v1",
				"schema_version": "1.0.0",
				"aggregate_type": "parcel",
				"aggregate_id": "0192e2b3-7c8a-7890-a123-000000000010",
				"payload": {
					"parcel_id": "0192e2b3-7c8a-7890-a123-000000000010",
					"tracking_number": "SD-20261110-000001",
					"new_status": "IN_TRANSIT"
				}
			}`,
		},
		{
			name: "invalid tracking_number format",
			json: `{
				"event_id": "0192e2b3-7c8a-7890-a123-000000000001",
				"event_type": "parcel.in_transit.v1",
				"schema_version": "1.0.0",
				"aggregate_type": "parcel",
				"aggregate_id": "0192e2b3-7c8a-7890-a123-000000000010",
				"payload": {
					"parcel_id": "0192e2b3-7c8a-7890-a123-000000000010",
					"tracking_number": "INVALID-FORMAT",
					"new_status": "IN_TRANSIT",
					"status_history_id": "0192e2b3-7c8a-7890-a123-000000000042"
				}
			}`,
		},
		{
			name: "invalid status enum",
			json: `{
				"event_id": "0192e2b3-7c8a-7890-a123-000000000001",
				"event_type": "parcel.in_transit.v1",
				"schema_version": "1.0.0",
				"aggregate_type": "parcel",
				"aggregate_id": "0192e2b3-7c8a-7890-a123-000000000010",
				"payload": {
					"parcel_id": "0192e2b3-7c8a-7890-a123-000000000010",
					"tracking_number": "SD-20261110-000001",
					"new_status": "NOT_A_VALID_STATUS",
					"status_history_id": "0192e2b3-7c8a-7890-a123-000000000042"
				}
			}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := &mockAMQPDeliveryImpl{body: []byte(tc.json), deliveryTag: 99}
			consumer.ProcessDelivery(context.Background(), d)

			if !d.nacked {
				t.Fatalf("expected delivery to be NACKed to DLQ, but it was not")
			}
			if d.requeued {
				t.Errorf("expected requeue=false (routing to DLQ), but requeued was true")
			}
			if d.acked {
				t.Errorf("expected not to be ACKed")
			}
		})
	}
}

func TestRabbitMQConsumer_UnsupportedEventsSafelySkipped(t *testing.T) {
	mockPub := &mockPublisherImpl{}
	consumer := NewRabbitMQConsumer("amqp://localhost", "q.tracking.updates", 10, time.Second, nil, mockPub, nil)

	// Non-tracking event routed to q.tracking.updates by topic binding (e.g. courier.unassigned.v1)
	rawJSON := `{
		"event_id": "0192e2b3-7c8a-7890-a123-000000000005",
		"event_type": "courier.unassigned.v1",
		"aggregate_type": "courier",
		"aggregate_id": "0192e2b3-7c8a-7890-a123-000000000050",
		"aggregate_version": 1,
		"schema_version": "1.0.0",
		"occurred_at": "2026-11-10T15:00:00Z",
		"correlation_id": "corr-5",
		"producer": "core-api.dispatch",
		"payload": {"courier_id":"0192e2b3-7c8a-7890-a123-000000000050"}
	}`

	d := &mockAMQPDeliveryImpl{body: []byte(rawJSON), deliveryTag: 5}
	consumer.ProcessDelivery(context.Background(), d)

	// Should be ACKed (acknowledged cleanly) rather than cluttering DLQ
	if !d.acked {
		t.Errorf("expected unsupported event to be cleanly ACKed")
	}
	if d.nacked {
		t.Errorf("expected unsupported event not to be NACKed")
	}
	if len(mockPub.published) != 0 {
		t.Errorf("expected 0 published events, got %d", len(mockPub.published))
	}
}

func TestRabbitMQConsumer_TransientErrorsRequeued(t *testing.T) {
	rawJSON := `{
		"event_id": "0192e2b3-7c8a-7890-a123-000000000006",
		"event_type": "parcel.delivered.v1",
		"aggregate_type": "parcel",
		"aggregate_id": "0192e2b3-7c8a-7890-a123-000000000060",
		"aggregate_version": 4,
		"schema_version": "1.0.0",
		"occurred_at": "2026-11-10T15:00:00Z",
		"correlation_id": "corr-6",
		"producer": "core-api.parcels",
		"payload": {
			"parcel_id": "0192e2b3-7c8a-7890-a123-000000000060",
			"tracking_number": "SD-20261110-000006",
			"new_status": "DELIVERED",
			"status_history_id": "0192e2b3-7c8a-7890-a123-000000000066",
			"occurred_at": "2026-11-10T15:00:00Z"
		}
	}`

	t.Run("history record not yet replicated: requeued for retry", func(t *testing.T) {
		mockPub := &mockPublisherImpl{}
		mockRepo := &mockRepoForConsumer{
			findHistoryFn: func(ctx context.Context, historyID string) (*domain.MilestoneRecord, error) {
				return nil, domain.ErrTrackingNotFound // Replication delay
			},
		}

		consumer := NewRabbitMQConsumer("amqp://localhost", "q.tracking.updates", 10, time.Second, mockRepo, mockPub, nil)
		d := &mockAMQPDeliveryImpl{body: []byte(rawJSON), deliveryTag: 6}
		consumer.ProcessDelivery(context.Background(), d)

		if !d.nacked {
			t.Fatalf("expected delivery to be NACKed")
		}
		if !d.requeued {
			t.Errorf("expected requeue=true for transient replication retry")
		}
	})

	t.Run("redis pubsub failure: requeued for retry", func(t *testing.T) {
		mockPub := &mockPublisherImpl{
			err: errors.New("redis connection refused"),
		}
		mockRepo := &mockRepoForConsumer{
			findHistoryFn: func(ctx context.Context, historyID string) (*domain.MilestoneRecord, error) {
				return &domain.MilestoneRecord{
					ID:       "0192e2b3-7c8a-7890-a123-000000000066",
					ParcelID: "0192e2b3-7c8a-7890-a123-000000000060",
					ToStatus: "DELIVERED",
				}, nil
			},
		}

		consumer := NewRabbitMQConsumer("amqp://localhost", "q.tracking.updates", 10, time.Second, mockRepo, mockPub, nil)
		d := &mockAMQPDeliveryImpl{body: []byte(rawJSON), deliveryTag: 7}
		consumer.ProcessDelivery(context.Background(), d)

		if !d.nacked {
			t.Fatalf("expected delivery to be NACKed on Redis failure")
		}
		if !d.requeued {
			t.Errorf("expected requeue=true for Redis failure retry")
		}
	})
}

func TestRabbitMQConsumer_BoundedTransientRetriesDeadLettersToDLQ(t *testing.T) {
	rawJSON := `{
		"event_id": "0192e2b3-7c8a-7890-a123-000000000099",
		"event_type": "parcel.delivered.v1",
		"aggregate_type": "parcel",
		"aggregate_id": "0192e2b3-7c8a-7890-a123-000000000090",
		"aggregate_version": 5,
		"schema_version": "1.0.0",
		"occurred_at": "2026-11-10T15:00:00Z",
		"correlation_id": "corr-bounded-retry",
		"producer": "core-api.parcels",
		"payload": {
			"parcel_id": "0192e2b3-7c8a-7890-a123-000000000090",
			"tracking_number": "SD-20261110-000099",
			"new_status": "DELIVERED",
			"status_history_id": "0192e2b3-7c8a-7890-a123-000000000098",
			"occurred_at": "2026-11-10T15:00:00Z"
		}
	}`

	mockPub := &mockPublisherImpl{}
	// Persistent replication delay or non-existent status_history_id
	mockRepo := &mockRepoForConsumer{
		findHistoryFn: func(ctx context.Context, historyID string) (*domain.MilestoneRecord, error) {
			return nil, domain.ErrTrackingNotFound
		},
	}

	consumer := NewRabbitMQConsumer("amqp://localhost", "q.tracking.updates", 10, time.Second, mockRepo, mockPub, nil)

	// Attempts 1, 2, 3: requeue=true
	for i := 1; i <= 3; i++ {
		d := &mockAMQPDeliveryImpl{body: []byte(rawJSON), deliveryTag: uint64(i)}
		consumer.ProcessDelivery(context.Background(), d)
		if !d.nacked {
			t.Fatalf("attempt %d: expected message NACKed", i)
		}
		if !d.requeued {
			t.Fatalf("attempt %d: expected requeue=true for bounded transient retry", i)
		}
	}

	// Attempt 4 (exceeds maxTransientRetries=3): MUST dead-letter with requeue=false
	d4 := &mockAMQPDeliveryImpl{body: []byte(rawJSON), deliveryTag: 4}
	consumer.ProcessDelivery(context.Background(), d4)

	if !d4.nacked {
		t.Fatalf("attempt 4: expected message NACKed")
	}
	if d4.requeued {
		t.Fatalf("attempt 4: expected requeue=false (routed to DLQ), but was requeued (uncontrolled redelivery loop!)")
	}
}

func TestRabbitMQConsumer_BrokerDeliveryCountExceeded_DeadLettersToDLQ(t *testing.T) {
	rawJSON := `{
		"event_id": "0192e2b3-7c8a-7890-a123-000000000077",
		"event_type": "parcel.delivered.v1",
		"aggregate_type": "parcel",
		"aggregate_id": "0192e2b3-7c8a-7890-a123-000000000070",
		"aggregate_version": 5,
		"schema_version": "1.0.0",
		"occurred_at": "2026-11-10T15:00:00Z",
		"correlation_id": "corr-quorum-delivery-count",
		"producer": "core-api.parcels",
		"payload": {
			"parcel_id": "0192e2b3-7c8a-7890-a123-000000000070",
			"tracking_number": "SD-20261110-000077",
			"new_status": "DELIVERED",
			"status_history_id": "0192e2b3-7c8a-7890-a123-000000000078",
			"occurred_at": "2026-11-10T15:00:00Z"
		}
	}`

	mockPub := &mockPublisherImpl{}
	mockRepo := &mockRepoForConsumer{
		findHistoryFn: func(ctx context.Context, historyID string) (*domain.MilestoneRecord, error) {
			return nil, domain.ErrTrackingNotFound
		},
	}

	consumer := NewRabbitMQConsumer("amqp://localhost", "q.tracking.updates", 10, time.Second, mockRepo, mockPub, nil)

	// Simulate Quorum Queue header x-delivery-count = 3 (reached max retries across replicas)
	d := &mockAMQPDeliveryImpl{
		body:        []byte(rawJSON),
		deliveryTag: 10,
		headers: amqp.Table{
			"x-delivery-count": int64(3),
		},
		redelivered: true,
	}

	consumer.ProcessDelivery(context.Background(), d)

	if !d.nacked {
		t.Fatalf("expected message to be NACKed")
	}
	if d.requeued {
		t.Fatalf("expected requeue=false (routed to DLQ via broker x-delivery-count metadata), got requeue=true")
	}
}

func TestRabbitMQConsumer_BrokerDeathCountExceeded_DeadLettersToDLQ(t *testing.T) {
	rawJSON := `{
		"event_id": "0192e2b3-7c8a-7890-a123-000000000088",
		"event_type": "parcel.delivered.v1",
		"aggregate_type": "parcel",
		"aggregate_id": "0192e2b3-7c8a-7890-a123-000000000080",
		"aggregate_version": 5,
		"schema_version": "1.0.0",
		"occurred_at": "2026-11-10T15:00:00Z",
		"correlation_id": "corr-dlx-death-count",
		"producer": "core-api.parcels",
		"payload": {
			"parcel_id": "0192e2b3-7c8a-7890-a123-000000000080",
			"tracking_number": "SD-20261110-000088",
			"new_status": "DELIVERED",
			"status_history_id": "0192e2b3-7c8a-7890-a123-000000000089",
			"occurred_at": "2026-11-10T15:00:00Z"
		}
	}`

	mockPub := &mockPublisherImpl{}
	mockRepo := &mockRepoForConsumer{
		findHistoryFn: func(ctx context.Context, historyID string) (*domain.MilestoneRecord, error) {
			return nil, domain.ErrTrackingNotFound
		},
	}

	consumer := NewRabbitMQConsumer("amqp://localhost", "q.tracking.updates", 10, time.Second, mockRepo, mockPub, nil)

	// Simulate x-death header from DLX retry loop
	d := &mockAMQPDeliveryImpl{
		body:        []byte(rawJSON),
		deliveryTag: 20,
		headers: amqp.Table{
			"x-death": []interface{}{
				amqp.Table{
					"count": int64(4),
					"queue": "q.tracking.updates",
				},
			},
		},
		redelivered: true,
	}

	consumer.ProcessDelivery(context.Background(), d)

	if !d.nacked {
		t.Fatalf("expected message to be NACKed")
	}
	if d.requeued {
		t.Fatalf("expected requeue=false (routed to DLQ via broker x-death count), got requeue=true")
	}
}
