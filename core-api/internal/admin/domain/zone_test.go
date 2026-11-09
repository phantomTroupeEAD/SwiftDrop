package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func mustID(t *testing.T, s string) ZoneID {
	t.Helper()
	id, err := NewZoneID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustCode(t *testing.T, s string) ZoneCode {
	t.Helper()
	c, err := NewZoneCode(s)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func prefixes(t *testing.T, ss ...string) []PostalPrefix {
	t.Helper()
	out := make([]PostalPrefix, 0, len(ss))
	for _, s := range ss {
		p, err := NewPostalPrefix(s)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

func aZone(t *testing.T) *Zone {
	t.Helper()
	z, err := NewZone(mustID(t, "ZONE_COLOMBO_01"), mustCode(t, "CMB-01"), "Colombo Central", "", prefixes(t, "00100", "00200", "00300"))
	if err != nil {
		t.Fatal(err)
	}
	return z
}

// UT-AD-05: a new zone is enabled, version 1, and records ZONE_CREATED.
func TestNewZoneHappyPath(t *testing.T) {
	z := aZone(t)
	if !z.Enabled() || z.Version() != 1 || z.Name() != "Colombo Central" {
		t.Fatalf("bad zone: %+v", z)
	}
	ev := z.PullEvents()
	if len(ev) != 1 || ev[0].Type != ZoneCreated || ev[0].ZoneID != "ZONE_COLOMBO_01" {
		t.Fatalf("events: %+v", ev)
	}
	if len(z.PullEvents()) != 0 {
		t.Error("PullEvents must clear events")
	}
}

// UT-AD-06: invalid input is refused.
func TestNewZoneInvalid(t *testing.T) {
	id, code := mustID(t, "ZONE_KANDY_01"), mustCode(t, "KDY-01")
	if _, err := NewZone(id, code, "Kandy", "", nil); !errors.Is(err, ErrNoPrefixes) {
		t.Errorf("no prefixes: %v", err)
	}
	if _, err := NewZone(id, code, "Kandy", "", prefixes(t, "20000", "20000")); !errors.Is(err, ErrDuplicatePrefixInZone) {
		t.Errorf("duplicate: %v", err)
	}
	if _, err := NewZone(id, code, "   ", "", prefixes(t, "20000")); !errors.Is(err, ErrInvalidZoneName) {
		t.Errorf("blank name: %v", err)
	}
	if _, err := NewZone(id, code, strings.Repeat("a", 121), "", prefixes(t, "20000")); !errors.Is(err, ErrInvalidZoneName) {
		t.Errorf("long name: %v", err)
	}
	if _, err := NewZone(id, code, strings.Repeat("a", 120), "", prefixes(t, "20000")); err != nil {
		t.Errorf("120-character name must pass: %v", err)
	}
	if _, err := NewZone(id, code, "Kandy", strings.Repeat("d", 501), prefixes(t, "20000")); !errors.Is(err, ErrInvalidDescription) {
		t.Errorf("long description: %v", err)
	}
}

// UT-AD-07: changes bump the version and record what changed.
func TestZoneChangesBumpVersionAndRecordEvents(t *testing.T) {
	z := aZone(t)
	z.PullEvents()
	if err := z.Rename("Colombo Centre"); err != nil {
		t.Fatal(err)
	}
	if err := z.ChangeCode(mustCode(t, "CMB-1C")); err != nil {
		t.Fatal(err)
	}
	if err := z.ReplacePrefixes(prefixes(t, "00100", "00300")); err != nil {
		t.Fatal(err)
	}
	if z.Version() != 2 {
		t.Fatalf("version = %d, want 2 (one save, three changes)", z.Version())
	}
	ev := z.PullEvents()
	if len(ev) != 3 {
		t.Fatalf("want 3 events, got %d", len(ev))
	}
	want := [2]any{[]string{"00100", "00200", "00300"}, []string{"00100", "00300"}}
	if !reflect.DeepEqual(ev[2].Changes["prefixes"], want) {
		t.Errorf("prefix change = %v", ev[2].Changes["prefixes"])
	}
	if ev[1].Changes["code"] != [2]any{"CMB-01", "CMB-1C"} {
		t.Errorf("code change = %v", ev[1].Changes["code"])
	}
	if err := z.ChangeDescription("Fort and Pettah"); err != nil || z.Version() != 3 {
		t.Fatalf("description change after a save: %v, version %d", err, z.Version())
	}
	if err := z.ChangeDescription(strings.Repeat("d", 501)); !errors.Is(err, ErrInvalidDescription) || z.Version() != 3 {
		t.Errorf("bad description must be refused without a change: %v", err)
	}
}

// UT-AD-08: no-op edits do not create versions or audit noise.
func TestNoOpEditsDoNothing(t *testing.T) {
	z := aZone(t)
	z.PullEvents()
	_ = z.Rename("Colombo Central")
	_ = z.ChangeCode(mustCode(t, "CMB-01"))
	_ = z.ChangeDescription("")
	if z.Version() != 1 || len(z.PullEvents()) != 0 {
		t.Error("no-op edit changed version or recorded events")
	}
}

// UT-AD-09: enable/disable state machine.
func TestEnableDisable(t *testing.T) {
	z := aZone(t)
	if err := z.Enable(); !errors.Is(err, ErrAlreadyEnabled) {
		t.Errorf("enable twice: %v", err)
	}
	if err := z.Disable(); err != nil || z.Enabled() {
		t.Fatalf("disable: %v", err)
	}
	z.PullEvents() // saved
	if err := z.Disable(); !errors.Is(err, ErrAlreadyDisabled) {
		t.Errorf("disable twice: %v", err)
	}
	if err := z.Enable(); err != nil || !z.Enabled() {
		t.Fatalf("enable: %v", err)
	}
	if z.Version() != 3 {
		t.Errorf("version = %d, want 3", z.Version())
	}
}

// UT-AD-10: invalid replace keeps the old prefixes (no half change).
func TestReplacePrefixesInvalidKeepsOld(t *testing.T) {
	z := aZone(t)
	if err := z.ReplacePrefixes(nil); !errors.Is(err, ErrNoPrefixes) {
		t.Fatalf("got %v", err)
	}
	if len(z.Prefixes()) != 3 || z.Version() != 1 {
		t.Error("invalid replace changed the zone")
	}
}

// UT-AD-11: callers cannot change prefixes through the returned slice.
func TestPrefixesAreCopied(t *testing.T) {
	z := aZone(t)
	p := z.Prefixes()
	p[0], _ = NewPostalPrefix("99999")
	if z.Prefixes()[0].String() != "00100" {
		t.Error("aggregate state leaked")
	}
}

func TestRestoreRecordsNoEvents(t *testing.T) {
	z := Restore(mustID(t, "ZONE_GALLE_01"), mustCode(t, "GAL-01"), "Galle", "South coast", prefixes(t, "80000"), false, 7)
	if z.Enabled() || z.Version() != 7 || z.Description() != "South coast" || z.ID().String() != "ZONE_GALLE_01" || len(z.PullEvents()) != 0 {
		t.Fatalf("bad restore: %+v", z)
	}
}
