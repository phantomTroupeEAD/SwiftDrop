package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	payload := map[string]string{"message": "hello"}

	err := WriteJSON(rr, http.StatusOK, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("expected application/json; charset=utf-8, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if body["message"] != "hello" {
		t.Errorf("expected hello, got %v", body["message"])
	}
}

func TestWriteData(t *testing.T) {
	rr := httptest.NewRecorder()
	item := map[string]string{"item": "book"}

	err := WriteData(rr, http.StatusCreated, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rr.Code)
	}

	var env SingleDataEnvelope[map[string]string]
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode envelope: %v", err)
	}
	if env.Data["item"] != "book" {
		t.Errorf("expected book, got %v", env.Data["item"])
	}
}

func TestWriteError(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), correlationIDKey, "test-corr-123"))

	WriteError(rr, req, http.StatusBadRequest, "INVALID_INPUT", "Input validation failed", ErrorDetail{
		Field: "tracking_number",
		Issue: "Must start with SD-",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}

	var env ErrorEnvelope
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode error envelope: %v", err)
	}

	if env.Error.Code != "INVALID_INPUT" {
		t.Errorf("expected INVALID_INPUT, got %s", env.Error.Code)
	}
	if env.Error.Message != "Input validation failed" {
		t.Errorf("expected 'Input validation failed', got %s", env.Error.Message)
	}
	if env.Error.CorrelationID != "test-corr-123" {
		t.Errorf("expected test-corr-123, got %s", env.Error.CorrelationID)
	}
	if env.Error.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
	if len(env.Error.Details) != 1 || env.Error.Details[0].Field != "tracking_number" {
		t.Errorf("expected 1 detail for tracking_number, got %v", env.Error.Details)
	}
}
