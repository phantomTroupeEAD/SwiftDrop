package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/swiftdrop/tracking/internal/domain"
	"github.com/swiftdrop/tracking/internal/dto"
	"github.com/swiftdrop/tracking/internal/events"
	"github.com/swiftdrop/tracking/internal/service"
)

// SSEHandler coordinates real-time Server-Sent Events (SSE) tracking connections.
type SSEHandler struct {
	trackingSvc    *service.PublicTrackingService
	eventSource    events.TrackingEventSource
	heartbeatDelay time.Duration
	logger         *slog.Logger
}

// NewSSEHandler constructs an SSEHandler with the provided dependencies and heartbeat interval.
func NewSSEHandler(
	trackingSvc *service.PublicTrackingService,
	eventSource events.TrackingEventSource,
	heartbeatDelay time.Duration,
	logger *slog.Logger,
) *SSEHandler {
	if heartbeatDelay <= 0 {
		heartbeatDelay = 15 * time.Second
	}
	return &SSEHandler{
		trackingSvc:    trackingSvc,
		eventSource:    eventSource,
		heartbeatDelay: heartbeatDelay,
		logger:         logger,
	}
}

// StreamTracking serves real-time Server-Sent Events for a public tracking number.
// Strictly adheres to SEC-009 public privacy standards, Document 4 §5.2 wire protocol,
// and 15-second heartbeat keepalive framing.
//
// @Summary      Stream real-time parcel tracking events via SSE
// @Description  Streams real-time status update events over HTTP text/event-stream. Public, unauthenticated, and PII-redacted per SEC-009.
// @Tags         tracking
// @Produce      text/event-stream
// @Param        tracking_number  path  string  true  "Tracking Number (SD-YYYYMMDD-XXXXXX)"
// @Header       200              {string}  Content-Type       "text/event-stream"
// @Header       200              {string}  Cache-Control      "no-cache, no-transform"
// @Header       200              {string}  Connection         "keep-alive"
// @Header       200              {string}  X-Accel-Buffering  "no"
// @Success      200              {string}  string             "event: status_update\ndata: ...\n\n"
// @Failure      400              {object}  ErrorEnvelope      "Invalid tracking number format"
// @Failure      404              {object}  ErrorEnvelope      "Shipment not found"
// @Failure      503              {object}  ErrorEnvelope      "Real-time event stream or database unavailable"
// @Router       /api/v1/tracking/public/{tracking_number}/stream [get]
func (h *SSEHandler) StreamTracking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Check early context cancellation
	if ctx.Err() != nil {
		return
	}

	// 2. Extract and validate tracking number syntax
	trackingNumber := r.PathValue("tracking_number")
	if trackingNumber == "" {
		trackingNumber = extractTrackingNumberFromPath(r.URL.Path)
	}

	trackingNumber = strings.TrimSpace(trackingNumber)
	if trackingNumber == "" {
		WriteError(w, r, http.StatusBadRequest, "INVALID_TRACKING_NUMBER",
			"Tracking number path parameter is required.",
			ErrorDetail{Field: "tracking_number", Issue: "missing tracking number parameter"},
		)
		return
	}

	tn, err := domain.NewTrackingNumber(trackingNumber)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_TRACKING_NUMBER",
			"Tracking number does not conform to the required format (SD-YYYYMMDD-XXXXXX).",
			ErrorDetail{Field: "tracking_number", Issue: err.Error()},
		)
		return
	}

	// 3. Verify event source availability (Section 3: never claim active stream if real source unavailable)
	if h.eventSource == nil || !h.eventSource.IsAvailable() {
		if h.logger != nil {
			h.logger.Warn("SSE stream request rejected: real-time event source unavailable",
				"tracking_number", tn.String(),
				"correlation_id", CorrelationID(ctx),
			)
		}
		WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE",
			"Real-time tracking stream is currently unavailable. Please poll the public tracking endpoint.",
		)
		return
	}

	// 4. Verify tracking service / persistence availability
	if h.trackingSvc == nil {
		if h.logger != nil {
			h.logger.Warn("SSE stream request rejected: tracking service unconfigured",
				"tracking_number", tn.String(),
				"correlation_id", CorrelationID(ctx),
			)
		}
		WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE",
			"Tracking data temporarily unavailable.",
		)
		return
	}

	// 5. Ensure client/proxy supports HTTP response flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		WriteError(w, r, http.StatusInternalServerError, "STREAMING_UNSUPPORTED",
			"Streaming unsupported by client or reverse proxy.",
		)
		return
	}

	// 6. Subscribe to real-time event updates FIRST before querying initial database state.
	// Subscribing before DB hydration guarantees that any event committed/published
	// concurrently during initial state hydration is not dropped in a race gap.
	eventChan, unsubscribe, err := h.eventSource.Subscribe(ctx, tn.String())
	if err != nil {
		if errors.Is(err, events.ErrSubscriptionLimitExceeded) {
			WriteError(w, r, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED",
				"Concurrent tracking stream limit reached for this parcel. Please try again shortly.",
			)
			return
		}
		WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE",
			"Failed to register real-time tracking subscription.",
		)
		return
	}
	defer unsubscribe()

	// 7. Retrieve authoritative initial state from PostgreSQL
	initialState, err := h.trackingSvc.GetPublicTracking(ctx, tn.String())
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			return
		}
		if errors.Is(err, domain.ErrTrackingNotFound) {
			WriteError(w, r, http.StatusNotFound, "TRACKING_NOT_FOUND",
				fmt.Sprintf("Shipment not found for tracking number %s", tn.String()),
			)
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE",
				"Tracking data temporarily unavailable.",
			)
			return
		}

		if h.logger != nil {
			h.logger.Error("Unexpected error retrieving initial tracking state for SSE",
				"tracking_number", tn.String(),
				"error", err,
				"correlation_id", CorrelationID(ctx),
			)
		}
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR",
			"An unexpected error occurred retrieving tracking state.",
		)
		return
	}

	// 8. Parse and validate Last-Event-ID for reconnect and milestone replay
	lastEventID := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if lastEventID == "" {
		lastEventID = strings.TrimSpace(r.URL.Query().Get("last_event_id"))
	}

	var parsedLastID *events.ParsedEventID
	if lastEventID != "" {
		pID, parseErr := events.ParseEventID(lastEventID)
		if parseErr != nil {
			if h.logger != nil {
				h.logger.Debug("Malformed Last-Event-ID header; defaulting to full initial hydration",
					"last_event_id", lastEventID,
					"error", parseErr.Error(),
					"correlation_id", CorrelationID(ctx),
				)
			}
		} else if !pID.IsOpaque && pID.TrackingNumber != tn.String() {
			if h.logger != nil {
				h.logger.Debug("Mismatched tracking number in Last-Event-ID; defaulting to full initial hydration",
					"header_tracking", pID.TrackingNumber,
					"url_tracking", tn.String(),
					"correlation_id", CorrelationID(ctx),
				)
			}
		} else {
			parsedLastID = pID
		}
	}

	// 9. Set standard SSE response headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Manage connection write deadlines via Go 1.20+ http.ResponseController:
	// Clear the server's global WriteTimeout on this long-lived connection so heartbeats can stream indefinitely.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	// 10. Replay milestones or send initial state hydration
	var lastEmittedStatus string
	var lastEmittedTime time.Time
	var lastEmittedHistoryID string

	if parsedLastID != nil && !parsedLastID.IsOpaque {
		// Deterministic milestone replay:
		// Locate the milestone the client last acknowledged in the chronological milestone slice.
		matchedIdx := -1

		// First try exact match using TieBreaker (status_history_id)
		if parsedLastID.TieBreaker != "" {
			for i, m := range initialState.Milestones {
				if m.StatusHistoryID != "" && m.StatusHistoryID == parsedLastID.TieBreaker {
					matchedIdx = i
					break
				}
			}
		}

		// Fallback to (Status, Timestamp) match if TieBreaker not matched or omitted
		if matchedIdx == -1 {
			for i, m := range initialState.Milestones {
				if m.Status == parsedLastID.Status && m.Timestamp.UTC().Unix() == parsedLastID.Timestamp.Unix() {
					matchedIdx = i
				}
			}
		}

		var replayedMilestones []dto.PublicMilestone
		if matchedIdx != -1 {
			// Replay all milestones occurring chronologically after the matched milestone.
			// Using slice index guarantees that milestones sharing the exact same timestamp
			// as the previous milestone are NOT skipped.
			if matchedIdx+1 < len(initialState.Milestones) {
				replayedMilestones = initialState.Milestones[matchedIdx+1:]
			}
		} else {
			// Fallback if not found by index (e.g. truncated history or timestamp precision drift):
			// replay all milestones with timestamp strictly after client's last seen timestamp.
			for _, m := range initialState.Milestones {
				if m.Timestamp.UTC().After(parsedLastID.Timestamp) {
					replayedMilestones = append(replayedMilestones, m)
				}
			}
		}

		for _, m := range replayedMilestones {
			replayEvent := dto.PublicStatusUpdateEvent{
				TrackingNumber:  tn.String(),
				Status:          m.Status,
				Description:     domain.SafeStatusDescription(m.Status),
				Timestamp:       m.Timestamp.UTC(),
				StatusHistoryID: m.StatusHistoryID,
			}
			if err := writeSSEFrame(w, flusher, rc, replayEvent); err != nil {
				return
			}
			lastEmittedStatus = m.Status
			lastEmittedTime = m.Timestamp.UTC()
			lastEmittedHistoryID = m.StatusHistoryID
		}

		// If no replayed milestones and current parcel status differs from client's last seen,
		// push current status to synchronize client
		if len(replayedMilestones) == 0 && initialState.Status != parsedLastID.Status {
			var currentHistoryID string
			if len(initialState.Milestones) > 0 {
				currentHistoryID = initialState.Milestones[len(initialState.Milestones)-1].StatusHistoryID
			}
			currentEvent := dto.PublicStatusUpdateEvent{
				TrackingNumber:  initialState.TrackingNumber,
				Status:          initialState.Status,
				Description:     initialState.StatusDescription,
				Timestamp:       latestMilestoneTime(initialState),
				StatusHistoryID: currentHistoryID,
			}
			if err := writeSSEFrame(w, flusher, rc, currentEvent); err != nil {
				return
			}
			lastEmittedStatus = currentEvent.Status
			lastEmittedTime = currentEvent.Timestamp
			lastEmittedHistoryID = currentHistoryID
		}
	} else {
		// New connection (or opaque/unrecognized Last-Event-ID): Push initial parcel status frame
		var initialHistoryID string
		if len(initialState.Milestones) > 0 {
			initialHistoryID = initialState.Milestones[len(initialState.Milestones)-1].StatusHistoryID
		}
		initialEvent := dto.PublicStatusUpdateEvent{
			TrackingNumber:  initialState.TrackingNumber,
			Status:          initialState.Status,
			Description:     initialState.StatusDescription,
			Timestamp:       latestMilestoneTime(initialState),
			StatusHistoryID: initialHistoryID,
		}
		if err := writeSSEFrame(w, flusher, rc, initialEvent); err != nil {
			return
		}
		lastEmittedStatus = initialEvent.Status
		lastEmittedTime = initialEvent.Timestamp
		lastEmittedHistoryID = initialHistoryID
	}

	// 11. Start 15-second heartbeat keepalive ticker (Agreement A2)
	heartbeatTicker := time.NewTicker(h.heartbeatDelay)
	defer heartbeatTicker.Stop()

	// 12. Stream loop: dispatch events, emit keepalives, detect disconnects
	for {
		select {
		case <-ctx.Done():
			// Client disconnected or connection cancelled
			return

		case <-heartbeatTicker.C:
			// Emit 15-second keepalive comment frame (:keepalive\n\n) per Agreement A2
			if err := writeSSEComment(w, flusher, rc, "keepalive"); err != nil {
				return
			}

		case update, ok := <-eventChan:
			if !ok {
				// Event source terminated subscription (e.g. slow client buffer overflow or hub close)
				if h.logger != nil {
					h.logger.Info("SSE subscription channel closed; terminating client connection",
						"tracking_number", tn.String(),
					)
				}
				return
			}

			// Deduplicate events that were already transmitted during initial state hydration/replay
			// (handles events queued during the subscription-to-hydration race gap)
			if update.StatusHistoryID != "" && lastEmittedHistoryID != "" {
				if update.StatusHistoryID == lastEmittedHistoryID {
					continue
				}
			} else {
				if update.Status == lastEmittedStatus && !update.Timestamp.UTC().After(lastEmittedTime) {
					continue
				}
			}

			// Strictly enforce SEC-009 public privacy: sanitize event before wire serialization
			sanitized := dto.PublicStatusUpdateEvent{
				TrackingNumber:  tn.String(),
				Status:          update.Status,
				Description:     domain.SafeStatusDescription(update.Status),
				Timestamp:       update.Timestamp.UTC(),
				StatusHistoryID: update.StatusHistoryID,
			}

			if err := writeSSEFrame(w, flusher, rc, sanitized); err != nil {
				return
			}
			lastEmittedStatus = sanitized.Status
			lastEmittedTime = sanitized.Timestamp
			lastEmittedHistoryID = sanitized.StatusHistoryID
		}
	}
}

func latestMilestoneTime(resp *dto.PublicTrackingResponse) time.Time {
	if resp != nil && len(resp.Milestones) > 0 {
		return resp.Milestones[len(resp.Milestones)-1].Timestamp.UTC()
	}
	return time.Now().UTC()
}

// writeSSEFrame formats and flushes a single SSE data frame according to Document 4 §5.2.
// Wire format:
//
//	id: {tracking_number}:{status}:{unix_timestamp}
//	event: status_update
//	data: {"tracking_number":"...","status":"...","description":"...","timestamp":"..."}
//
// (terminated by double newline)
func writeSSEFrame(w io.Writer, flusher http.Flusher, rc *http.ResponseController, event dto.PublicStatusUpdateEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal SSE event data: %w", err)
	}

	var eventID string
	if strings.TrimSpace(event.StatusHistoryID) != "" {
		eventID = events.FormatEventIDWithTieBreaker(event.TrackingNumber, event.Status, event.Timestamp, event.StatusHistoryID)
	} else {
		eventID = events.FormatEventID(event.TrackingNumber, event.Status, event.Timestamp)
	}

	// Apply a bounded 10-second deadline during the write operation to catch stalled/hung TCP sockets
	if rc != nil {
		_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
	}

	frame := fmt.Sprintf("id: %s\nevent: status_update\ndata: %s\n\n", eventID, data)
	if _, err := io.WriteString(w, frame); err != nil {
		return err
	}

	flusher.Flush()

	// Clear write deadline while waiting idle for next update or heartbeat
	if rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}

	return nil
}

// writeSSEComment formats and flushes an SSE comment line (:keepalive\n\n).
func writeSSEComment(w io.Writer, flusher http.Flusher, rc *http.ResponseController, comment string) error {
	if rc != nil {
		_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
	}

	frame := fmt.Sprintf(":%s\n\n", comment)
	if _, err := io.WriteString(w, frame); err != nil {
		return err
	}

	flusher.Flush()

	if rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}

	return nil
}
