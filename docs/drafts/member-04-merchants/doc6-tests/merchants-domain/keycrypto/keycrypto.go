// Package keycrypto is reference code for core-api/internal/merchants/infrastructure
// (random_generator.go and hmac_key_hasher.go).
package keycrypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"

	"github.com/swiftdrop/merchants-draft/domain"
)

// NewAPIKey generates a key in ADR-003 format: <env><8 prefix>_<32 secret>.
func NewAPIKey(envPrefix string) (domain.PlainAPIKey, error) {
	prefix, err := GenerateBody(domain.KeyPrefixLen)
	if err != nil {
		return domain.PlainAPIKey{}, err
	}
	secret, err := GenerateBody(domain.KeySecretLen)
	if err != nil {
		return domain.PlainAPIKey{}, err
	}
	return domain.NewPlainAPIKey(envPrefix, prefix, secret)
}

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// randRead is crypto/rand.Read; replaced only in tests to exercise failure paths (FM-08).
var randRead = rand.Read

// GenerateBody returns n uniformly random base62 characters from crypto/rand.
// Rejection sampling (bytes >= 248 are discarded) avoids modulo bias.
func GenerateBody(n int) (string, error) {
	out := make([]byte, 0, n)
	buf := make([]byte, n*2)
	for len(out) < n {
		if _, err := randRead(buf); err != nil {
			return "", fmt.Errorf("keycrypto: random source: %w", err)
		}
		for _, b := range buf {
			if b >= 248 { // 248 = 62*4
				continue
			}
			out = append(out, alphabet[b%62])
			if len(out) == n {
				break
			}
		}
	}
	return string(out), nil
}

// Hash returns hex(HMAC-SHA256(pepper, fullKey)) (BR-K-03, DD-02).
func Hash(pepper []byte, fullKey string) string {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(fullKey))
	return hex.EncodeToString(m.Sum(nil))
}

// Equal compares two hex hashes in constant time (Doc 1 invariant 11).
func Equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
