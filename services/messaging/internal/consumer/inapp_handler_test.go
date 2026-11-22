package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/swiftdrop/messaging/internal/idempotency"
	"github.com/swiftdrop/messaging/internal/relay"
)

func TestInAppNotificationHandler_ParcelBooked(t *testing.T) {
	repo := newMockDeliveryRepo()
	idemp := idempotency.NewMemoryStore()
	handler := NewInAppNotificationHandler(repo, idemp, nil)

	senderID := "user-cust-123"
	payload, _ := json.Marshal(InAppNotificationPayload{
		ParcelID:       "parcel-abc-001",
		TrackingNumber: "SD-20261120-ABC123",
		SenderID:       &senderID,
		Status:         "CREATED",
	})

	env := &relay.OutboxEnvelope{
		EventID:       "0192e2b3-7c8a-7890-a123-000000000001",
		EventType:     "parcel.booked.v1",
		AggregateType: "parcel",
		AggregateID:   "parcel-abc-001",
		Payload:       payload,
		OccurredAt:    time.Now().Format(time.RFC3339),
	}

	err := handler(context.Background(), amqp.Delivery{}, env)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if len(repo.inAppNotifications) != 1 {
		t.Fatalf("expected 1 notification inserted, got %d", len(repo.inAppNotifications))
	}

	notif := repo.inAppNotifications[0]
	if notif.UserID != senderID {
		t.Errorf("expected user_id %s, got %s", senderID, notif.UserID)
	}
	if notif.Title != "Parcel Booked" {
		t.Errorf("expected Title 'Parcel Booked', got %s", notif.Title)
	}
	if notif.EventType != "parcel.booked.v1" {
		t.Errorf("expected event_type parcel.booked.v1, got %s", notif.EventType)
	}

	// Idempotency: re-running same event_id should be skipped
	err = handler(context.Background(), amqp.Delivery{}, env)
	if err != nil {
		t.Fatalf("unexpected error on duplicate run: %v", err)
	}
	if len(repo.inAppNotifications) != 1 {
		t.Errorf("expected still 1 notification after duplicate, got %d", len(repo.inAppNotifications))
	}
}

func TestInAppNotificationHandler_ParcelDeliveryFailed_IncludesDispatcher(t *testing.T) {
	repo := newMockDeliveryRepo()
	idemp := idempotency.NewMemoryStore()
	handler := NewInAppNotificationHandler(repo, idemp, nil)

	senderID := "user-cust-456"
	payload, _ := json.Marshal(InAppNotificationPayload{
		ParcelID:       "parcel-fail-002",
		TrackingNumber: "SD-20261120-FAIL02",
		SenderID:       &senderID,
		Reason:         "Customer not at address",
	})

	env := &relay.OutboxEnvelope{
		EventID:       "0192e2b3-7c8a-7890-a123-000000000002",
		EventType:     "parcel.delivery_failed.v1",
		AggregateType: "parcel",
		AggregateID:   "parcel-fail-002",
		Payload:       payload,
		OccurredAt:    time.Now().Format(time.RFC3339),
	}

	err := handler(context.Background(), amqp.Delivery{}, env)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	// Should create 2 notifications: 1 for customer, 1 for role:DISPATCHER
	if len(repo.inAppNotifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(repo.inAppNotifications))
	}

	hasCustomer := false
	hasDispatcher := false
	for _, n := range repo.inAppNotifications {
		if n.UserID == senderID {
			hasCustomer = true
		}
		if n.UserID == "role:DISPATCHER" {
			hasDispatcher = true
		}
	}

	if !hasCustomer {
		t.Error("missing notification for customer")
	}
	if !hasDispatcher {
		t.Error("missing broadcast notification for role:DISPATCHER")
	}
}

func TestInAppNotificationHandler_UnsupportedEvent(t *testing.T) {
	repo := newMockDeliveryRepo()
	idemp := idempotency.NewMemoryStore()
	handler := NewInAppNotificationHandler(repo, idemp, nil)

	env := &relay.OutboxEnvelope{
		EventID:       "0192e2b3-7c8a-7890-a123-000000000003",
		EventType:     "system.unknown.v1",
		AggregateType: "system",
		AggregateID:   "sys-1",
		Payload:       []byte(`{}`),
		OccurredAt:    time.Now().Format(time.RFC3339),
	}

	err := handler(context.Background(), amqp.Delivery{}, env)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if len(repo.inAppNotifications) != 0 {
		t.Errorf("expected 0 notifications for unsupported event, got %d", len(repo.inAppNotifications))
	}
}
