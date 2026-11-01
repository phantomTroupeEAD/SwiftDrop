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
	"strconv"
	"strings"

	"swiftdrop.local/m7/core-api/internal/admin/application"
	"swiftdrop.local/m7/core-api/internal/platform/apperr"
	"swiftdrop.local/m7/core-api/internal/platform/obs"
	"swiftdrop.local/m7/core-api/internal/platform/web"
)

type Handlers struct {
	Svc     *application.Service
	Metrics *obs.Metrics
}

// Register mounts every admin route behind RequireRole(ADMIN). There is no admin
// route without the guard (Doc 7 B-01).
func (h *Handlers) Register(mux *http.ServeMux) {
	admin := web.RequireRole(web.RoleAdmin)
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
