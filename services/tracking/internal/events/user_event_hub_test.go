package events

import (
	"context"
	"testing"
	"time"

	"github.com/swiftdrop/tracking/internal/dto"
)

func TestUserEventHub_SubscribeAndBroadcast(t *testing.T) {
	hub := NewUserEventHub(nil)

	userID := "user-123"
	roleKey := "role:DISPATCHER"

	ch, unsub, err := hub.Subscribe(context.Background(), userID, roleKey)
	if err != nil {
		t.Fatalf("unexpected subscribe error: %v", err)
	}
	defer unsub()

	if hub.ActiveSubscriberCount() != 1 {
		t.Errorf("expected 1 active subscriber, got %d", hub.ActiveSubscriberCount())
	}

	testEvent := dto.InAppNotificationDTO{
		ID:        "notif-1",
		UserID:    userID,
		Title:     "Test Booked",
		Body:      "Your parcel is booked.",
		CreatedAt: time.Now(),
	}

	// 1. Broadcast by userID
	hub.Broadcast(userID, testEvent)

	select {
	case received := <-ch:
		if received.ID != "notif-1" || received.Title != "Test Booked" {
			t.Errorf("unexpected event received: %+v", received)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for notification on user channel")
	}

	// 2. Broadcast by roleKey
	roleEvent := dto.InAppNotificationDTO{
		ID:        "notif-2",
		UserID:    roleKey,
		Title:     "Delivery Exception",
		CreatedAt: time.Now(),
	}
	hub.Broadcast(roleKey, roleEvent)

	select {
	case received := <-ch:
		if received.ID != "notif-2" {
			t.Errorf("unexpected event received: %+v", received)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for role event on channel")
	}

	// 3. Unsubscribe cleans up
	unsub()
	if hub.ActiveSubscriberCount() != 0 {
		t.Errorf("expected 0 active subscribers after unsub, got %d", hub.ActiveSubscriberCount())
	}
}

func TestUserEventHub_BroadcastMulti_DeduplicatesClient(t *testing.T) {
	hub := NewUserEventHub(nil)

	ch, unsub, err := hub.Subscribe(context.Background(), "user-abc", "role:DISPATCHER")
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer unsub()

	ev := dto.InAppNotificationDTO{ID: "multi-1", Title: "Single Receive"}
	hub.BroadcastMulti([]string{"user-abc", "role:DISPATCHER"}, ev)

	select {
	case <-ch:
		// First received ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected message")
	}

	// Channel should have no duplicate message
	select {
	case second := <-ch:
		t.Fatalf("unexpected duplicate delivery on channel: %+v", second)
	case <-time.After(50 * time.Millisecond):
		// Clean - no duplicate
	}
}
