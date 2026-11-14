package events

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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

func TestEventHub_SlowSubscriberNoPanicAndCleanup(t *testing.T) {
	hub := NewEventHub(WithBufferSize(1))
	tn := "SD-20261025-A8F3K2"

	ch, unsub, err := hub.Subscribe(context.Background(), tn)
	if err != nil {
		t.Fatalf("unexpected subscribe error: %v", err)
	}
	defer unsub()

	event := dto.PublicStatusUpdateEvent{
		TrackingNumber: tn,
		Status:         "IN_TRANSIT",
		Timestamp:      time.Now().UTC(),
	}

	// First broadcast fills buffer (1/1)
	if d := hub.Broadcast(event); d != 1 {
		t.Fatalf("expected 1 delivery, got %d", d)
	}

	// Second broadcast overflows buffer, marks subscriber closed and closes channel
	if d := hub.Broadcast(event); d != 0 {
		t.Fatalf("expected 0 delivery on overflow, got %d", d)
	}

	// Verify channel is closed
	<-ch // drain buffered
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel closed")
		}
	default:
		t.Fatal("expected closed channel")
	}

	// Third broadcast occurs while slow subscriber is still in hub (before unsub is called)
	// MUST NOT panic on send to closed channel!
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("Broadcast panicked on closed subscriber channel: %v", rec)
		}
	}()

	d := hub.Broadcast(event)
	if d != 0 {
		t.Errorf("expected 0 deliveries to closed subscriber, got %d", d)
	}

	// Verify slow subscriber cleanup pruned the closed subscriber
	if hub.SubscriberCount(tn) != 0 {
		t.Errorf("expected closed subscriber to be cleaned up from hub, got %d", hub.SubscriberCount(tn))
	}
	if hub.TotalSubscribers() != 0 {
		t.Errorf("expected TotalSubscribers 0 after cleanup, got %d", hub.TotalSubscribers())
	}
}

func TestEventHub_ConcurrentBroadcastCloseUnsubscribe(t *testing.T) {
	hub := NewEventHub(WithBufferSize(4), WithMaxSubscribersPerKey(500), WithMaxTotalSubscribers(1000))
	tn := "SD-20261110-CONC01"

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	clientCount := 30
	broadcastCount := 100

	var panicCount atomic.Int64

	// Start subscriber goroutines that rapidly subscribe, read events, and unsubscribe
	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
					t.Errorf("subscriber goroutine %d panicked: %v", id, r)
				}
			}()

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				ch, unsub, err := hub.Subscribe(ctx, tn)
				if err != nil {
					time.Sleep(time.Millisecond)
					continue
				}

				// Consume channel asynchronously
				doneReading := make(chan struct{})
				go func() {
					defer close(doneReading)
					for range ch {
						// consume
					}
				}()

				time.Sleep(time.Duration(id%5) * time.Millisecond)
				unsub()
				<-doneReading
			}
		}(i)
	}

	// Start concurrent broadcasters
	for b := 0; b < 4; b++ {
		wg.Add(1)
		go func(bid int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
					t.Errorf("broadcaster goroutine %d panicked: %v", bid, r)
				}
			}()

			for j := 0; j < broadcastCount; j++ {
				hub.Broadcast(dto.PublicStatusUpdateEvent{
					TrackingNumber: tn,
					Status:         "IN_TRANSIT",
					Timestamp:      time.Now().UTC(),
				})
				time.Sleep(500 * time.Microsecond)
			}
		}(b)
	}

	time.Sleep(50 * time.Millisecond)
	_ = hub.Close()

	cancel()
	wg.Wait()

	if panicCount.Load() > 0 {
		t.Fatalf("Encountered %d panics during concurrent operations", panicCount.Load())
	}
}

func BenchmarkEventHub_Broadcast_FanOut(b *testing.B) {
	for _, subCount := range []int{1, 10, 50, 100} {
		b.Run(fmt.Sprintf("subscribers_%d", subCount), func(b *testing.B) {
			hub := NewEventHub(WithBufferSize(b.N+100), WithMaxSubscribersPerKey(200))
			tn := "SD-20261110-BENCH1"

			unsubs := make([]func(), subCount)
			for i := 0; i < subCount; i++ {
				_, unsub, err := hub.Subscribe(context.Background(), tn)
				if err != nil {
					b.Fatalf("failed to subscribe: %v", err)
				}
				unsubs[i] = unsub
			}
			defer func() {
				for _, u := range unsubs {
					u()
				}
			}()

			event := dto.PublicStatusUpdateEvent{
				TrackingNumber: tn,
				Status:         "IN_TRANSIT",
				Timestamp:      time.Now().UTC(),
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				hub.Broadcast(event)
			}
		})
	}
}
