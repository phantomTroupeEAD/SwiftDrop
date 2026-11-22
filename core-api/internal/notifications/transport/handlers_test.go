package transport_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	identityapplication "github.com/swiftdrop/core-api/internal/identity/application"
	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/notifications/application"
	"github.com/swiftdrop/core-api/internal/notifications/domain"
	"github.com/swiftdrop/core-api/internal/notifications/transport"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

type mockRepo struct {
	items map[string]*domain.Notification
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		items: make(map[string]*domain.Notification),
	}
}

func (m *mockRepo) ListByUserID(ctx context.Context, filter application.ListFilter) ([]*domain.Notification, *application.PaginationCursor, error) {
	var list []*domain.Notification
	for _, it := range m.items {
		if it.IsForUser(filter.UserID, filter.IsDispatcher) {
			list = append(list, it)
		}
	}
	var next *application.PaginationCursor
	if len(list) > filter.Limit {
		list = list[:filter.Limit]
		last := list[len(list)-1]
		next = &application.PaginationCursor{
			CreatedAt: last.CreatedAt,
			ID:        last.ID,
		}
	}
	return list, next, nil
}

func (m *mockRepo) GetUnreadCount(ctx context.Context, userID string, isDispatcher bool) (int, error) {
	count := 0
	for _, it := range m.items {
		if it.IsForUser(userID, isDispatcher) && !it.IsRead {
			count++
		}
	}
	return count, nil
}

func (m *mockRepo) FindByID(ctx context.Context, id string) (*domain.Notification, error) {
	it, ok := m.items[id]
	if !ok {
		return nil, domain.ErrNotificationNotFound
	}
	return it, nil
}

func (m *mockRepo) MarkAsRead(ctx context.Context, id string, userID string, isDispatcher bool, readAt time.Time) (*domain.Notification, error) {
	it, ok := m.items[id]
	if !ok {
		return nil, domain.ErrNotificationNotFound
	}
	if !it.IsForUser(userID, isDispatcher) {
		return nil, domain.ErrUnauthorizedAccess
	}
	_ = it.MarkRead(readAt)
	return it, nil
}

func (m *mockRepo) MarkAllAsRead(ctx context.Context, userID string, isDispatcher bool, readAt time.Time) (int64, error) {
	var updated int64
	for _, it := range m.items {
		if it.IsForUser(userID, isDispatcher) && !it.IsRead {
			_ = it.MarkRead(readAt)
			updated++
		}
	}
	return updated, nil
}

func withAuth(r *http.Request, userID string, role identitydomain.Role) *http.Request {
	principal := &identityapplication.Principal{
		UserID: userID,
		Role:   role,
	}
	ctx := identitytransport.WithPrincipal(r.Context(), principal)
	return r.WithContext(ctx)
}

func TestHandler_ListNotifications(t *testing.T) {
	repo := newMockRepo()
	svc := application.NewService(repo)
	h := transport.NewHandler(svc)

	notif1, _ := domain.NewNotification("id-1", "user-1", nil, "parcel.created", "p-1", "Created", "Body", nil, time.Now())
	notif2, _ := domain.NewNotification("id-2", "role:DISPATCHER", nil, "dispatch.needed", "p-2", "Dispatch", "Body", nil, time.Now())
	notif3, _ := domain.NewNotification("id-3", "user-2", nil, "parcel.delivered", "p-3", "Delivered", "Body", nil, time.Now())
	repo.items["id-1"] = notif1
	repo.items["id-2"] = notif2
	repo.items["id-3"] = notif3

	// 1. Unauthorized when missing principal
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
	rec := httptest.NewRecorder()
	h.ListNotifications(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// 2. Regular user list
	req = httptest.NewRequest(http.MethodGet, "/api/v1/notifications?limit=10", nil)
	req = withAuth(req, "user-1", identitydomain.RoleCustomer)
	rec = httptest.NewRecorder()
	h.ListNotifications(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	var res httpx.CursorPaginatedResponse[transport.NotificationResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(res.Data) != 1 || res.Data[0].ID != "id-1" {
		t.Errorf("expected 1 item with id-1, got %d", len(res.Data))
	}

	// 3. Dispatcher list (includes role:DISPATCHER)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
	req = withAuth(req, "user-1", identitydomain.RoleDispatcher)
	rec = httptest.NewRecorder()
	h.ListNotifications(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(res.Data) != 2 {
		t.Errorf("expected 2 items for dispatcher, got %d", len(res.Data))
	}

	// 4. Invalid limit parameter
	req = httptest.NewRequest(http.MethodGet, "/api/v1/notifications?limit=abc", nil)
	req = withAuth(req, "user-1", identitydomain.RoleCustomer)
	rec = httptest.NewRecorder()
	h.ListNotifications(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on invalid limit, got %d", rec.Code)
	}

	// 5. Invalid cursor parameter
	req = httptest.NewRequest(http.MethodGet, "/api/v1/notifications?cursor=invalid--cursor", nil)
	req = withAuth(req, "user-1", identitydomain.RoleCustomer)
	rec = httptest.NewRecorder()
	h.ListNotifications(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on invalid cursor, got %d", rec.Code)
	}
}

func TestHandler_GetUnreadCount(t *testing.T) {
	repo := newMockRepo()
	svc := application.NewService(repo)
	h := transport.NewHandler(svc)

	notif1, _ := domain.NewNotification("id-1", "user-1", nil, "parcel.created", "p-1", "Created", "Body", nil, time.Now())
	notif2, _ := domain.NewNotification("id-2", "role:DISPATCHER", nil, "dispatch.needed", "p-2", "Dispatch", "Body", nil, time.Now())
	repo.items["id-1"] = notif1
	repo.items["id-2"] = notif2

	// Unauthorized
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
	rec := httptest.NewRecorder()
	h.GetUnreadCount(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// Customer count
	req = httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
	req = withAuth(req, "user-1", identitydomain.RoleCustomer)
	rec = httptest.NewRecorder()
	h.GetUnreadCount(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	var res httpx.SingleDataEnvelope[transport.UnreadCountResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.Data.UnreadCount != 1 {
		t.Errorf("expected 1 unread notification, got %d", res.Data.UnreadCount)
	}

	// Dispatcher count
	req = httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
	req = withAuth(req, "user-1", identitydomain.RoleDispatcher)
	rec = httptest.NewRecorder()
	h.GetUnreadCount(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.Data.UnreadCount != 2 {
		t.Errorf("expected 2 unread notifications for dispatcher, got %d", res.Data.UnreadCount)
	}
}

func TestHandler_MarkAsRead(t *testing.T) {
	repo := newMockRepo()
	svc := application.NewService(repo)
	h := transport.NewHandler(svc)

	notif1, _ := domain.NewNotification("id-1", "user-1", nil, "parcel.created", "p-1", "Created", "Body", nil, time.Now())
	notif2, _ := domain.NewNotification("id-2", "user-2", nil, "parcel.created", "p-2", "Created", "Body", nil, time.Now())
	repo.items["id-1"] = notif1
	repo.items["id-2"] = notif2

	// Unauthorized
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/notifications/id-1/read", nil)
	req.SetPathValue("id", "id-1")
	rec := httptest.NewRecorder()
	h.MarkAsRead(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// Success
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/notifications/id-1/read", nil)
	req.SetPathValue("id", "id-1")
	req = withAuth(req, "user-1", identitydomain.RoleCustomer)
	rec = httptest.NewRecorder()
	h.MarkAsRead(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rec.Code, rec.Body.String())
	}
	var res httpx.SingleDataEnvelope[transport.NotificationResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !res.Data.IsRead || res.Data.ReadAt == nil {
		t.Errorf("expected notification to be marked as read")
	}

	// Forbidden (user-1 attempting to mark user-2 notification)
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/notifications/id-2/read", nil)
	req.SetPathValue("id", "id-2")
	req = withAuth(req, "user-1", identitydomain.RoleCustomer)
	rec = httptest.NewRecorder()
	h.MarkAsRead(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", rec.Code)
	}

	// Not Found
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/notifications/nonexistent/read", nil)
	req.SetPathValue("id", "nonexistent")
	req = withAuth(req, "user-1", identitydomain.RoleCustomer)
	rec = httptest.NewRecorder()
	h.MarkAsRead(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", rec.Code)
	}

	// Missing ID in path
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/notifications//read", nil)
	req.SetPathValue("id", "")
	req = withAuth(req, "user-1", identitydomain.RoleCustomer)
	rec = httptest.NewRecorder()
	h.MarkAsRead(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestHandler_MarkAllAsRead(t *testing.T) {
	repo := newMockRepo()
	svc := application.NewService(repo)
	h := transport.NewHandler(svc)

	notif1, _ := domain.NewNotification("id-1", "user-1", nil, "parcel.created", "p-1", "Created", "Body", nil, time.Now())
	notif2, _ := domain.NewNotification("id-2", "user-1", nil, "parcel.created", "p-2", "Created", "Body", nil, time.Now())
	repo.items["id-1"] = notif1
	repo.items["id-2"] = notif2

	// Unauthorized
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/notifications/read-all", nil)
	rec := httptest.NewRecorder()
	h.MarkAllAsRead(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// Success
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/notifications/read-all", nil)
	req = withAuth(req, "user-1", identitydomain.RoleCustomer)
	rec = httptest.NewRecorder()
	h.MarkAllAsRead(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	var res httpx.SingleDataEnvelope[transport.MarkAllReadResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.Data.UpdatedCount != 2 {
		t.Errorf("expected updated count 2, got %d", res.Data.UpdatedCount)
	}
}
