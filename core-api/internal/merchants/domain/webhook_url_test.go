package domain

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// BR-W-02, BR-W-03: syntax, scheme, port, userinfo, fragment.
func TestParseWebhookURL(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		allowHTTP bool
		want      error
		norm      string
	}{
		{"https ok", "https://shop.lk/hooks/swiftdrop", false, nil, "https://shop.lk/hooks/swiftdrop"},
		{"https with query", "https://shop.lk/h?src=sd", false, nil, "https://shop.lk/h?src=sd"},
		{"explicit 443 normalised", "https://shop.lk:443/h", false, nil, "https://shop.lk/h"},
		{"uppercase normalised", "HTTPS://Shop.LK/h", false, nil, "https://shop.lk/h"},
		{"trailing dot normalised", "https://shop.lk./h", false, nil, "https://shop.lk/h"},
		{"http rejected in prod", "http://shop.lk/h", false, ErrURLSchemeNotAllowed, ""},
		{"http allowed locally", "http://webhook-sink/h", true, nil, "http://webhook-sink/h"},
		{"http port 80 locally", "http://webhook-sink:80/h", true, nil, "http://webhook-sink/h"},
		{"ftp rejected", "ftp://shop.lk/h", true, ErrURLSchemeNotAllowed, ""},
		{"file rejected", "file:///etc/passwd", true, ErrURLSchemeNotAllowed, ""},
		{"gopher rejected", "gopher://shop.lk:70/", true, ErrURLSchemeNotAllowed, ""},
		{"port 8443 rejected", "https://shop.lk:8443/h", false, ErrURLInvalid, ""},
		{"port 5432 rejected", "https://shop.lk:5432/", false, ErrURLInvalid, ""},
		{"http on 443 rejected", "http://shop.lk:443/h", true, ErrURLInvalid, ""},
		{"userinfo rejected", "https://user:pass@shop.lk/h", false, ErrURLInvalid, ""},
		{"userinfo trick rejected", "https://shop.lk@127.0.0.1/h", false, ErrURLInvalid, ""},
		{"fragment rejected", "https://shop.lk/h#x", false, ErrURLInvalid, ""},
		{"empty fragment rejected", "https://shop.lk/h#", false, ErrURLInvalid, ""},
		{"relative rejected", "/hooks", false, ErrURLInvalid, ""},
		{"no host rejected", "https:///h", false, ErrURLInvalid, ""},
		{"empty", "", false, ErrURLInvalid, ""},
		{"too long", "https://shop.lk/" + strings.Repeat("a", 2040), false, ErrURLInvalid, ""},
		{"IDN must be punycoded first", "https://සිංහල.lk/h", false, ErrURLInvalid, ""},
		{"ipv6 literal parsed", "https://[2001:db8::1]/h", false, nil, "https://[2001:db8::1]/h"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u, err := ParseWebhookURL(c.raw, c.allowHTTP)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if c.want == nil && u.String() != c.norm {
				t.Fatalf("normalised = %q, want %q", u.String(), c.norm)
			}
		})
	}
}

// BR-W-04, BR-W-05, EC-09: blocked names and IP literals, no DNS needed.
func TestPolicyCheckHost(t *testing.T) {
	blocked := []string{
		"https://localhost/h",
		"https://LOCALHOST./h",
		"https://api.localhost/h",
		"https://printer.local/h",
		"https://db.internal/h",
		"https://core-api.swiftdrop.svc/h",
		"https://core-api.swiftdrop.svc.cluster.local/h",
		"https://127.0.0.1/h",
		"https://127.255.255.254/h",
		"https://10.1.2.3/h",
		"https://172.16.0.1/h",
		"https://172.31.255.255/h",
		"https://192.168.1.10/h",
		"https://169.254.169.254/latest/meta-data", // cloud metadata (EC-09)
		"https://0.0.0.0/h",
		"https://[::1]/h",
		"https://[::]/h",
		"https://[fe80::1]/h",
		"https://[fe80::1%25eth0]/h",  // zone id must not bypass the check
		"https://[fd12:3456::1]/h",    // fc00::/7
		"https://[::ffff:10.0.0.1]/h", // IPv4-mapped IPv6
		"https://[::ffff:127.0.0.1]/h",
		"https://2130706433/h", // decimal 127.0.0.1
		"https://127.1/h",      // short form
		"https://0x7f000001/h", // hex
		"https://0x7f.0.0.1/h",
	}
	allowed := []string{
		"https://shop.lk/h",
		"https://93.184.216.34/h",
		"https://172.32.0.1/h", // just outside 172.16.0.0/12
		"https://172.15.255.255/h",
		"https://[2001:db8::1]/h",
		"https://localhost.shop.lk/h", // only suffix match counts
		"https://internal-tools.shop.lk/h",
		"https://v1.api.shop.lk/h",
	}
	p := WebhookURLPolicy{}
	for _, raw := range blocked {
		u, err := ParseWebhookURL(raw, false)
		if err != nil {
			t.Errorf("%s: parse error %v (should parse, then be blocked)", raw, err)
			continue
		}
		if err := p.CheckHost(u); !errors.Is(err, ErrSSRFProhibited) {
			t.Errorf("%s: expected SSRF_PROHIBITED, got %v", raw, err)
		}
	}
	for _, raw := range allowed {
		u, err := ParseWebhookURL(raw, false)
		if err != nil {
			t.Errorf("%s: parse error %v", raw, err)
			continue
		}
		if err := p.CheckHost(u); err != nil {
			t.Errorf("%s: expected allowed, got %v", raw, err)
		}
	}
}

// BR-W-04, BR-W-06, EC-10: every resolved address is checked.
func TestPolicyCheckResolved(t *testing.T) {
	a := func(ss ...string) []netip.Addr {
		out := make([]netip.Addr, len(ss))
		for i, s := range ss {
			out[i] = netip.MustParseAddr(s)
		}
		return out
	}
	cases := []struct {
		name  string
		addrs []netip.Addr
		want  error
	}{
		{"single public v4", a("93.184.216.34"), nil},
		{"public v4 + v6", a("93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"), nil},
		{"public + private (EC-10)", a("93.184.216.34", "10.0.0.12"), ErrSSRFProhibited},
		{"metadata", a("169.254.169.254"), ErrSSRFProhibited},
		{"mapped private", a("::ffff:192.168.0.5"), ErrSSRFProhibited},
		{"ula v6", a("fd00::5"), ErrSSRFProhibited},
		{"no addresses", nil, ErrURLUnresolvable},
	}
	p := WebhookURLPolicy{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := p.CheckResolved(c.addrs); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
}

// Canonical list from webhook-security.md §3.1 must stay in sync (U-23).
func TestBlockedPrefixesContainCanonicalList(t *testing.T) {
	canonical := []string{
		"0.0.0.0/8", "127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"169.254.0.0/16", "::1/128", "fe80::/10", "fc00::/7",
	}
	have := map[string]bool{}
	for _, p := range blockedPrefixes {
		have[p.String()] = true
	}
	for _, c := range canonical {
		if !have[c] {
			t.Errorf("missing canonical CIDR %s", c)
		}
	}
}

// Parser must never panic on hostile input.
func FuzzParseWebhookURL(f *testing.F) {
	for _, s := range []string{"https://shop.lk/h", "https://[::1]:443/", "http://a@b@c/", "https://%zz/", "https://[fe80::1%25en0]/", ":", "https://"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		u, err := ParseWebhookURL(raw, true)
		if err == nil {
			_ = WebhookURLPolicy{}.CheckHost(u)
			_ = u.String()
		}
	})
}
