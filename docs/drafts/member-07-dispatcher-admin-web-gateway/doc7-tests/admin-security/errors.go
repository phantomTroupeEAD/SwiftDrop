package adminsec

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

var (
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrForbidden          = errors.New("forbidden")
	ErrNotFound           = errors.New("not found")
	ErrValidation         = errors.New("validation failed")
	ErrUnknownSetting     = errors.New("unknown setting")
	ErrConcurrentConflict = errors.New("concurrent modification")
	ErrPrefixConflict     = errors.New("postal prefix conflict")
	ErrReasonRequired     = errors.New("reason required")
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
	{ErrUnauthenticated, mapping{401, "AUTH_REQUIRED", "Sign in to continue"}},
	{ErrForbidden, mapping{403, "ACCESS_DENIED", "You do not have access to this action"}},
	{ErrNotFound, mapping{404, "NOT_FOUND", "Not found"}},
	{ErrValidation, mapping{422, "VALIDATION_FAILED", "Some fields are not valid"}},
	{ErrUnknownSetting, mapping{422, "SETTING_UNKNOWN", "This setting cannot be changed"}},
	{ErrReasonRequired, mapping{422, "REASON_REQUIRED", "Give a reason for this change"}},
	{ErrConcurrentConflict, mapping{409, "CONCURRENT_MODIFICATION", "Someone else changed this. Reload and try again"}},
	{ErrPrefixConflict, mapping{409, "ZONE_POSTAL_CODE_CONFLICT", "A postal prefix is already used by another zone"}},
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
	if m.status >= 500 {
		slog.Error("admin request failed", "correlation_id", body.Error.CorrelationID, "err", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", body.Error.CorrelationID)
	w.WriteHeader(m.status)
	_ = json.NewEncoder(w).Encode(body)
}
