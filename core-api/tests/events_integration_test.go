package tests

import (
	"context"
	"testing"

	eventsapp "github.com/swiftdrop/core-api/internal/events/application"
	eventsinfra "github.com/swiftdrop/core-api/internal/events/infrastructure"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

func TestEventsReadRepository_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := eventsinfra.NewPostgresEventsRepository(db)

	eventID := "0192e2b3-7c8a-7890-a123-000000000001"
	parcelID := "0192e2b3-7c8a-7890-a123-000000000010"
	payload := `{"parcel_id":"` + parcelID + `","status":"BOOKED"}`

	// 1. Seed outbox event
	insertOutbox := `
		INSERT INTO outbox_events (
			id, producer, correlation_id, aggregate_type, aggregate_id, 
			aggregate_version, event_type, schema_version, payload, status
		) VALUES (
			$1, 'core-api.parcels', 'corr-events-1', 'parcel', $2, 
			1, 'parcel.booked.v1', '1.0.0', $3::jsonb, 'PENDING'
		);
	`
	if _, err := db.ExecContext(ctx, insertOutbox, eventID, parcelID, payload); err != nil {
		t.Fatalf("failed to insert outbox event: %v", err)
	}

	t.Run("ListOutbox and verify ID and EventID mapping", func(t *testing.T) {
		items, _, err := repo.ListOutbox(ctx, eventsapp.OutboxFilter{}, httpx.Cursor{}, 10)
		if err != nil {
			t.Fatalf("ListOutbox failed: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got len=%d", len(items))
		}
		if items[0].ID != eventID || items[0].EventID != eventID {
			t.Errorf("expected ID and EventID %s, got ID=%s EventID=%s", eventID, items[0].ID, items[0].EventID)
		}
		if items[0].EventType != "parcel.booked.v1" {
			t.Errorf("expected parcel.booked.v1, got %s", items[0].EventType)
		}
	})

	t.Run("GetOutboxStats returns correct aggregates", func(t *testing.T) {
		stats, err := repo.GetOutboxStats(ctx)
		if err != nil {
			t.Fatalf("GetOutboxStats failed: %v", err)
		}
		if stats.PendingCount != 1 {
			t.Errorf("expected pending_count 1, got %d", stats.PendingCount)
		}
	})

	t.Run("RetryOutbox resets failed event to PENDING", func(t *testing.T) {
		// Mark failed first
		_, err := db.ExecContext(ctx, "UPDATE outbox_events SET status = 'FAILED', last_error = 'temporary error' WHERE id = $1", eventID)
		if err != nil {
			t.Fatalf("failed to mark outbox as failed: %v", err)
		}

		if err := repo.RetryOutbox(ctx, eventID); err != nil {
			t.Fatalf("RetryOutbox failed: %v", err)
		}

		var status string
		_ = db.QueryRowContext(ctx, "SELECT status FROM outbox_events WHERE id = $1", eventID).Scan(&status)
		if status != "PENDING" {
			t.Errorf("expected status PENDING after retry, got %s", status)
		}
	})

	// 2. Seed notification and webhook deliveries
	notifID := "0192e2b3-7c8a-7890-a123-000000000021"
	hookID := "0192e2b3-7c8a-7890-a123-000000000022"
	endpointID := "0192e2b3-7c8a-7890-a123-000000000088"
	testMerchantID := "0192e2b3-7c8a-7890-a123-000000000077"
	adminUserID := "018f4a7c-1001-7000-8000-000000000001"
	merchantUserID := "018f4a7c-1004-7000-8000-000000000004"

	insertMerchant := `
		INSERT INTO merchants (
			id, owner_user_id, business_name, status, contact_email, contact_phone, address, version, created_at, updated_at
		) VALUES (
			$1, $2, 'Test Merchant', 'PENDING', 'merchant@vendor.com', '+94770000004', '{"line1":"123 Main","city":"Colombo","country":"LK"}'::jsonb, 1, NOW(), NOW()
		) ON CONFLICT (id) DO NOTHING;
	`
	if _, err := db.ExecContext(ctx, insertMerchant, testMerchantID, merchantUserID); err != nil {
		t.Fatalf("failed to insert test merchant: %v", err)
	}

	insertEndpoint := `
		INSERT INTO webhook_endpoints (
			id, merchant_id, url, secret_ciphertext, secret_key_version, secret_last4, subscribed_event_types, enabled, created_by, created_at, updated_at
		) VALUES (
			$1, $2, 'https://example.com/webhook', decode('0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c', 'hex'), 1, 'abcd', '{"parcel.booked.v1"}', true, $3, NOW(), NOW()
		) ON CONFLICT (id) DO NOTHING;
	`
	if _, err := db.ExecContext(ctx, insertEndpoint, endpointID, testMerchantID, adminUserID); err != nil {
		t.Fatalf("failed to insert test webhook endpoint: %v", err)
	}

	insertNotif := `
		INSERT INTO notification_deliveries (
			id, event_id, event_type, aggregate_id, aggregate_version, payload, recipient, channel, status
		) VALUES (
			$1, $2, 'parcel.booked.v1', $3, 1, $4::jsonb, 'recipient@example.com', 'EMAIL', 'SUCCESS'
		);
	`
	if _, err := db.ExecContext(ctx, insertNotif, notifID, eventID, parcelID, payload); err != nil {
		t.Fatalf("failed to insert notification delivery: %v", err)
	}

	insertHook := `
		INSERT INTO webhook_deliveries (
			id, endpoint_id, event_id, event_type, aggregate_id, aggregate_version, occurred_at, payload, status
		) VALUES (
			$1, $2, $3, 'parcel.booked.v1', $4, 1, NOW(), $5::jsonb, 'FAILED'
		);
	`
	if _, err := db.ExecContext(ctx, insertHook, hookID, endpointID, eventID, parcelID, payload); err != nil {
		t.Fatalf("failed to insert webhook delivery: %v", err)
	}

	insertAttempt := `
		INSERT INTO webhook_delivery_attempts (
			delivery_id, attempt_no, started_at, response_status_code, response_body_snippet, error_class, error_message, created_at
		) VALUES (
			$1, 1, NOW(), 500, 'Internal Server Error', 'HTTP_500', 'server returned 500', NOW()
		);
	`
	if _, err := db.ExecContext(ctx, insertAttempt, hookID); err != nil {
		t.Fatalf("failed to insert webhook attempt: %v", err)
	}

	t.Run("ListDeliveries returns unified union of notifications and webhooks", func(t *testing.T) {
		deliveries, _, err := repo.ListDeliveries(ctx, eventsapp.DeliveryFilter{}, httpx.Cursor{}, 10)
		if err != nil {
			t.Fatalf("ListDeliveries failed: %v", err)
		}
		if len(deliveries) != 2 {
			t.Errorf("expected 2 total deliveries, got len=%d", len(deliveries))
		}
	})

	t.Run("RetryWebhookDelivery resets failed webhook to PENDING", func(t *testing.T) {
		if err := repo.RetryWebhookDelivery(ctx, hookID); err != nil {
			t.Fatalf("RetryWebhookDelivery failed: %v", err)
		}
		var status string
		_ = db.QueryRowContext(ctx, "SELECT status FROM webhook_deliveries WHERE id = $1", hookID).Scan(&status)
		if status != "PENDING" {
			t.Errorf("expected status PENDING after retry, got %s", status)
		}
	})
}
