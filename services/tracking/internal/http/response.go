package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SingleDataEnvelope wraps single resource data per api-conventions.md §2.1.
type SingleDataEnvelope[T any] struct {
	Data T `json:"data"`
}

// ErrorDetail describes a field-level or semantic error detail.
type ErrorDetail struct {
	Field string `json:"field,omitempty"`
	Issue string `json:"issue"`
}

// ErrorEnvelopeBody represents the structured body of an error response.
type ErrorEnvelopeBody struct {
	Code          string        `json:"code"`
	Message       string        `json:"message"`
	CorrelationID string        `json:"correlation_id,omitempty"`
	Timestamp     string        `json:"timestamp"`
	Details       []ErrorDetail `json:"details,omitempty"`
}

// ErrorEnvelope wraps an ErrorEnvelopeBody per api-conventions.md §2.3.
type ErrorEnvelope struct {
	Error ErrorEnvelopeBody `json:"error"`
}

// WriteJSON marshals v as JSON and writes it with the provided HTTP status and standard headers.
func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}
	return nil
}

// WriteData wraps v in SingleDataEnvelope {"data": v} and writes it with the provided status.
func WriteData(w http.ResponseWriter, status int, v any) error {
	return WriteJSON(w, status, SingleDataEnvelope[any]{Data: v})
}

// WriteError writes a canonical error envelope response.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details ...ErrorDetail) {
	corrID := extractCorrelationID(r)

	envelope := ErrorEnvelope{
		Error: ErrorEnvelopeBody{
			Code:          code,
			Message:       message,
			CorrelationID: corrID,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
			Details:       details,
		},
	}

	_ = WriteJSON(w, status, envelope)
}

func extractCorrelationID(r *http.Request) string {
	if r == nil {
		return ""
	}
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	ctx := r.Context()
	if ctx != nil {
		return CorrelationID(ctx)
	}
	return ""
}
