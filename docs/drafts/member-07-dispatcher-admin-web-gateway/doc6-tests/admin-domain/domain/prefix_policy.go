package domain

// PrefixOwner is a prefix already stored for some zone.
type PrefixOwner struct {
	Prefix   PostalPrefix
	ZoneCode string
}

// Clash is a new prefix that overlaps a prefix owned by another zone.
type Clash struct {
	New      PostalPrefix
	Existing PrefixOwner
}

// PrefixOverlapPolicy is a domain service: it compares a zone's prefixes with
// prefixes owned by other zones. The use case runs it under an advisory lock.
type PrefixOverlapPolicy struct{}

func (PrefixOverlapPolicy) Clashes(zoneCode string, newPrefixes []PostalPrefix, existing []PrefixOwner) []Clash {
	var out []Clash
	for _, n := range newPrefixes {
		for _, e := range existing {
			if e.ZoneCode == zoneCode {
				continue // the zone's own prefixes never clash with themselves
			}
			if n.Overlaps(e.Prefix) {
				out = append(out, Clash{New: n, Existing: e})
			}
		}
	}
	return out
}
