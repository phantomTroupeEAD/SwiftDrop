package transport

import (
	"time"

	"github.com/swiftdrop/core-api/internal/notifications/domain"
)

// NotificationResponse represents the JSON response body for a single notification.
type NotificationResponse struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	TenantID    *string        `json:"tenant_id,omitempty"`
	EventType   string         `json:"event_type"`
	AggregateID string         `json:"aggregate_id"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Metadata    map[string]any `json:"metadata"`
	IsRead      bool           `json:"is_read"`
	CreatedAt   string         `json:"created_at"`
	ReadAt      *string        `json:"read_at,omitempty"`
}

// ToNotificationResponse converts a domain.Notification into a transport NotificationResponse.
func ToNotificationResponse(n *domain.Notification) NotificationResponse {
	var readAtStr *string
	if n.ReadAt != nil {
		s := n.ReadAt.UTC().Format(time.RFC3339)
		readAtStr = &s
	}

	meta := n.Metadata
	if meta == nil {
		meta = make(map[string]any)
	}

	return NotificationResponse{
		ID:          n.ID,
		UserID:      n.UserID,
		TenantID:    n.TenantID,
		EventType:   n.EventType,
		AggregateID: n.AggregateID,
		Title:       n.Title,
		Body:        n.Body,
		Metadata:    meta,
		IsRead:      n.IsRead,
		CreatedAt:   n.CreatedAt.UTC().Format(time.RFC3339),
		ReadAt:      readAtStr,
	}
}

// ToNotificationResponses converts a slice of domain.Notification into a slice of transport NotificationResponse.
func ToNotificationResponses(notifications []*domain.Notification) []NotificationResponse {
	res := make([]NotificationResponse, 0, len(notifications))
	for _, n := range notifications {
		res = append(res, ToNotificationResponse(n))
	}
	return res
}

// UnreadCountResponse represents the JSON response body for the unread counter.
type UnreadCountResponse struct {
	UnreadCount int `json:"unread_count"`
}

// MarkAllReadResponse represents the JSON response body for the mark-all-read operation.
type MarkAllReadResponse struct {
	UpdatedCount int64 `json:"updated_count"`
}
