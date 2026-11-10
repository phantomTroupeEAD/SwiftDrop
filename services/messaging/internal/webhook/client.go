package webhook

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrSSRFBlocked  = errors.New("webhook: blocked outbound request to prohibited/private IP range (SSRF guard)")
	ErrInsecureHTTP = errors.New("webhook: plain HTTP is not permitted outside local environment")
)

// BlacklistedCIDRs contains all private, loopback, link-local, and reserved IP ranges.
var BlacklistedCIDRs = []string{
	"0.0.0.0/8",      // "This" network (RFC 1122)
	"10.0.0.0/8",     // RFC 1918 private
	"100.64.0.0/10",  // Carrier-Grade NAT (RFC 6598)
	"127.0.0.0/8",    // IPv4 loopback (RFC 1122)
	"169.254.0.0/16", // Link-local / Cloud metadata (169.254.169.254)
	"172.16.0.0/12",  // RFC 1918 private
	"192.0.0.0/24",   // IETF Protocol Assignments (RFC 6890)
	"192.168.0.0/16", // RFC 1918 private
	"198.18.0.0/15",  // Benchmark testing (RFC 2544)
	"224.0.0.0/4",    // Multicast (RFC 5771)
	"240.0.0.0/4",    // Reserved (RFC 1112)
	"::1/128",        // IPv6 loopback (RFC 4291)
	"fe80::/10",      // IPv6 Link-Local (RFC 4291)
	"fc00::/7",       // IPv6 Unique Local Address (ULA, RFC 4193)
	"ff00::/8",       // IPv6 Multicast (RFC 4291)
	"64:ff9b::/96",   // IPv4/IPv6 translation (RFC 6052)
}

var parsedBlacklist []*net.IPNet

func init() {
	for _, cidr := range BlacklistedCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			parsedBlacklist = append(parsedBlacklist, ipNet)
		}
	}
}

// IsPrivateOrBlockedIP returns true if ip falls into any private, loopback, or metadata CIDR.
func IsPrivateOrBlockedIP(ip net.IP) bool {
	// 1. Direct match on native IP (matches native IPv6: ::1, fe80::/10, fc00::/7, ff00::/8, 64:ff9b::/96)
	for _, block := range parsedBlacklist {
		if block.Contains(ip) {
			return true
		}
	}

	// 2. Unmap IPv4-mapped IPv6 addresses (e.g. ::ffff:10.0.0.1 -> 10.0.0.1) and check against IPv4 blocks
	if ipv4 := ip.To4(); ipv4 != nil {
		for _, block := range parsedBlacklist {
			if block.Contains(ipv4) {
				return true
			}
		}
	}

	return false
}

// Resolver defines DNS lookup abstraction for testability.
type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// SafeClient wraps http.Client with strict SSRF immunity and redirect blocking.
type SafeClient struct {
	httpClient        *http.Client
	allowPrivateHosts bool
	resolver          Resolver
}

// NewSafeClient initializes a SafeClient.
func NewSafeClient(timeout time.Duration, allowPrivateHosts bool, resolver Resolver) *SafeClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	sc := &SafeClient{
		allowPrivateHosts: allowPrivateHosts,
		resolver:          resolver,
	}

	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("failed to split host/port: %w", err)
			}

			// Check if host is direct IP
			if directIP := net.ParseIP(host); directIP != nil {
				if !sc.allowPrivateHosts && IsPrivateOrBlockedIP(directIP) {
					return nil, ErrSSRFBlocked
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(directIP.String(), port))
			}

			// Resolve host via resolver
			ips, err := sc.resolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("dns resolution failed: %w", err)
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("dns resolution returned no IP addresses for %s", host)
			}

			// Validate all resolved IPs; reject if ANY resolved IP is in blacklisted CIDR
			for _, ipAddr := range ips {
				if !sc.allowPrivateHosts && IsPrivateOrBlockedIP(ipAddr.IP) {
					return nil, fmt.Errorf("%w: %s resolves to %s", ErrSSRFBlocked, host, ipAddr.IP.String())
				}
			}

			// Dial first safe IP directly
			safeTarget := net.JoinHostPort(ips[0].IP.String(), port)
			return dialer.DialContext(ctx, network, safeTarget)
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: timeout,
	}

	sc.httpClient = &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			// Rule: NEVER follow HTTP redirects
			return http.ErrUseLastResponse
		},
	}

	return sc
}

// DispatchResult represents the HTTP execution outcome.
type DispatchResult struct {
	StatusCode   int
	Snippet      string
	ErrorClass   string
	ErrorMessage string
	IsSuccess    bool
	IsRetryable  bool
	IsTerminal   bool
}

// Do executes an outbound HTTP POST request with SSRF guard and sanitizes the response snippet.
func (c *SafeClient) Do(ctx context.Context, targetURL string, headers map[string]string, body []byte) DispatchResult {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return DispatchResult{
			ErrorClass:   "INVALID_URL",
			ErrorMessage: fmt.Sprintf("invalid webhook target url: %v", err),
			IsTerminal:   true,
		}
	}

	// Validate scheme
	if parsedURL.Scheme != "https" {
		if !c.allowPrivateHosts {
			return DispatchResult{
				ErrorClass:   "INSECURE_SCHEME",
				ErrorMessage: "plain http scheme is not allowed in non-local environments",
				IsTerminal:   true,
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, strings.NewReader(string(body)))
	if err != nil {
		return DispatchResult{
			ErrorClass:   "REQUEST_INIT_FAILED",
			ErrorMessage: err.Error(),
			IsTerminal:   true,
		}
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, ErrSSRFBlocked) || strings.Contains(err.Error(), "blocked outbound request") {
			return DispatchResult{
				ErrorClass:   "SSRF_BLOCKED",
				ErrorMessage: err.Error(),
				IsTerminal:   true,
			}
		}
		// Network errors or timeouts are retryable
		return DispatchResult{
			ErrorClass:   "NETWORK_ERROR",
			ErrorMessage: err.Error(),
			IsRetryable:  true,
		}
	}
	defer resp.Body.Close()

	// Read up to 512 bytes for snippet
	limitReader := io.LimitReader(resp.Body, 512)
	snippetBytes, _ := io.ReadAll(limitReader)
	snippet := SanitizeSnippet(snippetBytes)

	res := DispatchResult{
		StatusCode: resp.StatusCode,
		Snippet:    snippet,
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		res.IsSuccess = true

	case resp.StatusCode == http.StatusGone: // 410 Gone is explicitly terminal
		res.ErrorClass = "HTTP_410_GONE"
		res.ErrorMessage = "endpoint returned 410 Gone (terminal)"
		res.IsTerminal = true

	case resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		res.ErrorClass = fmt.Sprintf("HTTP_%d", resp.StatusCode)
		res.ErrorMessage = fmt.Sprintf("server returned transient status code %d", resp.StatusCode)
		res.IsRetryable = true

	default: // 3xx or 4xx other than 408/429
		res.ErrorClass = fmt.Sprintf("HTTP_%d", resp.StatusCode)
		res.ErrorMessage = fmt.Sprintf("server returned client error status code %d", resp.StatusCode)
		res.IsTerminal = true
	}

	return res
}

// SanitizeSnippet cleans a response snippet: truncates to 500 bytes and strips control/NUL characters.
func SanitizeSnippet(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	if len(data) > 500 {
		data = data[:500]
	}

	// Remove NUL and non-printable control characters (except newline, tab)
	cleaned := make([]rune, 0, len(data))
	for _, r := range string(data) {
		if r == 0 {
			continue
		}
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			continue
		}
		cleaned = append(cleaned, r)
	}

	return string(cleaned)
}
