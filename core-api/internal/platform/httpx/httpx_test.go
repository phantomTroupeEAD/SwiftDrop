package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/platform/middleware"
)

type testPayload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	payload := testPayload{Name: "parcel", Count: 5}

	err := WriteJSON(rec, http.StatusCreated, payload)
	if err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type application/json; charset=utf-8, got %q", rec.Header().Get("Content-Type"))
	}

	var parsed testPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if parsed.Name != "parcel" || parsed.Count != 5 {
		t.Errorf("unexpected body content: %+v", parsed)
	}
}

func TestWriteError_CanonicalEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/parcels/123/assign", nil)
	req.Header.Set("X-Request-ID", "trace-err-123")

	WriteError(rec, req, http.StatusUnprocessableEntity, "PARCEL_INVALID_TRANSITION", "Cannot transition parcel", ErrorDetail{
		Field: "status",
		Issue: "Already delivered",
	})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", rec.Code)
	}

	var envelope ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to unmarshal error envelope: %v", err)
	}

	if envelope.Error.Code != "PARCEL_INVALID_TRANSITION" {
		t.Errorf("expected code PARCEL_INVALID_TRANSITION, got %s", envelope.Error.Code)
	}
	if envelope.Error.CorrelationID != "trace-err-123" {
		t.Errorf("expected correlation_id trace-err-123, got %s", envelope.Error.CorrelationID)
	}
	if envelope.Error.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
	if len(envelope.Error.Details) != 1 || envelope.Error.Details[0].Field != "status" {
		t.Errorf("unexpected error details: %+v", envelope.Error.Details)
	}
}

func TestDecodeJSON_Strictness(t *testing.T) {
	// 1. Success on valid JSON
	{
		req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"name":"test","count":10}`))
		rec := httptest.NewRecorder()
		var p testPayload
		err := DecodeJSON(rec, req, &p, 1024)
		if err != nil {
			t.Fatalf("expected valid decode, got: %v", err)
		}
		if p.Name != "test" || p.Count != 10 {
			t.Errorf("unexpected decoded payload: %+v", p)
		}
	}

	// 2. Rejects unknown fields
	{
		req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"name":"test","count":10,"unexpected_field":"danger"}`))
		rec := httptest.NewRecorder()
		var p testPayload
		err := DecodeJSON(rec, req, &p, 1024)
		if err == nil {
			t.Fatal("expected error on unknown field, got nil")
		}
	}

	// 3. Rejects multiple JSON documents in single stream
	{
		req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"name":"first","count":1}{"name":"second","count":2}`))
		rec := httptest.NewRecorder()
		var p testPayload
		err := DecodeJSON(rec, req, &p, 1024)
		if !errors.Is(err, ErrMultipleDocuments) {
			t.Fatalf("expected ErrMultipleDocuments, got: %v", err)
		}
	}

	// 4. Enforces maxBytes limit
	{
		req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"name":"large payload","count":100}`))
		rec := httptest.NewRecorder()
		var p testPayload
		err := DecodeJSON(rec, req, &p, 10)
		if err == nil {
			t.Fatal("expected size limit error, got nil")
		}
	}
}

func TestPaginatedResponse(t *testing.T) {
	items := []string{"item1", "item2"}
	resp := NewPaginatedResponse(items, 1, 20, 142)

	if resp.Meta.TotalPages != 8 {
		t.Errorf("expected 8 total pages for 142 items with 20/page, got %d", resp.Meta.TotalPages)
	}
	if resp.Meta.Page != 1 || resp.Meta.PerPage != 20 || resp.Meta.TotalItems != 142 {
		t.Errorf("unexpected metadata: %+v", resp.Meta)
	}
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Data))
	}
}

func TestCursor_EncodeDecode(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	id := "0192e4a1-0001-7abc-8def-0123456789ab"

	cursor := EncodeCursor(now, id)
	if cursor == "" {
		t.Fatal("expected non-empty cursor")
	}

	decodedTime, decodedID, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("DecodeCursor failed: %v", err)
	}

	if decodedID != id {
		t.Errorf("expected ID %s, got %s", id, decodedID)
	}
	if !decodedTime.Equal(now) {
		t.Errorf("expected time %v, got %v", now, decodedTime)
	}

	// Tampered / malformed cursor
	_, _, err = DecodeCursor("invalid!!base64--")
	if !errors.Is(err, ErrInvalidCursor) {
		t.Errorf("expected ErrInvalidCursor, got: %v", err)
	}

	_, _, err = DecodeCursor("")
	if !errors.Is(err, ErrInvalidCursor) {
		t.Errorf("expected ErrInvalidCursor on empty cursor, got: %v", err)
	}
}

// A client that sends no X-Request-ID still gets a correlation_id: the RequestID middleware
// generates one and puts it in the response header, which WriteError reads.
func TestWriteError_CorrelationIDWithoutClientHeader(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "missing")
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/x", nil))

	var envelope ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid envelope: %v", err)
	}
	headerID := rec.Header().Get("X-Request-ID")
	if headerID == "" || envelope.Error.CorrelationID != headerID {
		t.Fatalf("correlation_id %q must equal X-Request-ID response header %q", envelope.Error.CorrelationID, headerID)
	}
}

// An invalid client X-Request-ID is replaced by the middleware; the error body must carry the
// validated ID, not echo the client's value.
func TestWriteError_CorrelationIDUsesValidatedID(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, http.StatusBadRequest, "BAD", "bad")
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	req.Header.Set("X-Request-ID", "<script>not valid</script>")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var envelope ErrorEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &envelope)
	if envelope.Error.CorrelationID == "<script>not valid</script>" || envelope.Error.CorrelationID != rec.Header().Get("X-Request-ID") {
		t.Fatalf("correlation_id must be the validated ID, got %q", envelope.Error.CorrelationID)
	}
}
