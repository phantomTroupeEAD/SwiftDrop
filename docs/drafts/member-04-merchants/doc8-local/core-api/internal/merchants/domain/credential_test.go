package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"
)

// ADR-003 §6.1 format: <env><8-char prefix>_<32-char secret>
const (
	validPrefix = "Ab3dE9xK"
	validSecret = "q7Lm2Np4Rs6Tu8Vw0Yz1Bc3De5Fg7Hi9" // 32 chars
	validBody   = validPrefix + "_" + validSecret
)

func TestParseAPIKey(t *testing.T) {
	cases := []struct {
		name, raw, env string
		ok             bool
	}{
		{"valid live", EnvPrefixLive + validBody, EnvPrefixLive, true},
		{"valid test", EnvPrefixTest + validBody, EnvPrefixTest, true},
		{"test key on live env", EnvPrefixTest + validBody, EnvPrefixLive, false},
		{"unknown env config", "sd_prod_" + validBody, "sd_prod_", false},
		{"ADR typo sk_live_ not accepted", "sk_live_" + validBody, EnvPrefixLive, false},
		{"missing env prefix", validBody, EnvPrefixLive, false},
		{"missing separator", EnvPrefixLive + validPrefix + validSecret + "x", EnvPrefixLive, false},
		{"separator in wrong place", EnvPrefixLive + validPrefix[:7] + "_" + validPrefix[7:] + validSecret, EnvPrefixLive, false},
		{"prefix 7 chars", EnvPrefixLive + validPrefix[:7] + "_" + validSecret + "x", EnvPrefixLive, false},
		{"secret 31 chars", EnvPrefixLive + validPrefix + "_" + validSecret[:31], EnvPrefixLive, false},
		{"secret 33 chars", EnvPrefixLive + validBody + "x", EnvPrefixLive, false},
		{"non base62 in secret", EnvPrefixLive + validPrefix + "_" + validSecret[:31] + "-", EnvPrefixLive, false},
		{"non base62 in prefix", EnvPrefixLive + "Ab3d-9xK_" + validSecret, EnvPrefixLive, false},
		{"unicode char", EnvPrefixLive + validPrefix + "_" + validSecret[:30] + "é", EnvPrefixLive, false},
		{"empty", "", EnvPrefixLive, false},
		{"bearer prefix included", "Bearer " + EnvPrefixLive + validBody, EnvPrefixLive, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			k, err := ParseAPIKey(c.raw, c.env)
			if c.ok != (err == nil) {
				t.Fatalf("err=%v want ok=%v", err, c.ok)
			}
			if !c.ok && !errors.Is(err, ErrAPIKeyInvalid) {
				t.Fatalf("want ErrAPIKeyInvalid, got %v", err)
			}
			if c.ok && string(k.Prefix()) != validPrefix {
				t.Fatalf("prefix = %s", k.Prefix())
			}
		})
	}
}

func TestNewPlainAPIKeyRoundTrip(t *testing.T) {
	k, err := NewPlainAPIKey(EnvPrefixTest, validPrefix, validSecret)
	if err != nil {
		t.Fatal(err)
	}
	if k.Reveal() != EnvPrefixTest+validBody {
		t.Fatalf("Reveal = %q", k.Reveal())
	}
	if _, err := NewPlainAPIKey(EnvPrefixTest, validPrefix, "short"); !errors.Is(err, ErrAPIKeyInvalid) {
		t.Fatal("short secret must be rejected")
	}
}

// BR-K-03 / BR-X-03: plaintext must never leak through formatting or JSON.
func TestPlainAPIKeyIsRedactedEverywhere(t *testing.T) {
	k, err := ParseAPIKey(EnvPrefixLive+validBody, EnvPrefixLive)
	if err != nil {
		t.Fatal(err)
	}
	var logBuf strings.Builder
	l := log.New(&logBuf, "", 0)
	l.Printf("key=%v %s %+v %#v", k, k, k, k)
	j, _ := json.Marshal(map[string]any{"key": k})
	outputs := []string{fmt.Sprint(k), logBuf.String(), string(j)}
	for _, out := range outputs {
		if strings.Contains(out, validSecret[:6]) {
			t.Fatalf("plaintext leaked: %q", out)
		}
		if !strings.Contains(out, "sd_live_Ab3dE9xK_") {
			t.Fatalf("redacted form should keep prefix: %q", out)
		}
	}
	if k.Reveal() != EnvPrefixLive+validBody {
		t.Fatal("Reveal must return the full key")
	}
}

func TestScopeSet(t *testing.T) {
	s, err := NewScopeSet(nil)
	if err != nil || !s.Has(ScopeParcelsRead) || !s.Has(ScopeParcelsCreate) {
		t.Fatalf("default scopes wrong: %v %v", s, err)
	}
	s, err = NewScopeSet([]string{"parcels:read", "parcels:read"})
	if err != nil || len(s) != 1 || s.Has(ScopeParcelsCreate) {
		t.Fatalf("dedupe failed: %v %v", s, err)
	}
	// parcels:write (old draft name) and webhooks:manage (ADR scope, not grantable to keys — C-3) are rejected.
	for _, bad := range [][]string{{"parcels:write"}, {"webhooks:manage"}, {"tracking:read"}, {"parcels:*"}, {""}, {"PARCELS:READ"}} {
		if _, err := NewScopeSet(bad); !errors.Is(err, ErrInvalidScope) {
			t.Errorf("NewScopeSet(%v) err=%v", bad, err)
		}
	}
}

func ptr(t time.Time) *time.Time { return &t }

func TestCredentialStatus(t *testing.T) {
	now := t0
	cases := []struct {
		name    string
		expires *time.Time
		revoked *time.Time
		want    CredentialStatus
	}{
		{"active no expiry", nil, nil, CredentialActive},
		{"active future expiry", ptr(now.Add(time.Hour)), nil, CredentialActive},
		{"expired exactly now", ptr(now), nil, CredentialExpired},
		{"expired past", ptr(now.Add(-time.Second)), nil, CredentialExpired},
		{"revoked", nil, ptr(now.Add(-time.Hour)), CredentialRevoked},
		{"revoked wins over expired", ptr(now.Add(-time.Hour)), ptr(now.Add(-2 * time.Hour)), CredentialRevoked},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cr := &APICredential{ExpiresAt: c.expires, RevokedAt: c.revoked}
			if got := cr.Status(now); got != c.want {
				t.Fatalf("got %s want %s", got, c.want)
			}
		})
	}
}

// BR-K-10 / BR-K-11: revoke is permanent and idempotent.
func TestRevokeIdempotent(t *testing.T) {
	c := &APICredential{}
	if !c.Revoke(t0) {
		t.Fatal("first revoke should return true")
	}
	if c.Revoke(t0.Add(time.Hour)) {
		t.Fatal("second revoke should return false")
	}
	if !c.RevokedAt.Equal(t0) {
		t.Fatal("second revoke must not change revoked_at")
	}
}

func TestValidateExpiry(t *testing.T) {
	day := 24 * time.Hour
	cases := []struct {
		name string
		exp  *time.Time
		ok   bool
	}{
		{"none", nil, true},
		{"23h", ptr(t0.Add(23 * time.Hour)), false},
		{"1 day", ptr(t0.Add(day)), true},
		{"365 days", ptr(t0.Add(365 * day)), true},
		{"366 days", ptr(t0.Add(366 * day)), false},
		{"past", ptr(t0.Add(-day)), false},
	}
	for _, c := range cases {
		if err := ValidateExpiry(t0, c.exp); (err == nil) != c.ok {
			t.Errorf("%s: err=%v want ok=%v", c.name, err, c.ok)
		}
	}
}

func TestValidateLabel(t *testing.T) {
	if l, err := ValidateLabel("  prod  "); err != nil || l != "prod" {
		t.Fatalf("trim failed: %q %v", l, err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("x", 61)} {
		if _, err := ValidateLabel(bad); !errors.Is(err, ErrInvalidLabel) {
			t.Errorf("ValidateLabel(%q) err=%v", bad, err)
		}
	}
	if _, err := ValidateLabel(strings.Repeat("x", 60)); err != nil {
		t.Error("60 chars should be valid")
	}
}
