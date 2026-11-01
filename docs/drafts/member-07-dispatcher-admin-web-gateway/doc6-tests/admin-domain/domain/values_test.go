package domain

import (
	"errors"
	"testing"
)

// UT-AD-01..04: value objects check themselves on creation.
func TestNewZoneCode(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"ZONE_COLOMBO_CENTRAL", true},
		{"ZONE_KANDY", true},
		{"ZONE_A", false},         // too short after prefix
		{"zone_colombo", false},   // lower case (TD-05)
		{"COLOMBO", false},        // missing ZONE_ prefix
		{"ZONE_COLOMBO-1", false}, // dash not allowed
		{"", false},
	}
	for _, c := range cases {
		_, err := NewZoneCode(c.in)
		if c.ok && err != nil {
			t.Errorf("%q: unexpected error %v", c.in, err)
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

func TestNewLKR(t *testing.T) {
	m, err := NewLKR(45000)
	if err != nil || m.Cents() != 45000 || m.Currency() != "LKR" {
		t.Fatalf("got %v %v", m, err)
	}
	if _, err := NewLKR(0); err != nil {
		t.Errorf("zero allowed: %v", err)
	}
	if _, err := NewLKR(-100); !errors.Is(err, ErrNegativeMoney) { // TD-06
		t.Errorf("want ErrNegativeMoney, got %v", err)
	}
}
