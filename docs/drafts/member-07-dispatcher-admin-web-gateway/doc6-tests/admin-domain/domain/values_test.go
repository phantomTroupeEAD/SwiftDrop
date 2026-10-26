package domain

import (
	"errors"
	"testing"
)

// UT-AD-01..04: value objects check themselves on creation.
func TestNewZoneID(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"ZONE_COLOMBO_01", true},
		{"ZONE_KANDY", true},
		{"ZONE_A", false},                                 // too short after prefix
		{"zone_colombo", false},                           // lower case (TD-05)
		{"COLOMBO", false},                                // missing ZONE_ prefix
		{"ZONE_COLOMBO-1", false},                         // dash not allowed in the id
		{"ZONE_" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ12", false}, // 33 characters, column is VARCHAR(32)
		{"", false},
	}
	for _, c := range cases {
		_, err := NewZoneID(c.in)
		if c.ok && err != nil {
			t.Errorf("%q: unexpected error %v", c.in, err)
		}
		if !c.ok && !errors.Is(err, ErrInvalidZoneID) {
			t.Errorf("%q: want ErrInvalidZoneID, got %v", c.in, err)
		}
	}
}

func TestNewZoneCode(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"CMB-01", true}, {"KDY_01", true}, {"AB", true},
		{"A", false}, {"cmb-01", false}, {"CMB 01", false}, {"", false}, // TD-06
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567", false}, // 33 characters
	}
	for _, c := range cases {
		_, err := NewZoneCode(c.in)
		if c.ok != (err == nil) {
			t.Errorf("%q: ok=%v err=%v", c.in, c.ok, err)
		}
		if !c.ok && !errors.Is(err, ErrInvalidZoneCode) {
			t.Errorf("%q: want ErrInvalidZoneCode, got %v", c.in, err)
		}
	}
}

func TestNewPostalPrefix(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"00300", true}, {"0030", true}, {"003", true}, // TD-01
		{"00", false}, {"003001", false}, {"00A00", false}, {" 0030", false}, {"", false}, // TD-02
	}
	for _, c := range cases {
		_, err := NewPostalPrefix(c.in)
		if c.ok != (err == nil) {
			t.Errorf("%q: ok=%v err=%v", c.in, c.ok, err)
		}
		if !c.ok && !errors.Is(err, ErrInvalidPrefix) {
			t.Errorf("%q: want ErrInvalidPrefix, got %v", c.in, err)
		}
	}
}

func TestPrefixOverlaps(t *testing.T) {
	p := func(s string) PostalPrefix { x, _ := NewPostalPrefix(s); return x }
	if !p("003").Overlaps(p("00300")) || !p("00300").Overlaps(p("003")) {
		t.Error("003 and 00300 must overlap both ways")
	}
	if !p("00300").Overlaps(p("00300")) {
		t.Error("same prefix overlaps")
	}
	if p("00300").Overlaps(p("00400")) || p("003").Overlaps(p("004")) {
		t.Error("different areas must not overlap")
	}
}

func TestPrefixCovers(t *testing.T) {
	p, _ := NewPostalPrefix("003")
	if !p.Covers("00300") || !p.Covers("003") || p.Covers("00400") {
		t.Error("Covers must match postal codes that start with the prefix")
	}
}
