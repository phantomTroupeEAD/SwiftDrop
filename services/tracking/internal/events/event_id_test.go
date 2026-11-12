package events

import (
	"errors"
	"testing"
	"time"
)

func TestFormatEventID(t *testing.T) {
	ts := time.Unix(1730036700, 0).UTC()
	id := FormatEventID("SD-20261025-A8F3K2", "OUT_FOR_DELIVERY", ts)

	expected := "SD-20261025-A8F3K2:OUT_FOR_DELIVERY:1730036700"
	if id != expected {
		t.Fatalf("expected formatted ID %q, got %q", expected, id)
	}
}

func TestFormatEventIDWithTieBreaker(t *testing.T) {
	ts := time.Unix(1730036700, 0).UTC()

	// With tiebreaker (e.g. milestone ID)
	idWithTie := FormatEventIDWithTieBreaker("SD-20261025-A8F3K2", "ATTEMPT_FAILED", ts, "ms-attempt-1")
	expected := "SD-20261025-A8F3K2:ATTEMPT_FAILED:1730036700:ms-attempt-1"
	if idWithTie != expected {
		t.Fatalf("expected formatted ID with tiebreaker %q, got %q", expected, idWithTie)
	}

	// Without tiebreaker falls back to 3-part canonical
	idNoTie := FormatEventIDWithTieBreaker("SD-20261025-A8F3K2", "ATTEMPT_FAILED", ts, "")
	expectedNoTie := "SD-20261025-A8F3K2:ATTEMPT_FAILED:1730036700"
	if idNoTie != expectedNoTie {
		t.Fatalf("expected fallback ID %q, got %q", expectedNoTie, idNoTie)
	}
}

func TestParseEventID_Success(t *testing.T) {
	t.Run("canonical 3-part ID", func(t *testing.T) {
		raw := "SD-20261025-A8F3K2:OUT_FOR_DELIVERY:1730036700"
		parsed, err := ParseEventID(raw)
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}

		if parsed.TrackingNumber != "SD-20261025-A8F3K2" {
			t.Errorf("expected tracking number 'SD-20261025-A8F3K2', got %q", parsed.TrackingNumber)
		}
		if parsed.Status != "OUT_FOR_DELIVERY" {
			t.Errorf("expected status 'OUT_FOR_DELIVERY', got %q", parsed.Status)
		}
		if parsed.Timestamp.Unix() != 1730036700 {
			t.Errorf("expected timestamp 1730036700, got %d", parsed.Timestamp.Unix())
		}
		if parsed.TieBreaker != "" {
			t.Errorf("expected empty tiebreaker, got %q", parsed.TieBreaker)
		}
		if parsed.IsOpaque {
			t.Error("expected IsOpaque to be false")
		}
	})

	t.Run("extended 4-part ID with tiebreaker", func(t *testing.T) {
		raw := "SD-20261025-A8F3K2:ATTEMPT_FAILED:1730036700:attempt-1"
		parsed, err := ParseEventID(raw)
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}

		if parsed.TrackingNumber != "SD-20261025-A8F3K2" {
			t.Errorf("expected tracking number 'SD-20261025-A8F3K2', got %q", parsed.TrackingNumber)
		}
		if parsed.Status != "ATTEMPT_FAILED" {
			t.Errorf("expected status 'ATTEMPT_FAILED', got %q", parsed.Status)
		}
		if parsed.Timestamp.Unix() != 1730036700 {
			t.Errorf("expected timestamp 1730036700, got %d", parsed.Timestamp.Unix())
		}
		if parsed.TieBreaker != "attempt-1" {
			t.Errorf("expected tiebreaker 'attempt-1', got %q", parsed.TieBreaker)
		}
	})

	t.Run("opaque Member 8 broker event ID", func(t *testing.T) {
		raw := "evt_01HXYZ9876543210ABC"
		parsed, err := ParseEventID(raw)
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}

		if !parsed.IsOpaque {
			t.Error("expected IsOpaque to be true")
		}
		if parsed.Raw != raw {
			t.Errorf("expected raw ID %q, got %q", raw, parsed.Raw)
		}
	})
}

func TestParseEventID_SameSecondAndRepeatedTransitions(t *testing.T) {
	ts := time.Unix(1730036700, 0).UTC()
	tn := "SD-20261025-A8F3K2"

	// 1. Two events in same second with different status: naturally distinct
	idAssigned := FormatEventID(tn, "ASSIGNED", ts)
	idPickedUp := FormatEventID(tn, "PICKED_UP", ts)
	if idAssigned == idPickedUp {
		t.Fatalf("expected distinct IDs for different statuses in same second, got identical %q", idAssigned)
	}

	// 2. Repeated transitions to same status (e.g. ATTEMPT_FAILED): tiebreaker differentiates
	idAttempt1 := FormatEventIDWithTieBreaker(tn, "ATTEMPT_FAILED", ts, "attempt-1")
	idAttempt2 := FormatEventIDWithTieBreaker(tn, "ATTEMPT_FAILED", ts, "attempt-2")
	if idAttempt1 == idAttempt2 {
		t.Fatalf("expected distinct IDs for repeated status with tiebreakers, got identical %q", idAttempt1)
	}

	p1, err1 := ParseEventID(idAttempt1)
	p2, err2 := ParseEventID(idAttempt2)
	if err1 != nil || err2 != nil {
		t.Fatalf("failed parsing tiebreaker IDs: %v, %v", err1, err2)
	}
	if p1.TieBreaker != "attempt-1" || p2.TieBreaker != "attempt-2" {
		t.Errorf("failed to extract tiebreakers correctly: %q, %q", p1.TieBreaker, p2.TieBreaker)
	}
}

func TestParseEventID_ValidationFailures(t *testing.T) {
	testCases := []struct {
		name string
		raw  string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"missing timestamp", "SD-20261025-A8F3K2:OUT_FOR_DELIVERY"},
		{"too many parts (5 parts)", "SD-20261025-A8F3K2:OUT_FOR_DELIVERY:1730036700:part4:part5"},
		{"invalid tracking prefix", "XX-20261025-A8F3K2:OUT_FOR_DELIVERY:1730036700"},
		{"invalid tracking date", "SD-20269999-A8F3K2:OUT_FOR_DELIVERY:1730036700"},
		{"invalid tracking suffix", "SD-20261025-A8F3!@:OUT_FOR_DELIVERY:1730036700"},
		{"unknown status", "SD-20261025-A8F3K2:FLYING_IN_SPACE:1730036700"},
		{"non-numeric timestamp", "SD-20261025-A8F3K2:OUT_FOR_DELIVERY:not-a-number"},
		{"negative timestamp", "SD-20261025-A8F3K2:OUT_FOR_DELIVERY:-100"},
		{"short unrecognized token", "short_unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseEventID(tc.raw)
			if err == nil {
				t.Fatalf("expected error for %s (%q), got parsed: %+v", tc.name, tc.raw, parsed)
			}
			if !errors.Is(err, ErrInvalidEventID) {
				t.Errorf("expected error wrapping ErrInvalidEventID, got: %v", err)
			}
		})
	}
}
