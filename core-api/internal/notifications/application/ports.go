package application

import (
	"context"
	"time"

	"github.com/swiftdrop/core-api/internal/notifications/domain"
)

// PaginationCursor models cursor data for keyset pagination.
type PaginationCursor struct {
	CreatedAt time.Time
	ID        string
}

// ListFilter encapsulates query filtering and pagination parameters.
type ListFilter struct {
	UserID       string
	IsDispatcher bool
	Cursor       *PaginationCursor
	Limit        int
}

// NotificationRepository defines persistence operations for in-app user notifications.
type NotificationRepository interface {
	// ListByUserID retrieves paginated notifications for the specified user (and role:DISPATCHER if isDispatcher is true).
	ListByUserID(ctx context.Context, filter ListFilter) ([]*domain.Notification, *PaginationCursor, error)

	// GetUnreadCount counts unread notifications for the specified user (and role:DISPATCHER if isDispatcher is true).
	GetUnreadCount(ctx context.Context, userID string, isDispatcher bool) (int, error)

	// FindByID retrieves a single notification by its unique ID.
	FindByID(ctx context.Context, id string) (*domain.Notification, error)

	// MarkAsRead marks a single notification as read if it is owned by the user or accessible via role:DISPATCHER.
	MarkAsRead(ctx context.Context, id string, userID string, isDispatcher bool, readAt time.Time) (*domain.Notification, error)

	// MarkAllAsRead marks all unread notifications as read for the user or role:DISPATCHER, returning the count updated.
	MarkAllAsRead(ctx context.Context, userID string, isDispatcher bool, readAt time.Time) (int64, error)
}
