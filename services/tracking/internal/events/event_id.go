package events

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/swiftdrop/tracking/internal/domain"
)

var (
	// ErrInvalidEventID indicates that an event ID string is malformed or invalid.
	ErrInvalidEventID = errors.New("invalid event id format")
)

// ParsedEventID holds the validated components of an SSE event ID.
// Authoritative specification: {tracking_number}:{status}:{unix_timestamp}[:{tiebreaker}].
type ParsedEventID struct {
	Raw            string
	TrackingNumber string
	Status         string
	Timestamp      time.Time
	TieBreaker     string
	IsOpaque       bool
}

// FormatEventID constructs a canonical SSE event ID string: {tracking_number}:{status}:{unix_timestamp}.
func FormatEventID(trackingNumber, status string, t time.Time) string {
	return fmt.Sprintf("%s:%s:%d", strings.TrimSpace(trackingNumber), strings.TrimSpace(status), t.UTC().Unix())
}

// FormatEventIDWithTieBreaker constructs an SSE event ID with an optional tiebreaker:
// {tracking_number}:{status}:{unix_timestamp}[:{tiebreaker}].
// The tiebreaker (e.g., milestone ID or attempt sequence) guarantees uniqueness
// even when multiple transitions to the same status occur within the same wall-clock second.
func FormatEventIDWithTieBreaker(trackingNumber, status string, t time.Time, tiebreaker string) string {
	base := FormatEventID(trackingNumber, status, t)
	cleanTie := strings.TrimSpace(tiebreaker)
	if cleanTie == "" {
		return base
	}
	return fmt.Sprintf("%s:%s", base, cleanTie)
}

// ParseEventID parses and validates an SSE event ID string.
// Supports:
// 1. Canonical 3-part: {tracking_number}:{status}:{unix_timestamp}
// 2. Extended 4-part: {tracking_number}:{status}:{unix_timestamp}:{tiebreaker}
// 3. Opaque broker IDs: e.g. evt_01HXYZ9876543210ABC or standard UUIDs
func ParseEventID(raw string) (*ParsedEventID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%w: empty event ID", ErrInvalidEventID)
	}

	parts := strings.Split(raw, ":")

	// Check if this is an opaque broker event ID (e.g. evt_... or UUID without tracking colon structure)
	if len(parts) == 1 {
		if strings.HasPrefix(raw, "evt_") || len(raw) >= 32 {
			return &ParsedEventID{
				Raw:      raw,
				IsOpaque: true,
			}, nil
		}
		return nil, fmt.Errorf("%w: unrecognized single-token event ID format", ErrInvalidEventID)
	}

	if len(parts) < 3 || len(parts) > 4 {
		return nil, fmt.Errorf("%w: expected 3 or 4 colon-separated parts, got %d", ErrInvalidEventID, len(parts))
	}

	rawTN, rawStatus, rawTime := parts[0], parts[1], parts[2]

	tn, err := domain.NewTrackingNumber(rawTN)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid tracking number %q: %v", ErrInvalidEventID, rawTN, err)
	}

	status := domain.ParcelStatus(strings.TrimSpace(rawStatus))
	if err := status.Validate(); err != nil {
		return nil, fmt.Errorf("%w: invalid status %q: %v", ErrInvalidEventID, rawStatus, err)
	}

	sec, err := strconv.ParseInt(strings.TrimSpace(rawTime), 10, 64)
	if err != nil || sec < 0 {
		return nil, fmt.Errorf("%w: invalid unix timestamp %q", ErrInvalidEventID, rawTime)
	}

	var tiebreaker string
	if len(parts) == 4 {
		tiebreaker = strings.TrimSpace(parts[3])
	}

	return &ParsedEventID{
		Raw:            raw,
		TrackingNumber: tn.String(),
		Status:         string(status),
		Timestamp:      time.Unix(sec, 0).UTC(),
		TieBreaker:     tiebreaker,
		IsOpaque:       false,
	}, nil
}
