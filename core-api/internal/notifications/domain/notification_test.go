package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/notifications/domain"
)

func TestNewNotification_Success(t *testing.T) {
	tenantID := "tenant-xyz"
	meta := map[string]any{"order_id": "ord-123"}
	now := time.Now().UTC()

	n, err := domain.NewNotification(
		"notif-1",
		"user-1",
		&tenantID,
		"parcel.delivered",
		"parcel-100",
		"Parcel Delivered",
		"Your package has been safely delivered.",
		meta,
		now,
	)
	if err != nil {
		t.Fatalf("unexpected error creating notification: %v", err)
	}

	if n.ID != "notif-1" {
		t.Errorf("expected ID 'notif-1', got %q", n.ID)
	}
	if n.UserID != "user-1" {
		t.Errorf("expected UserID 'user-1', got %q", n.UserID)
	}
	if n.TenantID == nil || *n.TenantID != "tenant-xyz" {
		t.Errorf("expected TenantID 'tenant-xyz', got %v", n.TenantID)
	}
	if n.EventType != "parcel.delivered" {
		t.Errorf("expected EventType 'parcel.delivered', got %q", n.EventType)
	}
	if n.AggregateID != "parcel-100" {
		t.Errorf("expected AggregateID 'parcel-100', got %q", n.AggregateID)
	}
	if n.Title != "Parcel Delivered" {
		t.Errorf("expected Title 'Parcel Delivered', got %q", n.Title)
	}
	if n.Body != "Your package has been safely delivered." {
		t.Errorf("expected Body, got %q", n.Body)
	}
	if n.IsRead {
		t.Errorf("expected IsRead to be false initially")
	}
	if n.ReadAt != nil {
		t.Errorf("expected ReadAt to be nil initially")
	}
	if n.CreatedAt != now {
		t.Errorf("expected CreatedAt %v, got %v", now, n.CreatedAt)
	}
}

func TestNewNotification_Defaults(t *testing.T) {
	n, err := domain.NewNotification(
		"notif-2",
		"user-2",
		nil,
		"parcel.dispatched",
		"parcel-200",
		"Dispatched",
		"Courier on the way",
		nil,
		time.Time{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.CreatedAt.IsZero() {
		t.Errorf("expected CreatedAt to be populated when zero")
	}
	if n.Metadata == nil {
		t.Errorf("expected Metadata to be non-nil empty map")
	}
	if err := n.Validate(); err != nil {
		t.Errorf("expected valid notification, got %v", err)
	}
}

func TestNewNotification_ValidationFailures(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		userID      string
		eventType   string
		aggregateID string
		title       string
		body        string
		expectedErr error
	}{
		{"empty id", "  ", "u1", "e1", "a1", "t1", "b1", domain.ErrEmptyNotificationID},
		{"empty user_id", "id1", "  ", "e1", "a1", "t1", "b1", domain.ErrEmptyUserID},
		{"empty event_type", "id1", "u1", "  ", "a1", "t1", "b1", domain.ErrEmptyEventType},
		{"empty aggregate_id", "id1", "u1", "e1", "  ", "t1", "b1", domain.ErrEmptyAggregateID},
		{"empty title", "id1", "u1", "e1", "a1", "  ", "b1", domain.ErrEmptyTitle},
		{"empty body", "id1", "u1", "e1", "a1", "t1", "  ", domain.ErrEmptyBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewNotification(tt.id, tt.userID, nil, tt.eventType, tt.aggregateID, tt.title, tt.body, nil, time.Now())
			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestNotification_MarkRead(t *testing.T) {
	n, err := domain.NewNotification("id1", "u1", nil, "evt", "agg", "title", "body", nil, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	readTime := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	if err := n.MarkRead(readTime); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !n.IsRead {
		t.Errorf("expected IsRead to be true")
	}
	if n.ReadAt == nil || *n.ReadAt != readTime {
		t.Errorf("expected ReadAt %v, got %v", readTime, n.ReadAt)
	}

	// Test zero timestamp fallback
	if err := n.MarkRead(time.Time{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.ReadAt.IsZero() {
		t.Errorf("expected non-zero ReadAt")
	}
}

func TestNotification_IsForUser(t *testing.T) {
	userNotif, _ := domain.NewNotification("id1", "user-100", nil, "evt", "agg", "t", "b", nil, time.Now())
	dispatcherNotif, _ := domain.NewNotification("id2", "role:DISPATCHER", nil, "evt", "agg", "t", "b", nil, time.Now())

	// Regular user notification
	if !userNotif.IsForUser("user-100", false) {
		t.Errorf("expected true for owner user")
	}
	if userNotif.IsForUser("user-200", false) {
		t.Errorf("expected false for different user")
	}
	if userNotif.IsForUser("", false) {
		t.Errorf("expected false for empty user")
	}

	// Dispatcher broadcast notification
	if !dispatcherNotif.IsForUser("user-dispatcher-1", true) {
		t.Errorf("expected true for dispatcher accessing role:DISPATCHER notification")
	}
	if dispatcherNotif.IsForUser("user-customer-1", false) {
		t.Errorf("expected false for non-dispatcher accessing role:DISPATCHER notification")
	}
}
