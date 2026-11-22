// Package transport holds the admin HTTP handlers (/api/v1/admin/*). Handlers stay
// thin: decode, call the use case, map the error, encode (Engineering Standards 4.2).
package transport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/swiftdrop/core-api/internal/admin/application"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
	"github.com/swiftdrop/core-api/internal/platform/middleware"
)

// RoleAdmin is the role allowed on every /api/v1/admin/* route. RoleDispatcher may
// also read the delivery report (SRS 6.2, FR-ADM-008).
const (
	RoleAdmin      = "ADMIN"
	RoleDispatcher = "DISPATCHER"
)

// Paging rules (api-conventions 2.2): page starts at 1, per_page is 1 to 100, default 20.
const (
	defaultPerPage = 20
	maxPerPage     = 100
)

// maxBodyBytes caps admin request bodies (mass assignment and size, Doc 7 T-15).
const maxBodyBytes = 1 << 20

// Principal is the verified caller, as the auth adapter reports it.
type Principal struct {
	UserID string
	Role   string
}

// Auth is supplied by the wiring code (identity's middleware). Require must reject
// a missing or invalid token with 401 and a wrong role with 403.
type Auth interface {
	Require(roles ...string) func(http.Handler) http.Handler
	Principal(ctx context.Context) (Principal, bool)
}

type Handlers struct {
	Svc  *application.Service
	Auth Auth
	Log  *slog.Logger
	// Reports reads delivery numbers from Members 2 and 3 through a Go interface.
	// Nil means the stand-in (NoDeliveryData), which says so in the report's "source".
	Reports      application.ReportReader
	ReportSource string
}

// Register mounts every admin route behind Require(ADMIN), except the delivery report
// which also admits DISPATCHER. There is no admin route without the guard (Doc 7 B-01).
// Only GET /api/v1/zones is public (Doc 1 R-18).
//
// The merchant approval routes (GET /api/v1/admin/merchants, GET .../{id},
// POST .../{id}/{action}) are NOT mounted here: Member 4's merchants module already
// serves them, and registering the same patterns twice makes the router panic.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/zones", h.publicZones)
	mux.Handle("GET /api/v1/admin/reports/deliveries", h.Auth.Require(RoleAdmin, RoleDispatcher)(http.HandlerFunc(h.deliveryReport)))
	admin := h.Auth.Require(RoleAdmin)
	routes := map[string]http.HandlerFunc{
		"GET /api/v1/admin/zones":               h.listZones,
		"POST /api/v1/admin/zones":              h.createZone,
		"GET /api/v1/admin/zones/{id}":          h.getZone,
		"PATCH /api/v1/admin/zones/{id}":        h.updateZone,
		"PATCH /api/v1/admin/zones/{id}/status": h.setStatus,
		"GET /api/v1/admin/settings":            h.listSettings,
		"PUT /api/v1/admin/settings/{key}":      h.updateSetting,
		"GET /api/v1/admin/audit-logs":          h.listAudit,
	}
	for pattern, fn := range routes {
		mux.Handle(pattern, admin(fn))
	}
}

// actor builds the audit actor from the verified principal and request facts.
func (h *Handlers) actor(r *http.Request) application.Actor {
	p, _ := h.Auth.Principal(r.Context())
	return application.Actor{ID: p.UserID, Role: p.Role, IP: clientIP(r), UserAgent: r.UserAgent(), RequestID: requestID(r)}
}

// requestID prefers the id set by the RequestID middleware, which validates it.
func requestID(r *http.Request) string {
	if id := middleware.CorrelationID(r.Context()); id != "" {
		return id
	}
	return r.Header.Get("X-Request-ID")
}

// clientIP uses the right-most X-Forwarded-For entry, which our gateway wrote
// (Doc 7 section 6.2). Without it, the direct peer address.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// decode reads at most 1 MB and refuses unknown fields (mass assignment, Doc 7 T-15).
func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	if err := httpx.DecodeJSON(w, r, dst, maxBodyBytes); err != nil {
		return fmt.Errorf("%w: %v", application.ErrBadRequest, err)
	}
	return nil
}

func reply(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Cache-Control", "no-store")
	_ = httpx.WriteData(w, status, data)
}

type mapping struct {
	status        int
	code, message string
}

// Known errors get a fixed, safe message. Anything else becomes a generic 500.
var known = []struct {
	err error
	m   mapping
}{
	{application.ErrNotFound, mapping{404, "NOT_FOUND", "Not found"}},
	{application.ErrValidation, mapping{422, "VALIDATION_FAILED", "Some fields are not valid"}},
	{application.ErrUnknownSetting, mapping{422, "SETTING_UNKNOWN", "This setting cannot be changed"}},
	{application.ErrReasonRequired, mapping{422, "REASON_REQUIRED", "Give a reason of 10 to 500 characters for this change"}},
	{application.ErrConcurrentConflict, mapping{409, "CONCURRENT_MODIFICATION", "Someone else changed this. Reload and try again"}},
	{application.ErrPrefixConflict, mapping{409, "ZONE_POSTAL_CODE_CONFLICT", "A postal prefix is already used by another zone"}},
	{application.ErrZoneCodeTaken, mapping{409, "ZONE_CODE_ALREADY_EXISTS", "This zone id or code is already used"}},
	{application.ErrZoneNotFound, mapping{404, "ZONE_NOT_FOUND", "No delivery zone covers this postal code"}},
	{application.ErrBadRequest, mapping{400, "BAD_REQUEST", "The request body is not valid JSON for this action"}},
	{application.ErrBusy, mapping{503, "TRY_AGAIN", "Another change is in progress. Try again"}},
	{application.ErrUnavailable, mapping{503, "SERVICE_UNAVAILABLE", "The service is not available right now. Try again shortly"}},
}

// fail writes the standard error envelope. The raw error text goes to the server
// log only, never to the client (no SQL details or constraint names leak).
func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	m := mapping{500, "INTERNAL_SERVER_ERROR", "Something went wrong. Try again later"}
	for _, k := range known {
		if errors.Is(err, k.err) {
			m = k.m
			break
		}
	}
	id := requestID(r)
	if m.status >= 500 && h.Log != nil {
		h.Log.Error("admin request failed", "correlation_id", id, "method", r.Method, "path", r.URL.Path, "err", err)
	}
	httpx.WriteError(w, withRequestIDHeader(r, id), m.status, m.code, m.message)
}

// withRequestIDHeader works around httpx reading the correlation id only from the
// X-Request-ID request header: middleware.RequestID keeps a generated id in the
// context under its own key type, which httpx cannot see. It copies the request
// and its headers, so the caller's request is not changed.
func withRequestIDHeader(r *http.Request, id string) *http.Request {
	if id == "" || r.Header.Get("X-Request-ID") != "" {
		return r
	}
	c := r.WithContext(r.Context())
	c.Header = r.Header.Clone()
	c.Header.Set("X-Request-ID", id)
	return c
}

// pageParams reads ?page= and ?per_page= (api-conventions 2.2). Missing values use the
// defaults; a value that is not a whole number in range is a 422 VALIDATION_FAILED.
func pageParams(r *http.Request) (page, perPage int, err error) {
	page, perPage = 1, defaultPerPage
	q := r.URL.Query()
	if v := q.Get("page"); v != "" {
		n, convErr := strconv.Atoi(v)
		if convErr != nil || n < 1 {
			return 0, 0, fmt.Errorf("%w: page must be a whole number from 1", application.ErrValidation)
		}
		page = n
	}
	if v := q.Get("per_page"); v != "" {
		n, convErr := strconv.Atoi(v)
		if convErr != nil || n < 1 || n > maxPerPage {
			return 0, 0, fmt.Errorf("%w: per_page must be 1 to %d", application.ErrValidation, maxPerPage)
		}
		perPage = n
	}
	return page, perPage, nil
}

// auditFilterParams reads the audit filters from the query string. A parameter that is sent
// twice is refused, so a request can never mean two things at once.
func auditFilterParams(r *http.Request) (application.AuditFilter, error) {
	q := r.URL.Query()
	for _, name := range []string{"action", "target_type", "target_id", "actor_role", "from", "to"} {
		if len(q[name]) > 1 {
			return application.AuditFilter{}, fmt.Errorf("%w: %s was sent more than once", application.ErrValidation, name)
		}
	}
	return application.ParseAuditFilter(q.Get("action"), q.Get("target_type"), q.Get("target_id"), q.Get("actor_role"), q.Get("from"), q.Get("to"))
}

// replyPage writes {"data": [...], "meta": {page, per_page, total_items, total_pages}}.
func replyPage[T any](w http.ResponseWriter, data []T, page, perPage, total int) {
	w.Header().Set("Cache-Control", "no-store")
	_ = httpx.WriteJSON(w, http.StatusOK, httpx.NewPaginatedResponse(data, page, perPage, total))
}

// @Summary      List delivery zones (admin)
// @Tags         admin-zones
// @Produce      json
// @Security     BearerAuth
// @Param        page      query int false "Page, from 1"          default(1)
// @Param        per_page  query int false "Items per page, 1-100" default(20)
// @Success      200 {object} httpx.PaginatedEnvelope{data=[]application.ZoneView}
// @Failure      401 {object} httpx.ErrorResponse "Missing or invalid token"
// @Failure      403 {object} httpx.ErrorResponse "Role is not ADMIN"
// @Failure      422 {object} httpx.ErrorResponse "VALIDATION_FAILED: bad page or per_page"
// @Router       /api/v1/admin/zones [get]
func (h *Handlers) listZones(w http.ResponseWriter, r *http.Request) {
	page, perPage, err := pageParams(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	zs, total, err := h.Svc.Store.ListZones(r.Context(), (page-1)*perPage, perPage)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]application.ZoneView, 0, len(zs))
	for _, z := range zs {
		out = append(out, application.View(z))
	}
	replyPage(w, out, page, perPage, total)
}

// @Summary      Get one delivery zone (admin)
// @Tags         admin-zones
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Zone id, e.g. ZONE_COLOMBO_01"
// @Success      200 {object} httpx.DataEnvelope{data=application.ZoneView}
// @Failure      401 {object} httpx.ErrorResponse "Missing or invalid token"
// @Failure      403 {object} httpx.ErrorResponse "Role is not ADMIN"
// @Failure      404 {object} httpx.ErrorResponse "NOT_FOUND"
// @Router       /api/v1/admin/zones/{id} [get]
func (h *Handlers) getZone(w http.ResponseWriter, r *http.Request) {
	z, err := h.Svc.Store.GetZone(r.Context(), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, http.StatusOK, application.View(z))
}

// @Summary      Create a delivery zone (admin)
// @Tags         admin-zones
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body application.CreateZoneInput true "New zone"
// @Success      201 {object} httpx.DataEnvelope{data=application.ZoneView}
// @Failure      400 {object} httpx.ErrorResponse "BAD_REQUEST: malformed JSON"
// @Failure      401 {object} httpx.ErrorResponse "Missing or invalid token"
// @Failure      403 {object} httpx.ErrorResponse "Role is not ADMIN"
// @Failure      409 {object} httpx.ErrorResponse "ZONE_CODE_ALREADY_EXISTS or ZONE_POSTAL_CODE_CONFLICT"
// @Failure      422 {object} httpx.ErrorResponse "VALIDATION_FAILED"
// @Router       /api/v1/admin/zones [post]
func (h *Handlers) createZone(w http.ResponseWriter, r *http.Request) {
	var in application.CreateZoneInput
	if err := decode(w, r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.Svc.CreateZone(r.Context(), h.actor(r), in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/admin/zones/"+v.ID)
	reply(w, http.StatusCreated, v)
}

// @Summary      Edit a delivery zone (admin)
// @Tags         admin-zones
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path string                    true "Zone id"
// @Param        request body application.UpdateZoneInput true "Fields to change plus current version"
// @Success      200 {object} httpx.DataEnvelope{data=application.ZoneView}
// @Failure      400 {object} httpx.ErrorResponse "BAD_REQUEST: malformed JSON"
// @Failure      401 {object} httpx.ErrorResponse "Missing or invalid token"
// @Failure      403 {object} httpx.ErrorResponse "Role is not ADMIN"
// @Failure      404 {object} httpx.ErrorResponse "NOT_FOUND"
// @Failure      409 {object} httpx.ErrorResponse "CONCURRENT_MODIFICATION, ZONE_CODE_ALREADY_EXISTS or ZONE_POSTAL_CODE_CONFLICT"
// @Failure      422 {object} httpx.ErrorResponse "VALIDATION_FAILED"
// @Router       /api/v1/admin/zones/{id} [patch]
func (h *Handlers) updateZone(w http.ResponseWriter, r *http.Request) {
	var in application.UpdateZoneInput
	if err := decode(w, r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.Svc.UpdateZone(r.Context(), h.actor(r), r.PathValue("id"), in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, http.StatusOK, v)
}

// @Summary      Enable or disable a delivery zone (admin)
// @Tags         admin-zones
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path string                 true "Zone id"
// @Param        request body application.StatusInput true "enabled, current version and a 10-500 char reason"
// @Success      200 {object} httpx.DataEnvelope{data=application.ZoneView}
// @Failure      400 {object} httpx.ErrorResponse "BAD_REQUEST: malformed JSON"
// @Failure      401 {object} httpx.ErrorResponse "Missing or invalid token"
// @Failure      403 {object} httpx.ErrorResponse "Role is not ADMIN"
// @Failure      404 {object} httpx.ErrorResponse "NOT_FOUND"
// @Failure      409 {object} httpx.ErrorResponse "CONCURRENT_MODIFICATION"
// @Failure      422 {object} httpx.ErrorResponse "VALIDATION_FAILED or REASON_REQUIRED"
// @Router       /api/v1/admin/zones/{id}/status [patch]
func (h *Handlers) setStatus(w http.ResponseWriter, r *http.Request) {
	var in application.StatusInput
	if err := decode(w, r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.Svc.SetZoneEnabled(r.Context(), h.actor(r), r.PathValue("id"), in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, http.StatusOK, v)
}

// @Summary      List system settings (admin)
// @Tags         admin-settings
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} httpx.DataEnvelope{data=[]application.Setting}
// @Failure      401 {object} httpx.ErrorResponse "Missing or invalid token"
// @Failure      403 {object} httpx.ErrorResponse "Role is not ADMIN"
// @Router       /api/v1/admin/settings [get]
func (h *Handlers) listSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.Svc.Store.ListSettings(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, http.StatusOK, s)
}

// @Summary      Change one system setting (admin)
// @Tags         admin-settings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        key     path string                   true "reports.max_range_days, ops.board_refresh_seconds, ops.maintenance_banner, parcels.max_weight_kg or parcels.max_cod_cents"
// @Param        request body application.SettingInput true "value, current version and a 10-500 char reason"
// @Success      200 {object} httpx.DataEnvelope{data=application.Setting}
// @Failure      400 {object} httpx.ErrorResponse "BAD_REQUEST: malformed JSON"
// @Failure      401 {object} httpx.ErrorResponse "Missing or invalid token"
// @Failure      403 {object} httpx.ErrorResponse "Role is not ADMIN"
// @Failure      409 {object} httpx.ErrorResponse "CONCURRENT_MODIFICATION"
// @Failure      422 {object} httpx.ErrorResponse "SETTING_UNKNOWN, REASON_REQUIRED or VALIDATION_FAILED"
// @Router       /api/v1/admin/settings/{key} [put]
func (h *Handlers) updateSetting(w http.ResponseWriter, r *http.Request) {
	var in application.SettingInput
	if err := decode(w, r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	s, err := h.Svc.UpdateSetting(r.Context(), h.actor(r), r.PathValue("key"), in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, http.StatusOK, s)
}

// @Summary      List audit log rows, newest first (admin)
// @Tags         admin-audit
// @Produce      json
// @Security     BearerAuth
// @Param        page      query int false "Page, from 1"          default(1)
// @Param        per_page  query int false "Items per page, 1-100" default(20)
// @Param        action      query string false "Exact action, for example ZONE_CREATED"
// @Param        target_type query string false "Exact target type, for example ZONE"
// @Param        target_id   query string false "Exact target id, for example ZONE_COLOMBO_01"
// @Param        actor_role  query string false "ADMIN, DISPATCHER, COURIER, MERCHANT, CUSTOMER or SYSTEM"
// @Param        from        query string false "First Sri Lanka day to include, YYYY-MM-DD"
// @Param        to          query string false "Last Sri Lanka day to include, YYYY-MM-DD"
// @Success      200 {object} httpx.PaginatedEnvelope{data=[]application.AuditRow}
// @Failure      401 {object} httpx.ErrorResponse "Missing or invalid token"
// @Failure      403 {object} httpx.ErrorResponse "Role is not ADMIN"
// @Failure      422 {object} httpx.ErrorResponse "VALIDATION_FAILED: bad page, per_page or filter"
// @Router       /api/v1/admin/audit-logs [get]
func (h *Handlers) listAudit(w http.ResponseWriter, r *http.Request) {
	page, perPage, err := pageParams(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	filter, err := auditFilterParams(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	rows, total, err := h.Svc.Store.ListAudit(r.Context(), filter, (page-1)*perPage, perPage)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	replyPage(w, rows, page, perPage, total)
}

// deliveryReport serves GET /api/v1/admin/reports/deliveries?from=&to= for ADMIN and
// DISPATCHER. Numbers come through the ReportReader interface, never from SQL here.
//
// @Summary      Delivery report for a date range (admin, dispatcher)
// @Tags         admin-reports
// @Produce      json
// @Security     BearerAuth
// @Param        from query string true "Start date, YYYY-MM-DD"
// @Param        to   query string true "End date, YYYY-MM-DD"
// @Success      200 {object} httpx.DataEnvelope{data=application.DeliveryReport}
// @Failure      401 {object} httpx.ErrorResponse "Missing or invalid token"
// @Failure      403 {object} httpx.ErrorResponse "Role is not ADMIN or DISPATCHER"
// @Failure      422 {object} httpx.ErrorResponse "VALIDATION_FAILED: bad dates or range too long"
// @Router       /api/v1/admin/reports/deliveries [get]
func (h *Handlers) deliveryReport(w http.ResponseWriter, r *http.Request) {
	reader, source := h.Reports, h.ReportSource
	if reader == nil {
		reader, source = application.NoDeliveryData{}, application.NoDeliveryDataSource
	}
	q := r.URL.Query()
	rep, err := h.Svc.DeliveryReport(r.Context(), reader, source, q.Get("from"), q.Get("to"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, http.StatusOK, rep)
}

var postalCodePattern = regexp.MustCompile(`^[0-9]{5}$`)

// publicZones lists enabled zones for booking forms, or finds the zone of one
// postal code. No sign-in; only id, code and name are shown (Doc 1 R-18).
//
// @Summary      List enabled zones, or find the zone of a postal code (public)
// @Tags         zones
// @Produce      json
// @Param        postal_code query string false "5-digit postal code; returns the one matching zone"
// @Success      200 {object} httpx.DataEnvelope{data=[]application.PublicZone}
// @Failure      404 {object} httpx.ErrorResponse "ZONE_NOT_FOUND: no zone covers this postal code"
// @Failure      422 {object} httpx.ErrorResponse "VALIDATION_FAILED: postal_code must be 5 digits"
// @Router       /api/v1/zones [get]
func (h *Handlers) publicZones(w http.ResponseWriter, r *http.Request) {
	if pc, ok := r.URL.Query()["postal_code"]; ok {
		if len(pc) != 1 || !postalCodePattern.MatchString(pc[0]) {
			h.fail(w, r, fmt.Errorf("%w: postal_code must be 5 digits", application.ErrValidation))
			return
		}
		z, err := h.Svc.Store.ZoneForPostalCode(r.Context(), pc[0])
		if err != nil {
			h.fail(w, r, err)
			return
		}
		replyCached(w, z)
		return
	}
	zs, err := h.Svc.Store.ListPublicZones(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if zs == nil {
		zs = []application.PublicZone{}
	}
	replyCached(w, zs)
}

func replyCached(w http.ResponseWriter, data any) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = httpx.WriteData(w, http.StatusOK, data)
}
