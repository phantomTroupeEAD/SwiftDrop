package domain

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ErrInvalidProfile carries the field that failed (BR-M-02..04).
type ErrInvalidProfile struct{ Field, Issue string }

func (e ErrInvalidProfile) Error() string { return "merchants: invalid " + e.Field + ": " + e.Issue }

var (
	phoneRe   = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)
	countryRe = regexp.MustCompile(`^[A-Z]{2}$`)
	emailRe   = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// Address is the merchant business address (display/contact only).
type Address struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	PostalCode string `json:"postal_code,omitempty"`
	Country    string `json:"country"`
}

// ValidateContact checks email, phone and address and rejects control characters (Doc 7 §6.1).
func ValidateContact(email, phone string, a Address) error {
	if len(email) > 254 || !emailRe.MatchString(email) {
		return ErrInvalidProfile{"contact_email", "must be a valid email address"}
	}
	if !phoneRe.MatchString(phone) {
		return ErrInvalidProfile{"contact_phone", "must be E.164, e.g. +94771234567"}
	}
	fields := []struct {
		name, v  string
		required bool
		max      int
	}{
		{"address.line1", a.Line1, true, 200},
		{"address.line2", a.Line2, false, 200},
		{"address.city", a.City, true, 100},
		{"address.postal_code", a.PostalCode, false, 20},
	}
	for _, f := range fields {
		v := strings.TrimSpace(f.v)
		if f.required && v == "" {
			return ErrInvalidProfile{f.name, "is required"}
		}
		if utf8.RuneCountInString(v) > f.max {
			return ErrInvalidProfile{f.name, "is too long"}
		}
		if HasControlChars(v) {
			return ErrInvalidProfile{f.name, "contains control characters"}
		}
	}
	if !countryRe.MatchString(a.Country) {
		return ErrInvalidProfile{"address.country", "must be ISO 3166-1 alpha-2, e.g. LK"}
	}
	return nil
}

// HasControlChars reports ASCII control characters (U+0000-U+001F, U+007F).
func HasControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// ErrControlChars is returned for names/labels with control characters (AB-12).
var ErrControlChars = errors.New("merchants: text contains control characters")
