package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/notifications/application"
	"github.com/swiftdrop/core-api/internal/notifications/domain"
)

type mockNotificationRepository struct {
	notifications map[string]*domain.Notification
}

func newMockNotificationRepository() *mockNotificationRepository {
	return &mockNotificationRepository{
		notifications: make(map[string]*domain.Notification),
	}
}

func (m *mockNotificationRepository) ListByUserID(ctx context.Context, filter application.ListFilter) ([]*domain.Notification, *application.PaginationCursor, error) {
	var results []*domain.Notification
	for _, n := range m.notifications {
		if n.IsForUser(filter.UserID, filter.IsDispatcher) {
			results = append(results, n)
		}
	}
	var nextCursor *application.PaginationCursor
	if len(results) > filter.Limit {
		results = results[:filter.Limit]
		nextCursor = &application.PaginationCursor{
			CreatedAt: results[len(results)-1].CreatedAt,
			ID:        results[len(results)-1].ID,
		}
	}
	return results, nextCursor, nil
}

func (m *mockNotificationRepository) GetUnreadCount(ctx context.Context, userID string, isDispatcher bool) (int, error) {
	count := 0
	for _, n := range m.notifications {
		if n.IsForUser(userID, isDispatcher) && !n.IsRead {
			count++
		}
	}
	return count, nil
}

func (m *mockNotificationRepository) FindByID(ctx context.Context, id string) (*domain.Notification, error) {
	n, exists := m.notifications[id]
	if !exists {
		return nil, domain.ErrNotificationNotFound
	}
	return n, nil
}

func (m *mockNotificationRepository) MarkAsRead(ctx context.Context, id string, userID string, isDispatcher bool, readAt time.Time) (*domain.Notification, error) {
	n, exists := m.notifications[id]
	if !exists {
		return nil, domain.ErrNotificationNotFound
	}
	if !n.IsForUser(userID, isDispatcher) {
		return nil, domain.ErrUnauthorizedAccess
	}
	_ = n.MarkRead(readAt)
	return n, nil
}

func (m *mockNotificationRepository) MarkAllAsRead(ctx context.Context, userID string, isDispatcher bool, readAt time.Time) (int64, error) {
	var count int64
	for _, n := range m.notifications {
		if n.IsForUser(userID, isDispatcher) && !n.IsRead {
			_ = n.MarkRead(readAt)
			count++
		}
	}
	return count, nil
}

func TestService_ListNotifications(t *testing.T) {
	ctx := context.Background()
	repo := newMockNotificationRepository()
	svc := application.NewService(repo)

	n1, _ := domain.NewNotification("id1", "user-1", nil, "evt", "agg1", "Title 1", "Body 1", nil, time.Now())
	n2, _ := domain.NewNotification("id2", "role:DISPATCHER", nil, "evt", "agg2", "Title 2", "Body 2", nil, time.Now())
	n3, _ := domain.NewNotification("id3", "user-2", nil, "evt", "agg3", "Title 3", "Body 3", nil, time.Now())
	repo.notifications["id1"] = n1
	repo.notifications["id2"] = n2
	repo.notifications["id3"] = n3

	// Regular user should see only their notification
	res, _, err := svc.ListNotifications(ctx, application.ListNotificationsQuery{
		UserID:       "user-1",
		IsDispatcher: false,
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 || res[0].ID != "id1" {
		t.Errorf("expected 1 notification with ID id1, got %d items", len(res))
	}

	// Dispatcher should see their notification AND role:DISPATCHER notification
	resDisp, _, err := svc.ListNotifications(ctx, application.ListNotificationsQuery{
		UserID:       "user-1",
		IsDispatcher: true,
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resDisp) != 2 {
		t.Errorf("expected 2 notifications for dispatcher, got %d", len(resDisp))
	}

	// Empty userID error
	_, _, err = svc.ListNotifications(ctx, application.ListNotificationsQuery{
		UserID: "  ",
	})
	if !errors.Is(err, domain.ErrEmptyUserID) {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestService_GetUnreadCount(t *testing.T) {
	ctx := context.Background()
	repo := newMockNotificationRepository()
	svc := application.NewService(repo)

	n1, _ := domain.NewNotification("id1", "user-1", nil, "evt", "agg1", "T1", "B1", nil, time.Now())
	n2, _ := domain.NewNotification("id2", "user-1", nil, "evt", "agg2", "T2", "B2", nil, time.Now())
	_ = n2.MarkRead(time.Now())
	n3, _ := domain.NewNotification("id3", "role:DISPATCHER", nil, "evt", "agg3", "T3", "B3", nil, time.Now())
	repo.notifications["id1"] = n1
	repo.notifications["id2"] = n2
	repo.notifications["id3"] = n3

	count, err := svc.GetUnreadCount(ctx, "user-1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 unread notification, got %d", count)
	}

	countDisp, err := svc.GetUnreadCount(ctx, "user-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if countDisp != 2 {
		t.Errorf("expected 2 unread notifications for dispatcher, got %d", countDisp)
	}

	_, err = svc.GetUnreadCount(ctx, "", false)
	if !errors.Is(err, domain.ErrEmptyUserID) {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestService_FindByID(t *testing.T) {
	ctx := context.Background()
	repo := newMockNotificationRepository()
	svc := application.NewService(repo)

	n1, _ := domain.NewNotification("id1", "user-1", nil, "evt", "agg1", "T1", "B1", nil, time.Now())
	repo.notifications["id1"] = n1

	found, err := svc.FindByID(ctx, "id1", "user-1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.ID != "id1" {
		t.Errorf("expected id1, got %s", found.ID)
	}

	// Unauthorized
	_, err = svc.FindByID(ctx, "id1", "user-2", false)
	if !errors.Is(err, domain.ErrUnauthorizedAccess) {
		t.Errorf("expected ErrUnauthorizedAccess, got %v", err)
	}

	// Not found
	_, err = svc.FindByID(ctx, "non-existent", "user-1", false)
	if !errors.Is(err, domain.ErrNotificationNotFound) {
		t.Errorf("expected ErrNotificationNotFound, got %v", err)
	}

	// Validation errors
	_, err = svc.FindByID(ctx, "", "user-1", false)
	if !errors.Is(err, domain.ErrInvalidNotificationID) {
		t.Errorf("expected ErrInvalidNotificationID, got %v", err)
	}
	_, err = svc.FindByID(ctx, "id1", "", false)
	if !errors.Is(err, domain.ErrEmptyUserID) {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestService_MarkAsRead(t *testing.T) {
	ctx := context.Background()
	repo := newMockNotificationRepository()
	svc := application.NewService(repo)

	n1, _ := domain.NewNotification("id1", "user-1", nil, "evt", "agg1", "T1", "B1", nil, time.Now())
	repo.notifications["id1"] = n1

	updated, err := svc.MarkAsRead(ctx, "id1", "user-1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated.IsRead {
		t.Errorf("expected notification to be marked read")
	}
	if updated.ReadAt == nil {
		t.Errorf("expected ReadAt to be set")
	}

	// Unauthorized mark read
	_, err = svc.MarkAsRead(ctx, "id1", "user-2", false)
	if !errors.Is(err, domain.ErrUnauthorizedAccess) {
		t.Errorf("expected ErrUnauthorizedAccess, got %v", err)
	}
}

func TestService_MarkAllAsRead(t *testing.T) {
	ctx := context.Background()
	repo := newMockNotificationRepository()
	svc := application.NewService(repo)

	n1, _ := domain.NewNotification("id1", "user-1", nil, "evt", "agg1", "T1", "B1", nil, time.Now())
	n2, _ := domain.NewNotification("id2", "user-1", nil, "evt", "agg2", "T2", "B2", nil, time.Now())
	n3, _ := domain.NewNotification("id3", "user-2", nil, "evt", "agg3", "T3", "B3", nil, time.Now())
	repo.notifications["id1"] = n1
	repo.notifications["id2"] = n2
	repo.notifications["id3"] = n3

	count, err := svc.MarkAllAsRead(ctx, "user-1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 updated, got %d", count)
	}
	if !n1.IsRead || !n2.IsRead {
		t.Errorf("expected both notifications of user-1 to be marked read")
	}
	if n3.IsRead {
		t.Errorf("user-2 notification must not be marked read")
	}

	_, err = svc.MarkAllAsRead(ctx, "  ", false)
	if !errors.Is(err, domain.ErrEmptyUserID) {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}
