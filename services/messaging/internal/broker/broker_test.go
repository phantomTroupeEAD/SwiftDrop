package broker

import (
	"context"
	"os"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestWorkQueuesDefinitions(t *testing.T) {
	queues := WorkQueues()
	if len(queues) != 3 {
		t.Fatalf("expected 3 work queues, got %d", len(queues))
	}

	queueMap := make(map[string][]string)
	for _, q := range queues {
		queueMap[q.Name] = q.RoutingKeys
	}

	// Verify q.notifications.worker
	notifKeys, ok := queueMap[QueueNotificationsWorker]
	if !ok {
		t.Errorf("missing %s", QueueNotificationsWorker)
	} else if len(notifKeys) != 1 || notifKeys[0] != "parcel.#" {
		t.Errorf("expected parcel.# for notifications queue, got %v", notifKeys)
	}

	// Verify q.webhooks.dispatcher
	webhookKeys, ok := queueMap[QueueWebhooksDispatcher]
	if !ok {
		t.Errorf("missing %s", QueueWebhooksDispatcher)
	} else if len(webhookKeys) != 3 {
		t.Errorf("expected 3 routing keys for webhooks queue, got %v", webhookKeys)
	}

	// Verify q.tracking.updates
	trackingKeys, ok := queueMap[QueueTrackingUpdates]
	if !ok {
		t.Errorf("missing %s", QueueTrackingUpdates)
	} else if len(trackingKeys) != 2 {
		t.Errorf("expected 2 routing keys for tracking queue, got %v", trackingKeys)
	}
}

func TestManager_DisconnectedBehavior(t *testing.T) {
	mgr := NewManager("amqp://guest:guest@localhost:5672/", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// CheckHealth should fail before connect
	if err := mgr.CheckHealth(ctx); err != ErrNotConnected {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}

	// Publish should fail before connect
	err := mgr.Publish(ctx, TopicExchange, "parcel.booked.v1", amqp.Publishing{
		Body: []byte(`{}`),
	})
	if err != ErrNotConnected {
		t.Errorf("expected ErrNotConnected on publish, got %v", err)
	}

	// OpenConsumerChannel should fail before connect
	ch, err := mgr.OpenConsumerChannel(10)
	if err != ErrNotConnected || ch != nil {
		t.Errorf("expected ErrNotConnected on OpenConsumerChannel, got %v", err)
	}

	// Close should succeed without error
	if err := mgr.Close(); err != nil {
		t.Errorf("expected clean close, got %v", err)
	}
}

func TestManager_WaitForConnection_Timeout(t *testing.T) {
	mgr := NewManager("amqp://guest:guest@localhost:5672/", nil)
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := mgr.WaitForConnection(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestManager_WaitForConnection_CloseUnblocks(t *testing.T) {
	mgr := NewManager("amqp://guest:guest@localhost:5672/", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- mgr.WaitForConnection(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	_ = mgr.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Errorf("expected error after close, got nil")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("WaitForConnection did not unblock after Close()")
	}
}

func TestManager_Connect_ContextExpiryStartsRetry(t *testing.T) {
	// Point to an invalid port where connection fails immediately
	mgr := NewManager("amqp://guest:guest@127.0.0.1:59999/", nil)
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := mgr.Connect(ctx)
	if err == nil {
		t.Fatal("expected Connect to fail on unreachable address")
	}

	// Verify manager is in reconnecting state
	mgr.mu.RLock()
	reconnecting := mgr.reconnecting
	mgr.mu.RUnlock()

	if !reconnecting {
		t.Errorf("expected reconnecting=true after Connect context expired, got false")
	}
}

func TestIntegration_TopologyAndRoundTrip(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration test; set INTEGRATION=1 to run against active RabbitMQ")
	}

	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@localhost:5672/"
	}

	mgr := NewManager(amqpURL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("failed to connect to rabbitmq: %v", err)
	}
	defer mgr.Close()

	// Test 1: Health check passes
	if err := mgr.CheckHealth(ctx); err != nil {
		t.Fatalf("health check failed: %v", err)
	}

	// Test 2: Idempotent Topology Declaration (call DeclareTopology again)
	ch, err := mgr.OpenConsumerChannel(1)
	if err != nil {
		t.Fatalf("failed to open consumer channel: %v", err)
	}
	defer ch.Close()

	if err := DeclareTopology(ch); err != nil {
		t.Fatalf("second DeclareTopology failed (not idempotent): %v", err)
	}

	// Purge queue before test roundtrip to ensure clean state
	_, _ = ch.QueuePurge(QueueNotificationsWorker, false)

	// Test 3: Publish and consume round-trip
	msgID := "test-msg-uuid"
	routingKey := "parcel.booked.v1"
	testPayload := []byte(`{"test":"broker_roundtrip"}`)

	pubErr := mgr.Publish(ctx, TopicExchange, routingKey, amqp.Publishing{
		MessageId:    msgID,
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         testPayload,
	})
	if pubErr != nil {
		t.Fatalf("publish failed: %v", pubErr)
	}

	msgs, err := ch.Consume(
		QueueNotificationsWorker,
		"test-consumer",
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,
	)
	if err != nil {
		t.Fatalf("consume failed: %v", err)
	}

	select {
	case d := <-msgs:
		if d.MessageId != msgID {
			t.Errorf("expected message id %s, got %s", msgID, d.MessageId)
		}
		_ = d.Ack(false)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for message on notifications queue")
	}
}
