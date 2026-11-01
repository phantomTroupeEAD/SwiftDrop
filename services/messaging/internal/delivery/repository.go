package delivery

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/swiftdrop/messaging/internal/idempotency"
)

// MerchantWebhookTarget represents a row from the merchant_webhook_targets view.
type MerchantWebhookTarget struct {
	EndpointID           string
	MerchantID           string
	URL                  string
	SecretCiphertext     []byte
	SecretKeyVersion     int
	SubscribedEventTypes []string
}

// Repository defines delivery data access for both notifications and webhooks.
type Repository interface {
	ClaimNotifications(ctx context.Context, limit int, leaseDuration time.Duration) ([]NotificationTask, error)
	ClaimWebhooks(ctx context.Context, limit int, leaseDuration time.Duration) ([]WebhookTask, error)

	RecordNotificationSuccess(ctx context.Context, id, eventID, providerMessageID string) error
	RecordNotificationRetry(ctx context.Context, id string, lastError string, nextAttemptAt time.Time) error
	RecordNotificationTerminalFailure(ctx context.Context, id, eventID string, lastError string) error

	RecordWebhookSuccess(ctx context.Context, deliveryID, endpointID, eventID string, attemptNo int, statusCode int, snippet string) error
	RecordWebhookRetry(ctx context.Context, deliveryID string, attemptNo int, statusCode int, snippet, errorClass, errorMsg string, nextAttemptAt time.Time) error
	RecordWebhookTerminalFailure(ctx context.Context, deliveryID, endpointID, eventID string, attemptNo int, statusCode int, snippet, errorClass, errorMsg string) error

	InsertNotification(ctx context.Context, task NotificationTask) error
	InsertWebhook(ctx context.Context, task WebhookTask) error
	GetMerchantTargets(ctx context.Context, merchantID, eventType string) ([]MerchantWebhookTarget, error)
	GetEndpointByID(ctx context.Context, endpointID string) (*MerchantWebhookTarget, error)
}

// PostgresDeliveryRepository implements Repository using PostgreSQL.
type PostgresDeliveryRepository struct {
	db          *sql.DB
	idempotency idempotency.Store
}

// NewPostgresDeliveryRepository creates a new PostgresDeliveryRepository.
func NewPostgresDeliveryRepository(db *sql.DB, idemp idempotency.Store) *PostgresDeliveryRepository {
	return &PostgresDeliveryRepository{
		db:          db,
		idempotency: idemp,
	}
}

// ClaimNotifications claims pending notification delivery rows using SKIP LOCKED.
func (r *PostgresDeliveryRepository) ClaimNotifications(ctx context.Context, limit int, leaseDuration time.Duration) ([]NotificationTask, error) {
	leaseSeconds := int(leaseDuration.Seconds())
	if leaseSeconds <= 0 {
		leaseSeconds = 120
	}

	const query = `
		UPDATE notification_deliveries
		SET status = 'IN_PROGRESS',
		    lease_expires_at = now() + ($1 * interval '1 second'),
		    attempt_count = attempt_count + 1
		WHERE id IN (
		    SELECT id FROM notification_deliveries
		    WHERE status IN ('PENDING', 'IN_PROGRESS')
		      AND (lease_expires_at IS NULL OR lease_expires_at < now())
		      AND (next_attempt_at IS NULL OR next_attempt_at <= now())
		    ORDER BY next_attempt_at NULLS FIRST
		    LIMIT $2
		    FOR UPDATE SKIP LOCKED
		)
		RETURNING id, event_id, event_type, aggregate_id, aggregate_version, payload, recipient, channel, template_name, attempt_count;
	`

	rows, err := r.db.QueryContext(ctx, query, leaseSeconds, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to claim notifications: %w", err)
	}
	defer rows.Close()

	var tasks []NotificationTask
	for rows.Next() {
		var t NotificationTask
		if err := rows.Scan(
			&t.ID,
			&t.EventID,
			&t.EventType,
			&t.AggregateID,
			&t.AggregateVersion,
			&t.Payload,
			&t.Recipient,
			&t.Channel,
			&t.TemplateName,
			&t.AttemptCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan notification task: %w", err)
		}
		tasks = append(tasks, t)
	}

	return tasks, rows.Err()
}

// ClaimWebhooks claims pending webhook delivery rows using SKIP LOCKED.
func (r *PostgresDeliveryRepository) ClaimWebhooks(ctx context.Context, limit int, leaseDuration time.Duration) ([]WebhookTask, error) {
	leaseSeconds := int(leaseDuration.Seconds())
	if leaseSeconds <= 0 {
		leaseSeconds = 120
	}

	const query = `
		UPDATE webhook_deliveries
		SET status = 'IN_PROGRESS',
		    lease_expires_at = now() + ($1 * interval '1 second'),
		    attempt_count = attempt_count + 1
		WHERE id IN (
		    SELECT id FROM webhook_deliveries
		    WHERE status IN ('PENDING', 'IN_PROGRESS')
		      AND (lease_expires_at IS NULL OR lease_expires_at < now())
		      AND (next_attempt_at IS NULL OR next_attempt_at <= now())
		    ORDER BY next_attempt_at NULLS FIRST
		    LIMIT $2
		    FOR UPDATE SKIP LOCKED
		)
		RETURNING id, endpoint_id, event_id, event_type, aggregate_id, aggregate_version, occurred_at, payload, attempt_count;
	`

	rows, err := r.db.QueryContext(ctx, query, leaseSeconds, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to claim webhooks: %w", err)
	}
	defer rows.Close()

	var tasks []WebhookTask
	for rows.Next() {
		var t WebhookTask
		if err := rows.Scan(
			&t.ID,
			&t.EndpointID,
			&t.EventID,
			&t.EventType,
			&t.AggregateID,
			&t.AggregateVersion,
			&t.OccurredAt,
			&t.Payload,
			&t.AttemptCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan webhook task: %w", err)
		}
		tasks = append(tasks, t)
	}

	return tasks, rows.Err()
}

// RecordNotificationSuccess marks a notification as SUCCESS and records idempotency.
func (r *PostgresDeliveryRepository) RecordNotificationSuccess(ctx context.Context, id, eventID, providerMessageID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin notification success tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const updateQuery = `
		UPDATE notification_deliveries
		SET status = 'SUCCESS',
		    delivered_at = now(),
		    provider_message_id = $1,
		    last_error = NULL
		WHERE id = $2;
	`
	if _, err := tx.ExecContext(ctx, updateQuery, providerMessageID, id); err != nil {
		return fmt.Errorf("failed to update notification to SUCCESS: %w", err)
	}

	if err := r.idempotency.MarkDone(ctx, tx, "notifier", eventID); err != nil {
		return fmt.Errorf("failed to mark notifier done in idempotency: %w", err)
	}

	return tx.Commit()
}

// RecordNotificationRetry reschedules a notification with exponential backoff.
func (r *PostgresDeliveryRepository) RecordNotificationRetry(ctx context.Context, id string, lastError string, nextAttemptAt time.Time) error {
	const query = `
		UPDATE notification_deliveries
		SET status = 'PENDING',
		    last_error = $1,
		    next_attempt_at = $2,
		    lease_expires_at = NULL
		WHERE id = $3;
	`
	_, err := r.db.ExecContext(ctx, query, lastError, nextAttemptAt, id)
	if err != nil {
		return fmt.Errorf("failed to update notification retry: %w", err)
	}
	return nil
}

// RecordNotificationTerminalFailure marks a notification as FAILED and records idempotency.
func (r *PostgresDeliveryRepository) RecordNotificationTerminalFailure(ctx context.Context, id, eventID string, lastError string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin notification failure tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const query = `
		UPDATE notification_deliveries
		SET status = 'FAILED',
		    last_error = $1,
		    next_attempt_at = NULL,
		    lease_expires_at = NULL
		WHERE id = $2;
	`
	if _, err := tx.ExecContext(ctx, query, lastError, id); err != nil {
		return fmt.Errorf("failed to update notification to FAILED: %w", err)
	}

	if err := r.idempotency.MarkDone(ctx, tx, "notifier", eventID); err != nil {
		return fmt.Errorf("failed to mark notifier done in idempotency: %w", err)
	}

	return tx.Commit()
}

// RecordWebhookSuccess records attempt and marks webhook delivery as SUCCESS in one transaction.
func (r *PostgresDeliveryRepository) RecordWebhookSuccess(ctx context.Context, deliveryID, endpointID, eventID string, attemptNo int, statusCode int, snippet string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin webhook success tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const insertAttempt = `
		INSERT INTO webhook_delivery_attempts (
			delivery_id, attempt_no, started_at, response_status_code, response_body_snippet, created_at
		) VALUES (
			$1, $2, now(), $3, $4, now()
		);
	`
	if _, err := tx.ExecContext(ctx, insertAttempt, deliveryID, attemptNo, statusCode, snippet); err != nil {
		return fmt.Errorf("failed to insert webhook attempt: %w", err)
	}

	const updateDelivery = `
		UPDATE webhook_deliveries
		SET status = 'SUCCESS',
		    delivered_at = now(),
		    response_status_code = $1,
		    response_body_snippet = $2,
		    last_error = NULL,
		    next_attempt_at = NULL,
		    lease_expires_at = NULL
		WHERE id = $3;
	`
	if _, err := tx.ExecContext(ctx, updateDelivery, statusCode, snippet, deliveryID); err != nil {
		return fmt.Errorf("failed to update webhook delivery to SUCCESS: %w", err)
	}

	consumerName := fmt.Sprintf("webhook:%s", endpointID)
	if err := r.idempotency.MarkDone(ctx, tx, consumerName, eventID); err != nil {
		return fmt.Errorf("failed to mark webhook consumer done in idempotency: %w", err)
	}

	return tx.Commit()
}

// RecordWebhookRetry records attempt and reschedules webhook delivery.
func (r *PostgresDeliveryRepository) RecordWebhookRetry(ctx context.Context, deliveryID string, attemptNo int, statusCode int, snippet, errorClass, errorMsg string, nextAttemptAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin webhook retry tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const insertAttempt = `
		INSERT INTO webhook_delivery_attempts (
			delivery_id, attempt_no, started_at, response_status_code, response_body_snippet, error_class, error_message, next_attempt_at, created_at
		) VALUES (
			$1, $2, now(), $3, $4, $5, $6, $7, now()
		);
	`
	var sc *int
	if statusCode > 0 {
		sc = &statusCode
	}
	if _, err := tx.ExecContext(ctx, insertAttempt, deliveryID, attemptNo, sc, snippet, errorClass, errorMsg, nextAttemptAt); err != nil {
		return fmt.Errorf("failed to insert webhook attempt: %w", err)
	}

	const updateDelivery = `
		UPDATE webhook_deliveries
		SET status = 'PENDING',
		    response_status_code = $1,
		    response_body_snippet = $2,
		    last_error = $3,
		    next_attempt_at = $4,
		    lease_expires_at = NULL
		WHERE id = $5;
	`
	if _, err := tx.ExecContext(ctx, updateDelivery, sc, snippet, errorMsg, nextAttemptAt, deliveryID); err != nil {
		return fmt.Errorf("failed to update webhook delivery retry: %w", err)
	}

	return tx.Commit()
}

// RecordWebhookTerminalFailure records attempt, sets FAILED status, and marks idempotency.
func (r *PostgresDeliveryRepository) RecordWebhookTerminalFailure(ctx context.Context, deliveryID, endpointID, eventID string, attemptNo int, statusCode int, snippet, errorClass, errorMsg string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin webhook failure tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const insertAttempt = `
		INSERT INTO webhook_delivery_attempts (
			delivery_id, attempt_no, started_at, response_status_code, response_body_snippet, error_class, error_message, created_at
		) VALUES (
			$1, $2, now(), $3, $4, $5, $6, now()
		);
	`
	var sc *int
	if statusCode > 0 {
		sc = &statusCode
	}
	if _, err := tx.ExecContext(ctx, insertAttempt, deliveryID, attemptNo, sc, snippet, errorClass, errorMsg); err != nil {
		return fmt.Errorf("failed to insert webhook failure attempt: %w", err)
	}

	const updateDelivery = `
		UPDATE webhook_deliveries
		SET status = 'FAILED',
		    response_status_code = $1,
		    response_body_snippet = $2,
		    last_error = $3,
		    next_attempt_at = NULL,
		    lease_expires_at = NULL
		WHERE id = $4;
	`
	if _, err := tx.ExecContext(ctx, updateDelivery, sc, snippet, errorMsg, deliveryID); err != nil {
		return fmt.Errorf("failed to update webhook delivery to FAILED: %w", err)
	}

	consumerName := fmt.Sprintf("webhook:%s", endpointID)
	if err := r.idempotency.MarkDone(ctx, tx, consumerName, eventID); err != nil {
		return fmt.Errorf("failed to mark webhook consumer done in idempotency: %w", err)
	}

	return tx.Commit()
}

// InsertNotification stages a new customer notification task with ON CONFLICT DO NOTHING.
func (r *PostgresDeliveryRepository) InsertNotification(ctx context.Context, task NotificationTask) error {
	const query = `
		INSERT INTO notification_deliveries (
			event_id, event_type, aggregate_id, aggregate_version, payload, recipient, channel, template_name, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, 'PENDING'
		)
		ON CONFLICT (event_id, channel, recipient) DO NOTHING;
	`
	_, err := r.db.ExecContext(ctx, query,
		task.EventID,
		task.EventType,
		task.AggregateID,
		task.AggregateVersion,
		task.Payload,
		task.Recipient,
		task.Channel,
		task.TemplateName,
	)
	if err != nil {
		return fmt.Errorf("failed to insert notification delivery: %w", err)
	}
	return nil
}

// InsertWebhook stages a new merchant webhook task with ON CONFLICT DO NOTHING.
func (r *PostgresDeliveryRepository) InsertWebhook(ctx context.Context, task WebhookTask) error {
	const query = `
		INSERT INTO webhook_deliveries (
			endpoint_id, event_id, event_type, aggregate_id, aggregate_version, occurred_at, payload, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, 'PENDING'
		)
		ON CONFLICT (endpoint_id, event_id) DO NOTHING;
	`
	_, err := r.db.ExecContext(ctx, query,
		task.EndpointID,
		task.EventID,
		task.EventType,
		task.AggregateID,
		task.AggregateVersion,
		task.OccurredAt,
		task.Payload,
	)
	if err != nil {
		return fmt.Errorf("failed to insert webhook delivery: %w", err)
	}
	return nil
}

// GetMerchantTargets queries active webhook targets for a merchant subscribed to an event type.
func (r *PostgresDeliveryRepository) GetMerchantTargets(ctx context.Context, merchantID, eventType string) ([]MerchantWebhookTarget, error) {
	const query = `
		SELECT
			endpoint_id, merchant_id, url, secret_ciphertext, secret_key_version, subscribed_event_types
		FROM merchant_webhook_targets
		WHERE merchant_id = $1
		  AND $2 = ANY(subscribed_event_types);
	`
	rows, err := r.db.QueryContext(ctx, query, merchantID, eventType)
	if err != nil {
		return nil, fmt.Errorf("failed to query merchant_webhook_targets: %w", err)
	}
	defer rows.Close()

	var targets []MerchantWebhookTarget
	for rows.Next() {
		var tgt MerchantWebhookTarget
		var types pq.StringArray
		if err := rows.Scan(
			&tgt.EndpointID,
			&tgt.MerchantID,
			&tgt.URL,
			&tgt.SecretCiphertext,
			&tgt.SecretKeyVersion,
			&types,
		); err != nil {
			return nil, fmt.Errorf("failed to scan merchant webhook target: %w", err)
		}
		tgt.SubscribedEventTypes = []string(types)
		targets = append(targets, tgt)
	}

	return targets, rows.Err()
}

// GetEndpointByID retrieves an active merchant webhook target by its endpoint ID.
func (r *PostgresDeliveryRepository) GetEndpointByID(ctx context.Context, endpointID string) (*MerchantWebhookTarget, error) {
	const query = `
		SELECT
			endpoint_id, merchant_id, url, secret_ciphertext, secret_key_version, subscribed_event_types
		FROM merchant_webhook_targets
		WHERE endpoint_id = $1;
	`
	var tgt MerchantWebhookTarget
	var types pq.StringArray
	err := r.db.QueryRowContext(ctx, query, endpointID).Scan(
		&tgt.EndpointID,
		&tgt.MerchantID,
		&tgt.URL,
		&tgt.SecretCiphertext,
		&tgt.SecretKeyVersion,
		&types,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found or disabled
		}
		return nil, fmt.Errorf("failed to query merchant_webhook_targets by id %s: %w", endpointID, err)
	}
	tgt.SubscribedEventTypes = []string(types)
	return &tgt, nil
}
