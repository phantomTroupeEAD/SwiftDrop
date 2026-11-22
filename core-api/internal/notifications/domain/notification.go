package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	// ErrEmptyNotificationID is returned when a notification ID is empty.
	ErrEmptyNotificationID = errors.New("notification id cannot be empty")

	// ErrEmptyUserID is returned when a user ID is empty.
	ErrEmptyUserID = errors.New("user id cannot be empty")

	// ErrEmptyEventType is returned when an event type is empty.
	ErrEmptyEventType = errors.New("event type cannot be empty")

	// ErrEmptyAggregateID is returned when an aggregate ID is empty.
	ErrEmptyAggregateID = errors.New("aggregate id cannot be empty")

	// ErrEmptyTitle is returned when a notification title is empty.
	ErrEmptyTitle = errors.New("notification title cannot be empty")

	// ErrEmptyBody is returned when a notification body is empty.
	ErrEmptyBody = errors.New("notification body cannot be empty")

	// ErrNotificationNotFound is returned when a notification cannot be found.
	ErrNotificationNotFound = errors.New("notification not found")

	// ErrUnauthorizedAccess is returned when a user tries to access a notification they do not own.
	ErrUnauthorizedAccess = errors.New("unauthorized access to notification")

	// ErrInvalidNotificationID is returned when a notification ID is invalid or malformed.
	ErrInvalidNotificationID = errors.New("invalid notification id")
)

// Notification represents a persistent in-app notification delivered to a user.
type Notification struct {
	ID          string
	UserID      string
	TenantID    *string
	EventType   string
	AggregateID string
	Title       string
	Body        string
	Metadata    map[string]any
	IsRead      bool
	CreatedAt   time.Time
	ReadAt      *time.Time
}

// NewNotification constructs and validates a new Notification aggregate.
func NewNotification(
	id string,
	userID string,
	tenantID *string,
	eventType string,
	aggregateID string,
	title string,
	body string,
	metadata map[string]any,
	createdAt time.Time,
) (*Notification, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrEmptyNotificationID
	}
	if strings.TrimSpace(userID) == "" {
		return nil, ErrEmptyUserID
	}
	if strings.TrimSpace(eventType) == "" {
		return nil, ErrEmptyEventType
	}
	if strings.TrimSpace(aggregateID) == "" {
		return nil, ErrEmptyAggregateID
	}
	if strings.TrimSpace(title) == "" {
		return nil, ErrEmptyTitle
	}
	if strings.TrimSpace(body) == "" {
		return nil, ErrEmptyBody
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if metadata == nil {
		metadata = make(map[string]any)
	}

	return &Notification{
		ID:          strings.TrimSpace(id),
		UserID:      strings.TrimSpace(userID),
		TenantID:    tenantID,
		EventType:   strings.TrimSpace(eventType),
		AggregateID: strings.TrimSpace(aggregateID),
		Title:       strings.TrimSpace(title),
		Body:        strings.TrimSpace(body),
		Metadata:    metadata,
		IsRead:      false,
		CreatedAt:   createdAt,
		ReadAt:      nil,
	}, nil
}

// Validate verifies that required notification invariant fields are present.
func (n *Notification) Validate() error {
	if strings.TrimSpace(n.ID) == "" {
		return ErrEmptyNotificationID
	}
	if strings.TrimSpace(n.UserID) == "" {
		return ErrEmptyUserID
	}
	if strings.TrimSpace(n.EventType) == "" {
		return ErrEmptyEventType
	}
	if strings.TrimSpace(n.AggregateID) == "" {
		return ErrEmptyAggregateID
	}
	if strings.TrimSpace(n.Title) == "" {
		return ErrEmptyTitle
	}
	if strings.TrimSpace(n.Body) == "" {
		return ErrEmptyBody
	}
	return nil
}

// MarkRead transitions the notification to the read state and records the timestamp.
func (n *Notification) MarkRead(now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	n.IsRead = true
	n.ReadAt = &now
	return nil
}

// IsForUser verifies if the notification is addressed to the specified user,
// or if it is addressed to the dispatcher role broadcast and the caller is a dispatcher.
func (n *Notification) IsForUser(userID string, isDispatcher bool) bool {
	trimmedUser := strings.TrimSpace(userID)
	if trimmedUser == "" {
		return false
	}
	if n.UserID == trimmedUser {
		return true
	}
	if isDispatcher && n.UserID == "role:DISPATCHER" {
		return true
	}
	return false
}
