// Package transport holds the admin HTTP handlers (/api/v1/admin/*). Handlers stay
// thin: decode, call the use case, map the error, encode (Engineering Standards 4.2).
package transport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"swiftdrop.local/m7/core-api/internal/admin/application"
	"swiftdrop.local/m7/core-api/internal/merchants"
	"swiftdrop.local/m7/core-api/internal/platform/apperr"
	"swiftdrop.local/m7/core-api/internal/platform/obs"
	"swiftdrop.local/m7/core-api/internal/platform/web"
)

type Handlers struct {
	Svc          *application.Service
	Metrics      *obs.Metrics
	Merchants    merchants.AdminService   // Member 4's facade (Doc 1 R-02)
	Reports      application.ReportReader // Members 2 and 3 read interfaces
	ReportSource string                   // shown in the report so stand-in data is never mistaken for real data
}

// Register mounts every admin route behind RequireRole. There is no admin route
// without the guard (Doc 7 B-01). Only GET /api/v1/zones is public (Doc 1 R-18).
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.Handle(obs.Route("GET /api/v1/zones", http.HandlerFunc(h.publicZones)))
	reports := web.RequireRole(web.RoleAdmin, web.RoleDispatcher) // Doc 1 R-16
	mux.Handle(obs.Route("GET /api/v1/admin/reports/deliveries", reports(http.HandlerFunc(h.deliveryReport))))
	admin := web.RequireRole(web.RoleAdmin)
	routes := map[string]http.HandlerFunc{
		"GET /api/v1/admin/merchants":                h.listMerchants,
		"GET /api/v1/admin/merchants/{id}":           h.getMerchant,
		"POST /api/v1/admin/merchants/{id}/{action}": h.changeMerchantStatus,
		"GET /api/v1/admin/zones":                    h.listZones,
		"POST /api/v1/admin/zones":                   h.createZone,
		"GET /api/v1/admin/zones/{id}":               h.getZone,
		"PATCH /api/v1/admin/zones/{id}":             h.updateZone,
		"PATCH /api/v1/admin/zones/{id}/status":      h.setStatus,
		"GET /api/v1/admin/settings":                 h.listSettings,
		"PUT /api/v1/admin/settings/{key}":           h.updateSetting,
		"GET /api/v1/admin/audit-logs":               h.listAudit,
	}
	for pattern, fn := range routes {
		mux.Handle(obs.Route(pattern, admin(fn)))
	}
}

// actor builds the audit actor from the verified principal and request facts.
func actor(r *http.Request) application.Actor {
	p, _ := web.PrincipalFrom(r.Context())
	return application.Actor{ID: p.UserID, Role: p.Role, IP: clientIP(r), UserAgent: r.UserAgent(), RequestID: r.Header.Get("X-Request-ID")}
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
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: %v", apperr.ErrBadRequest, err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("%w: one JSON object only", apperr.ErrBadRequest)
	}
	return nil
}

func reply(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, apperr.ErrConcurrentConflict):
		h.Metrics.Inc("admin_conflicts_total", "Admin writes refused by a conflict.", "kind", "version")
	case errors.Is(err, apperr.ErrPrefixConflict):
		h.Metrics.Inc("admin_conflicts_total", "Admin writes refused by a conflict.", "kind", "prefix_overlap")
	case errors.Is(err, apperr.ErrBusy):
		h.Metrics.Inc("admin_conflicts_total", "Admin writes refused by a conflict.", "kind", "lock_timeout")
	}
	web.WriteError(w, r, err)
}

func (h *Handlers) changed(action string) {
	h.Metrics.Inc("admin_changes_total", "Successful admin changes (each has one audit row).", "action", action)
}

func (h *Handlers) listZones(w http.ResponseWriter, r *http.Request) {
	zs, err := h.Svc.Store.ListZones(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]application.ZoneView, 0, len(zs))
	for _, z := range zs {
		out = append(out, application.View(z))
	}
	reply(w, 200, out)
}

func (h *Handlers) getZone(w http.ResponseWriter, r *http.Request) {
	z, err := h.Svc.Store.GetZone(r.Context(), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, 200, application.View(z))
}

func (h *Handlers) createZone(w http.ResponseWriter, r *http.Request) {
	var in application.CreateZoneInput
	if err := decode(w, r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.Svc.CreateZone(r.Context(), actor(r), in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.changed("ZONE_CREATED")
	w.Header().Set("Location", "/api/v1/admin/zones/"+v.ID)
	reply(w, 201, v)
}

func (h *Handlers) updateZone(w http.ResponseWriter, r *http.Request) {
	var in application.UpdateZoneInput
	if err := decode(w, r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.Svc.UpdateZone(r.Context(), actor(r), r.PathValue("id"), in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.changed("ZONE_UPDATED")
	reply(w, 200, v)
}

func (h *Handlers) setStatus(w http.ResponseWriter, r *http.Request) {
	var in application.StatusInput
	if err := decode(w, r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.Svc.SetZoneEnabled(r.Context(), actor(r), r.PathValue("id"), in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.changed("ZONE_STATUS_CHANGED")
	reply(w, 200, v)
}

func (h *Handlers) listSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.Svc.Store.ListSettings(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, 200, s)
}

func (h *Handlers) updateSetting(w http.ResponseWriter, r *http.Request) {
	var in application.SettingInput
	if err := decode(w, r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	s, err := h.Svc.UpdateSetting(r.Context(), actor(r), r.PathValue("key"), in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.changed("SETTING_CHANGED")
	reply(w, 200, s)
}

func (h *Handlers) listAudit(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			h.fail(w, r, fmt.Errorf("%w: limit must be 1 to 100", apperr.ErrValidation))
			return
		}
		limit = n
	}
	rows, err := h.Svc.Store.ListAudit(r.Context(), limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, 200, rows)
}

var postalCodePattern = regexp.MustCompile(`^[0-9]{5}$`)

// publicZones lists enabled zones for booking forms, or finds the zone of one
// postal code. No sign-in; only id, code and name are shown (Doc 1 R-18).
func (h *Handlers) publicZones(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	if pc, ok := r.URL.Query()["postal_code"]; ok {
		if len(pc) != 1 || !postalCodePattern.MatchString(pc[0]) {
			h.fail(w, r, fmt.Errorf("%w: postal_code must be 5 digits", apperr.ErrValidation))
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func (h *Handlers) deliveryReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rep, err := h.Svc.DeliveryReport(r.Context(), h.Reports, h.ReportSource, q.Get("from"), q.Get("to"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, 200, rep)
}

// ---- Merchant admin: route and role check here, rules in Member 4's module ----

var merchantActions = map[string]string{"approve": "APPROVE", "reject": "REJECT", "suspend": "SUSPEND", "reinstate": "REINSTATE", "disable": "DISABLE"}

var merchantStatuses = map[string]bool{"PENDING": true, "APPROVED": true, "REJECTED": true, "SUSPENDED": true, "DISABLED": true}

// merchantFail passes Member 4's error codes through unchanged (their OpenAPI).
func (h *Handlers) merchantFail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, merchants.ErrNotFound):
		web.WriteCode(w, r, 404, "MERCHANT_NOT_FOUND", "Merchant not found")
	case errors.Is(err, merchants.ErrVersionConflict):
		h.Metrics.Inc("admin_conflicts_total", "Admin writes refused by a conflict.", "kind", "merchant_version")
		web.WriteCode(w, r, 409, "MERCHANT_VERSION_CONFLICT", "Someone else changed this merchant. Reload and try again")
	case errors.Is(err, merchants.ErrInvalidTransition):
		web.WriteCode(w, r, 409, "MERCHANT_INVALID_TRANSITION", "This action is not allowed in the merchant's current status")
	case errors.Is(err, merchants.ErrReasonRequired):
		web.WriteError(w, r, apperr.ErrReasonRequired)
	case errors.Is(err, merchants.ErrValidation):
		web.WriteError(w, r, apperr.ErrValidation)
	default:
		h.fail(w, r, err)
	}
}

func (h *Handlers) listMerchants(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := merchants.ListFilter{Query: strings.TrimSpace(q.Get("q"))}
	if s := q.Get("status"); s != "" {
		if !merchantStatuses[s] {
			h.fail(w, r, fmt.Errorf("%w: unknown status", apperr.ErrValidation))
			return
		}
		f.Status = &s
	}
	if len(f.Query) > 100 {
		h.fail(w, r, fmt.Errorf("%w: q is at most 100 characters", apperr.ErrValidation))
		return
	}
	page, okP := intParam(q.Get("page"), 1, 1, 10000)
	per, okS := intParam(q.Get("per_page"), 20, 1, 100)
	if !okP || !okS {
		h.fail(w, r, fmt.Errorf("%w: page must be 1 or more and per_page 1 to 100", apperr.ErrValidation))
		return
	}
	res, err := h.Merchants.ListMerchants(r.Context(), f, merchants.Page{Number: page, Size: per})
	if err != nil {
		h.merchantFail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": res.Items, "meta": map[string]int{"page": page, "per_page": per, "total": res.Total}})
}

func intParam(s string, def, lo, hi int) (int, bool) {
	if s == "" {
		return def, true
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= lo && n <= hi
}

func (h *Handlers) getMerchant(w http.ResponseWriter, r *http.Request) {
	m, err := h.Merchants.GetMerchant(r.Context(), r.PathValue("id"))
	if err != nil {
		h.merchantFail(w, r, err)
		return
	}
	reply(w, 200, m)
}

type merchantStatusInput struct {
	ExpectedVersion int    `json:"expected_version"`
	Reason          string `json:"reason,omitempty"`
}

func (h *Handlers) changeMerchantStatus(w http.ResponseWriter, r *http.Request) {
	action, ok := merchantActions[r.PathValue("action")]
	if !ok {
		web.WriteError(w, r, apperr.ErrNotFound)
		return
	}
	var in merchantStatusInput
	if err := decode(w, r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	if in.ExpectedVersion < 1 {
		h.fail(w, r, fmt.Errorf("%w: expected_version is required", apperr.ErrValidation))
		return
	}
	a := actor(r)
	m, err := h.Merchants.ChangeStatus(r.Context(), merchants.ChangeStatusCommand{
		MerchantID: r.PathValue("id"), Action: action, ExpectedVersion: in.ExpectedVersion, Reason: in.Reason,
		Actor: merchants.Actor{UserID: a.ID, Role: a.Role, RequestID: a.RequestID, ClientIP: a.IP, UserAgent: a.UserAgent},
	})
	if err != nil {
		h.merchantFail(w, r, err)
		return
	}
	h.changed("MERCHANT_" + action) // Member 4 writes the audit row in its own transaction
	reply(w, 200, m)
}
