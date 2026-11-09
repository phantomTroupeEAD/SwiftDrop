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

// RoleAdmin is the only role allowed on /api/v1/admin/* routes.
const RoleAdmin = "ADMIN"

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
}

// Register mounts every admin route behind Require(ADMIN). There is no admin route
// without the guard (Doc 7 B-01). Only GET /api/v1/zones is public (Doc 1 R-18).
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/zones", h.publicZones)
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
	reply(w, http.StatusOK, out)
}

func (h *Handlers) getZone(w http.ResponseWriter, r *http.Request) {
	z, err := h.Svc.Store.GetZone(r.Context(), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, http.StatusOK, application.View(z))
}

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

func (h *Handlers) listSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.Svc.Store.ListSettings(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, http.StatusOK, s)
}

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

func (h *Handlers) listAudit(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			h.fail(w, r, fmt.Errorf("%w: limit must be 1 to 100", application.ErrValidation))
			return
		}
		limit = n
	}
	rows, err := h.Svc.Store.ListAudit(r.Context(), limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, http.StatusOK, rows)
}

var postalCodePattern = regexp.MustCompile(`^[0-9]{5}$`)

// publicZones lists enabled zones for booking forms, or finds the zone of one
// postal code. No sign-in; only id, code and name are shown (Doc 1 R-18).
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
