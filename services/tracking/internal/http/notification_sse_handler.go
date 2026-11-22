package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/swiftdrop/tracking/internal/auth"
	"github.com/swiftdrop/tracking/internal/dto"
	"github.com/swiftdrop/tracking/internal/events"
	"github.com/swiftdrop/tracking/internal/repository"
)

// NotificationSSEHandler coordinates authenticated real-time Server-Sent Events (SSE) notification streaming.
type NotificationSSEHandler struct {
	verifier       auth.TokenVerifier
	hub            *events.UserEventHub
	repo           repository.NotificationRepository
	heartbeatDelay time.Duration
	logger         *slog.Logger
}

// NewNotificationSSEHandler constructs a NotificationSSEHandler.
func NewNotificationSSEHandler(
	verifier auth.TokenVerifier,
	hub *events.UserEventHub,
	repo repository.NotificationRepository,
	heartbeatDelay time.Duration,
	logger *slog.Logger,
) *NotificationSSEHandler {
	if heartbeatDelay <= 0 {
		heartbeatDelay = 15 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &NotificationSSEHandler{
		verifier:       verifier,
		hub:            hub,
		repo:           repo,
		heartbeatDelay: heartbeatDelay,
		logger:         logger,
	}
}

// StreamNotifications streams real-time in-app user notifications over text/event-stream.
// Authenticates via Bearer token header or ?token=<jwt> query parameter.
//
// @Summary      Stream real-time user in-app notifications via SSE
// @Description  Streams authenticated in-app notifications and unread counters over HTTP text/event-stream.
// @Tags         notifications
// @Produce      text/event-stream
// @Security     BearerAuth
// @Header       200  {string}  Content-Type       "text/event-stream"
// @Header       200  {string}  Cache-Control      "no-cache, no-transform"
// @Header       200  {string}  Connection         "keep-alive"
// @Header       200  {string}  X-Accel-Buffering  "no"
// @Failure      401  {object}  ErrorEnvelope      "Missing, expired, or invalid Bearer access token"
// @Failure      503  {object}  ErrorEnvelope      "Event hub unavailable"
// @Router       /api/v1/notifications/stream [get]
func (h *NotificationSSEHandler) StreamNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if ctx.Err() != nil {
		return
	}

	// 1. Verify ResponseWriter supports streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		WriteError(w, r, http.StatusInternalServerError, "STREAMING_UNSUPPORTED", "Streaming is not supported by connection.")
		return
	}

	// 2. Extract access token from Authorization header or ?token query param
	tokenStr := extractToken(r)
	if tokenStr == "" {
		WriteError(w, r, http.StatusBadRequest, "MISSING_TOKEN", "Missing access token in Authorization header or query parameter.")
		return
	}

	// 3. Authenticate token
	principal, err := h.verifier.VerifyToken(tokenStr)
	if err != nil {
		WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired access token.")
		return
	}

	if h.hub == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "HUB_UNAVAILABLE", "Notification stream hub is unavailable.")
		return
	}

	// 4. Build subscription keys
	subKeys := []string{principal.UserID}
	if principal.Role == auth.RoleDispatcher {
		subKeys = append(subKeys, "role:DISPATCHER")
	}

	eventCh, unsub, err := h.hub.Subscribe(ctx, subKeys...)
	if err != nil {
		WriteError(w, r, http.StatusServiceUnavailable, "SUBSCRIPTION_FAILED", "Failed to subscribe to notification stream: "+err.Error())
		return
	}
	defer unsub()

	// 5. Set standard SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// 6. Initial Hydration: unread counter & recent notifications from repository
	if h.repo != nil {
		if count, err := h.repo.GetUnreadNotificationCount(ctx, principal.UserID); err == nil {
			countPayload, _ := json.Marshal(dto.NotificationUnreadCountDTO{Count: count})
			fmt.Fprintf(w, "event: unread_count\ndata: %s\n\n", countPayload)
			flusher.Flush()
		}

		if recent, err := h.repo.ListRecentNotifications(ctx, principal.UserID, 15); err == nil {
			for _, notif := range recent {
				if !notif.IsRead {
					data, _ := json.Marshal(notif)
					fmt.Fprintf(w, "id: %s\nevent: notification\ndata: %s\n\n", notif.ID, data)
				}
			}
			flusher.Flush()
		}
	}

	// 7. Event Stream Loop with 15s keepalive
	ticker := time.NewTicker(h.heartbeatDelay)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			fmt.Fprintf(w, ":keepalive\n\n")
			flusher.Flush()

		case notif, ok := <-eventCh:
			if !ok {
				return
			}
			data, err := json.Marshal(notif)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "id: %s\nevent: notification\ndata: %s\n\n", notif.ID, data)
			flusher.Flush()
		}
	}
}

func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	// Fallback to query param
	return strings.TrimSpace(r.URL.Query().Get("token"))
}
