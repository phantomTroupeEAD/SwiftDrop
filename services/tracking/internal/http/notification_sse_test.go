package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/tracking/internal/auth"
	"github.com/swiftdrop/tracking/internal/dto"
	"github.com/swiftdrop/tracking/internal/events"
	"github.com/swiftdrop/tracking/internal/testutil"
)

type mockNotificationRepo struct {
	unreadCount int
	recent      []dto.InAppNotificationDTO
}

func (m *mockNotificationRepo) ListRecentNotifications(ctx context.Context, userID string, limit int) ([]dto.InAppNotificationDTO, error) {
	return m.recent, nil
}

func (m *mockNotificationRepo) GetUnreadNotificationCount(ctx context.Context, userID string) (int, error) {
	return m.unreadCount, nil
}

func TestNotificationSSE_MissingToken(t *testing.T) {
	verifier := auth.NewJWTVerifier("test-secret", "swiftdrop-core-api")
	hub := events.NewUserEventHub(nil)
	handler := NewNotificationSSEHandler(verifier, hub, nil, 100*time.Millisecond, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/stream", nil)
	rec := httptest.NewRecorder()

	handler.StreamNotifications(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing token, got %d", rec.Code)
	}
}

func TestNotificationSSE_InvalidToken(t *testing.T) {
	verifier := auth.NewJWTVerifier("test-secret", "swiftdrop-core-api")
	hub := events.NewUserEventHub(nil)
	handler := NewNotificationSSEHandler(verifier, hub, nil, 100*time.Millisecond, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/stream?token=invalid.jwt.token", nil)
	rec := httptest.NewRecorder()

	handler.StreamNotifications(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d", rec.Code)
	}
}

func TestNotificationSSE_StreamingAndBroadcast(t *testing.T) {
	secret := "test-secret-at-least-32-chars-long!"
	verifier := auth.NewJWTVerifier(secret, "swiftdrop-core-api")
	hub := events.NewUserEventHub(nil)
	repo := &mockNotificationRepo{
		unreadCount: 3,
		recent: []dto.InAppNotificationDTO{
			{
				ID:        "notif-prev-1",
				UserID:    "user-101",
				Title:     "Old unread",
				IsRead:    false,
				CreatedAt: time.Now(),
			},
		},
	}
	handler := NewNotificationSSEHandler(verifier, hub, repo, 50*time.Millisecond, nil)

	token, err := testutil.GenerateTestToken(secret, auth.JWTClaims{
		Issuer:    "swiftdrop-core-api",
		Subject:   "user-101",
		Role:      auth.RoleCustomer,
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/stream", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()

	// Run handler in background
	doneCh := make(chan struct{})
	go func() {
		handler.StreamNotifications(rec, req)
		close(doneCh)
	}()

	// Wait briefly for initial headers & hydration to flush
	time.Sleep(20 * time.Millisecond)

	// Broadcast an event to user-101
	hub.Broadcast("user-101", dto.InAppNotificationDTO{
		ID:        "notif-live-2",
		UserID:    "user-101",
		Title:     "New Live Delivery",
		Body:      "Your parcel is arriving.",
		CreatedAt: time.Now(),
	})

	// Wait for keepalive tick and live event
	time.Sleep(80 * time.Millisecond)

	// Cancel context to cleanly shut down SSE loop
	cancel()
	<-doneCh

	// Validate response headers
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Errorf("expected X-Accel-Buffering: no, got %s", rec.Header().Get("X-Accel-Buffering"))
	}

	body := rec.Body.String()

	// Check unread_count initial hydration event
	if !strings.Contains(body, "event: unread_count") {
		t.Errorf("missing unread_count event in stream output: %s", body)
	}

	// Check hydrated notification
	if !strings.Contains(body, "notif-prev-1") {
		t.Errorf("missing hydrated notification in stream output: %s", body)
	}

	// Check live broadcasted notification
	if !strings.Contains(body, "notif-live-2") || !strings.Contains(body, "New Live Delivery") {
		t.Errorf("missing live notification in stream output: %s", body)
	}

	// Check keepalive comments
	if !strings.Contains(body, ":keepalive") {
		t.Errorf("missing keepalive frame in stream output: %s", body)
	}
}

func TestNotificationSSE_QueryParamToken(t *testing.T) {
	secret := "test-secret-at-least-32-chars-long!"
	verifier := auth.NewJWTVerifier(secret, "swiftdrop-core-api")
	hub := events.NewUserEventHub(nil)
	handler := NewNotificationSSEHandler(verifier, hub, nil, 50*time.Millisecond, nil)

	token, _ := testutil.GenerateTestToken(secret, auth.JWTClaims{
		Issuer:    "swiftdrop-core-api",
		Subject:   "user-query-token",
		Role:      auth.RoleCustomer,
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/stream?token="+token, nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	doneCh := make(chan struct{})
	go func() {
		handler.StreamNotifications(rec, req)
		close(doneCh)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()
	<-doneCh

	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected text/event-stream Content-Type for query param token, got %s", rec.Header().Get("Content-Type"))
	}
}
