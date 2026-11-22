package application

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseAuditFilterEmptyLetsEverythingThrough(t *testing.T) {
	f, err := ParseAuditFilter("", "", "", "", "", "")
	if err != nil || !f.IsZero() {
		t.Fatalf("empty filter: %+v, %v", f, err)
	}
}

func TestParseAuditFilterValidValues(t *testing.T) {
	f, err := ParseAuditFilter("ZONE_CREATED", "ZONE", "ZONE_COLOMBO_01", "ADMIN", "2026-11-14", "2026-11-15")
	if err != nil {
		t.Fatalf("valid filter refused: %v", err)
	}
	if f.Action != "ZONE_CREATED" || f.TargetType != "ZONE" || f.TargetID != "ZONE_COLOMBO_01" || f.ActorRole != "ADMIN" {
		t.Fatalf("text fields not kept: %+v", f)
	}
	if f.IsZero() {
		t.Fatal("a filter with values must not be zero")
	}
}

// Dates are Sri Lanka days (UTC+5:30), both ends inclusive.
func TestParseAuditFilterColomboDayBoundaries(t *testing.T) {
	f, err := ParseAuditFilter("", "", "", "", "2026-11-14", "2026-11-14")
	if err != nil {
		t.Fatal(err)
	}
	wantFrom := time.Date(2026, 11, 13, 18, 30, 0, 0, time.UTC) // 14 Nov 00:00 in Colombo
	wantTo := time.Date(2026, 11, 14, 18, 30, 0, 0, time.UTC)   // 15 Nov 00:00 in Colombo, exclusive
	if f.From == nil || !f.From.Equal(wantFrom) {
		t.Fatalf("from = %v, want %v", f.From, wantFrom)
	}
	if f.To == nil || !f.To.Equal(wantTo) {
		t.Fatalf("to = %v, want %v", f.To, wantTo)
	}
}

func TestParseAuditFilterOnlyOneDateIsFine(t *testing.T) {
	if f, err := ParseAuditFilter("", "", "", "", "2026-11-14", ""); err != nil || f.From == nil || f.To != nil {
		t.Fatalf("from only: %+v, %v", f, err)
	}
	if f, err := ParseAuditFilter("", "", "", "", "", "2026-11-14"); err != nil || f.From != nil || f.To == nil {
		t.Fatalf("to only: %+v, %v", f, err)
	}
}

func TestParseAuditFilterRefusesBadInput(t *testing.T) {
	cases := []struct {
		name                                         string
		action, targetType, targetID, role, from, to string
	}{
		{"lower-case action", "zone_created", "", "", "", "", ""},
		{"action with a space", "ZONE CREATED", "", "", "", "", ""},
		{"action starting with a digit", "1ZONE", "", "", "", "", ""},
		{"action too long", strings.Repeat("A", 65), "", "", "", "", ""},
		{"lower-case target type", "", "zone", "", "", "", ""},
		{"target id with a quote", "", "", "ZONE_1'; DROP TABLE audit_logs;--", "", "", ""},
		{"target id with a space", "", "", "ZONE 1", "", "", ""},
		{"unknown role", "", "", "", "ROOT", "", ""},
		{"lower-case role", "", "", "", "admin", "", ""},
		{"bad from date", "", "", "", "", "14/11/2026", ""},
		{"bad to date", "", "", "", "", "", "2026-13-40"},
		{"from after to", "", "", "", "", "2026-11-15", "2026-11-14"},
	}
	for _, c := range cases {
		f, err := ParseAuditFilter(c.action, c.targetType, c.targetID, c.role, c.from, c.to)
		if !errors.Is(err, ErrValidation) {
			t.Errorf("%s: want ErrValidation, got %v", c.name, err)
		}
		if !f.IsZero() {
			t.Errorf("%s: a refused filter must not carry values: %+v", c.name, f)
		}
	}
}

func TestParseAuditFilterAllRolesAccepted(t *testing.T) {
	for _, role := range []string{"ADMIN", "DISPATCHER", "COURIER", "MERCHANT", "CUSTOMER", "SYSTEM"} {
		if _, err := ParseAuditFilter("", "", "", role, "", ""); err != nil {
			t.Errorf("role %s refused: %v", role, err)
		}
	}
}
