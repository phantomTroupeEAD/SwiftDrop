package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/swiftdrop/core-api/internal/events/application"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

// MaskEmail masks the local part of an email address for privacy.
func MaskEmail(email string) string {
	parts := strings.Split(strings.TrimSpace(email), "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "***"
	}
	user := parts[0]
	domain := parts[1]
	if len(user) <= 1 {
		return "*@" + domain
	}
	return string(user[0]) + "***@" + domain
}

// PostgresEventsRepository implements application.EventsRepository.
type PostgresEventsRepository struct {
	db *sql.DB
}

// NewPostgresEventsRepository creates a new PostgresEventsRepository.
func NewPostgresEventsRepository(db *sql.DB) *PostgresEventsRepository {
	return &PostgresEventsRepository{db: db}
}

// ListOutbox retrieves paginated outbox events.
func (r *PostgresEventsRepository) ListOutbox(ctx context.Context, filter application.OutboxFilter, cursor httpx.Cursor, limit int) ([]application.OutboxItem, *httpx.Cursor, error) {
	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
		SELECT
			id, event_id, event_type, aggregate_type, aggregate_id, aggregate_version,
			schema_version, occurred_at, correlation_id, producer, payload, status,
			attempt_count, last_error, next_attempt_at, published_at, created_at
		FROM outbox_events
		WHERE 1=1
	`)

	var args []interface{}
	argIdx := 1

	if filter.Status != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}

	if filter.EventType != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND event_type = $%d", argIdx))
		args = append(args, filter.EventType)
		argIdx++
	}

	if filter.From != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND created_at >= $%d", argIdx))
		args = append(args, *filter.From)
		argIdx++
	}

	if filter.To != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND created_at <= $%d", argIdx))
		args = append(args, *filter.To)
		argIdx++
	}

	if !cursor.CreatedAt.IsZero() && cursor.ID != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argIdx, argIdx+1))
		args = append(args, cursor.CreatedAt, cursor.ID)
	}

	queryBuilder.WriteString(fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT %d", limit+1))

	rows, err := r.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query outbox: %w", err)
	}
	defer rows.Close()

	var items []application.OutboxItem
	for rows.Next() {
		var it application.OutboxItem
		var lastErr, pubAt, nextAt sql.NullString
		var pubTime, nextTime sql.NullTime

		if err := rows.Scan(
			&it.ID,
			&it.EventID,
			&it.EventType,
			&it.AggregateType,
			&it.AggregateID,
			&it.AggregateVersion,
			&it.SchemaVersion,
			&it.OccurredAt,
			&it.CorrelationID,
			&it.Producer,
			&it.Payload,
			&it.Status,
			&it.AttemptCount,
			&lastErr,
			&nextTime,
			&pubTime,
			&it.CreatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("failed to scan outbox row: %w", err)
		}

		if lastErr.Valid {
			it.LastError = &lastErr.String
		}
		if pubTime.Valid {
			it.PublishedAt = &pubTime.Time
		}
		if nextTime.Valid {
			it.NextAttemptAt = &nextTime.Time
		}

		_ = pubAt
		_ = nextAt

		items = append(items, it)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("row error: %w", err)
	}

	var nextCursor *httpx.Cursor
	if len(items) > limit {
		lastItem := items[limit-1]
		nextCursor = &httpx.Cursor{
			CreatedAt: lastItem.CreatedAt,
			ID:        lastItem.ID,
		}
		items = items[:limit]
	}

	return items, nextCursor, nil
}

// GetOutboxStats retrieves outbox summary counts and oldest pending age.
func (r *PostgresEventsRepository) GetOutboxStats(ctx context.Context) (*application.OutboxStats, error) {
	const query = `
		SELECT
			COUNT(*) FILTER (WHERE status = 'PENDING') AS pending_count,
			COUNT(*) FILTER (WHERE status = 'PUBLISHED') AS published_count,
			COUNT(*) FILTER (WHERE status = 'FAILED') AS failed_count,
			COALESCE(EXTRACT(EPOCH FROM (now() - MIN(created_at) FILTER (WHERE status = 'PENDING'))), 0) AS oldest_pending_age_seconds
		FROM outbox_events;
	`
	var s application.OutboxStats
	err := r.db.QueryRowContext(ctx, query).Scan(
		&s.PendingCount,
		&s.PublishedCount,
		&s.FailedCount,
		&s.OldestPendingAgeSeconds,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query outbox stats: %w", err)
	}
	return &s, nil
}

// RetryOutbox un-stalls a FAILED outbox event and resets it to PENDING.
func (r *PostgresEventsRepository) RetryOutbox(ctx context.Context, id string) error {
	var currentStatus string
	const checkQuery = `SELECT status FROM outbox_events WHERE id = $1;`
	err := r.db.QueryRowContext(ctx, checkQuery, id).Scan(&currentStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return application.ErrNotFound
		}
		return fmt.Errorf("failed to check outbox status: %w", err)
	}

	if currentStatus != "FAILED" {
		return fmt.Errorf("%w: current status is %s", application.ErrInvalidState, currentStatus)
	}

	const updateQuery = `
		UPDATE outbox_events
		SET status = 'PENDING',
		    attempt_count = 0,
		    next_attempt_at = NULL,
		    last_error = NULL
		WHERE id = $1;
	`
	_, err = r.db.ExecContext(ctx, updateQuery, id)
	if err != nil {
		return fmt.Errorf("failed to update outbox event to PENDING: %w", err)
	}
	return nil
}

// ListDeliveries retrieves unified delivery logs for notifications and webhooks.
func (r *PostgresEventsRepository) ListDeliveries(ctx context.Context, filter application.DeliveryFilter, cursor httpx.Cursor, limit int) ([]application.UnifiedDeliveryItem, *httpx.Cursor, error) {
	var queryBuilder strings.Builder

	includeNotifs := filter.Kind == "" || filter.Kind == "notification"
	includeWebhooks := filter.Kind == "" || filter.Kind == "webhook"

	var parts []string
	var args []interface{}
	argIdx := 1

	if includeNotifs {
		q := `
			SELECT
				id, 'notification' AS kind, recipient AS endpoint_or_recipient, event_id, event_type,
				status, attempt_count, next_attempt_at, last_error, created_at, delivered_at
			FROM notification_deliveries
			WHERE 1=1
		`
		if filter.Status != "" {
			q += fmt.Sprintf(" AND status = $%d", argIdx)
		}
		parts = append(parts, q)
	}

	if includeWebhooks {
		q := `
			SELECT
				id, 'webhook' AS kind, endpoint_id::text AS endpoint_or_recipient, event_id, event_type,
				status, attempt_count, next_attempt_at, last_error, created_at, delivered_at
			FROM webhook_deliveries
			WHERE 1=1
		`
		if filter.Status != "" {
			q += fmt.Sprintf(" AND status = $%d", argIdx)
		}
		parts = append(parts, q)
	}

	if filter.Status != "" {
		args = append(args, filter.Status)
		argIdx++
	}

	queryBuilder.WriteString(strings.Join(parts, " UNION ALL "))

	if !cursor.CreatedAt.IsZero() && cursor.ID != "" {
		queryBuilder.WriteString(fmt.Sprintf(" WHERE (created_at, id) < ($%d, $%d)", argIdx, argIdx+1))
		args = append(args, cursor.CreatedAt, cursor.ID)
	}

	queryBuilder.WriteString(fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT %d", limit+1))

	rows, err := r.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query deliveries: %w", err)
	}
	defer rows.Close()

	var items []application.UnifiedDeliveryItem
	for rows.Next() {
		var it application.UnifiedDeliveryItem
		var rawTarget string
		var nextTime, delivTime sql.NullTime
		var lastErr sql.NullString

		if err := rows.Scan(
			&it.ID,
			&it.Kind,
			&rawTarget,
			&it.EventID,
			&it.EventType,
			&it.Status,
			&it.AttemptCount,
			&nextTime,
			&lastErr,
			&it.CreatedAt,
			&delivTime,
		); err != nil {
			return nil, nil, fmt.Errorf("failed to scan delivery row: %w", err)
		}

		if it.Kind == "notification" {
			it.EndpointOrRecipient = MaskEmail(rawTarget)
		} else {
			it.EndpointOrRecipient = rawTarget
		}

		if lastErr.Valid {
			it.LastError = &lastErr.String
		}
		if nextTime.Valid {
			it.NextAttemptAt = &nextTime.Time
		}
		if delivTime.Valid {
			it.DeliveredAt = &delivTime.Time
		}

		items = append(items, it)
	}

	var nextCursor *httpx.Cursor
	if len(items) > limit {
		lastItem := items[limit-1]
		nextCursor = &httpx.Cursor{
			CreatedAt: lastItem.CreatedAt,
			ID:        lastItem.ID,
		}
		items = items[:limit]
	}

	return items, nextCursor, nil
}

// RetryWebhookDelivery resets a FAILED webhook delivery to PENDING.
func (r *PostgresEventsRepository) RetryWebhookDelivery(ctx context.Context, id string) error {
	var currentStatus string
	const checkQuery = `SELECT status FROM webhook_deliveries WHERE id = $1;`
	err := r.db.QueryRowContext(ctx, checkQuery, id).Scan(&currentStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return application.ErrNotFound
		}
		return fmt.Errorf("failed to check webhook delivery status: %w", err)
	}

	if currentStatus != "FAILED" {
		return fmt.Errorf("%w: current status is %s", application.ErrInvalidState, currentStatus)
	}

	const updateQuery = `
		UPDATE webhook_deliveries
		SET status = 'PENDING',
		    attempt_count = 0,
		    next_attempt_at = NULL,
		    last_error = NULL
		WHERE id = $1;
	`
	_, err = r.db.ExecContext(ctx, updateQuery, id)
	if err != nil {
		return fmt.Errorf("failed to update webhook delivery to PENDING: %w", err)
	}
	return nil
}

// ListMerchantWebhookDeliveries returns deliveries for endpoints owned by merchant.
func (r *PostgresEventsRepository) ListMerchantWebhookDeliveries(ctx context.Context, merchantID string, cursor httpx.Cursor, limit int) ([]application.MerchantDeliveryItem, *httpx.Cursor, error) {
	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
		SELECT
			id, endpoint_id, event_id, event_type, status, attempt_count,
			next_attempt_at, response_status_code, response_body_snippet, last_error, created_at, delivered_at
		FROM webhook_deliveries
		WHERE endpoint_id IN (
			SELECT endpoint_id FROM merchant_webhook_targets WHERE merchant_id = $1
		)
	`)

	args := []interface{}{merchantID}
	argIdx := 2

	if !cursor.CreatedAt.IsZero() && cursor.ID != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argIdx, argIdx+1))
		args = append(args, cursor.CreatedAt, cursor.ID)
	}

	queryBuilder.WriteString(fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT %d", limit+1))

	rows, err := r.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query merchant webhook deliveries: %w", err)
	}
	defer rows.Close()

	var items []application.MerchantDeliveryItem
	for rows.Next() {
		var it application.MerchantDeliveryItem
		var nextTime, delivTime sql.NullTime
		var statusCode sql.NullInt64
		var snippet, lastErr sql.NullString

		if err := rows.Scan(
			&it.ID,
			&it.EndpointID,
			&it.EventID,
			&it.EventType,
			&it.Status,
			&it.AttemptCount,
			&nextTime,
			&statusCode,
			&snippet,
			&lastErr,
			&it.CreatedAt,
			&delivTime,
		); err != nil {
			return nil, nil, fmt.Errorf("failed to scan merchant webhook delivery: %w", err)
		}

		if statusCode.Valid {
			sc := int(statusCode.Int64)
			it.ResponseStatusCode = &sc
		}
		if snippet.Valid {
			it.ResponseBodySnippet = &snippet.String
		}
		if lastErr.Valid {
			it.LastError = &lastErr.String
		}
		if nextTime.Valid {
			it.NextAttemptAt = &nextTime.Time
		}
		if delivTime.Valid {
			it.DeliveredAt = &delivTime.Time
		}

		items = append(items, it)
	}

	var nextCursor *httpx.Cursor
	if len(items) > limit {
		lastItem := items[limit-1]
		nextCursor = &httpx.Cursor{
			CreatedAt: lastItem.CreatedAt,
			ID:        lastItem.ID,
		}
		items = items[:limit]
	}

	return items, nextCursor, nil
}

// ListMerchantWebhookAttempts returns attempt history for an owned delivery.
func (r *PostgresEventsRepository) ListMerchantWebhookAttempts(ctx context.Context, merchantID, deliveryID string) ([]application.WebhookAttemptItem, error) {
	// Guard against IDOR: confirm delivery belongs to endpoint of merchant
	const ownershipQuery = `
		SELECT 1 FROM webhook_deliveries
		WHERE id = $1 AND endpoint_id IN (
			SELECT endpoint_id FROM merchant_webhook_targets WHERE merchant_id = $2
		);
	`
	var exists bool
	err := r.db.QueryRowContext(ctx, ownershipQuery, deliveryID, merchantID).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, application.ErrNotFound // Foreign delivery ID returned as 404
		}
		return nil, fmt.Errorf("failed to check delivery ownership: %w", err)
	}

	const attemptsQuery = `
		SELECT
			attempt_no, started_at, response_status_code, response_body_snippet,
			error_class, error_message, next_attempt_at
		FROM webhook_delivery_attempts
		WHERE delivery_id = $1
		ORDER BY attempt_no ASC;
	`
	rows, err := r.db.QueryContext(ctx, attemptsQuery, deliveryID)
	if err != nil {
		return nil, fmt.Errorf("failed to query webhook attempts: %w", err)
	}
	defer rows.Close()

	var attempts []application.WebhookAttemptItem
	for rows.Next() {
		var att application.WebhookAttemptItem
		var statusCode sql.NullInt64
		var snippet, errClass, errMsg sql.NullString
		var nextTime sql.NullTime

		if err := rows.Scan(
			&att.AttemptNo,
			&att.StartedAt,
			&statusCode,
			&snippet,
			&errClass,
			&errMsg,
			&nextTime,
		); err != nil {
			return nil, fmt.Errorf("failed to scan attempt record: %w", err)
		}

		if statusCode.Valid {
			sc := int(statusCode.Int64)
			att.ResponseStatusCode = &sc
		}
		if snippet.Valid {
			att.ResponseBodySnippet = &snippet.String
		}
		if errClass.Valid {
			att.ErrorClass = &errClass.String
		}
		if errMsg.Valid {
			att.ErrorMessage = &errMsg.String
		}
		if nextTime.Valid {
			att.NextAttemptAt = &nextTime.Time
		}

		attempts = append(attempts, att)
	}

	return attempts, nil
}
