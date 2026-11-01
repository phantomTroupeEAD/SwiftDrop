package transport

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/swiftdrop/core-api/internal/events/application"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

// Handler serves HTTP endpoints for events admin operations and merchant delivery history.
type Handler struct {
	service *application.EventsService
}

// NewHandler initializes a new Handler.
func NewHandler(service *application.EventsService) *Handler {
	return &Handler{service: service}
}

// HandleListOutbox handles GET /api/v1/admin/events/outbox
func (h *Handler) HandleListOutbox(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := 25
	if limitStr := q.Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed <= 0 || parsed > 100 {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "INVALID_LIMIT", "limit must be an integer between 1 and 100")
			return
		}
		limit = parsed
	}

	var cursor httpx.Cursor
	if cursorStr := q.Get("cursor"); cursorStr != "" {
		cAt, cID, err := httpx.DecodeCursor(cursorStr)
		if err != nil {
			httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "opaque cursor string is malformed")
			return
		}
		cursor = httpx.Cursor{CreatedAt: cAt, ID: cID}
	}

	filter := application.OutboxFilter{
		Status:    q.Get("status"),
		EventType: q.Get("event_type"),
	}

	if fromStr := q.Get("from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_FROM_TIMESTAMP", "from must be ISO-8601 RFC3339")
			return
		}
		filter.From = &t
	}

	if toStr := q.Get("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_TO_TIMESTAMP", "to must be ISO-8601 RFC3339")
			return
		}
		filter.To = &t
	}

	items, nextCursor, err := h.service.ListOutbox(r.Context(), filter, cursor, limit)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "OUTBOX_QUERY_FAILED", "failed to query outbox")
		return
	}

	meta := httpx.CursorMeta{
		Limit:   limit,
		HasMore: nextCursor != nil,
	}
	if nextCursor != nil {
		nextStr := httpx.EncodeCursor(nextCursor.CreatedAt, nextCursor.ID)
		meta.NextCursor = &nextStr
	}

	_ = httpx.WriteCursorPaginated(w, http.StatusOK, items, meta)
}

// HandleGetOutboxStats handles GET /api/v1/admin/events/outbox/stats
func (h *Handler) HandleGetOutboxStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetOutboxStats(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "OUTBOX_STATS_FAILED", "failed to compute outbox stats")
		return
	}
	_ = httpx.WriteData(w, http.StatusOK, stats)
}

// HandleRetryOutbox handles POST /api/v1/admin/events/outbox/{id}/retry
func (h *Handler) HandleRetryOutbox(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRegex.MatchString(id) {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_EVENT_ID", "event id must be a valid UUID")
		return
	}

	err := h.service.RetryOutbox(r.Context(), id)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			httpx.WriteError(w, r, http.StatusNotFound, "OUTBOX_EVENT_NOT_FOUND", "outbox event not found")
			return
		}
		if errors.Is(err, application.ErrInvalidState) {
			httpx.WriteError(w, r, http.StatusConflict, "OUTBOX_EVENT_NOT_FAILED", "outbox event can only be retried if currently in FAILED state")
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "OUTBOX_RETRY_FAILED", err.Error())
		return
	}

	_ = httpx.WriteData(w, http.StatusOK, map[string]string{
		"message": "outbox event reset to PENDING",
		"id":      id,
	})
}

// HandleListDeliveries handles GET /api/v1/admin/events/deliveries
func (h *Handler) HandleListDeliveries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := 25
	if limitStr := q.Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed <= 0 || parsed > 100 {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "INVALID_LIMIT", "limit must be an integer between 1 and 100")
			return
		}
		limit = parsed
	}

	var cursor httpx.Cursor
	if cursorStr := q.Get("cursor"); cursorStr != "" {
		cAt, cID, err := httpx.DecodeCursor(cursorStr)
		if err != nil {
			httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "opaque cursor string is malformed")
			return
		}
		cursor = httpx.Cursor{CreatedAt: cAt, ID: cID}
	}

	filter := application.DeliveryFilter{
		Kind:   q.Get("kind"),
		Status: q.Get("status"),
	}

	items, nextCursor, err := h.service.ListDeliveries(r.Context(), filter, cursor, limit)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "DELIVERY_QUERY_FAILED", "failed to query deliveries")
		return
	}

	meta := httpx.CursorMeta{
		Limit:   limit,
		HasMore: nextCursor != nil,
	}
	if nextCursor != nil {
		nextStr := httpx.EncodeCursor(nextCursor.CreatedAt, nextCursor.ID)
		meta.NextCursor = &nextStr
	}

	_ = httpx.WriteCursorPaginated(w, http.StatusOK, items, meta)
}

// HandleRetryWebhookDelivery handles POST /api/v1/admin/events/webhook-deliveries/{id}/retry
func (h *Handler) HandleRetryWebhookDelivery(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRegex.MatchString(id) {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_DELIVERY_ID", "delivery id must be a valid UUID")
		return
	}

	err := h.service.RetryWebhookDelivery(r.Context(), id)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			httpx.WriteError(w, r, http.StatusNotFound, "WEBHOOK_DELIVERY_NOT_FOUND", "webhook delivery not found")
			return
		}
		if errors.Is(err, application.ErrInvalidState) {
			httpx.WriteError(w, r, http.StatusConflict, "WEBHOOK_DELIVERY_NOT_FAILED", "webhook delivery can only be retried if currently in FAILED state")
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "WEBHOOK_RETRY_FAILED", err.Error())
		return
	}

	_ = httpx.WriteData(w, http.StatusOK, map[string]string{
		"message": "webhook delivery reset to PENDING",
		"id":      id,
	})
}

// HandleListMerchantWebhookDeliveries handles GET /api/v1/merchant/webhook-deliveries
func (h *Handler) HandleListMerchantWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := GetMerchantID(r.Context())
	if !ok || merchantID == "" {
		httpx.WriteError(w, r, http.StatusForbidden, "MISSING_MERCHANT_IDENTITY", "merchant identity could not be verified")
		return
	}

	q := r.URL.Query()
	limit := 25
	if limitStr := q.Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed <= 0 || parsed > 100 {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "INVALID_LIMIT", "limit must be an integer between 1 and 100")
			return
		}
		limit = parsed
	}

	var cursor httpx.Cursor
	if cursorStr := q.Get("cursor"); cursorStr != "" {
		cAt, cID, err := httpx.DecodeCursor(cursorStr)
		if err != nil {
			httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "opaque cursor string is malformed")
			return
		}
		cursor = httpx.Cursor{CreatedAt: cAt, ID: cID}
	}

	items, nextCursor, err := h.service.ListMerchantWebhookDeliveries(r.Context(), merchantID, cursor, limit)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "MERCHANT_DELIVERY_QUERY_FAILED", "failed to query deliveries")
		return
	}

	meta := httpx.CursorMeta{
		Limit:   limit,
		HasMore: nextCursor != nil,
	}
	if nextCursor != nil {
		nextStr := httpx.EncodeCursor(nextCursor.CreatedAt, nextCursor.ID)
		meta.NextCursor = &nextStr
	}

	_ = httpx.WriteCursorPaginated(w, http.StatusOK, items, meta)
}

// HandleListMerchantWebhookAttempts handles GET /api/v1/merchant/webhook-deliveries/{id}/attempts
func (h *Handler) HandleListMerchantWebhookAttempts(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := GetMerchantID(r.Context())
	if !ok || merchantID == "" {
		httpx.WriteError(w, r, http.StatusForbidden, "MISSING_MERCHANT_IDENTITY", "merchant identity could not be verified")
		return
	}

	id := r.PathValue("id")
	if !uuidRegex.MatchString(id) {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_DELIVERY_ID", "delivery id must be a valid UUID")
		return
	}

	attempts, err := h.service.ListMerchantWebhookAttempts(r.Context(), merchantID, id)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			// Foreign delivery ID returns 404 (IDOR prevention)
			httpx.WriteError(w, r, http.StatusNotFound, "DELIVERY_NOT_FOUND", "webhook delivery not found or not owned by merchant")
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "ATTEMPTS_QUERY_FAILED", err.Error())
		return
	}

	_ = httpx.WriteData(w, http.StatusOK, attempts)
}
