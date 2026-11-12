package transport

import (
	"net/http"

	"github.com/swiftdrop/core-api/internal/identity/domain"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

// RouteDefinition holds metadata for an exported HTTP endpoint.
type RouteDefinition struct {
	Method       string
	Path         string
	RequiredRole string
	HandlerFunc  http.HandlerFunc
}

// ExportedRoutes contains all authoritative Phase 7 endpoints for spec comparison.
func ExportedRoutes(h *Handler) []RouteDefinition {
	return []RouteDefinition{
		{
			Method:       http.MethodGet,
			Path:         "/api/v1/admin/events/outbox",
			RequiredRole: string(domain.RoleAdmin),
			HandlerFunc:  h.HandleListOutbox,
		},
		{
			Method:       http.MethodGet,
			Path:         "/api/v1/admin/events/outbox/stats",
			RequiredRole: string(domain.RoleAdmin),
			HandlerFunc:  h.HandleGetOutboxStats,
		},
		{
			Method:       http.MethodPost,
			Path:         "/api/v1/admin/events/outbox/{id}/retry",
			RequiredRole: string(domain.RoleAdmin),
			HandlerFunc:  h.HandleRetryOutbox,
		},
		{
			Method:       http.MethodGet,
			Path:         "/api/v1/admin/events/deliveries",
			RequiredRole: string(domain.RoleAdmin),
			HandlerFunc:  h.HandleListDeliveries,
		},
		{
			Method:       http.MethodPost,
			Path:         "/api/v1/admin/events/webhook-deliveries/{id}/retry",
			RequiredRole: string(domain.RoleAdmin),
			HandlerFunc:  h.HandleRetryWebhookDelivery,
		},
		{
			Method:       http.MethodGet,
			Path:         "/api/v1/merchant/webhook-deliveries",
			RequiredRole: string(domain.RoleMerchant),
			HandlerFunc:  h.HandleListMerchantWebhookDeliveries,
		},
		{
			Method:       http.MethodGet,
			Path:         "/api/v1/merchant/webhook-deliveries/{id}/attempts",
			RequiredRole: string(domain.RoleMerchant),
			HandlerFunc:  h.HandleListMerchantWebhookAttempts,
		},
	}
}

// RegisterRoutes binds all events operations routes to the main application HTTP multiplexer.
func RegisterRoutes(mux *http.ServeMux, h *Handler, sec *identitytransport.SecurityMiddleware) {
	// 1. Admin Outbox Endpoints
	mux.Handle("GET /api/v1/admin/events/outbox", sec.Authenticate(
		identitytransport.RequireRole(domain.RoleAdmin)(
			http.HandlerFunc(h.HandleListOutbox),
		),
	))

	mux.Handle("GET /api/v1/admin/events/outbox/stats", sec.Authenticate(
		identitytransport.RequireRole(domain.RoleAdmin)(
			http.HandlerFunc(h.HandleGetOutboxStats),
		),
	))

	mux.Handle("POST /api/v1/admin/events/outbox/{id}/retry", sec.Authenticate(
		identitytransport.RequireRole(domain.RoleAdmin)(
			http.HandlerFunc(h.HandleRetryOutbox),
		),
	))

	// 2. Admin Delivery Endpoints
	mux.Handle("GET /api/v1/admin/events/deliveries", sec.Authenticate(
		identitytransport.RequireRole(domain.RoleAdmin)(
			http.HandlerFunc(h.HandleListDeliveries),
		),
	))

	mux.Handle("POST /api/v1/admin/events/webhook-deliveries/{id}/retry", sec.Authenticate(
		identitytransport.RequireRole(domain.RoleAdmin)(
			http.HandlerFunc(h.HandleRetryWebhookDelivery),
		),
	))

	// 3. Merchant Webhook Delivery Endpoints (Tenant Guarded per Decision D25)
	mux.Handle("GET /api/v1/merchant/webhook-deliveries", sec.Authenticate(
		identitytransport.RequireRole(domain.RoleMerchant, domain.RoleAdmin)(
			identitytransport.RequireTenant()(
				TenantResolver()(
					http.HandlerFunc(h.HandleListMerchantWebhookDeliveries),
				),
			),
		),
	))

	mux.Handle("GET /api/v1/merchant/webhook-deliveries/{id}/attempts", sec.Authenticate(
		identitytransport.RequireRole(domain.RoleMerchant, domain.RoleAdmin)(
			identitytransport.RequireTenant()(
				TenantResolver()(
					http.HandlerFunc(h.HandleListMerchantWebhookAttempts),
				),
			),
		),
	))
}
