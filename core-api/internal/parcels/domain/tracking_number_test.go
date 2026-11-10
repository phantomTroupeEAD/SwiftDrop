package domain

import (
	"errors"
	"testing"
	"time"
)

func TestTrackingNumberValidate_Valid(t *testing.T) {
	validNumbers := []string{
		"SD-20261106-A3X9K7",
		"SD-20260101-012345",
		"SD-20261231-ZZZZZZ",
		"SD-20260715-8M4K2P",
	}

	for _, raw := range validNumbers {
		t.Run(raw, func(t *testing.T) {
			tn, err := NewTrackingNumber(raw)
			if err != nil {
				t.Fatalf("expected valid tracking number for %s, got: %v", raw, err)
			}
			if tn.String() != raw {
				t.Errorf("expected %s, got %s", raw, tn.String())
			}
		})
	}
}

func TestTrackingNumberValidate_Invalid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "too short",
			raw:  "SD-20261106-A3X9",
		},
		{
			name: "too long",
			raw:  "SD-20261106-A3X9K777",
		},
		{
			name: "wrong prefix",
			raw:  "XX-20261106-A3X9K7",
		},
		{
			name: "invalid date format",
			raw:  "SD-20261301-A3X9K7", // month 13
		},
		{
			name: "invalid day in date",
			raw:  "SD-20260231-A3X9K7", // Feb 31
		},
		{
			name: "missing separator",
			raw:  "SD-20261106_A3X9K7",
		},
		{
			name: "contains letter I (excluded from Crockford Base32)",
			raw:  "SD-20261106-A3I9K7",
		},
		{
			name: "contains letter L (excluded from Crockford Base32)",
			raw:  "SD-20261106-A3L9K7",
		},
		{
			name: "contains letter O (excluded from Crockford Base32)",
			raw:  "SD-20261106-A3O9K7",
		},
		{
			name: "contains letter U (excluded from Crockford Base32)",
			raw:  "SD-20261106-A3U9K7",
		},
		{
			name: "contains special characters",
			raw:  "SD-20261106-A3@9K7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTrackingNumber(tt.raw)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tt.raw)
			}
			if !errors.Is(err, ErrInvalidTrackingNumber) {
				t.Errorf("expected ErrInvalidTrackingNumber, got %v", err)
			}
		})
	}
}

func TestFormatTrackingNumber(t *testing.T) {
	date := time.Date(2026, time.November, 6, 0, 0, 0, 0, time.UTC)
	tn, err := FormatTrackingNumber(date, "A3X9K7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "SD-20261106-A3X9K7"
	if tn.String() != expected {
		t.Errorf("expected %s, got %s", expected, tn.String())
	}

	// Suffix length error.
	_, err = FormatTrackingNumber(date, "SHORT")
	if !errors.Is(err, ErrInvalidTrackingNumber) {
		t.Errorf("expected ErrInvalidTrackingNumber for short suffix, got %v", err)
	}
}
