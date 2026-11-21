package dto

import "time"

// InAppNotificationDTO represents an in-app user notification streamed over SSE.
type InAppNotificationDTO struct {
	ID          string         `json:"notification_id"`
	UserID      string         `json:"user_id"`
	TenantID    *string        `json:"tenant_id,omitempty"`
	EventType   string         `json:"event_type"`
	AggregateID string         `json:"aggregate_id"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	IsRead      bool           `json:"is_read"`
	CreatedAt   time.Time      `json:"created_at"`
}

// NotificationUnreadCountDTO represents the unread counter push event.
type NotificationUnreadCountDTO struct {
	Count int `json:"count"`
}
