package domain

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	zoneCodePattern = regexp.MustCompile(`^ZONE_[A-Z0-9_]{2,35}$`)
	prefixPattern   = regexp.MustCompile(`^[0-9]{3,5}$`)
)

// ZoneCode is a human code such as ZONE_COLOMBO_CENTRAL.
type ZoneCode struct{ v string }

func NewZoneCode(s string) (ZoneCode, error) {
	if !zoneCodePattern.MatchString(s) {
		return ZoneCode{}, fmt.Errorf("%w: %q", ErrInvalidZoneCode, s)
	}
	return ZoneCode{v: s}, nil
}

func (c ZoneCode) String() string { return c.v }

// PostalPrefix is a Sri Lankan postal code or the start of one (3 to 5 digits).
type PostalPrefix struct{ v string }

func NewPostalPrefix(s string) (PostalPrefix, error) {
	if !prefixPattern.MatchString(s) {
		return PostalPrefix{}, fmt.Errorf("%w: %q", ErrInvalidPrefix, s)
	}
	return PostalPrefix{v: s}, nil
}

func (p PostalPrefix) String() string { return p.v }

// Overlaps reports whether one prefix is the start of the other ("003" and "00300").
func (p PostalPrefix) Overlaps(o PostalPrefix) bool {
	return strings.HasPrefix(p.v, o.v) || strings.HasPrefix(o.v, p.v)
}

// Money is an amount in LKR cents. It never holds a float.
type Money struct{ cents int64 }

func NewLKR(cents int64) (Money, error) {
	if cents < 0 {
		return Money{}, fmt.Errorf("%w: %d", ErrNegativeMoney, cents)
	}
	return Money{cents: cents}, nil
}

func (m Money) Cents() int64     { return m.cents }
func (m Money) Currency() string { return "LKR" }
