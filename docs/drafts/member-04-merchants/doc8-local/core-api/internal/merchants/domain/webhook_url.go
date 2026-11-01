package domain

import (
	"net/netip"
	"net/url"
	"strings"
)

const maxWebhookURLLen = 2048

// blockedPrefixes is the canonical list from docs/contracts/webhook-security.md §3.1,
// plus "::/128" (IPv6 unspecified, behaves like 0.0.0.0) — proposed addition U-32.
var blockedPrefixes = func() []netip.Prefix {
	cidrs := []string{
		"0.0.0.0/8", "127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"169.254.0.0/16", "::1/128", "fe80::/10", "fc00::/7",
		"::/128",
	}
	out := make([]netip.Prefix, len(cidrs))
	for i, c := range cidrs {
		out[i] = netip.MustParsePrefix(c)
	}
	return out
}()

var blockedHostSuffixes = []string{".local", ".internal", ".svc", ".cluster.local", ".localhost"}

// WebhookURL is a parsed, normalised webhook destination (BR-W-02, BR-W-03).
type WebhookURL struct {
	scheme string
	host   string // lower-case, no trailing dot, no brackets for IPv6
	port   string // "" when default
	path   string // path + query as given
}

// ParseWebhookURL validates syntax, scheme, port, userinfo and fragment.
// Non-ASCII (IDN) hosts must be converted to punycode by infrastructure before
// calling this; the domain rejects them to stay stdlib-only (EC-14).
func ParseWebhookURL(raw string, allowHTTP bool) (WebhookURL, error) {
	if raw == "" || len(raw) > maxWebhookURLLen {
		return WebhookURL{}, ErrURLInvalid
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Opaque != "" {
		return WebhookURL{}, ErrURLInvalid
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "https":
	case "http":
		if !allowHTTP {
			return WebhookURL{}, ErrURLSchemeNotAllowed
		}
	default:
		return WebhookURL{}, ErrURLSchemeNotAllowed
	}
	if u.User != nil || u.Fragment != "" || strings.Contains(raw, "#") {
		return WebhookURL{}, ErrURLInvalid
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return WebhookURL{}, ErrURLInvalid
	}
	for i := 0; i < len(host); i++ {
		if host[i] >= 0x80 {
			return WebhookURL{}, ErrURLInvalid
		}
	}
	port := u.Port()
	switch {
	case port == "":
	case scheme == "https" && port == "443", scheme == "http" && port == "80":
		port = ""
	default:
		return WebhookURL{}, ErrURLInvalid
	}
	return WebhookURL{scheme: scheme, host: host, port: port, path: u.RequestURI()}, nil
}

func (w WebhookURL) Host() string { return w.host }

// String returns the normalised URL that is stored.
func (w WebhookURL) String() string {
	h := w.host
	if strings.Contains(h, ":") { // IPv6 literal
		h = "[" + h + "]"
	}
	return w.scheme + "://" + h + w.path
}

// WebhookURLPolicy decides whether a destination is allowed (SEC-010, SEC-011).
// It is pure: DNS is resolved by infrastructure and passed to CheckResolved.
type WebhookURLPolicy struct{}

// CheckHost rejects blocked names and IP literals without any DNS lookup (BR-W-05).
func (WebhookURLPolicy) CheckHost(w WebhookURL) error {
	h := w.host
	if h == "localhost" {
		return ErrSSRFProhibited
	}
	for _, s := range blockedHostSuffixes {
		if strings.HasSuffix(h, s) {
			return ErrSSRFProhibited
		}
	}
	if a, err := netip.ParseAddr(h); err == nil {
		if isBlocked(a) {
			return ErrSSRFProhibited
		}
		return nil
	}
	// Obfuscated numeric hosts ("2130706433", "127.1", "0x7f.0.0.1") are not valid
	// dotted-quads for netip but some resolvers treat them as IPs. Reject them.
	if looksNumericHost(h) {
		return ErrSSRFProhibited
	}
	return nil
}

// CheckResolved rejects if ANY resolved address is blocked (BR-W-04, EC-10).
func (WebhookURLPolicy) CheckResolved(addrs []netip.Addr) error {
	if len(addrs) == 0 {
		return ErrURLUnresolvable
	}
	for _, a := range addrs {
		if isBlocked(a) {
			return ErrSSRFProhibited
		}
	}
	return nil
}

func isBlocked(a netip.Addr) bool {
	a = a.WithZone("").Unmap() // fe80::1%eth0 and ::ffff:10.0.0.1 must still match
	if !a.IsValid() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

func looksNumericHost(h string) bool {
	if strings.HasPrefix(h, "0x") {
		return true
	}
	labels := strings.Split(h, ".")
	last := labels[len(labels)-1]
	if last == "" {
		return false
	}
	for i := 0; i < len(last); i++ {
		if last[i] < '0' || last[i] > '9' {
			return false
		}
	}
	return true // a real TLD is never all digits
}
