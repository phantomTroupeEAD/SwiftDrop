package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/swiftdrop/messaging/internal/broker"
	"github.com/swiftdrop/messaging/internal/config"
	"github.com/swiftdrop/messaging/internal/consumer"
	"github.com/swiftdrop/messaging/internal/delivery"
	"github.com/swiftdrop/messaging/internal/idempotency"
	"github.com/swiftdrop/messaging/internal/logging"
	"github.com/swiftdrop/messaging/internal/relay"
	"github.com/swiftdrop/messaging/internal/webhook"
)

func TestEndToEndLifecycle_Integration(t *testing.T) {
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("skipping end-to-end integration test; set INTEGRATION=1 to run against active Postgres & RabbitMQ")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/swiftdrop_db?sslmode=disable"
	}

	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}

	logger := logging.SetupLogger("info")

	// 1. Connect to PostgreSQL
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("cannot reach database at %s: %v", dbURL, err)
	}

	// 2. Connect to RabbitMQ (Connect automatically declares topology)
	mgr := broker.NewManager(rabbitURL, logger)
	if err := mgr.Connect(context.Background()); err != nil {
		t.Fatalf("failed to connect to RabbitMQ at %s: %v", rabbitURL, err)
	}
	defer mgr.Close()

	// 3. Set up test merchant target & endpoint
	endpointID := "0192e2b3-7c8a-7890-a123-000000000088"
	merchantID := "0192e2b3-7c8a-7890-a123-000000000042"
	rawSecret := "test_secret_for_e2e_verification_key_32c"

	receiverServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	defer receiverServer.Close()

	// 4. Staging transaction: Insert outbox event
	eventID := fmt.Sprintf("0192e2b3-7c8a-7890-a123-%012d", time.Now().UnixNano()%1000000000000)
	parcelID := "0192e2b3-7c8a-7890-a123-000000000010"

	payloadJSON := fmt.Sprintf(`{
		"parcel_id": "%s",
		"tracking_number": "SD-TEST-9999",
		"merchant_id": "%s",
		"status": "BOOKED",
		"notify": {
			"recipient_name": "Test Recipient",
			"recipient_email": "recipient@swiftdrop.test"
		}
	}`, parcelID, merchantID)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	insertOutbox := `
		INSERT INTO outbox_events (
			id, producer, occurred_at, correlation_id, aggregate_type, aggregate_id, 
			aggregate_version, event_type, schema_version, payload, status
		) VALUES (
			$1, 'core-api.parcels', NOW(), 'req-e2e-1', 'parcel', $2, 
			1, 'parcel.booked.v1', '1.0.0', $3::jsonb, 'PENDING'
		)
	`
	if _, err := tx.Exec(insertOutbox, eventID, parcelID, payloadJSON); err != nil {
		t.Fatalf("failed to insert outbox event: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit outbox insert: %v", err)
	}

	// 5. Run Relay pass to publish outbox event to RabbitMQ
	outboxRepo := relay.NewPostgresOutboxRepository(db)
	relayDaemon := relay.NewRelay(outboxRepo, mgr, &config.Config{
		OutboxBatchSize:    10,
		OutboxPollInterval: 100 * time.Millisecond,
		OutboxLeaseSeconds: 60,
	}, logger)

	publishedCount, err := relayDaemon.PollAndPublish(context.Background())
	if err != nil {
		t.Fatalf("relay failed to publish: %v", err)
	}
	if publishedCount == 0 {
		t.Fatalf("expected at least 1 outbox event published, got 0")
	}

	// Verify outbox row updated to PUBLISHED
	var status string
	var publishedAt sql.NullTime
	err = db.QueryRow("SELECT status, published_at FROM outbox_events WHERE id = $1", eventID).Scan(&status, &publishedAt)
	if err != nil {
		t.Fatalf("failed to query outbox event status: %v", err)
	}
	if status != "PUBLISHED" || !publishedAt.Valid {
		t.Errorf("expected status PUBLISHED with published_at, got status=%s publishedAt=%v", status, publishedAt)
	}

	// 6. Consumer pass: verify deduplication and notification staging
	idempotencyStore := idempotency.NewPostgresStore(db)
	delivRepo := delivery.NewPostgresDeliveryRepository(db, idempotencyStore)

	notifHandler := consumer.NewNotificationHandler(delivRepo, idempotencyStore, logger)
	outboxEnvelope := &relay.OutboxEnvelope{
		EventID:          eventID,
		EventType:        "parcel.booked.v1",
		AggregateType:    "parcel",
		AggregateID:      parcelID,
		AggregateVersion: 1,
		SchemaVersion:    "1.0.0",
		CorrelationID:    "req-e2e-1",
		Payload:          json.RawMessage(payloadJSON),
	}

	if err := notifHandler(context.Background(), amqp.Delivery{}, outboxEnvelope); err != nil {
		t.Fatalf("notification consumer handle failed: %v", err)
	}

	// Verify deduplication
	seen, err := idempotencyStore.Seen(context.Background(), "notifier", eventID)
	if err != nil || !seen {
		t.Errorf("expected event %s to be seen in processed_events, err=%v", eventID, err)
	}

	// Verify notification delivery row created
	var notifStatus string
	var recipientEmail string
	err = db.QueryRow("SELECT status, recipient_email FROM notification_deliveries WHERE event_id = $1", eventID).Scan(&notifStatus, &recipientEmail)
	if err != nil {
		t.Fatalf("failed to query notification_deliveries: %v", err)
	}
	if notifStatus != "PENDING" {
		t.Errorf("expected notification status PENDING, got %s", notifStatus)
	}
	if recipientEmail != "recipient@swiftdrop.test" {
		t.Errorf("expected recipient email recipient@swiftdrop.test, got %s", recipientEmail)
	}

	// 7. HMAC Signer & Webhook Dispatch verification
	body := []byte(`{"event_id":"` + eventID + `","data":{"parcel_id":"` + parcelID + `"}}`)
	timestamp := time.Now().Unix()
	sigHeader := webhook.Sign(timestamp, body, []byte(rawSecret))

	// Verify signature format "t=<unix>,v1=<hex>"
	if !strings.HasPrefix(sigHeader, fmt.Sprintf("t=%d,v1=", timestamp)) {
		t.Errorf("invalid signature header format: %s", sigHeader)
	}

	// Verify signature verification using SafeClient validator
	valid := webhook.VerifySignature(body, sigHeader, []byte(rawSecret), 5*time.Minute)
	if !valid {
		t.Errorf("expected signature verification to succeed")
	}

	logger.Info("e2e lifecycle integration test completed successfully",
		"event_id", eventID,
		"parcel_id", parcelID,
		"endpoint_id", endpointID,
	)
}
