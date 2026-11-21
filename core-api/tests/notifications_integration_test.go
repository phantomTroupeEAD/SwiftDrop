package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	identityapp "github.com/swiftdrop/core-api/internal/identity/application"
	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityinfra "github.com/swiftdrop/core-api/internal/identity/infrastructure"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/notifications/application"
	"github.com/swiftdrop/core-api/internal/notifications/domain"
	"github.com/swiftdrop/core-api/internal/notifications/infrastructure"
	"github.com/swiftdrop/core-api/internal/notifications/transport"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

// inMemoryNotificationRepo provides thread-safe mock storage for integration testing.
type inMemoryNotificationRepo struct {
	mu            sync.RWMutex
	notifications map[string]*domain.Notification
}

func newInMemoryNotificationRepo() *inMemoryNotificationRepo {
	return &inMemoryNotificationRepo{
		notifications: make(map[string]*domain.Notification),
	}
}

func (r *inMemoryNotificationRepo) seed(n *domain.Notification) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifications[n.ID] = n
}

func (r *inMemoryNotificationRepo) ListByUserID(ctx context.Context, filter application.ListFilter) ([]*domain.Notification, *application.PaginationCursor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matching []*domain.Notification
	for _, n := range r.notifications {
		isPersonal := n.UserID == filter.UserID
		isBroadcast := filter.IsDispatcher && n.UserID == "role:DISPATCHER"
		if isPersonal || isBroadcast {
			matching = append(matching, n)
		}
	}

	// Sort descending: CreatedAt DESC, ID DESC
	sort.Slice(matching, func(i, j int) bool {
		if matching[i].CreatedAt.Equal(matching[j].CreatedAt) {
			return matching[i].ID > matching[j].ID
		}
		return matching[i].CreatedAt.After(matching[j].CreatedAt)
	})

	// Apply keyset cursor filter
	var filtered []*domain.Notification
	for _, n := range matching {
		if filter.Cursor != nil {
			if n.CreatedAt.After(filter.Cursor.CreatedAt) {
				continue
			}
			if n.CreatedAt.Equal(filter.Cursor.CreatedAt) && n.ID >= filter.Cursor.ID {
				continue
			}
		}
		filtered = append(filtered, n)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}

	var results []*domain.Notification
	var nextCursor *application.PaginationCursor

	if len(filtered) > limit {
		results = filtered[:limit]
		last := results[limit-1]
		nextCursor = &application.PaginationCursor{
			CreatedAt: last.CreatedAt,
			ID:        last.ID,
		}
	} else {
		results = filtered
	}

	return results, nextCursor, nil
}

func (r *inMemoryNotificationRepo) GetUnreadCount(ctx context.Context, userID string, isDispatcher bool) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, n := range r.notifications {
		isPersonal := n.UserID == userID
		isBroadcast := isDispatcher && n.UserID == "role:DISPATCHER"
		if (isPersonal || isBroadcast) && !n.IsRead {
			count++
		}
	}
	return count, nil
}

func (r *inMemoryNotificationRepo) FindByID(ctx context.Context, id string) (*domain.Notification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	n, ok := r.notifications[id]
	if !ok {
		return nil, domain.ErrNotificationNotFound
	}
	return n, nil
}

func (r *inMemoryNotificationRepo) MarkAsRead(ctx context.Context, id string, userID string, isDispatcher bool, readAt time.Time) (*domain.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n, ok := r.notifications[id]
	if !ok {
		return nil, domain.ErrNotificationNotFound
	}

	canAccess := n.UserID == userID || (isDispatcher && n.UserID == "role:DISPATCHER")
	if !canAccess {
		return nil, domain.ErrUnauthorizedAccess
	}

	if err := n.MarkRead(readAt); err != nil {
		return nil, err
	}
	return n, nil
}

func (r *inMemoryNotificationRepo) MarkAllAsRead(ctx context.Context, userID string, isDispatcher bool, readAt time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var updated int64
	for _, n := range r.notifications {
		canAccess := n.UserID == userID || (isDispatcher && n.UserID == "role:DISPATCHER")
		if canAccess && !n.IsRead {
			_ = n.MarkRead(readAt)
			updated++
		}
	}
	return updated, nil
}

func buildNotificationTestRouter(repo application.NotificationRepository) (http.Handler, *identityinfra.JWTTokenProvider) {
	mux := http.NewServeMux()
	tokenProv := identityinfra.NewJWTTokenProvider("test-integration-secret-key-32chars")
	sec := identitytransport.NewSecurityMiddleware(tokenProv)

	svc := application.NewService(repo)
	handler := transport.NewHandler(svc)
	transport.RegisterRoutes(mux, handler, sec)

	return mux, tokenProv
}

// TestNotifications_EndToEndFlow verifies comprehensive end-to-end routing, user isolation,
// operational broadcast triage, keyset pagination, unread counters, and IDOR protections.
func TestNotifications_EndToEndFlow(t *testing.T) {
	repo := newInMemoryNotificationRepo()
	router, tokenProv := buildNotificationTestRouter(repo)

	baseTime := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	tenantID := "tenant-alpha"

	// Seed notifications:
	// Customer 1: 3 notifications (t+10s, t+20s, t+30s)
	cust1N1, _ := domain.NewNotification("0192e2b3-7c8a-7890-a123-000000000001", "usr_cust_1", &tenantID, "parcel.booked.v1", "prcl-01", "Parcel Booked", "Your parcel SD-01 is booked.", map[string]any{"tracking_number": "SD-01"}, baseTime.Add(10*time.Second))
	cust1N2, _ := domain.NewNotification("0192e2b3-7c8a-7890-a123-000000000002", "usr_cust_1", &tenantID, "parcel.picked_up.v1", "prcl-01", "Parcel Picked Up", "Your parcel SD-01 was picked up.", map[string]any{"tracking_number": "SD-01"}, baseTime.Add(20*time.Second))
	cust1N3, _ := domain.NewNotification("0192e2b3-7c8a-7890-a123-000000000003", "usr_cust_1", &tenantID, "parcel.out_for_delivery.v1", "prcl-01", "Out for Delivery", "Your parcel SD-01 is out for delivery.", map[string]any{"tracking_number": "SD-01"}, baseTime.Add(30*time.Second))

	// Courier: 1 notification
	courierN1, _ := domain.NewNotification("0192e2b3-7c8a-7890-a123-000000000010", "usr_courier_1", &tenantID, "parcel.assigned.v1", "prcl-01", "New Assignment", "You have a new parcel.", map[string]any{"tracking_number": "SD-01"}, baseTime.Add(15*time.Second))

	// Dispatcher Broadcast: 2 operational exception notifications
	dispB1, _ := domain.NewNotification("0192e2b3-7c8a-7890-a123-000000000020", "role:DISPATCHER", &tenantID, "parcel.delivery_failed.v1", "prcl-02", "Delivery Failed", "Delivery failed for SD-02.", map[string]any{"tracking_number": "SD-02", "reason": "CUSTOMER_UNAVAILABLE"}, baseTime.Add(25*time.Second))
	dispB2, _ := domain.NewNotification("0192e2b3-7c8a-7890-a123-000000000021", "role:DISPATCHER", &tenantID, "parcel.returned.v1", "prcl-02", "Parcel Returned", "Parcel SD-02 returned.", map[string]any{"tracking_number": "SD-02"}, baseTime.Add(35*time.Second))

	repo.seed(cust1N1)
	repo.seed(cust1N2)
	repo.seed(cust1N3)
	repo.seed(courierN1)
	repo.seed(dispB1)
	repo.seed(dispB2)

	// Issue Tokens
	cust1Token, err := tokenProv.GenerateAccessToken(identityapp.Principal{UserID: "usr_cust_1", Role: identitydomain.RoleCustomer, TenantID: &tenantID})
	require.NoError(t, err)

	cust2Token, err := tokenProv.GenerateAccessToken(identityapp.Principal{UserID: "usr_cust_2", Role: identitydomain.RoleCustomer, TenantID: &tenantID})
	require.NoError(t, err)

	courierToken, err := tokenProv.GenerateAccessToken(identityapp.Principal{UserID: "usr_courier_1", Role: identitydomain.RoleCourier, TenantID: &tenantID})
	require.NoError(t, err)

	dispatcherToken, err := tokenProv.GenerateAccessToken(identityapp.Principal{UserID: "usr_disp_1", Role: identitydomain.RoleDispatcher, TenantID: &tenantID})
	require.NoError(t, err)

	// Step 1: Authentication Requirement Check
	t.Run("401 Unauthorized when missing credentials", func(t *testing.T) {
		endpoints := []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/api/v1/notifications"},
			{http.MethodGet, "/api/v1/notifications/unread-count"},
			{http.MethodPatch, "/api/v1/notifications/0192e2b3-7c8a-7890-a123-000000000001/read"},
			{http.MethodPatch, "/api/v1/notifications/read-all"},
		}

		for _, ep := range endpoints {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusUnauthorized, rec.Code, "endpoint %s %s must require auth", ep.method, ep.path)
		}
	})

	// Step 2: Tenant & User Isolation
	t.Run("Customer 1 sees only own notifications and gets correct unread count", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
		req.Header.Set("Authorization", "Bearer "+cust1Token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var listResp struct {
			Data []transport.NotificationResponse `json:"data"`
			Meta httpx.CursorMeta                 `json:"meta"`
		}
		err := json.NewDecoder(rec.Body).Decode(&listResp)
		require.NoError(t, err)

		// Cust 1 should see exactly 3 notifications ordered newest first (N3, N2, N1)
		require.Len(t, listResp.Data, 3)
		assert.Equal(t, cust1N3.ID, listResp.Data[0].ID)
		assert.Equal(t, cust1N2.ID, listResp.Data[1].ID)
		assert.Equal(t, cust1N1.ID, listResp.Data[2].ID)

		// Verify unread count endpoint
		countReq := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
		countReq.Header.Set("Authorization", "Bearer "+cust1Token)
		countRec := httptest.NewRecorder()
		router.ServeHTTP(countRec, countReq)

		require.Equal(t, http.StatusOK, countRec.Code)
		var countResp struct {
			Data transport.UnreadCountResponse `json:"data"`
		}
		err = json.NewDecoder(countRec.Body).Decode(&countResp)
		require.NoError(t, err)
		assert.Equal(t, 3, countResp.Data.UnreadCount)
	})

	t.Run("Customer 2 has empty inbox and zero unread count", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
		req.Header.Set("Authorization", "Bearer "+cust2Token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var listResp struct {
			Data []transport.NotificationResponse `json:"data"`
			Meta httpx.CursorMeta                 `json:"meta"`
		}
		err := json.NewDecoder(rec.Body).Decode(&listResp)
		require.NoError(t, err)
		assert.Empty(t, listResp.Data)

		countReq := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
		countReq.Header.Set("Authorization", "Bearer "+cust2Token)
		countRec := httptest.NewRecorder()
		router.ServeHTTP(countRec, countReq)

		require.Equal(t, http.StatusOK, countRec.Code)
		var countResp struct {
			Data transport.UnreadCountResponse `json:"data"`
		}
		err = json.NewDecoder(countRec.Body).Decode(&countResp)
		require.NoError(t, err)
		assert.Equal(t, 0, countResp.Data.UnreadCount)
	})

	// Step 3: Keyset Cursor Pagination
	t.Run("Pagination divides items into deterministic non-overlapping pages", func(t *testing.T) {
		// Request page 1 with limit=2
		req1 := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?limit=2", nil)
		req1.Header.Set("Authorization", "Bearer "+cust1Token)
		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, req1)

		require.Equal(t, http.StatusOK, rec1.Code)
		var page1 struct {
			Data []transport.NotificationResponse `json:"data"`
			Meta httpx.CursorMeta                 `json:"meta"`
		}
		require.NoError(t, json.NewDecoder(rec1.Body).Decode(&page1))
		require.Len(t, page1.Data, 2)
		assert.True(t, page1.Meta.HasMore)
		require.NotNil(t, page1.Meta.NextCursor)
		assert.Equal(t, cust1N3.ID, page1.Data[0].ID)
		assert.Equal(t, cust1N2.ID, page1.Data[1].ID)

		// Request page 2 using returned NextCursor
		req2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/notifications?limit=2&cursor=%s", *page1.Meta.NextCursor), nil)
		req2.Header.Set("Authorization", "Bearer "+cust1Token)
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)

		require.Equal(t, http.StatusOK, rec2.Code)
		var page2 struct {
			Data []transport.NotificationResponse `json:"data"`
			Meta httpx.CursorMeta                 `json:"meta"`
		}
		require.NoError(t, json.NewDecoder(rec2.Body).Decode(&page2))
		require.Len(t, page2.Data, 1)
		assert.False(t, page2.Meta.HasMore)
		assert.Nil(t, page2.Meta.NextCursor)
		assert.Equal(t, cust1N1.ID, page2.Data[0].ID)
	})

	// Step 4: Mark Single Notification as Read
	t.Run("Customer marks single notification as read", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/notifications/%s/read", cust1N3.ID), nil)
		req.Header.Set("Authorization", "Bearer "+cust1Token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var patchResp struct {
			Data transport.NotificationResponse `json:"data"`
		}
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&patchResp))
		assert.True(t, patchResp.Data.IsRead)
		assert.NotNil(t, patchResp.Data.ReadAt)

		// Unread count should now decrease from 3 to 2
		countReq := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
		countReq.Header.Set("Authorization", "Bearer "+cust1Token)
		countRec := httptest.NewRecorder()
		router.ServeHTTP(countRec, countReq)

		var countResp struct {
			Data transport.UnreadCountResponse `json:"data"`
		}
		require.NoError(t, json.NewDecoder(countRec.Body).Decode(&countResp))
		assert.Equal(t, 2, countResp.Data.UnreadCount)
	})

	// Step 5: IDOR Access Protection
	t.Run("Customer cannot mark another user's notification as read", func(t *testing.T) {
		// Customer 1 attempts to mark Courier's notification as read
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/notifications/%s/read", courierN1.ID), nil)
		req.Header.Set("Authorization", "Bearer "+cust1Token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Must be rejected with 403 Forbidden or 404 Not Found to prevent enumeration
		assert.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, rec.Code)

		// Courier unread count remains 1
		countReq := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
		countReq.Header.Set("Authorization", "Bearer "+courierToken)
		countRec := httptest.NewRecorder()
		router.ServeHTTP(countRec, countReq)

		var countResp struct {
			Data transport.UnreadCountResponse `json:"data"`
		}
		require.NoError(t, json.NewDecoder(countRec.Body).Decode(&countResp))
		assert.Equal(t, 1, countResp.Data.UnreadCount)
	})

	// Step 6: Mark All Notifications as Read
	t.Run("Customer marks all notifications as read", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/notifications/read-all", nil)
		req.Header.Set("Authorization", "Bearer "+cust1Token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var markAllResp struct {
			Data transport.MarkAllReadResponse `json:"data"`
		}
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&markAllResp))
		assert.Equal(t, int64(2), markAllResp.Data.UpdatedCount)

		// Unread count should now be 0
		countReq := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
		countReq.Header.Set("Authorization", "Bearer "+cust1Token)
		countRec := httptest.NewRecorder()
		router.ServeHTTP(countRec, countReq)

		var countResp struct {
			Data transport.UnreadCountResponse `json:"data"`
		}
		require.NoError(t, json.NewDecoder(countRec.Body).Decode(&countResp))
		assert.Equal(t, 0, countResp.Data.UnreadCount)
	})

	// Step 7: Operational Dispatcher Broadcast Triage
	t.Run("Dispatcher sees operational broadcasts in inbox alongside unread count", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
		req.Header.Set("Authorization", "Bearer "+dispatcherToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var listResp struct {
			Data []transport.NotificationResponse `json:"data"`
			Meta httpx.CursorMeta                 `json:"meta"`
		}
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&listResp))

		// Dispatcher should see the 2 broadcast notifications (dispB2, dispB1)
		require.Len(t, listResp.Data, 2)
		assert.Equal(t, dispB2.ID, listResp.Data[0].ID)
		assert.Equal(t, dispB1.ID, listResp.Data[1].ID)

		// Dispatcher marks one broadcast notification as read
		patchReq := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/notifications/%s/read", dispB2.ID), nil)
		patchReq.Header.Set("Authorization", "Bearer "+dispatcherToken)
		patchRec := httptest.NewRecorder()
		router.ServeHTTP(patchRec, patchReq)
		require.Equal(t, http.StatusOK, patchRec.Code)

		// Dispatcher unread count becomes 1
		countReq := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
		countReq.Header.Set("Authorization", "Bearer "+dispatcherToken)
		countRec := httptest.NewRecorder()
		router.ServeHTTP(countRec, countReq)

		var countResp struct {
			Data transport.UnreadCountResponse `json:"data"`
		}
		require.NoError(t, json.NewDecoder(countRec.Body).Decode(&countResp))
		assert.Equal(t, 1, countResp.Data.UnreadCount)
	})
}

// TestNotifications_PostgresIntegration validates real database integration against
// the user_notifications PostgreSQL schema (requires INTEGRATION=1).
func TestNotifications_PostgresIntegration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := infrastructure.NewPostgresRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	tenantID := "tenant-live-01"

	// 1. Direct SQL insert simulating outbox processing into user_notifications
	insertStmt := `
		INSERT INTO user_notifications (
			id, user_id, tenant_id, event_type, aggregate_id, title, body, metadata, is_read, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10
		)
	`

	id1 := uuid.New().String()
	id2 := uuid.New().String()
	idBroadcast := uuid.New().String()

	_, err := db.ExecContext(ctx, insertStmt, id1, "usr_db_test_1", tenantID, "parcel.booked.v1", uuid.New().String(), "Parcel Booked", "Booking confirmed", `{"tracking_number":"SD-TEST-1"}`, false, now.Add(-10*time.Minute))
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, insertStmt, id2, "usr_db_test_1", tenantID, "parcel.delivered.v1", uuid.New().String(), "Parcel Delivered", "Delivered safely", `{"tracking_number":"SD-TEST-1"}`, false, now.Add(-5*time.Minute))
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, insertStmt, idBroadcast, "role:DISPATCHER", tenantID, "parcel.delivery_failed.v1", uuid.New().String(), "Delivery Failed", "Failed attempt", `{"tracking_number":"SD-TEST-2"}`, false, now.Add(-2*time.Minute))
	require.NoError(t, err)

	t.Run("PostgreSQL: List and Count Unread", func(t *testing.T) {
		items, cursor, err := repo.ListByUserID(ctx, application.ListFilter{
			UserID:       "usr_db_test_1",
			IsDispatcher: false,
			Limit:        10,
		})
		require.NoError(t, err)
		require.Len(t, items, 2)
		assert.Equal(t, id2, items[0].ID) // Newest first
		assert.Equal(t, id1, items[1].ID)
		assert.Nil(t, cursor)

		count, err := repo.GetUnreadCount(ctx, "usr_db_test_1", false)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("PostgreSQL: Keyset Cursor Pagination", func(t *testing.T) {
		page1, cursor1, err := repo.ListByUserID(ctx, application.ListFilter{
			UserID:       "usr_db_test_1",
			IsDispatcher: false,
			Limit:        1,
		})
		require.NoError(t, err)
		require.Len(t, page1, 1)
		assert.Equal(t, id2, page1[0].ID)
		require.NotNil(t, cursor1)

		page2, cursor2, err := repo.ListByUserID(ctx, application.ListFilter{
			UserID:       "usr_db_test_1",
			IsDispatcher: false,
			Cursor:       cursor1,
			Limit:        1,
		})
		require.NoError(t, err)
		require.Len(t, page2, 1)
		assert.Equal(t, id1, page2[0].ID)
		assert.Nil(t, cursor2)
	})

	t.Run("PostgreSQL: Mark as Read & Mark All", func(t *testing.T) {
		readAt := time.Now().UTC()
		marked, err := repo.MarkAsRead(ctx, id2, "usr_db_test_1", false, readAt)
		require.NoError(t, err)
		assert.True(t, marked.IsRead)

		count, err := repo.GetUnreadCount(ctx, "usr_db_test_1", false)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		updated, err := repo.MarkAllAsRead(ctx, "usr_db_test_1", false, readAt)
		require.NoError(t, err)
		assert.Equal(t, int64(1), updated)

		count, err = repo.GetUnreadCount(ctx, "usr_db_test_1", false)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("PostgreSQL: Dispatcher operational broadcast inclusion", func(t *testing.T) {
		items, _, err := repo.ListByUserID(ctx, application.ListFilter{
			UserID:       "usr_db_dispatcher",
			IsDispatcher: true,
			Limit:        10,
		})
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, idBroadcast, items[0].ID)

		count, err := repo.GetUnreadCount(ctx, "usr_db_dispatcher", true)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}
