package http

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/swiftdrop/tracking/internal/config"
	"github.com/swiftdrop/tracking/internal/domain"
	"github.com/swiftdrop/tracking/internal/dto"
	"github.com/swiftdrop/tracking/internal/events"
	"github.com/swiftdrop/tracking/internal/health"
	"github.com/swiftdrop/tracking/internal/service"
)

type nonFlusherResponseWriter struct {
	header http.Header
	code   int
	body   []byte
}

func (w *nonFlusherResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *nonFlusherResponseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *nonFlusherResponseWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

func setupTestSSEInfrastructure(
	mockRepo *mockRepoForHTTP,
	hub events.TrackingEventSource,
	heartbeat time.Duration,
) (http.Handler, *config.Config) {
	cfg := &config.Config{
		HTTPAddr:            ":8081",
		AppEnv:              "test",
		ShutdownTimeout:     5 * time.Second,
		ReadTimeout:         5 * time.Second,
		WriteTimeout:        30 * time.Second,
		IdleTimeout:         60 * time.Second,
		LogLevel:            "info",
		SSEHeartbeatSeconds: 15,
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	healthHandler := health.NewHandler()

	trackingSvc := service.NewPublicTrackingService(mockRepo)
	trackingHandler := NewPublicTrackingHandler(trackingSvc, logger)
	sseHandler := NewSSEHandler(trackingSvc, hub, heartbeat, logger)

	router := NewRouter(cfg, logger, healthHandler, trackingHandler, sseHandler)
	return router, cfg
}

func validTestParcelInfo(tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord) {
	trackingNum, _ := domain.NewTrackingNumber(tn)
	baseTime := time.Date(2026, 10, 25, 8, 30, 0, 0, time.UTC)

	parcel := &domain.ParcelTrackingInfo{
		ID:              "prcl-01HXYZ1234567890",
		TrackingNumber:  trackingNum,
		Status:          domain.StatusOutForDelivery,
		DestinationCity: "Colombo",
		CreatedAt:       baseTime,
		UpdatedAt:       baseTime.Add(5 * time.Hour),
	}

	history := []domain.MilestoneRecord{
		{
			ID:        "ms-001",
			ParcelID:  parcel.ID,
			ToStatus:  string(domain.StatusCreated),
			Timestamp: baseTime,
		},
		{
			ID:        "ms-002",
			ParcelID:  parcel.ID,
			ToStatus:  string(domain.StatusPickedUp),
			Timestamp: baseTime.Add(2 * time.Hour),
		},
		{
			ID:        "ms-003",
			ParcelID:  parcel.ID,
			ToStatus:  string(domain.StatusInTransit),
			Timestamp: baseTime.Add(4 * time.Hour),
		},
		{
			ID:        "ms-004",
			ParcelID:  parcel.ID,
			ToStatus:  string(domain.StatusOutForDelivery),
			Timestamp: baseTime.Add(5 * time.Hour),
		},
	}

	return parcel, history
}

func readSSEFrame(reader *bufio.Reader) (map[string]string, error) {
	frame := make(map[string]string)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")

		// Empty line marks end of an SSE frame
		if line == "" {
			if len(frame) > 0 {
				return frame, nil
			}
			continue
		}

		// Comment line (e.g., :keepalive)
		if strings.HasPrefix(line, ":") {
			frame["comment"] = strings.TrimPrefix(line, ":")
			return frame, nil
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			frame[key] = val
		}
	}
}

// 1. SSE Response Headers & Initial State Hydration
func TestSSE_ResponseHeadersAndInitialState(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	parcel, history := validTestParcelInfo(tn)

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	// Verify SSE Response Headers
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %q", contentType)
	}
	if cacheControl := resp.Header.Get("Cache-Control"); !strings.Contains(cacheControl, "no-cache") {
		t.Errorf("expected Cache-Control no-cache, got %q", cacheControl)
	}
	if connection := resp.Header.Get("Connection"); connection != "keep-alive" {
		t.Errorf("expected Connection keep-alive, got %q", connection)
	}
	if accelBuffering := resp.Header.Get("X-Accel-Buffering"); accelBuffering != "no" {
		t.Errorf("expected X-Accel-Buffering no, got %q", accelBuffering)
	}

	// Verify Initial Hydration Frame
	reader := bufio.NewReader(resp.Body)
	frame, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read initial SSE frame: %v", err)
	}

	if frame["event"] != "status_update" {
		t.Errorf("expected event 'status_update', got %q", frame["event"])
	}

	expectedIDPrefix := tn + ":OUT_FOR_DELIVERY:"
	if !strings.HasPrefix(frame["id"], expectedIDPrefix) {
		t.Errorf("expected id prefix %q, got %q", expectedIDPrefix, frame["id"])
	}

	var payload dto.PublicStatusUpdateEvent
	if err := json.Unmarshal([]byte(frame["data"]), &payload); err != nil {
		t.Fatalf("failed to unmarshal initial event data JSON: %v", err)
	}

	if payload.TrackingNumber != tn {
		t.Errorf("expected tracking number %s, got %s", tn, payload.TrackingNumber)
	}
	if payload.Status != "OUT_FOR_DELIVERY" {
		t.Errorf("expected status OUT_FOR_DELIVERY, got %s", payload.Status)
	}
	if payload.Description != "Courier is out for final doorstep delivery." {
		t.Errorf("expected safe description, got %q", payload.Description)
	}
}

// 2. Real-Time Event Fan-Out via Hub
func TestSSE_RealTimeBroadcastEvent(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	parcel, history := validTestParcelInfo(tn)

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	// Read initial hydration frame
	_, err = readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read initial frame: %v", err)
	}

	// Broadcast live update via Hub
	eventTime := time.Date(2026, 10, 25, 14, 30, 0, 0, time.UTC)
	update := dto.PublicStatusUpdateEvent{
		TrackingNumber: tn,
		Status:         "DELIVERED",
		Description:    "Parcel successfully delivered to recipient.",
		Timestamp:      eventTime,
	}

	delivered := hub.Broadcast(update)
	if delivered != 1 {
		t.Fatalf("expected 1 delivery, got %d", delivered)
	}

	// Read live broadcast frame from stream
	liveFrame, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read live broadcast frame: %v", err)
	}

	if liveFrame["event"] != "status_update" {
		t.Errorf("expected event 'status_update', got %q", liveFrame["event"])
	}

	expectedID := fmt.Sprintf("%s:DELIVERED:%d", tn, eventTime.Unix())
	if liveFrame["id"] != expectedID {
		t.Errorf("expected id %q, got %q", expectedID, liveFrame["id"])
	}

	var livePayload dto.PublicStatusUpdateEvent
	if err := json.Unmarshal([]byte(liveFrame["data"]), &livePayload); err != nil {
		t.Fatalf("failed to unmarshal live payload: %v", err)
	}
	if livePayload.Status != "DELIVERED" {
		t.Errorf("expected status DELIVERED, got %s", livePayload.Status)
	}
}

// 3. Heartbeat Keepalive Comment Frames
func TestSSE_HeartbeatKeepalive(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	parcel, history := validTestParcelInfo(tn)

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	// Use short deterministic heartbeat of 25ms for testing
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 25*time.Millisecond)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	// Initial hydration frame
	_, _ = readSSEFrame(reader)

	// Next frame must be keepalive comment frame
	frame, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read keepalive frame: %v", err)
	}

	if frame["comment"] != "keepalive" {
		t.Errorf("expected comment 'keepalive', got %+v", frame)
	}
}

// 4. Reconnection & Last-Event-ID Milestone Replay
func TestSSE_Reconnection_LastEventID_Replay(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	parcel, history := validTestParcelInfo(tn)

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	// Reconnect after PICKED_UP (history[1]), so IN_TRANSIT and OUT_FOR_DELIVERY should replay
	lastTimestamp := history[1].Timestamp.Unix()
	lastEventID := fmt.Sprintf("%s:PICKED_UP:%d", tn, lastTimestamp)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
	req.Header.Set("Last-Event-ID", lastEventID)

	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// 1st replayed frame should be IN_TRANSIT
	frame1, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read replayed frame 1: %v", err)
	}
	var p1 dto.PublicStatusUpdateEvent
	_ = json.Unmarshal([]byte(frame1["data"]), &p1)
	if p1.Status != "IN_TRANSIT" {
		t.Errorf("expected replayed status IN_TRANSIT, got %s", p1.Status)
	}

	// 2nd replayed frame should be OUT_FOR_DELIVERY
	frame2, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read replayed frame 2: %v", err)
	}
	var p2 dto.PublicStatusUpdateEvent
	_ = json.Unmarshal([]byte(frame2["data"]), &p2)
	if p2.Status != "OUT_FOR_DELIVERY" {
		t.Errorf("expected replayed status OUT_FOR_DELIVERY, got %s", p2.Status)
	}
}

// 5. Reconnection with Milestones Sharing the Exact Same Second (Same-Timestamp Replay)
func TestSSE_Reconnection_SameTimestampReplay(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	trackingNum, _ := domain.NewTrackingNumber(tn)
	sharedTime := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)

	// Multiple distinct status transitions occurring within the same wall-clock second
	parcel := &domain.ParcelTrackingInfo{
		ID:              "prcl-same-ts",
		TrackingNumber:  trackingNum,
		Status:          domain.StatusInTransit,
		DestinationCity: "Colombo",
		CreatedAt:       sharedTime,
		UpdatedAt:       sharedTime,
	}

	history := []domain.MilestoneRecord{
		{
			ID:        "ms-01",
			ParcelID:  parcel.ID,
			ToStatus:  string(domain.StatusCreated),
			Timestamp: sharedTime, // 10:00:00
		},
		{
			ID:        "ms-02",
			ParcelID:  parcel.ID,
			ToStatus:  string(domain.StatusAssigned),
			Timestamp: sharedTime, // 10:00:00 (SAME TIMESTAMP)
		},
		{
			ID:        "ms-03",
			ParcelID:  parcel.ID,
			ToStatus:  string(domain.StatusPickedUp),
			Timestamp: sharedTime, // 10:00:00 (SAME TIMESTAMP)
		},
		{
			ID:        "ms-04",
			ParcelID:  parcel.ID,
			ToStatus:  string(domain.StatusInTransit),
			Timestamp: sharedTime.Add(10 * time.Minute),
		},
	}

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	// Client last saw CREATED at 10:00:00.
	// Index-based replay must replay ASSIGNED (10:00:00), PICKED_UP (10:00:00), and IN_TRANSIT (10:10:00),
	// without skipping ASSIGNED and PICKED_UP due to identical timestamps!
	lastEventID := fmt.Sprintf("%s:CREATED:%d", tn, sharedTime.Unix())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
	req.Header.Set("Last-Event-ID", lastEventID)

	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	expectedStatuses := []string{"ASSIGNED", "PICKED_UP", "IN_TRANSIT"}
	for _, expectedStatus := range expectedStatuses {
		frame, err := readSSEFrame(reader)
		if err != nil {
			t.Fatalf("failed to read expected replayed frame for %s: %v", expectedStatus, err)
		}
		var p dto.PublicStatusUpdateEvent
		_ = json.Unmarshal([]byte(frame["data"]), &p)
		if p.Status != expectedStatus {
			t.Errorf("expected replayed status %s, got %s", expectedStatus, p.Status)
		}
	}
}

// 6. Deduplication of Events Queued Between Subscription and Initial Hydration
func TestSSE_HydrationSubscriptionRaceDeduplication(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	parcel, history := validTestParcelInfo(tn)

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	// Initial hydration frame (OUT_FOR_DELIVERY)
	initialFrame, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read initial frame: %v", err)
	}
	var pInit dto.PublicStatusUpdateEvent
	_ = json.Unmarshal([]byte(initialFrame["data"]), &pInit)
	if pInit.Status != "OUT_FOR_DELIVERY" {
		t.Fatalf("expected initial status OUT_FOR_DELIVERY, got %s", pInit.Status)
	}

	// Broadcast an event that has the exact same status and older/equal timestamp (simulating race-queued event)
	duplicateEvent := dto.PublicStatusUpdateEvent{
		TrackingNumber: tn,
		Status:         "OUT_FOR_DELIVERY",
		Timestamp:      parcel.UpdatedAt,
	}
	hub.Broadcast(duplicateEvent)

	// Broadcast a genuinely new event (DELIVERED)
	newEventTime := parcel.UpdatedAt.Add(1 * time.Hour)
	newEvent := dto.PublicStatusUpdateEvent{
		TrackingNumber: tn,
		Status:         "DELIVERED",
		Timestamp:      newEventTime,
	}
	hub.Broadcast(newEvent)

	// The next frame read should be DELIVERED (the duplicate OUT_FOR_DELIVERY must have been skipped)
	nextFrame, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read next frame: %v", err)
	}
	var pNext dto.PublicStatusUpdateEvent
	_ = json.Unmarshal([]byte(nextFrame["data"]), &pNext)
	if pNext.Status != "DELIVERED" {
		t.Errorf("expected duplicate OUT_FOR_DELIVERY to be deduplicated; got %s instead of DELIVERED", pNext.Status)
	}
}

// 7. Slow Subscriber Overflow Triggers Connection Close for Client Resynchronization
func TestSSE_SlowSubscriberResynchronizationTrigger(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	parcel, history := validTestParcelInfo(tn)

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	// Small buffer of 2 to easily trigger backpressure overflow
	hub := events.NewEventHub(events.WithBufferSize(2))
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	// Read initial frame
	_, _ = readSSEFrame(reader)

	// Overflow buffer with 3 rapid events while client does NOT read
	ts := time.Now().UTC()
	hub.Broadcast(dto.PublicStatusUpdateEvent{TrackingNumber: tn, Status: "IN_TRANSIT", Timestamp: ts})
	hub.Broadcast(dto.PublicStatusUpdateEvent{TrackingNumber: tn, Status: "OUT_FOR_DELIVERY", Timestamp: ts.Add(1 * time.Second)})
	hub.Broadcast(dto.PublicStatusUpdateEvent{TrackingNumber: tn, Status: "DELIVERED", Timestamp: ts.Add(2 * time.Second)})

	// Client should read the 2 buffered events, then stream terminates (EOF) to force client reconnect
	_, _ = readSSEFrame(reader)
	_, _ = readSSEFrame(reader)

	done := make(chan error, 1)
	go func() {
		_, err := reader.ReadString('\n')
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "closed") {
			t.Logf("connection terminated with: %v (expected EOF for resync)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected connection termination on slow subscriber overflow")
	}
}

// 8. Reconnection with Invalid or Mismatched Last-Event-ID Graceful Fallback
func TestSSE_Reconnection_InvalidLastEventID_Fallback(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	parcel, history := validTestParcelInfo(tn)

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	testIDs := []struct {
		name    string
		eventID string
	}{
		{"malformed format", "corrupted-id-without-colons"},
		{"mismatched tracking number", "SD-20261025-ZZZZZZ:CREATED:1730036700"},
		{"invalid status", "SD-20261025-A8F3K2:INVALID_STATUS:1730036700"},
	}

	for _, tc := range testIDs {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
			req.Header.Set("Last-Event-ID", tc.eventID)

			resp, err := server.Client().Do(req)
			if err != nil {
				t.Fatalf("failed to connect: %v", err)
			}
			defer resp.Body.Close()

			reader := bufio.NewReader(resp.Body)
			frame, err := readSSEFrame(reader)
			if err != nil {
				t.Fatalf("failed to read frame: %v", err)
			}

			// Should gracefully fall back to initial hydration
			var p dto.PublicStatusUpdateEvent
			_ = json.Unmarshal([]byte(frame["data"]), &p)
			if p.Status != "OUT_FOR_DELIVERY" {
				t.Errorf("expected fallback status OUT_FOR_DELIVERY, got %s", p.Status)
			}
		})
	}
}

// 9. Client Disconnect and Resource Cleanup (Zero Goroutine/Subscription Leak)
func TestSSE_ClientDisconnectCleanup(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	parcel, history := validTestParcelInfo(tn)

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	reader := bufio.NewReader(resp.Body)
	_, _ = readSSEFrame(reader)

	if hub.SubscriberCount(tn) != 1 {
		t.Fatalf("expected 1 active subscriber before disconnect, got %d", hub.SubscriberCount(tn))
	}

	// Cancel context to simulate client disconnect
	cancel()
	_ = resp.Body.Close()

	// Wait deterministically for unsubscription cleanup with a 2-second deadline
	deadline := time.Now().Add(2 * time.Second)
	cleanedUp := false
	for time.Now().Before(deadline) {
		if hub.SubscriberCount(tn) == 0 {
			cleanedUp = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if !cleanedUp {
		t.Fatalf("expected subscriber count to drop to 0 after disconnect, got %d", hub.SubscriberCount(tn))
	}
}

// 10. Concurrent Clients Fan-Out
func TestSSE_ConcurrentClientsFanOut(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	parcel, history := validTestParcelInfo(tn)

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	const clientCount = 15
	var wg sync.WaitGroup
	wg.Add(clientCount)

	cancels := make([]context.CancelFunc, clientCount)
	responses := make([]*http.Response, clientCount)
	readers := make([]*bufio.Reader, clientCount)

	// Connect all clients concurrently
	for i := 0; i < clientCount; i++ {
		idx := i
		ctx, cancel := context.WithCancel(context.Background())
		cancels[idx] = cancel

		go func() {
			defer wg.Done()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
			if err != nil {
				t.Errorf("client %d error creating request: %v", idx, err)
				return
			}
			resp, err := server.Client().Do(req)
			if err != nil {
				t.Errorf("client %d error connecting: %v", idx, err)
				return
			}
			responses[idx] = resp
			readers[idx] = bufio.NewReader(resp.Body)

			// Read initial frame
			_, readErr := readSSEFrame(readers[idx])
			if readErr != nil {
				t.Errorf("client %d failed reading initial frame: %v", idx, readErr)
			}
		}()
	}
	wg.Wait()

	if hub.SubscriberCount(tn) != clientCount {
		t.Fatalf("expected %d subscribers, got %d", clientCount, hub.SubscriberCount(tn))
	}

	// Broadcast live update
	delivered := hub.Broadcast(dto.PublicStatusUpdateEvent{
		TrackingNumber: tn,
		Status:         "DELIVERED",
		Timestamp:      time.Now().UTC(),
	})
	if delivered != clientCount {
		t.Fatalf("expected broadcast to reach %d clients, reached %d", clientCount, delivered)
	}

	// Verify all clients receive the broadcast frame
	wg.Add(clientCount)
	for i := 0; i < clientCount; i++ {
		idx := i
		go func() {
			defer wg.Done()
			frame, err := readSSEFrame(readers[idx])
			if err != nil {
				t.Errorf("client %d failed reading broadcast frame: %v", idx, err)
				return
			}
			var p dto.PublicStatusUpdateEvent
			_ = json.Unmarshal([]byte(frame["data"]), &p)
			if p.Status != "DELIVERED" {
				t.Errorf("client %d expected status DELIVERED, got %s", idx, p.Status)
			}
		}()
	}
	wg.Wait()

	// Disconnect all clients
	for i := 0; i < clientCount; i++ {
		cancels[i]()
		if responses[i] != nil {
			_ = responses[i].Body.Close()
		}
	}

	// Verify all subscriptions cleaned up
	deadline := time.Now().Add(2 * time.Second)
	cleanedUp := false
	for time.Now().Before(deadline) {
		if hub.SubscriberCount(tn) == 0 {
			cleanedUp = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if !cleanedUp {
		t.Fatalf("expected all %d subscribers cleaned up, got %d remaining", clientCount, hub.SubscriberCount(tn))
	}
}

// 11. Event Source Unavailable Handling (Section 3 Requirement)
func TestSSE_EventSourceUnavailable(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	parcel, history := validTestParcelInfo(tn)

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	t.Run("nil event source returns 503 DEPENDENCY_UNAVAILABLE", func(t *testing.T) {
		router, _ := setupTestSSEInfrastructure(mockRepo, nil, 15*time.Second)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/"+tn+"/stream", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 Service Unavailable, got %d", rr.Code)
		}

		var errResp ErrorEnvelope
		if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
			t.Fatalf("failed to decode error envelope: %v", err)
		}
		if errResp.Error.Code != "DEPENDENCY_UNAVAILABLE" {
			t.Errorf("expected error code DEPENDENCY_UNAVAILABLE, got %s", errResp.Error.Code)
		}
	})

	t.Run("inactive event source returns 503 DEPENDENCY_UNAVAILABLE", func(t *testing.T) {
		hub := events.NewEventHub()
		hub.SetAvailable(false) // Marked unavailable

		router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/"+tn+"/stream", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 Service Unavailable, got %d", rr.Code)
		}
	})
}

// 12. Error Conditions
func TestSSE_ErrorConditions(t *testing.T) {
	hub := events.NewEventHub()

	t.Run("invalid tracking number returns 400 INVALID_TRACKING_NUMBER", func(t *testing.T) {
		router, _ := setupTestSSEInfrastructure(&mockRepoForHTTP{}, hub, 15*time.Second)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/INVALID-TRACKING-FORMAT/stream", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rr.Code)
		}
		var errResp ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errResp)
		if errResp.Error.Code != "INVALID_TRACKING_NUMBER" {
			t.Errorf("expected INVALID_TRACKING_NUMBER, got %s", errResp.Error.Code)
		}
	})

	t.Run("tracking not found returns 404 TRACKING_NOT_FOUND", func(t *testing.T) {
		mockRepo := &mockRepoForHTTP{
			findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				return nil, nil, domain.ErrTrackingNotFound
			},
		}
		router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/SD-20261025-A8F3K2/stream", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rr.Code)
		}
		var errResp ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errResp)
		if errResp.Error.Code != "TRACKING_NOT_FOUND" {
			t.Errorf("expected TRACKING_NOT_FOUND, got %s", errResp.Error.Code)
		}
	})

	t.Run("database unavailable returns 503 DEPENDENCY_UNAVAILABLE", func(t *testing.T) {
		mockRepo := &mockRepoForHTTP{
			findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				return nil, nil, domain.ErrDatabaseUnavailable
			},
		}
		router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/SD-20261025-A8F3K2/stream", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 Service Unavailable, got %d", rr.Code)
		}
		var errResp ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errResp)
		if errResp.Error.Code != "DEPENDENCY_UNAVAILABLE" {
			t.Errorf("expected DEPENDENCY_UNAVAILABLE, got %s", errResp.Error.Code)
		}
	})

	t.Run("non-flusher response writer returns 500 STREAMING_UNSUPPORTED", func(t *testing.T) {
		parcel, history := validTestParcelInfo("SD-20261025-A8F3K2")
		mockRepo := &mockRepoForHTTP{
			findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				return parcel, history, nil
			},
		}

		logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
		trackingSvc := service.NewPublicTrackingService(mockRepo)
		handler := NewSSEHandler(trackingSvc, hub, 15*time.Second, logger)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/SD-20261025-A8F3K2/stream", nil)
		req.SetPathValue("tracking_number", "SD-20261025-A8F3K2")
		writer := &nonFlusherResponseWriter{}

		handler.StreamTracking(writer, req)

		if writer.code != http.StatusInternalServerError {
			t.Fatalf("expected 500 Internal Server Error, got %d", writer.code)
		}
		if !strings.Contains(string(writer.body), "STREAMING_UNSUPPORTED") {
			t.Errorf("expected STREAMING_UNSUPPORTED in body, got: %s", string(writer.body))
		}
	})
}

// 13. Public Data Privacy Enforcement (SEC-009)
func TestSSE_PublicDataPrivacy_SEC009(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	parcel, history := validTestParcelInfo(tn)

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	frame, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read frame: %v", err)
	}

	rawJSON := frame["data"]

	forbiddenSubstrings := []string{
		"sender_id",
		"customer_id",
		"merchant_id",
		"recipient_name",
		"recipient_phone",
		"phone",
		"street",
		"street_line1",
		"street_line2",
		"pickup_address",
		"dropoff_address",
		"postal_code",
		"zone_id",
		"origin_zone_id",
		"destination_zone_id",
		"courier_notes",
		"internal_notes",
		"reason",
		"reason_code",
		"actor_id",
		"actor_role",
		"proof_id",
		"proof_photo_url",
		"delivery_fee",
		"delivery_fee_cents",
		"fee_cents",
		"currency",
		"chargeable_weight",
		"pricing_rule",
	}

	for _, forbidden := range forbiddenSubstrings {
		if strings.Contains(strings.ToLower(rawJSON), `"`+forbidden+`"`) {
			t.Errorf("SEC-009 VIOLATION in SSE wire data: leaked sensitive field %q: %s", forbidden, rawJSON)
		}
	}
}

// 14. Strict Route Ownership: Alias Path Must NOT Be Exposed (Returns 404)
func TestSSE_RouteOwnership_AliasReturns404(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	parcel, history := validTestParcelInfo(tn)

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	// Alias path /api/v1/tracking/{tracking_number}/stream belongs to the authenticated namespace
	// and must return 404 from the public router to avoid route ownership conflict
	aliasURL := fmt.Sprintf("%s/api/v1/tracking/%s/stream", server.URL, tn)
	resp, err := server.Client().Get(aliasURL)
	if err != nil {
		t.Fatalf("failed request to alias: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for unauthenticated alias path, got %d", resp.StatusCode)
	}

	// Canonical path /api/v1/tracking/public/{tracking_number}/stream must return 200 OK
	canonicalURL := fmt.Sprintf("%s/api/v1/tracking/public/%s/stream", server.URL, tn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, canonicalURL, nil)
	respCanonical, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed request to canonical route: %v", err)
	}
	defer respCanonical.Body.Close()

	if respCanonical.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from canonical route, got %d", respCanonical.StatusCode)
	}
}

// 15. Hub Shutdown Terminates Active Streams
func TestSSE_HubShutdownTerminatesStream(t *testing.T) {
	tn := "SD-20261025-A8F3K2"
	parcel, history := validTestParcelInfo(tn)

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	_, _ = readSSEFrame(reader)

	if hub.SubscriberCount(tn) != 1 {
		t.Fatalf("expected 1 active subscriber before close, got %d", hub.SubscriberCount(tn))
	}

	// Close hub
	_ = hub.Close()

	// Stream should terminate (EOF)
	doneChan := make(chan error, 1)
	go func() {
		_, err := reader.ReadString('\n')
		doneChan <- err
	}()

	select {
	case err := <-doneChan:
		if !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "closed") {
			t.Logf("stream terminated on hub close with: %v (expected EOF)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not terminate after hub close")
	}
}

// Phase 5 Replay & Deduplication Tests
func TestSSE_Phase5_StatusHistoryID_TiebreakerReplay(t *testing.T) {
	tn := "SD-20261110-000001"
	tnObj, _ := domain.NewTrackingNumber(tn)
	baseTime := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)

	parcel := &domain.ParcelTrackingInfo{
		ID:              "parcel-uuid-1",
		TrackingNumber:  tnObj,
		Status:          domain.StatusAttemptFailed,
		DestinationCity: "Colombo",
		CreatedAt:       baseTime,
		UpdatedAt:       baseTime.Add(3 * time.Hour),
	}

	history := []domain.MilestoneRecord{
		{
			ID:        "hist-uuid-001",
			ParcelID:  "parcel-uuid-1",
			ToStatus:  "CREATED",
			Timestamp: baseTime,
		},
		{
			ID:        "hist-uuid-002",
			ParcelID:  "parcel-uuid-1",
			ToStatus:  "OUT_FOR_DELIVERY",
			Timestamp: baseTime.Add(1 * time.Hour),
		},
		{
			ID:        "hist-uuid-003",
			ParcelID:  "parcel-uuid-1",
			ToStatus:  "ATTEMPT_FAILED",
			Timestamp: baseTime.Add(2 * time.Hour),
		},
		{
			ID:        "hist-uuid-004",
			ParcelID:  "parcel-uuid-1",
			ToStatus:  "OUT_FOR_DELIVERY",
			Timestamp: baseTime.Add(3 * time.Hour),
		},
		{
			ID:        "hist-uuid-005",
			ParcelID:  "parcel-uuid-1",
			ToStatus:  "ATTEMPT_FAILED",
			Timestamp: baseTime.Add(4 * time.Hour),
		},
	}

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Client reconnects sending Last-Event-ID matching the FIRST ATTEMPT_FAILED (hist-uuid-003)
	lastEventID := fmt.Sprintf("%s:ATTEMPT_FAILED:%d:hist-uuid-003", tn, baseTime.Add(2*time.Hour).Unix())

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
	req.Header.Set("Last-Event-ID", lastEventID)

	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Frame 1 must be the second OUT_FOR_DELIVERY (hist-uuid-004)
	frame1, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read replayed frame 1: %v", err)
	}
	expectedID1 := fmt.Sprintf("%s:OUT_FOR_DELIVERY:%d:hist-uuid-004", tn, baseTime.Add(3*time.Hour).Unix())
	if frame1["id"] != expectedID1 {
		t.Errorf("expected frame 1 id %q, got %q", expectedID1, frame1["id"])
	}

	// Frame 2 must be the second ATTEMPT_FAILED (hist-uuid-005)
	frame2, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read replayed frame 2: %v", err)
	}
	expectedID2 := fmt.Sprintf("%s:ATTEMPT_FAILED:%d:hist-uuid-005", tn, baseTime.Add(4*time.Hour).Unix())
	if frame2["id"] != expectedID2 {
		t.Errorf("expected frame 2 id %q, got %q", expectedID2, frame2["id"])
	}
}

func TestSSE_Phase5_SameTimestampTiebreakerReplay(t *testing.T) {
	tn := "SD-20261110-000002"
	tnObj, _ := domain.NewTrackingNumber(tn)
	sameTime := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

	parcel := &domain.ParcelTrackingInfo{
		ID:              "parcel-uuid-2",
		TrackingNumber:  tnObj,
		Status:          domain.StatusInTransit,
		DestinationCity: "Galle",
		CreatedAt:       sameTime,
		UpdatedAt:       sameTime,
	}

	// Two milestones with the EXACT same timestamp in the same wall-clock second
	history := []domain.MilestoneRecord{
		{
			ID:        "hist-uuid-A",
			ParcelID:  "parcel-uuid-2",
			ToStatus:  "PICKED_UP",
			Timestamp: sameTime,
		},
		{
			ID:        "hist-uuid-B",
			ParcelID:  "parcel-uuid-2",
			ToStatus:  "IN_TRANSIT",
			Timestamp: sameTime,
		},
	}

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Client reconnects having acknowledged hist-uuid-A
	lastEventID := fmt.Sprintf("%s:PICKED_UP:%d:hist-uuid-A", tn, sameTime.Unix())

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
	req.Header.Set("Last-Event-ID", lastEventID)

	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Frame must be hist-uuid-B even though its timestamp is identical!
	frame, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read replayed frame: %v", err)
	}

	expectedID := fmt.Sprintf("%s:IN_TRANSIT:%d:hist-uuid-B", tn, sameTime.Unix())
	if frame["id"] != expectedID {
		t.Errorf("expected replayed frame id %q, got %q", expectedID, frame["id"])
	}
}

func TestSSE_Phase5_LiveStatusHistoryID_Deduplication(t *testing.T) {
	tn := "SD-20261110-000003"
	tnObj, _ := domain.NewTrackingNumber(tn)
	baseTime := time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)

	parcel := &domain.ParcelTrackingInfo{
		ID:              "parcel-uuid-3",
		TrackingNumber:  tnObj,
		Status:          domain.StatusAttemptFailed,
		DestinationCity: "Jaffna",
		CreatedAt:       baseTime,
		UpdatedAt:       baseTime,
	}

	history := []domain.MilestoneRecord{
		{
			ID:        "hist-uuid-first-attempt",
			ParcelID:  "parcel-uuid-3",
			ToStatus:  "ATTEMPT_FAILED",
			Timestamp: baseTime,
		},
	}

	mockRepo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return parcel, history, nil
		},
	}

	hub := events.NewEventHub()
	router, _ := setupTestSSEInfrastructure(mockRepo, hub, 15*time.Second)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/tracking/public/"+tn+"/stream", nil)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Initial frame (first attempt)
	initFrame, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read initial frame: %v", err)
	}
	if !strings.Contains(initFrame["id"], "hist-uuid-first-attempt") {
		t.Errorf("expected initial frame to contain hist-uuid-first-attempt, got %s", initFrame["id"])
	}

	// 1. Broadcast event with the SAME status_history_id: must be deduplicated
	hub.Broadcast(dto.PublicStatusUpdateEvent{
		TrackingNumber:  tn,
		Status:          "ATTEMPT_FAILED",
		Description:     domain.SafeStatusDescription("ATTEMPT_FAILED"),
		Timestamp:       baseTime,
		StatusHistoryID: "hist-uuid-first-attempt",
	})

	// 2. Broadcast event with a NEW status_history_id even with identical status: must be delivered!
	hub.Broadcast(dto.PublicStatusUpdateEvent{
		TrackingNumber:  tn,
		Status:          "ATTEMPT_FAILED",
		Description:     domain.SafeStatusDescription("ATTEMPT_FAILED"),
		Timestamp:       baseTime.Add(time.Hour),
		StatusHistoryID: "hist-uuid-second-attempt",
	})

	// Read next frame from stream
	nextFrame, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("failed to read next frame: %v", err)
	}

	// Must be second-attempt, not duplicate of first
	if !strings.Contains(nextFrame["id"], "hist-uuid-second-attempt") {
		t.Errorf("expected next frame to be second attempt, got: %s", nextFrame["id"])
	}
}
