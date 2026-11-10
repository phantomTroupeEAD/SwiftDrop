package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/swiftdrop/tracking/internal/dto"
)

type mockRedisClient struct {
	publishFunc func(ctx context.Context, channel string, message interface{}) *redis.IntCmd
	pingFunc    func(ctx context.Context) *redis.StatusCmd
	closeFunc   func() error
}

func (m *mockRedisClient) Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd {
	if m.publishFunc != nil {
		return m.publishFunc(ctx, channel, message)
	}
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(1)
	return cmd
}

func (m *mockRedisClient) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return nil
}

func (m *mockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	cmd := redis.NewStatusCmd(ctx)
	cmd.SetVal("PONG")
	return cmd
}

func (m *mockRedisClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestRedisPubSub_PublishAndLocalFanout(t *testing.T) {
	var publishedChannel string
	var publishedPayload []byte

	mockClient := &mockRedisClient{
		publishFunc: func(ctx context.Context, channel string, message interface{}) *redis.IntCmd {
			publishedChannel = channel
			if b, ok := message.([]byte); ok {
				publishedPayload = b
			}
			cmd := redis.NewIntCmd(ctx)
			cmd.SetVal(1)
			return cmd
		},
	}

	hub := NewEventHub()
	rps := NewRedisPubSub(mockClient, "tracking:updates", hub, nil)

	// Register subscriber in local hub
	tn := "SD-20261110-000001"
	eventChan, unsub, err := hub.Subscribe(context.Background(), tn)
	if err != nil {
		t.Fatalf("unexpected subscribe error: %v", err)
	}
	defer unsub()

	now := time.Date(2026, 11, 10, 16, 0, 0, 0, time.UTC)
	event := RedisTrackingEvent{
		EventID:         "0192e2b3-7c8a-7890-a123-000000000001",
		EventType:       "parcel.in_transit.v1",
		TrackingNumber:  tn,
		ParcelID:        "0192e2b3-7c8a-7890-a123-000000000010",
		Status:          "IN_TRANSIT",
		StatusHistoryID: "0192e2b3-7c8a-7890-a123-000000000099",
		OccurredAt:      now,
	}

	if err := rps.Publish(context.Background(), event); err != nil {
		t.Fatalf("unexpected publish error: %v", err)
	}

	if publishedChannel != "tracking:updates" {
		t.Errorf("expected channel tracking:updates, got %s", publishedChannel)
	}

	var wireEvent RedisTrackingEvent
	if err := json.Unmarshal(publishedPayload, &wireEvent); err != nil {
		t.Fatalf("failed to unmarshal wire payload: %v", err)
	}
	if wireEvent.TrackingNumber != tn || wireEvent.Status != "IN_TRANSIT" {
		t.Errorf("unexpected wire event contents: %+v", wireEvent)
	}

	// Verify local hub subscriber received the event
	select {
	case received := <-eventChan:
		if received.TrackingNumber != tn {
			t.Errorf("expected tracking number %s, got %s", tn, received.TrackingNumber)
		}
		if received.Status != "IN_TRANSIT" {
			t.Errorf("expected status IN_TRANSIT, got %s", received.Status)
		}
		if received.StatusHistoryID != "0192e2b3-7c8a-7890-a123-000000000099" {
			t.Errorf("expected StatusHistoryID preserved, got %s", received.StatusHistoryID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for local hub event")
	}
}

func TestRedisPubSub_HandleRedisMessage_Direct(t *testing.T) {
	hub := NewEventHub()
	rps := NewRedisPubSub(nil, "tracking:updates", hub, nil)

	tn := "SD-20261110-000002"
	eventChan, unsub, err := hub.Subscribe(context.Background(), tn)
	if err != nil {
		t.Fatalf("unexpected subscribe error: %v", err)
	}
	defer unsub()

	validMsg := `{
		"event_id": "0192e2b3-7c8a-7890-a123-000000000002",
		"event_type": "parcel.delivered.v1",
		"tracking_number": "SD-20261110-000002",
		"parcel_id": "0192e2b3-7c8a-7890-a123-000000000020",
		"status": "DELIVERED",
		"status_history_id": "0192e2b3-7c8a-7890-a123-000000000088",
		"occurred_at": "2026-11-10T16:30:00Z"
	}`

	rps.handleRedisMessage(validMsg)

	select {
	case received := <-eventChan:
		if received.Status != "DELIVERED" {
			t.Errorf("expected DELIVERED, got %s", received.Status)
		}
		if received.StatusHistoryID != "0192e2b3-7c8a-7890-a123-000000000088" {
			t.Errorf("expected status history ID, got %s", received.StatusHistoryID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for event delivery")
	}

	// Delivering identical message again should be deduplicated
	rps.handleRedisMessage(validMsg)

	select {
	case duplicate := <-eventChan:
		t.Fatalf("unexpected duplicate event delivered to subscriber: %+v", duplicate)
	case <-time.After(50 * time.Millisecond):
		// Success: duplicate was safely deduplicated
	}
}

func TestRedisPubSub_HandleMalformedAndInvalidMessages(t *testing.T) {
	hub := NewEventHub()
	rps := NewRedisPubSub(nil, "tracking:updates", hub, nil)

	// Should not panic on malformed JSON
	rps.handleRedisMessage("invalid-json")

	// Should not panic on invalid tracking number
	rps.handleRedisMessage(`{"tracking_number":"invalid-tn","status":"IN_TRANSIT"}`)

	// Should not panic on invalid status
	rps.handleRedisMessage(`{"tracking_number":"SD-20261110-000001","status":"UNKNOWN_STATUS"}`)
}

func TestRedisPubSub_PingAndAvailability(t *testing.T) {
	t.Run("nil client unavailable", func(t *testing.T) {
		rps := NewRedisPubSub(nil, "tracking:updates", nil, nil)
		if rps.IsAvailable() {
			t.Error("expected IsAvailable() false when client is nil")
		}
		if err := rps.Ping(context.Background()); !errors.Is(err, ErrRedisUnavailable) {
			t.Errorf("expected ErrRedisUnavailable, got %v", err)
		}
	})

	t.Run("ping failure", func(t *testing.T) {
		mockClient := &mockRedisClient{
			pingFunc: func(ctx context.Context) *redis.StatusCmd {
				cmd := redis.NewStatusCmd(ctx)
				cmd.SetErr(errors.New("connection refused"))
				return cmd
			},
		}
		rps := NewRedisPubSub(mockClient, "tracking:updates", nil, nil)
		if rps.IsAvailable() {
			t.Error("expected IsAvailable() false on ping failure")
		}
		if err := rps.Ping(context.Background()); err == nil {
			t.Error("expected error on ping failure, got nil")
		}
	})

	t.Run("ping success", func(t *testing.T) {
		mockClient := &mockRedisClient{}
		rps := NewRedisPubSub(mockClient, "tracking:updates", nil, nil)
		if !rps.IsAvailable() {
			t.Error("expected IsAvailable() true on ping success")
		}
		if err := rps.Ping(context.Background()); err != nil {
			t.Errorf("unexpected ping error: %v", err)
		}
	})
}

func TestRedisPubSub_MultipleLocalSubscribers(t *testing.T) {
	mockClient := &mockRedisClient{}
	hub := NewEventHub()
	rps := NewRedisPubSub(mockClient, "tracking:updates", hub, nil)

	tn := "SD-20261110-000003"
	sub1, unsub1, _ := hub.Subscribe(context.Background(), tn)
	defer unsub1()
	sub2, unsub2, _ := hub.Subscribe(context.Background(), tn)
	defer unsub2()

	msg := `{
		"event_id": "0192e2b3-7c8a-7890-a123-000000000003",
		"event_type": "parcel.out_for_delivery.v1",
		"tracking_number": "SD-20261110-000003",
		"parcel_id": "0192e2b3-7c8a-7890-a123-000000000030",
		"status": "OUT_FOR_DELIVERY",
		"status_history_id": "0192e2b3-7c8a-7890-a123-000000000077",
		"occurred_at": "2026-11-10T17:00:00Z"
	}`

	rps.handleRedisMessage(msg)

	var ev1, ev2 dto.PublicStatusUpdateEvent
	select {
	case ev1 = <-sub1:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for sub1")
	}

	select {
	case ev2 = <-sub2:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for sub2")
	}

	if ev1.Status != "OUT_FOR_DELIVERY" || ev2.Status != "OUT_FOR_DELIVERY" {
		t.Errorf("expected both subscribers to receive OUT_FOR_DELIVERY, got %s and %s", ev1.Status, ev2.Status)
	}
}
