package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/notifications/application"
	"github.com/swiftdrop/core-api/internal/notifications/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

// PostgresRepository implements application.NotificationRepository backed by PostgreSQL.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository constructs a new PostgresRepository.
func NewPostgresRepository(database *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: database}
}

// Ensure PostgresRepository implements application.NotificationRepository.
var _ application.NotificationRepository = (*PostgresRepository)(nil)

// ListByUserID queries user notifications with keyset cursor pagination and role:DISPATCHER support.
func (r *PostgresRepository) ListByUserID(ctx context.Context, filter application.ListFilter) ([]*domain.Notification, *application.PaginationCursor, error) {
	if r.db == nil {
		return nil, nil, db.ErrNilDB
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
		SELECT
			id, user_id, tenant_id, event_type, aggregate_id, title, body, metadata, is_read, created_at, read_at
		FROM user_notifications
		WHERE (user_id = $1 OR ($2 AND user_id = 'role:DISPATCHER'))
	`)

	args := []any{filter.UserID, filter.IsDispatcher}
	argIdx := 3

	if filter.Cursor != nil && !filter.Cursor.CreatedAt.IsZero() && filter.Cursor.ID != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argIdx, argIdx+1))
		args = append(args, filter.Cursor.CreatedAt, filter.Cursor.ID)
	}

	queryBuilder.WriteString(fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT %d", limit+1))

	rows, err := r.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*domain.Notification
	for rows.Next() {
		var (
			id          string
			userID      string
			tenantID    sql.NullString
			eventType   string
			aggregateID string
			title       string
			body        string
			metaRaw     []byte
			isRead      bool
			createdAt   time.Time
			readAt      sql.NullTime
		)

		if err := rows.Scan(
			&id,
			&userID,
			&tenantID,
			&eventType,
			&aggregateID,
			&title,
			&body,
			&metaRaw,
			&isRead,
			&createdAt,
			&readAt,
		); err != nil {
			return nil, nil, fmt.Errorf("failed to scan notification row: %w", err)
		}

		var tID *string
		if tenantID.Valid && tenantID.String != "" {
			t := tenantID.String
			tID = &t
		}

		meta := make(map[string]any)
		if len(metaRaw) > 0 {
			_ = json.Unmarshal(metaRaw, &meta)
		}

		var rAt *time.Time
		if readAt.Valid {
			r := readAt.Time
			rAt = &r
		}

		notifications = append(notifications, &domain.Notification{
			ID:          id,
			UserID:      userID,
			TenantID:    tID,
			EventType:   eventType,
			AggregateID: aggregateID,
			Title:       title,
			Body:        body,
			Metadata:    meta,
			IsRead:      isRead,
			CreatedAt:   createdAt,
			ReadAt:      rAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("error iterating notification rows: %w", err)
	}

	var nextCursor *application.PaginationCursor
	if len(notifications) > limit {
		lastItem := notifications[limit-1]
		nextCursor = &application.PaginationCursor{
			CreatedAt: lastItem.CreatedAt,
			ID:        lastItem.ID,
		}
		notifications = notifications[:limit]
	}

	return notifications, nextCursor, nil
}

// GetUnreadCount counts unread notifications for a user or dispatcher role broadcast.
func (r *PostgresRepository) GetUnreadCount(ctx context.Context, userID string, isDispatcher bool) (int, error) {
	if r.db == nil {
		return 0, db.ErrNilDB
	}

	const query = `
		SELECT COUNT(*)
		FROM user_notifications
		WHERE (user_id = $1 OR ($2 AND user_id = 'role:DISPATCHER'))
		  AND is_read = false;
	`

	var count int
	if err := r.db.QueryRowContext(ctx, query, userID, isDispatcher).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count unread notifications: %w", err)
	}
	return count, nil
}

// FindByID retrieves a notification by primary key.
func (r *PostgresRepository) FindByID(ctx context.Context, id string) (*domain.Notification, error) {
	if r.db == nil {
		return nil, db.ErrNilDB
	}

	const query = `
		SELECT
			id, user_id, tenant_id, event_type, aggregate_id, title, body, metadata, is_read, created_at, read_at
		FROM user_notifications
		WHERE id = $1;
	`

	var (
		nID         string
		userID      string
		tenantID    sql.NullString
		eventType   string
		aggregateID string
		title       string
		body        string
		metaRaw     []byte
		isRead      bool
		createdAt   time.Time
		readAt      sql.NullTime
	)

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&nID,
		&userID,
		&tenantID,
		&eventType,
		&aggregateID,
		&title,
		&body,
		&metaRaw,
		&isRead,
		&createdAt,
		&readAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotificationNotFound
		}
		return nil, fmt.Errorf("failed to find notification by id: %w", err)
	}

	var tID *string
	if tenantID.Valid && tenantID.String != "" {
		t := tenantID.String
		tID = &t
	}

	meta := make(map[string]any)
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &meta)
	}

	var rAt *time.Time
	if readAt.Valid {
		r := readAt.Time
		rAt = &r
	}

	return &domain.Notification{
		ID:          nID,
		UserID:      userID,
		TenantID:    tID,
		EventType:   eventType,
		AggregateID: aggregateID,
		Title:       title,
		Body:        body,
		Metadata:    meta,
		IsRead:      isRead,
		CreatedAt:   createdAt,
		ReadAt:      rAt,
	}, nil
}

// MarkAsRead marks a single notification as read if accessible to the user or dispatcher.
func (r *PostgresRepository) MarkAsRead(ctx context.Context, id string, userID string, isDispatcher bool, readAt time.Time) (*domain.Notification, error) {
	if r.db == nil {
		return nil, db.ErrNilDB
	}

	const query = `
		UPDATE user_notifications
		SET is_read = true,
		    read_at = COALESCE(read_at, $1)
		WHERE id = $2
		  AND (user_id = $3 OR ($4 AND user_id = 'role:DISPATCHER'))
		RETURNING id, user_id, tenant_id, event_type, aggregate_id, title, body, metadata, is_read, created_at, read_at;
	`

	var (
		nID         string
		uID         string
		tenantID    sql.NullString
		eventType   string
		aggregateID string
		title       string
		body        string
		metaRaw     []byte
		isRead      bool
		createdAt   time.Time
		rAt         sql.NullTime
	)

	err := r.db.QueryRowContext(ctx, query, readAt, id, userID, isDispatcher).Scan(
		&nID,
		&uID,
		&tenantID,
		&eventType,
		&aggregateID,
		&title,
		&body,
		&metaRaw,
		&isRead,
		&createdAt,
		&rAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotificationNotFound
		}
		return nil, fmt.Errorf("failed to mark notification as read: %w", err)
	}

	var tID *string
	if tenantID.Valid && tenantID.String != "" {
		t := tenantID.String
		tID = &t
	}

	meta := make(map[string]any)
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &meta)
	}

	var scannedReadAt *time.Time
	if rAt.Valid {
		r := rAt.Time
		scannedReadAt = &r
	}

	return &domain.Notification{
		ID:          nID,
		UserID:      uID,
		TenantID:    tID,
		EventType:   eventType,
		AggregateID: aggregateID,
		Title:       title,
		Body:        body,
		Metadata:    meta,
		IsRead:      isRead,
		CreatedAt:   createdAt,
		ReadAt:      scannedReadAt,
	}, nil
}

// MarkAllAsRead marks all unread notifications as read for a user or dispatcher.
func (r *PostgresRepository) MarkAllAsRead(ctx context.Context, userID string, isDispatcher bool, readAt time.Time) (int64, error) {
	if r.db == nil {
		return 0, db.ErrNilDB
	}

	const query = `
		UPDATE user_notifications
		SET is_read = true,
		    read_at = COALESCE(read_at, $1)
		WHERE (user_id = $2 OR ($3 AND user_id = 'role:DISPATCHER'))
		  AND is_read = false;
	`

	res, err := r.db.ExecContext(ctx, query, readAt, userID, isDispatcher)
	if err != nil {
		return 0, fmt.Errorf("failed to mark all notifications as read: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	return rowsAffected, nil
}

// Insert creates a new notification row in user_notifications.
func (r *PostgresRepository) Insert(ctx context.Context, n *domain.Notification) error {
	if r.db == nil {
		return db.ErrNilDB
	}

	if n.ID == "" {
		n.ID = uuid.NewString()
	}

	metaBytes, err := json.Marshal(n.Metadata)
	if err != nil {
		metaBytes = []byte("{}")
	}

	createdAt := n.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	const query = `
		INSERT INTO user_notifications (
			id, user_id, tenant_id, event_type, aggregate_id, title, body, metadata, is_read, created_at, read_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		);
	`

	_, err = r.db.ExecContext(
		ctx,
		query,
		n.ID,
		n.UserID,
		n.TenantID,
		n.EventType,
		n.AggregateID,
		n.Title,
		n.Body,
		metaBytes,
		n.IsRead,
		createdAt,
		n.ReadAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert notification: %w", err)
	}
	return nil
}
