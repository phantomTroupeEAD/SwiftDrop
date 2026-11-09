package infrastructure

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/merchants/domain"
)

var hexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)
var adrKeyRe = regexp.MustCompile(`^sd_(live|test)_[0-9A-Za-z]{8}_[0-9A-Za-z]{32}$`)

func TestKeyGenerator_MatchesFormat(t *testing.T) {
	gen := KeyGenerator{}
	for _, env := range []string{domain.EnvPrefixLive, domain.EnvPrefixTest} {
		k, err := gen.NewAPIKey(env)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !adrKeyRe.MatchString(k.Reveal()) {
			t.Fatalf("generated key %q does not match ADR-003 format", k.Reveal())
		}
		if _, err := domain.ParseAPIKey(k.Reveal(), env); err != nil {
			t.Fatalf("generated key rejected by domain: %v", err)
		}
	}
}

func TestKeyGenerator_SigningSecretFormat(t *testing.T) {
	gen := KeyGenerator{}
	sec, err := gen.NewSigningSecret()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(sec, "whsec_") {
		t.Fatalf("expected signing secret to start with 'whsec_', got %q", sec)
	}
	if len(sec) != 6+43 {
		t.Fatalf("expected length %d, got %d", 6+43, len(sec))
	}
}

func TestHasher_HMAC(t *testing.T) {
	hasher := Hasher{
		Peppers: map[int][]byte{
			1: []byte("pepper-v1"),
			2: []byte("pepper-v2"),
		},
		Current: 1,
	}

	key := domain.EnvPrefixLive + strings.Repeat("A", 8) + "_" + strings.Repeat("A", 32)
	h1, err := hasher.Hash(1, key)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := hasher.Hash(1, key)
	if err != nil {
		t.Fatal(err)
	}
	h3, err := hasher.Hash(2, key)
	if err != nil {
		t.Fatal(err)
	}

	if !hexRe.MatchString(h1) {
		t.Fatalf("hash format %q does not match expected 64-char hex", h1)
	}
	if h1 != h2 {
		t.Fatal("hash must be deterministic")
	}
	if h1 == h3 {
		t.Fatal("different pepper version must give different hash")
	}
	if !hasher.Equal(h1, h2) {
		t.Fatal("equal hashes must compare true")
	}
	if hasher.Equal(h1, h3) {
		t.Fatal("different hashes must not compare true")
	}

	if _, err := hasher.Hash(999, key); err == nil {
		t.Fatal("expected error for non-existent pepper version")
	}
}

func TestCipher_EncryptDecryptRoundTrip(t *testing.T) {
	key := []byte("01234567890123456789012345678901") // 32 bytes
	c := Cipher{Key: key, Version: 1}

	plain := "whsec_secret_value_12345"
	ct, v, err := c.Encrypt(plain)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}
	if v != 1 {
		t.Fatalf("expected version 1, got %d", v)
	}

	decrypted, err := c.Decrypt(ct)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}
	if decrypted != plain {
		t.Fatalf("expected %q, got %q", plain, decrypted)
	}
}

func TestCipher_Strict32ByteKeyEnforcement(t *testing.T) {
	// 16-byte key (AES-128) must be rejected
	c16 := Cipher{Key: []byte("1234567890123456"), Version: 1}
	if _, _, err := c16.Encrypt("test"); err == nil {
		t.Fatal("expected 16-byte key to be rejected by Encrypt")
	}
	if _, err := c16.Decrypt([]byte("short")); err == nil {
		t.Fatal("expected 16-byte key to be rejected by Decrypt")
	}

	// 24-byte key (AES-192) must be rejected
	c24 := Cipher{Key: []byte("123456789012345678901234"), Version: 1}
	if _, _, err := c24.Encrypt("test"); err == nil {
		t.Fatal("expected 24-byte key to be rejected by Encrypt")
	}
}

func TestCipher_CiphertextTampering(t *testing.T) {
	key := []byte("01234567890123456789012345678901") // 32 bytes
	c := Cipher{Key: key, Version: 1}

	ct, _, err := c.Encrypt("secret_message")
	if err != nil {
		t.Fatal(err)
	}

	// Truncated ciphertext (< nonce + tag)
	if _, err := c.Decrypt(ct[:15]); err == nil {
		t.Fatal("expected error on truncated ciphertext")
	}

	// Modified authentication tag / ciphertext
	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[len(tampered)-1] ^= 0xFF // flip bit in tag
	if _, err := c.Decrypt(tampered); err == nil {
		t.Fatal("expected error on modified authentication tag")
	}

	// Decrypt with wrong key
	wrongKey := []byte("99999999999999999999999999999999")
	cWrong := Cipher{Key: wrongKey, Version: 1}
	if _, err := cWrong.Decrypt(ct); err == nil {
		t.Fatal("expected error when decrypting with incorrect key")
	}
}

func TestResolver_LocalhostLookup(t *testing.T) {
	r := Resolver{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	addrs, err := r.LookupAddrs(ctx, "localhost")
	if err != nil {
		t.Skipf("skipping DNS test if network unavailable: %v", err)
	}
	if len(addrs) == 0 {
		t.Fatal("expected at least one IP for localhost")
	}
}
