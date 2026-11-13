package httpx

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrInvalidCursor is returned when a pagination cursor is malformed or invalid.
	ErrInvalidCursor = errors.New("httpx: invalid or malformed pagination cursor")

	// ErrMultipleDocuments is returned when a JSON request contains more than one root document.
	ErrMultipleDocuments = errors.New("httpx: request body must contain only a single JSON document")
)

// SingleDataEnvelope represents the standard single-resource wrapper {"data": ...} per api-conventions.md §2.1.
type SingleDataEnvelope[T any] struct {
	Data T `json:"data"`
}

// DataEnvelope models a standard single-resource JSON envelope {"data": ...} for documentation and non-generic APIs.
type DataEnvelope struct {
	Data interface{} `json:"data"`
}

// ErrorDetail describes a semantic or field-level validation problem.
type ErrorDetail struct {
	Field string `json:"field,omitempty"`
	Issue string `json:"issue"`
}

// ErrorEnvelopeBody defines the standard structured error envelope body per api-conventions.md §2.3.
type ErrorEnvelopeBody struct {
	Code          string        `json:"code"`
	Message       string        `json:"message"`
	CorrelationID string        `json:"correlation_id,omitempty"`
	Timestamp     string        `json:"timestamp"`
	Details       []ErrorDetail `json:"details,omitempty"`
}

// ErrorEnvelope wraps an ErrorEnvelopeBody.
type ErrorEnvelope struct {
	Error ErrorEnvelopeBody `json:"error"`
}

// ErrorResponse is an alias for ErrorEnvelope for documentation parity and conventions.
type ErrorResponse = ErrorEnvelope

// PageMeta models pagination metadata per api-conventions.md §2.2.
type PageMeta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// PaginatedResponse models the canonical list response wrapper per api-conventions.md §2.2.
type PaginatedResponse[T any] struct {
	Data []T      `json:"data"`
	Meta PageMeta `json:"meta"`
}

// PaginatedEnvelope models the standard list response wrapper {"data": [...], "meta": {...}} for documentation.
type PaginatedEnvelope struct {
	Data []interface{} `json:"data"`
	Meta PageMeta      `json:"meta"`
}

// NewPaginatedResponse constructs a canonical PaginatedResponse with computed total_pages.
func NewPaginatedResponse[T any](data []T, page, perPage, totalItems int) PaginatedResponse[T] {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = (totalItems + perPage - 1) / perPage
	}

	if data == nil {
		data = make([]T, 0)
	}

	return PaginatedResponse[T]{
		Data: data,
		Meta: PageMeta{
			Page:       page,
			PerPage:    perPage,
			TotalItems: totalItems,
			TotalPages: totalPages,
		},
	}
}

// CursorMeta models keyset pagination metadata.
type CursorMeta struct {
	Limit      int     `json:"limit"`
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor"`
}

// CursorPaginatedResponse models a keyset paginated response.
type CursorPaginatedResponse[T any] struct {
	Data []T        `json:"data"`
	Meta CursorMeta `json:"meta"`
}

// CursorPaginatedEnvelope models the standard keyset paginated response wrapper for documentation.
type CursorPaginatedEnvelope struct {
	Data []interface{} `json:"data"`
	Meta CursorMeta    `json:"meta"`
}

// WriteCursorPaginated writes a keyset paginated JSON response.
func WriteCursorPaginated[T any](w http.ResponseWriter, status int, data []T, meta CursorMeta) error {
	if data == nil {
		data = make([]T, 0)
	}
	return WriteJSON(w, status, CursorPaginatedResponse[T]{
		Data: data,
		Meta: meta,
	})
}

// WriteJSON marshals v as JSON and writes it with the provided HTTP status code and standard headers.
func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}
	return nil
}

// WriteData wraps v in a SingleDataEnvelope {"data": v} and writes it with status 200 OK.
func WriteData(w http.ResponseWriter, status int, v any) error {
	return WriteJSON(w, status, SingleDataEnvelope[any]{Data: v})
}

// WriteError writes a canonical error envelope per api-conventions.md §2.3.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details ...ErrorDetail) {
	// Prefer the X-Request-ID response header: the platform RequestID middleware sets it to the
	// validated (or freshly generated) correlation ID before any handler runs. The request header
	// alone is missing when the client sends none, and is unvalidated when it does.
	corrID := w.Header().Get("X-Request-ID")
	if corrID == "" {
		corrID = extractCorrelationID(r)
	}

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
		for _, key := range []any{"swiftdrop_correlation_id", "correlation_id"} {
			if val, ok := ctx.Value(key).(string); ok && val != "" {
				return val
			}
		}
	}
	return ""
}

// DecodeJSON strictly decodes the JSON request body into dst.
// Rejects unknown fields and verifies that exactly one JSON document is present.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	if r.Body == nil {
		return errors.New("httpx: missing request body")
	}

	if maxBytes > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}

	// Verify no trailing extra data
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrMultipleDocuments
	}

	return nil
}

// Cursor encapsulates a cursor point for keyset pagination.
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

// EncodeCursor produces a base64url encoded cursor string from a created_at timestamp and ID.
func EncodeCursor(createdAt time.Time, id string) string {
	raw := fmt.Sprintf("%d:%s", createdAt.UTC().UnixNano(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor parses a base64url cursor into the original created_at timestamp and ID.
func DecodeCursor(cursor string) (time.Time, string, error) {
	if strings.TrimSpace(cursor) == "" {
		return time.Time{}, "", ErrInvalidCursor
	}

	decodedBytes, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", ErrInvalidCursor
	}

	parts := strings.SplitN(string(decodedBytes), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return time.Time{}, "", ErrInvalidCursor
	}

	nano, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", ErrInvalidCursor
	}

	createdAt := time.Unix(0, nano).UTC()
	return createdAt, parts[1], nil
}
