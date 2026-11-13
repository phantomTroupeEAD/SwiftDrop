package transport_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/platform/middleware"
)

// Identity errors carry the request's correlation_id (api-conventions), even when the client
// sends no X-Request-ID. RequireTenant without a principal answers 403 through writeJSONError.
func TestIdentityErrors_IncludeCorrelationID(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := middleware.RequestID(transport.RequireTenant()(ok))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/merchant/portal", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
	var body transport.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid envelope: %v", err)
	}
	headerID := rec.Header().Get("X-Request-ID")
	if headerID == "" || body.Error.CorrelationID != headerID {
		t.Fatalf("correlation_id %q must equal X-Request-ID response header %q", body.Error.CorrelationID, headerID)
	}
}
