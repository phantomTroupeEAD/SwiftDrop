package events

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/swiftdrop/tracking/internal/dto"
)

func TestEventHub_SubscribeAndBroadcast(t *testing.T) {
	hub := NewEventHub()
	tn := "SD-20261025-A8F3K2"

	ch, unsub, err := hub.Subscribe(context.Background(), tn)
	if err != nil {
		t.Fatalf("unexpected subscribe error: %v", err)
	}
	defer unsub()

	if hub.SubscriberCount(tn) != 1 {
		t.Errorf("expected 1 subscriber for %s, got %d", tn, hub.SubscriberCount(tn))
	}
	if hub.TotalSubscribers() != 1 {
		t.Errorf("expected 1 total subscriber, got %d", hub.TotalSubscribers())
	}

	event := dto.PublicStatusUpdateEvent{
		TrackingNumber: tn,
		Status:         "IN_TRANSIT",
		Description:    "Parcel in transit",
		Timestamp:      time.Now().UTC(),
	}

	delivered := hub.Broadcast(event)
	if delivered != 1 {
		t.Fatalf("expected 1 delivered event, got %d", delivered)
	}

	select {
	case received := <-ch:
		if received.Status != "IN_TRANSIT" {
			t.Errorf("expected status 'IN_TRANSIT', got %q", received.Status)
		}
	default:
		t.Fatal("expected event in subscriber channel, but channel was empty")
	}

	// Unsubscribe and verify cleanup
	unsub()
	if hub.SubscriberCount(tn) != 0 {
		t.Errorf("expected 0 subscribers after unsub, got %d", hub.SubscriberCount(tn))
	}
	if hub.TotalSubscribers() != 0 {
		t.Errorf("expected 0 total subscribers after unsub, got %d", hub.TotalSubscribers())
	}

	// Double unsubscribe should be idempotent
	unsub()
}

func TestEventHub_BackpressureNonBlocking(t *testing.T) {
	// Buffer size 2
	hub := NewEventHub(WithBufferSize(2))
	tn := "SD-20261025-A8F3K2"

	ch, unsub, err := hub.Subscribe(context.Background(), tn)
	if err != nil {
		t.Fatalf("unexpected subscribe error: %v", err)
	}
	defer unsub()

	event := dto.PublicStatusUpdateEvent{
		TrackingNumber: tn,
		Status:         "OUT_FOR_DELIVERY",
		Timestamp:      time.Now().UTC(),
	}

	// Fill buffer of 2
	if d := hub.Broadcast(event); d != 1 {
		t.Errorf("expected first broadcast delivered, got %d", d)
	}
	if d := hub.Broadcast(event); d != 1 {
		t.Errorf("expected second broadcast delivered, got %d", d)
	}

	// 3rd broadcast should drop without blocking and close stalled channel to trigger resync
	if d := hub.Broadcast(event); d != 0 {
		t.Errorf("expected 3rd broadcast dropped due to full buffer, got %d delivered", d)
	}

	// Drain the 2 buffered events
	<-ch
	<-ch

	// Verify channel was closed upon buffer overflow to trigger client reconnect
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected subscriber channel to be closed on buffer overflow")
		}
	default:
		t.Fatal("expected closed channel signal")
	}
}

func TestEventHub_SubscriptionLimits(t *testing.T) {
	hub := NewEventHub(WithMaxSubscribersPerKey(2), WithMaxTotalSubscribers(3))
	tn1 := "SD-20261025-A8F3K2"
	tn2 := "SD-20261025-B9E4L3"

	// 2 subs on tn1
	_, unsub1, err := hub.Subscribe(context.Background(), tn1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer unsub1()

	_, unsub2, err := hub.Subscribe(context.Background(), tn1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer unsub2()

	// 3rd sub on tn1 exceeds per-key limit
	_, _, err = hub.Subscribe(context.Background(), tn1)
	if !errors.Is(err, ErrSubscriptionLimitExceeded) {
		t.Fatalf("expected ErrSubscriptionLimitExceeded, got: %v", err)
	}

	// 1 sub on tn2 brings total to 3
	_, unsub3, err := hub.Subscribe(context.Background(), tn2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer unsub3()

	// 4th sub exceeds global limit
	_, _, err = hub.Subscribe(context.Background(), tn2)
	if !errors.Is(err, ErrSubscriptionLimitExceeded) {
		t.Fatalf("expected ErrSubscriptionLimitExceeded for total limit, got: %v", err)
	}
}

func TestEventHub_AvailabilityAndClose(t *testing.T) {
	hub := NewEventHub()
	tn := "SD-20261025-A8F3K2"

	hub.SetAvailable(false)
	if hub.IsAvailable() {
		t.Fatal("expected hub to report unavailable")
	}

	_, _, err := hub.Subscribe(context.Background(), tn)
	if !errors.Is(err, ErrEventSourceUnavailable) {
		t.Fatalf("expected ErrEventSourceUnavailable, got: %v", err)
	}

	hub.SetAvailable(true)
	ch, unsub, err := hub.Subscribe(context.Background(), tn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer unsub()

	// Closing hub should close all active subscriber channels
	if err := hub.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}

	if hub.IsAvailable() {
		t.Fatal("expected closed hub to be unavailable")
	}

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected subscriber channel to be closed upon hub Close")
		}
	default:
		t.Fatal("expected closed channel read")
	}

	// Subscribing to closed hub should return ErrEventSourceClosed
	_, _, err = hub.Subscribe(context.Background(), tn)
	if !errors.Is(err, ErrEventSourceClosed) {
		t.Fatalf("expected ErrEventSourceClosed, got: %v", err)
	}
}

func TestEventHub_ConcurrentAccess(t *testing.T) {
	hub := NewEventHub(WithMaxSubscribersPerKey(200), WithMaxTotalSubscribers(1000))
	tn := "SD-20261025-A8F3K2"

	const clientCount = 50
	var wg sync.WaitGroup
	wg.Add(clientCount)

	unsubs := make([]func(), clientCount)
	for i := 0; i < clientCount; i++ {
		idx := i
		go func() {
			defer wg.Done()
			_, unsub, err := hub.Subscribe(context.Background(), tn)
			if err != nil {
				t.Errorf("subscriber %d error: %v", idx, err)
				return
			}
			unsubs[idx] = unsub
		}()
	}
	wg.Wait()

	if hub.SubscriberCount(tn) != clientCount {
		t.Fatalf("expected %d subscribers, got %d", clientCount, hub.SubscriberCount(tn))
	}

	// Concurrent broadcast
	delivered := hub.Broadcast(dto.PublicStatusUpdateEvent{
		TrackingNumber: tn,
		Status:         "DELIVERED",
		Timestamp:      time.Now().UTC(),
	})
	if delivered != clientCount {
		t.Errorf("expected %d deliveries, got %d", clientCount, delivered)
	}

	// Concurrent unsubscribe
	wg.Add(clientCount)
	for i := 0; i < clientCount; i++ {
		idx := i
		go func() {
			defer wg.Done()
			if unsubs[idx] != nil {
				unsubs[idx]()
			}
		}()
	}
	wg.Wait()

	if hub.SubscriberCount(tn) != 0 {
		t.Fatalf("expected 0 subscribers after all unsubs, got %d", hub.SubscriberCount(tn))
	}
}
