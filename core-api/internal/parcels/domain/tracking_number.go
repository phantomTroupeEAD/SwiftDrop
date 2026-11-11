package domain

import (
	"fmt"
	"strings"
	"time"
)

// CrockfordBase32Alphabet defines the 32 characters permitted in Crockford's Base32
// encoding (digits 0-9 and uppercase Latin letters excluding I, L, O, and U).
const CrockfordBase32Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// TrackingNumber represents an authoritative, non-sequential public tracking reference.
// The canonical format is SD-YYYYMMDD-XXXXXX, where XXXXXX is 6 Crockford Base32 characters.
type TrackingNumber string

// NewTrackingNumber parses and validates a raw tracking number string.
func NewTrackingNumber(raw string) (TrackingNumber, error) {
	trimmed := strings.TrimSpace(strings.ToUpper(raw))
	tn := TrackingNumber(trimmed)
	if err := tn.Validate(); err != nil {
		return "", err
	}
	return tn, nil
}

// FormatTrackingNumber constructs a tracking number from a given date and 6-character Crockford Base32 suffix.
func FormatTrackingNumber(date time.Time, suffix string) (TrackingNumber, error) {
	if date.IsZero() {
		return "", fmt.Errorf("%w: zero date", ErrInvalidTrackingNumber)
	}
	cleanSuffix := strings.TrimSpace(strings.ToUpper(suffix))
	if len(cleanSuffix) != 6 {
		return "", fmt.Errorf("%w: suffix must be exactly 6 characters", ErrInvalidTrackingNumber)
	}

	formatted := fmt.Sprintf("SD-%s-%s", date.Format("20060102"), cleanSuffix)
	return NewTrackingNumber(formatted)
}

// String returns the tracking number as a string.
func (t TrackingNumber) String() string {
	return string(t)
}

// Validate ensures the tracking number strictly follows the canonical SD-YYYYMMDD-XXXXXX format.
func (t TrackingNumber) Validate() error {
	raw := string(t)
	if len(raw) != 18 {
		return fmt.Errorf("%w: length must be 18 characters, got %d", ErrInvalidTrackingNumber, len(raw))
	}

	if !strings.HasPrefix(raw, "SD-") {
		return fmt.Errorf("%w: must begin with 'SD-' prefix", ErrInvalidTrackingNumber)
	}

	datePart := raw[3:11]
	if _, err := time.Parse("20060102", datePart); err != nil {
		return fmt.Errorf("%w: invalid date component %q: %v", ErrInvalidTrackingNumber, datePart, err)
	}

	if raw[11] != '-' {
		return fmt.Errorf("%w: missing hyphen delimiter at position 11", ErrInvalidTrackingNumber)
	}

	suffixPart := raw[12:18]
	for i := 0; i < len(suffixPart); i++ {
		ch := suffixPart[i]
		if !strings.ContainsRune(CrockfordBase32Alphabet, rune(ch)) {
			return fmt.Errorf("%w: invalid Crockford Base32 character %q at position %d", ErrInvalidTrackingNumber, ch, 12+i)
		}
	}

	return nil
}
