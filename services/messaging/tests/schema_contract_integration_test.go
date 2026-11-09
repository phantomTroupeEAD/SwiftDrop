package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/swiftdrop/messaging/internal/consumer"
	"github.com/swiftdrop/messaging/internal/delivery"
	"github.com/swiftdrop/messaging/internal/idempotency"
	"github.com/swiftdrop/messaging/internal/logging"
	"github.com/swiftdrop/messaging/internal/reconciler"
	"github.com/swiftdrop/messaging/internal/relay"
)

func findRepoRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}

func setupThrowawaySchema(t *testing.T) (*sql.DB, string, func()) {
	t.Helper()
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("skipping schema contract integration test; set INTEGRATION=1")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://swiftdrop_user:swiftdrop_password@localhost:5432/swiftdrop_db?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Fatalf("cannot reach database at %s: %v", dbURL, err)
	}

	schemaName := fmt.Sprintf("itest_contract_%d", time.Now().UnixNano())
	if _, err := db.Exec(fmt.Sprintf("CREATE SCHEMA %s;", schemaName)); err != nil {
		db.Close()
		t.Fatalf("failed to create schema %s: %v", schemaName, err)
	}

	if _, err := db.Exec(fmt.Sprintf("SET search_path TO %s, public;", schemaName)); err != nil {
		_, _ = db.Exec(fmt.Sprintf("DROP SCHEMA %s CASCADE;", schemaName))
		db.Close()
		t.Fatalf("failed to set search_path: %v", err)
	}

	cleanup := func() {
		_, _ = db.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE;", schemaName))
		db.Close()
	}

	repoRoot, err := findRepoRoot(".")
	if err != nil {
		cleanup()
		t.Fatalf("failed to find repo root: %v", err)
	}

	// Apply migrations from core-api/migrations
	migrationFiles := []string{
		filepath.Join(repoRoot, "core-api", "migrations", "20261021000001_create_identity_tables.up.sql"),
		filepath.Join(repoRoot, "core-api", "migrations", "20261101000050_create_outbox_events.up.sql"),
		filepath.Join(repoRoot, "core-api", "migrations", "20261101000500_create_worker_dedup_and_webhooks.up.sql"),
		filepath.Join(repoRoot, "services", "messaging", "tests", "fixtures", "m4_stub.sql"),
	}

	for _, mf := range migrationFiles {
		sqlBytes, err := os.ReadFile(mf)
		if err != nil {
			cleanup()
			t.Fatalf("failed to read migration file %s: %v", mf, err)
		}
		if _, err := db.Exec(string(sqlBytes)); err != nil {
			cleanup()
			t.Fatalf("failed to execute migration file %s: %v", mf, err)
		}
	}

	return db, schemaName, cleanup
}

func TestComprehensiveSQLSchemaContract_Integration(t *testing.T) {
	db, _, cleanup := setupThrowawaySchema(t)
	defer cleanup()

	ctx := context.Background()
	logger := logging.SetupLogger("info")

	// 1. Test devemit INSERT statement
	eventID := "0192e2b3-7c8a-7890-a123-000000000001"
	parcelID := "0192e2b3-7c8a-7890-a123-000000000010"
	merchantID := "0192e2b3-7c8a-7890-a123-000000000042"
	endpointID := "0192e2b3-7c8a-7890-a123-000000000088"

	devemitInsert := `
		INSERT INTO outbox_events (
			id, producer, occurred_at, correlation_id, aggregate_type, aggregate_id, 
			aggregate_version, event_type, schema_version, payload, status
		) VALUES (
			$1, 'core-api.parcels', NOW(), 'corr-contract-1', 'parcel', $2, 
			1, 'parcel.booked.v1', '1.0.0', $3::jsonb, 'PENDING'
		);
	`
	payloadJSON := fmt.Sprintf(`{
		"parcel_id": "%s",
		"merchant_id": "%s",
		"status": "BOOKED",
		"notify": {
			"recipient_name": "Contract Tester",
			"recipient_email": "contract@swiftdrop.test"
		}
	}`, parcelID, merchantID)

	t.Run("devemit SQL execution", func(t *testing.T) {
		res, err := db.ExecContext(ctx, devemitInsert, eventID, parcelID, payloadJSON)
		if err != nil {
			t.Fatalf("devemit INSERT failed: %v", err)
		}
		rows, _ := res.RowsAffected()
		if rows != 1 {
			t.Errorf("expected 1 row affected, got %d", rows)
		}
	})

	// 2. Test relay PostgresOutboxRepository methods
	outboxRepo := relay.NewPostgresOutboxRepository(db)

	t.Run("relay PostgresOutboxRepository methods", func(t *testing.T) {
		// ClaimBatch
		batch, err := outboxRepo.ClaimBatch(ctx, 10, 60)
		if err != nil {
			t.Fatalf("ClaimBatch failed: %v", err)
		}
		if len(batch) != 1 {
			t.Fatalf("expected 1 record claimed, got %d", len(batch))
		}
		if batch[0].ID != eventID || batch[0].EventID != eventID {
			t.Errorf("expected ID/EventID %s, got ID=%s EventID=%s", eventID, batch[0].ID, batch[0].EventID)
		}

		// RecordFailure
		if err := outboxRepo.RecordFailure(ctx, eventID, "simulated broker failure", time.Now().Add(5*time.Second), false); err != nil {
			t.Fatalf("RecordFailure failed: %v", err)
		}

		// Re-claim after resetting next_attempt_at for test
		_, _ = db.ExecContext(ctx, "UPDATE outbox_events SET next_attempt_at = NULL WHERE id = $1", eventID)
		reclaimed, err := outboxRepo.ClaimBatch(ctx, 10, 60)
		if err != nil || len(reclaimed) != 1 {
			t.Fatalf("failed to reclaim outbox event: err=%v len=%d", err, len(reclaimed))
		}

		// MarkPublished
		if err := outboxRepo.MarkPublished(ctx, eventID); err != nil {
			t.Fatalf("MarkPublished failed: %v", err)
		}

		// Verify status is PUBLISHED
		var status string
		var publishedAt sql.NullTime
		err = db.QueryRowContext(ctx, "SELECT status, published_at FROM outbox_events WHERE id = $1", eventID).Scan(&status, &publishedAt)
		if err != nil {
			t.Fatalf("failed to query outbox: %v", err)
		}
		if status != "PUBLISHED" || !publishedAt.Valid {
			t.Errorf("expected PUBLISHED with valid published_at, got %s, valid=%v", status, publishedAt.Valid)
		}

		// GetOutboxStats
		pendingCount, oldestAge, err := outboxRepo.GetOutboxStats(ctx)
		if err != nil {
			t.Fatalf("GetOutboxStats failed: %v", err)
		}
		if pendingCount != 0 {
			t.Errorf("expected 0 pending count, got %d (age %f)", pendingCount, oldestAge)
		}
	})

	// 3. Test consumers and delivery repository
	idempStore := idempotency.NewPostgresStore(db)
	delivRepo := delivery.NewPostgresDeliveryRepository(db, idempStore)

	// Seed merchant and endpoint in stub fixtures
	_, err := db.ExecContext(ctx, `INSERT INTO merchants (id, business_name, status) VALUES ($1, 'Acme Express', 'APPROVED');`, merchantID)
	if err != nil {
		t.Fatalf("failed to seed merchant: %v", err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO webhook_endpoints (id, merchant_id, url, secret_ciphertext, subscribed_event_types, enabled)
		VALUES ($1, $2, 'http://127.0.0.1:9099/webhook', 'raw_secret_32_bytes_long_here!!!', ARRAY['parcel.booked.v1'], true);`, endpointID, merchantID)
	if err != nil {
		t.Fatalf("failed to seed endpoint: %v", err)
	}

	envelope := &relay.OutboxEnvelope{
		EventID:          eventID,
		EventType:        "parcel.booked.v1",
		AggregateType:    "parcel",
		AggregateID:      parcelID,
		AggregateVersion: 1,
		SchemaVersion:    "1.0.0",
		CorrelationID:    "corr-contract-1",
		OccurredAt:       time.Now().UTC().Format(time.RFC3339),
		Payload:          json.RawMessage(payloadJSON),
	}

	t.Run("consumer staging: notifications and webhooks", func(t *testing.T) {
		notifHandler := consumer.NewNotificationHandler(delivRepo, idempStore, logger)
		if err := notifHandler(ctx, amqp.Delivery{}, envelope); err != nil {
			t.Fatalf("NewNotificationHandler failed: %v", err)
		}

		webhookHandler := consumer.NewWebhookFanoutHandler(delivRepo, idempStore, logger)
		if err := webhookHandler(ctx, amqp.Delivery{}, envelope); err != nil {
			t.Fatalf("NewWebhookFanoutHandler failed: %v", err)
		}

		// Verify notification row exists
		var notifID string
		err := db.QueryRowContext(ctx, "SELECT id FROM notification_deliveries WHERE event_id = $1", eventID).Scan(&notifID)
		if err != nil {
			t.Fatalf("failed to find staged notification: %v", err)
		}

		// Verify webhook row exists
		var hookID string
		err = db.QueryRowContext(ctx, "SELECT id FROM webhook_deliveries WHERE event_id = $1", eventID).Scan(&hookID)
		if err != nil {
			t.Fatalf("failed to find staged webhook: %v", err)
		}
	})

	t.Run("delivery repository methods: notifications", func(t *testing.T) {
		// ClaimNotifications
		tasks, err := delivRepo.ClaimNotifications(ctx, 10, 30*time.Second)
		if err != nil {
			t.Fatalf("ClaimNotifications failed: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 notification claimed, got %d", len(tasks))
		}
		task := tasks[0]

		// RecordNotificationRetry
		if err := delivRepo.RecordNotificationRetry(ctx, task.ID, "temporary smtp err", time.Now().Add(1*time.Second)); err != nil {
			t.Fatalf("RecordNotificationRetry failed: %v", err)
		}

		// Reset next_attempt_at to claim again
		_, _ = db.ExecContext(ctx, "UPDATE notification_deliveries SET next_attempt_at = NULL WHERE id = $1", task.ID)
		reclaimed, err := delivRepo.ClaimNotifications(ctx, 10, 30*time.Second)
		if err != nil || len(reclaimed) != 1 {
			t.Fatalf("failed to reclaim notification: %v", err)
		}

		// RecordNotificationSuccess
		if err := delivRepo.RecordNotificationSuccess(ctx, task.ID, eventID, "provider-smtp-123"); err != nil {
			t.Fatalf("RecordNotificationSuccess failed: %v", err)
		}

		// Verify processed_events recorded
		seen, err := idempStore.Seen(ctx, "notifier", eventID)
		if err != nil || !seen {
			t.Errorf("expected notifier/%s in processed_events, err=%v", eventID, err)
		}

		// RecordNotificationTerminalFailure on a new task
		task2 := delivery.NotificationTask{
			EventID:          "0192e2b3-7c8a-7890-a123-000000000002",
			EventType:        "parcel.booked.v1",
			AggregateID:      parcelID,
			AggregateVersion: 1,
			Payload:          []byte(payloadJSON),
			Recipient:        "invalid@recipient.test",
			Channel:          "EMAIL",
			TemplateName:     "milestone_notice",
		}
		if err := delivRepo.InsertNotification(ctx, task2); err != nil {
			t.Fatalf("InsertNotification failed: %v", err)
		}
		var task2ID string
		_ = db.QueryRowContext(ctx, "SELECT id FROM notification_deliveries WHERE event_id = $1", task2.EventID).Scan(&task2ID)
		if err := delivRepo.RecordNotificationTerminalFailure(ctx, task2ID, task2.EventID, "mailbox does not exist"); err != nil {
			t.Fatalf("RecordNotificationTerminalFailure failed: %v", err)
		}
	})

	t.Run("delivery repository methods: webhooks", func(t *testing.T) {
		// ClaimWebhooks
		tasks, err := delivRepo.ClaimWebhooks(ctx, 10, 30*time.Second)
		if err != nil {
			t.Fatalf("ClaimWebhooks failed: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 webhook claimed, got %d", len(tasks))
		}
		task := tasks[0]

		// RecordWebhookRetry
		if err := delivRepo.RecordWebhookRetry(ctx, task.ID, 1, 502, "bad gateway", "HTTP_502", "bad gateway", time.Now().Add(1*time.Second)); err != nil {
			t.Fatalf("RecordWebhookRetry failed: %v", err)
		}

		// Reset next_attempt_at to claim again
		_, _ = db.ExecContext(ctx, "UPDATE webhook_deliveries SET next_attempt_at = NULL WHERE id = $1", task.ID)
		reclaimed, err := delivRepo.ClaimWebhooks(ctx, 10, 30*time.Second)
		if err != nil || len(reclaimed) != 1 {
			t.Fatalf("failed to reclaim webhook: %v", err)
		}

		// RecordWebhookSuccess
		if err := delivRepo.RecordWebhookSuccess(ctx, task.ID, task.EndpointID, eventID, 2, 200, `{"received":true}`); err != nil {
			t.Fatalf("RecordWebhookSuccess failed: %v", err)
		}

		// Verify processed_events recorded
		consumerName := fmt.Sprintf("webhook:%s", task.EndpointID)
		seen, err := idempStore.Seen(ctx, consumerName, eventID)
		if err != nil || !seen {
			t.Errorf("expected %s/%s in processed_events, err=%v", consumerName, eventID, err)
		}

		// RecordWebhookTerminalFailure on a new task
		task2 := delivery.WebhookTask{
			EndpointID:       endpointID,
			EventID:          "0192e2b3-7c8a-7890-a123-000000000003",
			EventType:        "parcel.booked.v1",
			AggregateID:      parcelID,
			AggregateVersion: 1,
			OccurredAt:       time.Now().UTC(),
			Payload:          []byte(payloadJSON),
		}
		if err := delivRepo.InsertWebhook(ctx, task2); err != nil {
			t.Fatalf("InsertWebhook failed: %v", err)
		}
		var task2ID string
		_ = db.QueryRowContext(ctx, "SELECT id FROM webhook_deliveries WHERE event_id = $1", task2.EventID).Scan(&task2ID)
		if err := delivRepo.RecordWebhookTerminalFailure(ctx, task2ID, endpointID, task2.EventID, 1, 410, "Gone", "ENDPOINT_GONE", "endpoint returned 410"); err != nil {
			t.Fatalf("RecordWebhookTerminalFailure failed: %v", err)
		}

		// GetMerchantTargets
		targets, err := delivRepo.GetMerchantTargets(ctx, merchantID, "parcel.booked.v1")
		if err != nil {
			t.Fatalf("GetMerchantTargets failed: %v", err)
		}
		if len(targets) != 1 {
			t.Errorf("expected 1 active target, got %d", len(targets))
		}
	})

	t.Run("reconciler lease reset and retention", func(t *testing.T) {
		rec := reconciler.NewReconciler(db, logger, reconciler.Options{
			BatchLimit: 1000,
		})

		// Create expired leases
		_, err := db.ExecContext(ctx, `
			UPDATE notification_deliveries SET status = 'IN_PROGRESS', lease_expires_at = now() - INTERVAL '10 minutes';
			UPDATE webhook_deliveries SET status = 'IN_PROGRESS', lease_expires_at = now() - INTERVAL '10 minutes';
		`)
		if err != nil {
			t.Fatalf("failed to simulate expired leases: %v", err)
		}

		// ResetExpiredLeases
		resetNotifs, resetHooks, err := rec.ResetExpiredLeases(ctx)
		if err != nil {
			t.Fatalf("ResetExpiredLeases failed: %v", err)
		}
		if resetNotifs == 0 || resetHooks == 0 {
			t.Errorf("expected reset > 0, got notifs=%d hooks=%d", resetNotifs, resetHooks)
		}

		// Verify reset back to PENDING
		var notifPendingCount int
		_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM notification_deliveries WHERE status = 'PENDING'").Scan(&notifPendingCount)
		if notifPendingCount == 0 {
			t.Errorf("expected reset notifications to be PENDING")
		}

		// Simulate old rows for retention:
		// outbox > 7 days
		// deliveries > 30 days
		// processed_events > 35 days
		_, err = db.ExecContext(ctx, `
			UPDATE outbox_events SET status = 'PUBLISHED', published_at = now() - INTERVAL '10 days';
			UPDATE notification_deliveries SET status = 'SUCCESS', delivered_at = now() - INTERVAL '32 days';
			UPDATE webhook_deliveries SET status = 'SUCCESS', delivered_at = now() - INTERVAL '32 days';
			UPDATE processed_events SET processed_at = now() - INTERVAL '37 days';
		`)
		if err != nil {
			t.Fatalf("failed to stage old retention rows: %v", err)
		}

		// Run retention pass 1
		rep1, err := rec.RunRetention(ctx, 1000)
		if err != nil {
			t.Fatalf("RunRetention (pass 1) failed: %v", err)
		}
		if rep1.DeletedOutbox == 0 || rep1.DeletedNotifications == 0 || rep1.DeletedWebhooks == 0 || rep1.DeletedEvents == 0 {
			t.Errorf("expected non-zero deletions, got: %+v", rep1)
		}

		// Run retention pass 2 (verify idempotence)
		rep2, err := rec.RunRetention(ctx, 1000)
		if err != nil {
			t.Fatalf("RunRetention (pass 2) failed: %v", err)
		}
		if rep2.DeletedOutbox != 0 || rep2.DeletedNotifications != 0 || rep2.DeletedWebhooks != 0 || rep2.DeletedEvents != 0 {
			t.Errorf("expected 0 deletions on immediate second pass, got: %+v", rep2)
		}

		// Verify purged
		var remainingOutbox int
		_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM outbox_events WHERE status = 'PUBLISHED'").Scan(&remainingOutbox)
		if remainingOutbox != 0 {
			t.Errorf("expected 0 published outbox events after retention, got %d", remainingOutbox)
		}
	})
}
