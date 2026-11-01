package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"swiftdrop.local/m7/core-api/internal/platform/apperr"
	"time"
)

type mapping struct {
	status  int
	code    string
	message string
}

// Known errors get a fixed, safe message. Anything else becomes a generic 500.
var known = []struct {
	err error
	m   mapping
}{
	{apperr.ErrUnauthenticated, mapping{401, "AUTH_REQUIRED", "Sign in to continue"}},
	{apperr.ErrForbidden, mapping{403, "ACCESS_DENIED", "You do not have access to this action"}},
	{apperr.ErrNotFound, mapping{404, "NOT_FOUND", "Not found"}},
	{apperr.ErrValidation, mapping{422, "VALIDATION_FAILED", "Some fields are not valid"}},
	{apperr.ErrUnknownSetting, mapping{422, "SETTING_UNKNOWN", "This setting cannot be changed"}},
	{apperr.ErrReasonRequired, mapping{422, "REASON_REQUIRED", "Give a reason for this change"}},
	{apperr.ErrConcurrentConflict, mapping{409, "CONCURRENT_MODIFICATION", "Someone else changed this. Reload and try again"}},
	{apperr.ErrPrefixConflict, mapping{409, "ZONE_POSTAL_CODE_CONFLICT", "A postal prefix is already used by another zone"}},
	{apperr.ErrZoneCodeTaken, mapping{409, "ZONE_CODE_ALREADY_EXISTS", "This zone code is already used"}},
	{apperr.ErrBadRequest, mapping{400, "BAD_REQUEST", "The request body is not valid JSON for this action"}},
	{apperr.ErrBusy, mapping{503, "TRY_AGAIN", "Another change is in progress. Try again"}},
	{apperr.ErrUnavailable, mapping{503, "SERVICE_UNAVAILABLE", "The service is not available right now. Try again shortly"}},
}

var reqIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// CorrelationID reuses a well-formed X-Request-ID, or makes a new one. A bad value
// (too long, odd characters) is never echoed back, so it cannot inject into logs.
func CorrelationID(r *http.Request) string {
	if id := r.Header.Get("X-Request-ID"); reqIDPattern.MatchString(id) {
		return id
	}
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "corr_" + hex.EncodeToString(b)
}

type errorBody struct {
	Error struct {
		Code          string `json:"code"`
		Message       string `json:"message"`
		CorrelationID string `json:"correlation_id"`
		Timestamp     string `json:"timestamp"`
	} `json:"error"`
}

// WriteError sends the standard error envelope (api-conventions 2.3). The raw error
// text goes to the server log only, never to the client.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	m := mapping{500, "INTERNAL_ERROR", "Something went wrong. Try again later"}
	for _, k := range known {
		if errors.Is(err, k.err) {
			m = k.m
			break
		}
	}
	var body errorBody
	body.Error.Code, body.Error.Message = m.code, m.message
	body.Error.CorrelationID = CorrelationID(r)
	body.Error.Timestamp = time.Now().UTC().Format(time.RFC3339)
	if m.status >= 500 { // includes 503: the cause is needed in the log
		slog.Error("admin request failed", "correlation_id", body.Error.CorrelationID, "err", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", body.Error.CorrelationID)
	w.WriteHeader(m.status)
	_ = json.NewEncoder(w).Encode(body)
}
