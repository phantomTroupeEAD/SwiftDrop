package domain

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	zoneIDPattern   = regexp.MustCompile(`^ZONE_[A-Z0-9_]{2,27}$`)
	zoneCodePattern = regexp.MustCompile(`^[A-Z0-9_-]{2,32}$`)
	prefixPattern   = regexp.MustCompile(`^[0-9]{3,5}$`)
)

// ZoneID is the zone's primary key, such as ZONE_COLOMBO_01. Other modules'
// foreign keys point at it (VARCHAR(32)), so it never changes.
type ZoneID struct{ v string }

func NewZoneID(s string) (ZoneID, error) {
	if !zoneIDPattern.MatchString(s) {
		return ZoneID{}, fmt.Errorf("%w: %q", ErrInvalidZoneID, s)
	}
	return ZoneID{v: s}, nil
}

func (id ZoneID) String() string { return id.v }

// ZoneCode is a short human code such as CMB-01. It can be changed.
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

// Covers reports whether a full postal code starts with this prefix.
func (p PostalPrefix) Covers(postalCode string) bool { return strings.HasPrefix(postalCode, p.v) }
