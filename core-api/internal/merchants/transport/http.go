// Package transport exposes the merchants HTTP API (contracts/merchants-openapi.v1.yaml).
// Handlers are thin: authenticate, decode, call one use case, map the result.
package transport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/merchants/application"
	"github.com/swiftdrop/core-api/internal/merchants/domain"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

// maxBody is the maximum accepted JSON request body size (64 KB).
const maxBody = 65536

// Detail is a convenience alias so handlers can write Detail{Field,Issue} directly.
type Detail = httpx.ErrorDetail

// decode parses a JSON body into dst, rejecting unknown fields and bodies > maxBody.
func decode(r *http.Request, dst any) error {
	return httpx.DecodeJSON(nil, r, dst, maxBody)
}

type Handler struct {
	Svc      *application.Service
	Verifier TokenVerifier
	Log      *slog.Logger
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/merchants/applications", h.submitApplication)
	mux.HandleFunc("GET /api/v1/merchants/me", h.getMe)
	mux.HandleFunc("GET /api/v1/merchants/webhook-event-types", h.eventTypes)
	mux.HandleFunc("GET /api/v1/merchants/me/api-keys", h.listKeys)
	mux.HandleFunc("POST /api/v1/merchants/me/api-keys", h.createKey)
	mux.HandleFunc("DELETE /api/v1/merchants/me/api-keys/{key_id}", h.revokeKey)
	mux.HandleFunc("GET /api/v1/merchants/me/webhooks", h.listWebhooks)
	mux.HandleFunc("POST /api/v1/merchants/me/webhooks", h.registerWebhook)
	mux.HandleFunc("DELETE /api/v1/merchants/me/webhooks/{webhook_id}", h.deleteWebhook)
	mux.HandleFunc("POST /api/v1/merchant/orders", h.createOrder)
	mux.HandleFunc("GET /api/v1/merchant/orders", h.listOrders)
	// Admin routes: hosted by Member 7's admin module in the real repo (U-10); served here as a stand-in.
	mux.HandleFunc("GET /api/v1/admin/merchants", h.adminList)
	mux.HandleFunc("GET /api/v1/admin/merchants/{merchant_id}", h.adminGet)
	mux.HandleFunc("POST /api/v1/admin/merchants/{merchant_id}/{action}", h.adminChangeStatus)
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ---------------- authentication ----------------

// jwtActor authenticates JWT-only routes. API keys are rejected here (Doc 4 §3.1 rule 5).
func (h *Handler) jwtActor(w http.ResponseWriter, r *http.Request) (application.Actor, bool) {
	if r.Header.Get("X-API-Key") != "" {
		httpx.WriteError(w, r, http.StatusUnauthorized, "API_KEY_NOT_ALLOWED", "API keys cannot be used on this endpoint.")
		return application.Actor{}, false
	}
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || tok == "" {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Bearer token required.")
		return application.Actor{}, false
	}
	c, err := h.Verifier.Verify(tok)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Invalid or expired token.")
		return application.Actor{}, false
	}
	return application.Actor{UserID: c.Sub, Role: c.Role, TenantID: c.TenantID, RequestID: ""}, true
}

// merchantPrincipal authenticates order routes with X-API-Key or a merchant JWT (F-06).
func (h *Handler) merchantPrincipal(w http.ResponseWriter, r *http.Request) (application.MerchantPrincipal, bool) {
	key := r.Header.Get("X-API-Key")
	bearer := r.Header.Get("Authorization")
	switch {
	case key != "" && bearer != "":
		httpx.WriteError(w, r, http.StatusBadRequest, "AMBIGUOUS_CREDENTIALS", "Send either X-API-Key or Authorization, not both.")
		return application.MerchantPrincipal{}, false
	case key != "":
		p, err := h.Svc.VerifyAPIKey(r.Context(), key)
		if err != nil {
			h.fail(w, r, err, "")
			return p, false
		}
		return p, true
	case bearer != "":
		a, ok := h.jwtActor(w, r)
		if !ok {
			return application.MerchantPrincipal{}, false
		}
		p, err := h.Svc.PrincipalFromJWT(r.Context(), a)
		if err != nil {
			h.fail(w, r, err, "")
			return p, false
		}
		return p, true
	default: // ?api_key= in the query string is deliberately ignored (EC-04)
		httpx.WriteError(w, r, http.StatusUnauthorized, "API_KEY_MISSING", "X-API-Key header required.")
		return application.MerchantPrincipal{}, false
	}
}

// ---------------- error mapping (Doc 2 §6) ----------------

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error, notFoundCode string) {
	var ve application.ValidationError
	var ite domain.InvalidTransitionError
	type m struct {
		status int
		code   string
		msg    string
	}
	var res m
	switch {
	case errors.As(err, &ve):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Request validation failed.", Detail{Field: ve.Field, Issue: ve.Issue})
		return
	case errors.As(err, &ite):
		res = m{409, "MERCHANT_INVALID_TRANSITION", ite.Error()}
	case errors.Is(err, application.ErrMerchantNotFound):
		res = m{404, "MERCHANT_NOT_FOUND", "Merchant not found."}
	case errors.Is(err, application.ErrNotFound):
		if notFoundCode == "" {
			notFoundCode = "NOT_FOUND"
		}
		res = m{404, notFoundCode, "Resource not found."}
	case errors.Is(err, application.ErrMerchantExists):
		res = m{409, "MERCHANT_ALREADY_EXISTS", "You already have an active merchant application."}
	case errors.Is(err, application.ErrNotApproved):
		res = m{403, "MERCHANT_NOT_APPROVED", "Merchant account is not approved for this action."}
	case errors.Is(err, application.ErrVersionConflict):
		res = m{409, "MERCHANT_VERSION_CONFLICT", "Merchant was modified by another request. Reload and retry."}
	case errors.Is(err, domain.ErrReasonRequired), errors.Is(err, domain.ErrInvalidReason):
		res = m{422, "REASON_REQUIRED", "A reason of 5-500 characters is required."}
	case errors.Is(err, application.ErrForbidden), errors.Is(err, application.ErrNotAdmin):
		res = m{403, "FORBIDDEN", "Not allowed."}
	case errors.Is(err, application.ErrKeyLimit):
		res = m{409, "API_KEY_LIMIT_REACHED", "Active API key limit reached."}
	case errors.Is(err, application.ErrLabelDuplicate):
		res = m{409, "API_KEY_LABEL_DUPLICATE", "An active key already uses this label."}
	case errors.Is(err, application.ErrWebhookLimit):
		res = m{409, "WEBHOOK_LIMIT_REACHED", "Webhook endpoint limit reached."}
	case errors.Is(err, application.ErrWebhookDuplicate):
		res = m{409, "WEBHOOK_URL_DUPLICATE", "This URL is already registered."}
	case errors.Is(err, application.ErrEventUnsupported):
		res = m{422, "WEBHOOK_EVENT_UNSUPPORTED", "Unsupported or empty event list."}
	case errors.Is(err, domain.ErrSSRFProhibited):
		res = m{400, "SSRF_PROHIBITED_URL", "Webhook URL targets a prohibited network address."}
	case errors.Is(err, domain.ErrURLSchemeNotAllowed):
		res = m{422, "WEBHOOK_URL_SCHEME_NOT_ALLOWED", "Only HTTPS webhook URLs are allowed."}
	case errors.Is(err, domain.ErrURLUnresolvable):
		res = m{422, "WEBHOOK_URL_UNRESOLVABLE", "Webhook host could not be resolved."}
	case errors.Is(err, domain.ErrURLInvalid):
		res = m{422, "WEBHOOK_URL_INVALID", "Webhook URL is invalid."}
	case errors.Is(err, application.ErrScopeDenied):
		res = m{403, "MERCHANT_SCOPE_DENIED", "API key lacks the required scope."}
	case errors.Is(err, domain.ErrAPIKeyInvalid):
		res = m{401, "API_KEY_INVALID", "API key is invalid."}
	case errors.Is(err, application.ErrAPIKeyRevoked):
		res = m{401, "API_KEY_REVOKED", "This API key has been revoked."}
	case errors.Is(err, application.ErrAPIKeyExpired):
		res = m{401, "API_KEY_EXPIRED", "This API key has expired."}
	case errors.Is(err, application.ErrIdempotencyReused):
		res = m{409, "IDEMPOTENCY_KEY_PAYLOAD_MISMATCH", "Idempotency-Key was already used with a different request body."}
	default:
		h.Log.Error("unhandled error", "error", err.Error())
		res = m{500, "INTERNAL_ERROR", "Unexpected error."}
	}
	httpx.WriteError(w, r, res.status, res.code, res.msg)
}

func badJSON(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "Body must be valid JSON with only documented fields (max 64 KB).")
}

// ---------------- DTOs ----------------

func merchantDTO(v application.MerchantView) map[string]any {
	d := map[string]any{
		"merchant_id": v.ID, "business_name": v.BusinessName, "status": v.Status,
		"contact_email": v.ContactEmail, "contact_phone": v.ContactPhone, "address": v.Address,
		"approved_at": v.ApprovedAt, "version": v.Version, "created_at": v.CreatedAt, "updated_at": v.UpdatedAt,
	}
	return d
}

func keyDTO(v application.CredentialView, now time.Time) map[string]any {
	st := "ACTIVE"
	if v.RevokedAt != nil {
		st = "REVOKED"
	} else if v.ExpiresAt != nil && !now.Before(*v.ExpiresAt) {
		st = "EXPIRED"
	}
	return map[string]any{"key_id": v.ID, "label": v.Label, "key_prefix": v.Prefix, "scopes": v.Scopes, "status": st,
		"created_at": v.CreatedAt, "expires_at": v.ExpiresAt, "revoked_at": v.RevokedAt, "last_used_at": v.LastUsedAt}
}

func webhookDTO(v application.WebhookView) map[string]any {
	return map[string]any{"webhook_id": v.ID, "url": v.URL, "events": v.Events, "enabled": v.Enabled,
		"secret_last4": v.Last4, "created_at": v.CreatedAt, "updated_at": v.UpdatedAt}
}

// ---------------- self-service handlers ----------------

func (h *Handler) submitApplication(w http.ResponseWriter, r *http.Request) {
	a, ok := h.jwtActor(w, r)
	if !ok {
		return
	}
	var in struct {
		BusinessName string         `json:"business_name"`
		ContactEmail string         `json:"contact_email"`
		ContactPhone string         `json:"contact_phone"`
		Address      domain.Address `json:"address"`
	}
	if decode(r, &in) != nil {
		badJSON(w, r)
		return
	}
	v, err := h.Svc.SubmitApplication(r.Context(), a, application.ApplicationInput{
		BusinessName: in.BusinessName, ContactEmail: in.ContactEmail, ContactPhone: in.ContactPhone, Address: in.Address})
	if err != nil {
		h.fail(w, r, err, "")
		return
	}
	_ = httpx.WriteData(w, http.StatusCreated, merchantDTO(v))
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	a, ok := h.jwtActor(w, r)
	if !ok {
		return
	}
	v, err := h.Svc.MyMerchant(r.Context(), a)
	if err != nil {
		h.fail(w, r, err, "")
		return
	}
	_ = httpx.WriteData(w, http.StatusOK, merchantDTO(v))
}

func (h *Handler) eventTypes(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.jwtActor(w, r); !ok {
		return
	}
	out := make([]map[string]string, 0, len(application.EventCatalog))
	for _, e := range application.EventCatalog {
		out = append(out, map[string]string{"event_type": e.Type, "description": e.Description})
	}
	_ = httpx.WriteData(w, http.StatusOK, out)
}

func (h *Handler) listKeys(w http.ResponseWriter, r *http.Request) {
	a, ok := h.jwtActor(w, r)
	if !ok {
		return
	}
	keys, err := h.Svc.ListAPIKeys(r.Context(), a)
	if err != nil {
		h.fail(w, r, err, "")
		return
	}
	now := time.Now()
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, keyDTO(k, now))
	}
	_ = httpx.WriteData(w, http.StatusOK, out)
}

func (h *Handler) createKey(w http.ResponseWriter, r *http.Request) {
	a, ok := h.jwtActor(w, r)
	if !ok {
		return
	}
	var in struct {
		Label     string     `json:"label"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if decode(r, &in) != nil {
		badJSON(w, r)
		return
	}
	v, key, err := h.Svc.CreateAPIKey(r.Context(), a, in.Label, in.Scopes, in.ExpiresAt)
	if err != nil {
		h.fail(w, r, err, "")
		return
	}
	d := keyDTO(v, time.Now())
	d["api_key"] = key.Reveal() // the only place plaintext leaves the process (BR-K-04)
	d["note"] = "Save this key now. It will not be shown again."
	w.Header().Set("Cache-Control", "no-store")
	_ = httpx.WriteData(w, http.StatusCreated, d)
}

func (h *Handler) revokeKey(w http.ResponseWriter, r *http.Request) {
	a, ok := h.jwtActor(w, r)
	if !ok {
		return
	}
	id := r.PathValue("key_id")
	if !uuidRe.MatchString(id) {
		h.fail(w, r, application.ErrNotFound, "API_KEY_NOT_FOUND")
		return
	}
	if err := h.Svc.RevokeAPIKey(r.Context(), a, id); err != nil {
		h.fail(w, r, err, "API_KEY_NOT_FOUND")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listWebhooks(w http.ResponseWriter, r *http.Request) {
	a, ok := h.jwtActor(w, r)
	if !ok {
		return
	}
	hooks, err := h.Svc.ListWebhooks(r.Context(), a)
	if err != nil {
		h.fail(w, r, err, "")
		return
	}
	out := make([]map[string]any, 0, len(hooks))
	for _, x := range hooks {
		out = append(out, webhookDTO(x))
	}
	_ = httpx.WriteData(w, http.StatusOK, out)
}

func (h *Handler) registerWebhook(w http.ResponseWriter, r *http.Request) {
	a, ok := h.jwtActor(w, r)
	if !ok {
		return
	}
	var in struct {
		URL     string           `json:"url"`
		Events  []string         `json:"events"`
		Enabled *bool            `json:"enabled"`
		Secret  *json.RawMessage `json:"secret"`
	}
	if decode(r, &in) != nil {
		badJSON(w, r)
		return
	}
	if in.Secret != nil { // BR-W-10 / U-04
		httpx.WriteError(w, r, http.StatusBadRequest, "WEBHOOK_SECRET_NOT_ALLOWED", "The signing secret is generated by SwiftDrop.")
		return
	}
	v, secret, err := h.Svc.RegisterWebhook(r.Context(), a, in.URL, in.Events)
	if err != nil {
		h.fail(w, r, err, "")
		return
	}
	d := webhookDTO(v)
	d["signing_secret"] = secret
	d["note"] = "Save this signing secret now. It will not be shown again."
	w.Header().Set("Cache-Control", "no-store")
	_ = httpx.WriteData(w, http.StatusCreated, d)
}

func (h *Handler) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	a, ok := h.jwtActor(w, r)
	if !ok {
		return
	}
	id := r.PathValue("webhook_id")
	if !uuidRe.MatchString(id) {
		h.fail(w, r, application.ErrNotFound, "WEBHOOK_NOT_FOUND")
		return
	}
	if err := h.Svc.DeleteWebhook(r.Context(), a, id); err != nil {
		h.fail(w, r, err, "WEBHOOK_NOT_FOUND")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------- merchant orders (API key or JWT) ----------------

func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	p, ok := h.merchantPrincipal(w, r)
	if !ok {
		return
	}
	idem := strings.ToLower(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required.")
		return
	}
	if !uuidRe.MatchString(idem) {
		httpx.WriteError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_INVALID", "Idempotency-Key must be a UUID.")
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		badJSON(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(maxBody)+1))
	if err != nil || len(body) > maxBody {
		badJSON(w, r)
		return
	}
	var req application.OrderRequest // unknown fields (incl. merchant_id) ignored, never used (BR-O-04)
	if json.Unmarshal(body, &req) != nil {
		badJSON(w, r)
		return
	}
	sum := sha256.Sum256(body)
	res, replay, err := h.Svc.CreateOrder(r.Context(), p, idem, hex.EncodeToString(sum[:]), req)
	if err != nil {
		h.fail(w, r, err, "")
		return
	}
	if replay {
		w.Header().Set("Idempotent-Replay", "true")
	}
	_ = httpx.WriteData(w, http.StatusCreated, res)
}

func (h *Handler) listOrders(w http.ResponseWriter, r *http.Request) {
	p, ok := h.merchantPrincipal(w, r)
	if !ok {
		return
	}
	out, err := h.Svc.ListOrders(r.Context(), p)
	if err != nil {
		h.fail(w, r, err, "")
		return
	}
	_ = httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": out,
		"meta": map[string]int{"page": 1, "per_page": len(out), "total_items": len(out), "total_pages": 1}})
}

// ---------------- admin (stand-in for Member 7 routes) ----------------

func (h *Handler) adminList(w http.ResponseWriter, r *http.Request) {
	a, ok := h.jwtActor(w, r)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	per, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if page < 1 {
		page = 1
	}
	if per < 1 || per > 100 {
		per = 20
	}
	list, total, err := h.Svc.ListMerchants(r.Context(), a, r.URL.Query().Get("status"), page, per)
	if err != nil {
		h.fail(w, r, err, "")
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, v := range list {
		out = append(out, merchantDTO(v))
	}
	_ = httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": out, "meta": map[string]int{
		"page": page, "per_page": per, "total_items": total, "total_pages": (total + per - 1) / per}})
}

func (h *Handler) adminGet(w http.ResponseWriter, r *http.Request) {
	a, ok := h.jwtActor(w, r)
	if !ok {
		return
	}
	id := r.PathValue("merchant_id")
	if !uuidRe.MatchString(id) {
		h.fail(w, r, application.ErrMerchantNotFound, "")
		return
	}
	v, err := h.Svc.GetMerchantAdmin(r.Context(), a, id)
	if err != nil {
		h.fail(w, r, err, "")
		return
	}
	_ = httpx.WriteData(w, http.StatusOK, merchantDTO(v))
}

var actions = map[string]domain.StatusAction{
	"approve": domain.ActionApprove, "reject": domain.ActionReject, "suspend": domain.ActionSuspend,
	"reinstate": domain.ActionReinstate, "disable": domain.ActionDisable,
}

func (h *Handler) adminChangeStatus(w http.ResponseWriter, r *http.Request) {
	a, ok := h.jwtActor(w, r)
	if !ok {
		return
	}
	action, known := actions[r.PathValue("action")]
	id := r.PathValue("merchant_id")
	if !known || !uuidRe.MatchString(id) {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Resource not found.")
		return
	}
	var in struct {
		ExpectedVersion int    `json:"expected_version"`
		Reason          string `json:"reason"`
	}
	if decode(r, &in) != nil || in.ExpectedVersion < 1 {
		badJSON(w, r)
		return
	}
	v, err := h.Svc.ChangeStatus(r.Context(), a, id, action, in.ExpectedVersion, in.Reason)
	if err != nil {
		h.fail(w, r, err, "")
		return
	}
	_ = httpx.WriteData(w, http.StatusOK, merchantDTO(v))
}
