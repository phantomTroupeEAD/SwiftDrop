package events

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestDeduplicator_SeenOrAdd(t *testing.T) {
	d := NewDeduplicator(100, 1*time.Minute)

	// First time seeing key
	if seen := d.SeenOrAdd("evt-1"); seen {
		t.Errorf("expected evt-1 not seen on first call")
	}

	// Second time seeing key
	if seen := d.SeenOrAdd("evt-1"); !seen {
		t.Errorf("expected evt-1 seen on second call")
	}

	// Different key
	if seen := d.SeenOrAdd("evt-2"); seen {
		t.Errorf("expected evt-2 not seen on first call")
	}

	// Seen method
	if !d.Seen("evt-1") {
		t.Errorf("expected Seen(evt-1) to be true")
	}
	if !d.Seen("evt-2") {
		t.Errorf("expected Seen(evt-2) to be true")
	}
	if d.Seen("evt-3") {
		t.Errorf("expected Seen(evt-3) to be false")
	}

	// Empty key
	if d.SeenOrAdd("") {
		t.Errorf("expected SeenOrAdd(\"\") to be false")
	}
}

func TestDeduplicator_TTL_Expiration(t *testing.T) {
	d := NewDeduplicator(100, 50*time.Millisecond)

	if seen := d.SeenOrAdd("key-ttl"); seen {
		t.Fatalf("expected key-ttl not seen initially")
	}

	time.Sleep(100 * time.Millisecond)

	// After TTL expiration, key should be considered expired
	if d.Seen("key-ttl") {
		t.Errorf("expected key-ttl to be expired in Seen()")
	}

	// Calling SeenOrAdd after TTL should refresh and return false
	if seen := d.SeenOrAdd("key-ttl"); seen {
		t.Errorf("expected key-ttl to be accepted anew after TTL expiration")
	}
}

func TestDeduplicator_CapacityEviction(t *testing.T) {
	max := 5
	d := NewDeduplicator(max, 10*time.Minute)

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("k-%d", i)
		d.SeenOrAdd(key)
	}

	if d.Len() > max {
		t.Errorf("expected count <= %d, got %d", max, d.Len())
	}

	// Recent keys should still be present
	if !d.Seen("k-9") {
		t.Errorf("expected k-9 to be present")
	}
	// Oldest key should have been evicted
	if d.Seen("k-0") {
		t.Errorf("expected k-0 to have been evicted")
	}
}

func TestDeduplicator_Concurrent(t *testing.T) {
	d := NewDeduplicator(1000, 1*time.Minute)
	var wg sync.WaitGroup

	numGoroutines := 20
	iterations := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("shared-key-%d", i%10)
				d.SeenOrAdd(key)
				_ = d.Seen(key)
			}
		}(g)
	}

	wg.Wait()
}
