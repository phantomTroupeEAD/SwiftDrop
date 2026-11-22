package transport

import (
	"net/http"

	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

// RegisterRoutes registers all notification HTTP REST endpoints onto the provided ServeMux.
func RegisterRoutes(mux *http.ServeMux, h *Handler, sec *identitytransport.SecurityMiddleware) {
	mux.Handle("GET /api/v1/notifications", sec.Authenticate(http.HandlerFunc(h.ListNotifications)))
	mux.Handle("GET /api/v1/notifications/unread-count", sec.Authenticate(http.HandlerFunc(h.GetUnreadCount)))
	mux.Handle("PATCH /api/v1/notifications/{id}/read", sec.Authenticate(http.HandlerFunc(h.MarkAsRead)))
	mux.Handle("PATCH /api/v1/notifications/read-all", sec.Authenticate(http.HandlerFunc(h.MarkAllAsRead)))
}
