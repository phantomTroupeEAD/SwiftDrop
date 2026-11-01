package domain

import "testing"

// UT-AD-12..14: PrefixOverlapPolicy (BR: one postal code belongs to one zone).
func TestPrefixOverlapPolicy(t *testing.T) {
	p := func(s string) PostalPrefix { x, _ := NewPostalPrefix(s); return x }
	existing := []PrefixOwner{
		{Prefix: p("00300"), ZoneCode: "ZONE_COLOMBO_CENTRAL"},
		{Prefix: p("20000"), ZoneCode: "ZONE_KANDY"},
		{Prefix: p("00400"), ZoneCode: "ZONE_COLOMBO_SOUTH"},
	}
	pol := PrefixOverlapPolicy{}

	t.Run("exact duplicate in another zone clashes", func(t *testing.T) {
		c := pol.Clashes("ZONE_COLOMBO_SOUTH", []PostalPrefix{p("00300")}, existing)
		if len(c) != 1 || c[0].Existing.ZoneCode != "ZONE_COLOMBO_CENTRAL" {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("shorter prefix covering another zone clashes (003 vs 00300)", func(t *testing.T) {
		c := pol.Clashes("ZONE_NEW", []PostalPrefix{p("003")}, existing)
		if len(c) != 1 {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("own prefixes never clash", func(t *testing.T) {
		if c := pol.Clashes("ZONE_COLOMBO_CENTRAL", []PostalPrefix{p("00300")}, existing); len(c) != 0 {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("free prefixes pass", func(t *testing.T) {
		if c := pol.Clashes("ZONE_GALLE", []PostalPrefix{p("80000")}, existing); len(c) != 0 {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("all clashes are reported, not just the first", func(t *testing.T) {
		c := pol.Clashes("ZONE_NEW", []PostalPrefix{p("00300"), p("20000"), p("80000")}, existing)
		if len(c) != 2 {
			t.Fatalf("want 2, got %+v", c)
		}
	})
}
