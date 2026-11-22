package transport

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/notifications/application"
	"github.com/swiftdrop/core-api/internal/notifications/domain"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

// Handler handles HTTP requests for in-app notifications.
type Handler struct {
	service *application.Service
}

// NewHandler constructs a notifications HTTP Handler.
func NewHandler(service *application.Service) *Handler {
	return &Handler{service: service}
}

// ListNotifications godoc
// @Summary      List user notifications
// @Description  Queries notifications for the authenticated user (and role:DISPATCHER broadcasts if caller is a dispatcher) with keyset cursor pagination.
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Param        cursor query string false "Pagination cursor from previous page"
// @Param        limit  query int    false "Number of items to return (1-100, default 20)"
// @Success      200 {object} httpx.CursorPaginatedEnvelope
// @Failure      400 {object} httpx.ErrorEnvelope "Invalid pagination parameters"
// @Failure      401 {object} httpx.ErrorEnvelope "Unauthorized"
// @Failure      500 {object} httpx.ErrorEnvelope "Internal server error"
// @Router       /api/v1/notifications [get]
func (h *Handler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	userID := identitytransport.GetUserID(r.Context())
	if userID == "" {
		httpx.WriteError(w, r, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Authentication required")
		return
	}

	userRole := identitytransport.GetUserRole(r.Context())
	isDispatcher := userRole == identitydomain.RoleDispatcher

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed <= 0 || parsed > 100 {
			httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_LIMIT", "limit must be an integer between 1 and 100")
			return
		}
		limit = parsed
	}

	var cursor *application.PaginationCursor
	if cursorStr := r.URL.Query().Get("cursor"); cursorStr != "" {
		cAt, cID, err := httpx.DecodeCursor(cursorStr)
		if err != nil {
			httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "Pagination cursor is invalid or malformed")
			return
		}
		cursor = &application.PaginationCursor{CreatedAt: cAt, ID: cID}
	}

	notifications, nextCursor, err := h.service.ListNotifications(r.Context(), application.ListNotificationsQuery{
		UserID:       userID,
		IsDispatcher: isDispatcher,
		Cursor:       cursor,
		Limit:        limit,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve notifications")
		return
	}

	dtos := ToNotificationResponses(notifications)

	meta := httpx.CursorMeta{
		Limit:   limit,
		HasMore: nextCursor != nil,
	}
	if nextCursor != nil {
		nextStr := httpx.EncodeCursor(nextCursor.CreatedAt, nextCursor.ID)
		meta.NextCursor = &nextStr
	}

	_ = httpx.WriteCursorPaginated(w, http.StatusOK, dtos, meta)
}

// GetUnreadCount godoc
// @Summary      Get unread notification count
// @Description  Returns total unread notification count for the authenticated user.
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} httpx.SingleDataEnvelope[UnreadCountResponse]
// @Failure      401 {object} httpx.ErrorEnvelope "Unauthorized"
// @Failure      500 {object} httpx.ErrorEnvelope "Internal server error"
// @Router       /api/v1/notifications/unread-count [get]
func (h *Handler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID := identitytransport.GetUserID(r.Context())
	if userID == "" {
		httpx.WriteError(w, r, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Authentication required")
		return
	}

	userRole := identitytransport.GetUserRole(r.Context())
	isDispatcher := userRole == identitydomain.RoleDispatcher

	count, err := h.service.GetUnreadCount(r.Context(), userID, isDispatcher)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve unread notification count")
		return
	}

	_ = httpx.WriteData(w, http.StatusOK, UnreadCountResponse{UnreadCount: count})
}

// MarkAsRead godoc
// @Summary      Mark notification as read
// @Description  Marks a specific notification as read by ID if owned by the caller.
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Param        id  path   string true "Notification ID (UUID)"
// @Success      200 {object} httpx.SingleDataEnvelope[NotificationResponse]
// @Failure      400 {object} httpx.ErrorEnvelope "Invalid notification ID"
// @Failure      401 {object} httpx.ErrorEnvelope "Unauthorized"
// @Failure      403 {object} httpx.ErrorEnvelope "Forbidden - Notification not owned by caller"
// @Failure      404 {object} httpx.ErrorEnvelope "Notification not found"
// @Failure      500 {object} httpx.ErrorEnvelope "Internal server error"
// @Router       /api/v1/notifications/{id}/read [patch]
func (h *Handler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	userID := identitytransport.GetUserID(r.Context())
	if userID == "" {
		httpx.WriteError(w, r, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Authentication required")
		return
	}

	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_NOTIFICATION_ID", "Notification ID is required in URL path")
		return
	}

	userRole := identitytransport.GetUserRole(r.Context())
	isDispatcher := userRole == identitydomain.RoleDispatcher

	updated, err := h.service.MarkAsRead(r.Context(), id, userID, isDispatcher)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidNotificationID) {
			httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_NOTIFICATION_ID", "Notification ID is invalid")
			return
		}
		if errors.Is(err, domain.ErrNotificationNotFound) {
			httpx.WriteError(w, r, http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "Notification not found")
			return
		}
		if errors.Is(err, domain.ErrUnauthorizedAccess) {
			httpx.WriteError(w, r, http.StatusForbidden, "NOTIFICATION_FORBIDDEN", "You do not have permission to access this notification")
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark notification as read")
		return
	}

	_ = httpx.WriteData(w, http.StatusOK, ToNotificationResponse(updated))
}

// MarkAllAsRead godoc
// @Summary      Mark all notifications as read
// @Description  Marks all unread notifications as read for the authenticated user.
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} httpx.SingleDataEnvelope[MarkAllReadResponse]
// @Failure      401 {object} httpx.ErrorEnvelope "Unauthorized"
// @Failure      500 {object} httpx.ErrorEnvelope "Internal server error"
// @Router       /api/v1/notifications/read-all [patch]
func (h *Handler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	userID := identitytransport.GetUserID(r.Context())
	if userID == "" {
		httpx.WriteError(w, r, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Authentication required")
		return
	}

	userRole := identitytransport.GetUserRole(r.Context())
	isDispatcher := userRole == identitydomain.RoleDispatcher

	count, err := h.service.MarkAllAsRead(r.Context(), userID, isDispatcher)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark all notifications as read")
		return
	}

	_ = httpx.WriteData(w, http.StatusOK, MarkAllReadResponse{UpdatedCount: count})
}
