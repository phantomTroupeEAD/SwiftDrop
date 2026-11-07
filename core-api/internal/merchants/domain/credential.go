package domain

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Key format (ADR-003 §6.1): <env>_<8-char prefix>_<32-char secret>, e.g.
// sd_live_a1B2c3D4_9f8E7d6C5b4A3z2Y1x0W9v8U7t6S5r4Q. Prefix and secret are base62.
// 32 base62 chars ≈ 190 bits of entropy (ADR minimum 128).
const (
	EnvPrefixLive = "sd_live_"
	EnvPrefixTest = "sd_test_"
	KeyPrefixLen  = 8
	KeySecretLen  = 32
	keyBodyLen    = KeyPrefixLen + 1 + KeySecretLen
)

// KeyPrefix is the public lookup part of a key (BR-K-02).
type KeyPrefix string

// PlainAPIKey holds a plaintext key. Every formatting path is redacted;
// only Reveal() returns the secret, and only transport calls it, once.
type PlainAPIKey struct {
	env    string
	prefix string
	secret string
}

func isBase62(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}

// ParseAPIKey validates format against the configured environment prefix (F-06 step 2).
func ParseAPIKey(raw, envPrefix string) (PlainAPIKey, error) {
	if envPrefix != EnvPrefixLive && envPrefix != EnvPrefixTest {
		return PlainAPIKey{}, ErrAPIKeyInvalid
	}
	if !strings.HasPrefix(raw, envPrefix) {
		return PlainAPIKey{}, ErrAPIKeyInvalid
	}
	body := raw[len(envPrefix):]
	if len(body) != keyBodyLen || body[KeyPrefixLen] != '_' {
		return PlainAPIKey{}, ErrAPIKeyInvalid
	}
	prefix, secret := body[:KeyPrefixLen], body[KeyPrefixLen+1:]
	if !isBase62(prefix) || !isBase62(secret) {
		return PlainAPIKey{}, ErrAPIKeyInvalid
	}
	return PlainAPIKey{env: envPrefix, prefix: prefix, secret: secret}, nil
}

// NewPlainAPIKey assembles a key from generated parts and validates it.
func NewPlainAPIKey(envPrefix, prefix, secret string) (PlainAPIKey, error) {
	return ParseAPIKey(envPrefix+prefix+"_"+secret, envPrefix)
}

func (k PlainAPIKey) Prefix() KeyPrefix { return KeyPrefix(k.prefix) }
func (k PlainAPIKey) Reveal() string    { return k.env + k.prefix + "_" + k.secret }

func (k PlainAPIKey) redacted() string {
	if k.prefix == "" {
		return "<empty-key>"
	}
	return k.env + k.prefix + "_…"
}

func (k PlainAPIKey) String() string               { return k.redacted() }
func (k PlainAPIKey) GoString() string             { return k.redacted() }
func (k PlainAPIKey) MarshalJSON() ([]byte, error) { return json.Marshal(k.redacted()) }

// --- Scopes (BR-K-07) ---

type Scope string

// Scope names follow ADR-003 §6.1. "webhooks:manage" is intentionally NOT
// grantable to API keys in the baseline (Doc 7 §2 C-3).
const (
	ScopeParcelsCreate Scope = "parcels:create"
	ScopeParcelsRead   Scope = "parcels:read"
)

type ScopeSet []Scope

// NewScopeSet validates and normalises scopes. Empty input means the default (both).
func NewScopeSet(in []string) (ScopeSet, error) {
	if len(in) == 0 {
		return ScopeSet{ScopeParcelsCreate, ScopeParcelsRead}, nil
	}
	seen := map[Scope]bool{}
	out := ScopeSet{}
	for _, s := range in {
		sc := Scope(s)
		if sc != ScopeParcelsCreate && sc != ScopeParcelsRead {
			return nil, ErrInvalidScope
		}
		if !seen[sc] {
			seen[sc] = true
			out = append(out, sc)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func (s ScopeSet) Has(x Scope) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}

// --- Credential aggregate ---

type CredentialStatus string

const (
	CredentialActive  CredentialStatus = "ACTIVE"
	CredentialRevoked CredentialStatus = "REVOKED"
	CredentialExpired CredentialStatus = "EXPIRED"
)

type APICredential struct {
	ID         string
	MerchantID string
	Prefix     KeyPrefix
	Label      string
	Scopes     ScopeSet
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
}

// Status derives the key state; revoked wins over expired (Doc 2 §2.2).
func (c *APICredential) Status(now time.Time) CredentialStatus {
	if c.RevokedAt != nil {
		return CredentialRevoked
	}
	if c.ExpiresAt != nil && !now.Before(*c.ExpiresAt) {
		return CredentialExpired
	}
	return CredentialActive
}

// Revoke marks the key revoked. Returns false if it was already revoked (BR-K-11).
func (c *APICredential) Revoke(now time.Time) bool {
	if c.RevokedAt != nil {
		return false
	}
	t := now
	c.RevokedAt = &t
	return true
}

// ValidateLabel enforces BR-K-05 length rule.
func ValidateLabel(label string) (string, error) {
	l := strings.TrimSpace(label)
	if n := utf8.RuneCountInString(l); n < 1 || n > 60 {
		return "", ErrInvalidLabel
	}
	return l, nil
}

// ValidateExpiry enforces BR-K-08: optional, 1..365 days after now.
func ValidateExpiry(now time.Time, expiresAt *time.Time) error {
	if expiresAt == nil {
		return nil
	}
	d := expiresAt.Sub(now)
	if d < 24*time.Hour || d > 365*24*time.Hour {
		return ErrInvalidExpiry
	}
	return nil
}
