package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTrackingNumber_Validation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errIs   error
	}{
		{
			name:    "valid tracking number uppercase",
			input:   "SD-20261106-000001",
			wantErr: false,
		},
		{
			name:    "valid tracking number lowercase gets normalized",
			input:   "sd-20261106-000001",
			wantErr: false,
		},
		{
			name:    "valid tracking number with spaces gets trimmed",
			input:   "  SD-20261106-A8F3K2  ",
			wantErr: false,
		},
		{
			name:    "valid Crockford characters all allowed",
			input:   "SD-20261106-012345",
			wantErr: false,
		},
		{
			name:    "valid Crockford alphabet full check",
			input:   "SD-20261106-ABCDEF",
			wantErr: false,
		},
		{
			name:    "too short",
			input:   "SD-20261106-12345",
			wantErr: true,
			errIs:   ErrInvalidTrackingNumber,
		},
		{
			name:    "too long",
			input:   "SD-20261106-1234567",
			wantErr: true,
			errIs:   ErrInvalidTrackingNumber,
		},
		{
			name:    "missing SD prefix",
			input:   "XX-20261106-000001",
			wantErr: true,
			errIs:   ErrInvalidTrackingNumber,
		},
		{
			name:    "invalid date component - month 13",
			input:   "SD-20261301-000001",
			wantErr: true,
			errIs:   ErrInvalidTrackingNumber,
		},
		{
			name:    "invalid date component - non-digit",
			input:   "SD-2026ABCD-000001",
			wantErr: true,
			errIs:   ErrInvalidTrackingNumber,
		},
		{
			name:    "missing middle hyphen",
			input:   "SD-20261106_000001",
			wantErr: true,
			errIs:   ErrInvalidTrackingNumber,
		},
		{
			name:    "invalid Crockford character letter I",
			input:   "SD-20261106-00000I",
			wantErr: true,
			errIs:   ErrInvalidTrackingNumber,
		},
		{
			name:    "invalid Crockford character letter L",
			input:   "SD-20261106-00000L",
			wantErr: true,
			errIs:   ErrInvalidTrackingNumber,
		},
		{
			name:    "invalid Crockford character letter O",
			input:   "SD-20261106-00000O",
			wantErr: true,
			errIs:   ErrInvalidTrackingNumber,
		},
		{
			name:    "invalid Crockford character letter U",
			input:   "SD-20261106-00000U",
			wantErr: true,
			errIs:   ErrInvalidTrackingNumber,
		},
		{
			name:    "special character in suffix",
			input:   "SD-20261106-00000#",
			wantErr: true,
			errIs:   ErrInvalidTrackingNumber,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tn, err := NewTrackingNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewTrackingNumber(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.errIs != nil && !errors.Is(err, tt.errIs) {
					t.Errorf("expected error %v, got %v", tt.errIs, err)
				}
				return
			}
			if !strings.HasPrefix(tn.String(), "SD-") {
				t.Errorf("expected string to start with SD-, got %s", tn.String())
			}
			if len(tn.String()) != 18 {
				t.Errorf("expected length 18, got %d", len(tn.String()))
			}
		})
	}
}

func TestFormatTrackingNumber(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

	t.Run("valid formatting", func(t *testing.T) {
		tn, err := FormatTrackingNumber(now, "A8F3K2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "SD-20261106-A8F3K2"
		if tn.String() != expected {
			t.Errorf("expected %s, got %s", expected, tn.String())
		}
	})

	t.Run("zero date error", func(t *testing.T) {
		_, err := FormatTrackingNumber(time.Time{}, "000001")
		if err == nil {
			t.Fatal("expected error for zero date, got nil")
		}
		if !errors.Is(err, ErrInvalidTrackingNumber) {
			t.Errorf("expected ErrInvalidTrackingNumber, got %v", err)
		}
	})

	t.Run("invalid suffix length", func(t *testing.T) {
		_, err := FormatTrackingNumber(now, "SHORT")
		if err == nil {
			t.Fatal("expected error for short suffix, got nil")
		}
	})
}
