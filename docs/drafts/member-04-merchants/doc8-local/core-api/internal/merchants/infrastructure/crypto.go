package infrastructure

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"swiftdrop.local/m4/core-api/internal/merchants/domain"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// randomBase62 returns n uniform base62 chars (rejection sampling, no modulo bias).
func randomBase62(n int) (string, error) {
	out := make([]byte, 0, n)
	buf := make([]byte, n*2)
	for len(out) < n {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("random source: %w", err)
		}
		for _, b := range buf {
			if b < 248 {
				out = append(out, alphabet[b%62])
				if len(out) == n {
					break
				}
			}
		}
	}
	return string(out), nil
}

type KeyGenerator struct{}

func (KeyGenerator) NewAPIKey(env string) (domain.PlainAPIKey, error) {
	p, err := randomBase62(domain.KeyPrefixLen)
	if err != nil {
		return domain.PlainAPIKey{}, err
	}
	s, err := randomBase62(domain.KeySecretLen)
	if err != nil {
		return domain.PlainAPIKey{}, err
	}
	return domain.NewPlainAPIKey(env, p, s)
}

func (KeyGenerator) NewSigningSecret() (string, error) {
	s, err := randomBase62(43)
	return "whsec_" + s, err
}

// Hasher is HMAC-SHA256 with a versioned server pepper (DD-02, U-29).
type Hasher struct {
	Peppers map[int][]byte
	Current int
}

func (h Hasher) CurrentVersion() int { return h.Current }

func (h Hasher) Hash(version int, full string) (string, error) {
	p, ok := h.Peppers[version]
	if !ok {
		return "", errors.New("unknown pepper version")
	}
	m := hmac.New(sha256.New, p)
	m.Write([]byte(full))
	return hex.EncodeToString(m.Sum(nil)), nil
}

func (Hasher) Equal(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

// Cipher encrypts webhook signing secrets with AES-256-GCM (DD-05). Output: nonce||ct||tag.
type Cipher struct {
	Key     []byte
	Version int
}

func (c Cipher) Encrypt(plain string) ([]byte, int, error) {
	block, err := aes.NewCipher(c.Key)
	if err != nil {
		return nil, 0, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, 0, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, 0, err
	}
	return gcm.Seal(nonce, nonce, []byte(plain), nil), c.Version, nil
}

// Decrypt is what Member 8's worker does at send time; kept here for tests and demo.
func (c Cipher) Decrypt(ct []byte) (string, error) {
	block, err := aes.NewCipher(c.Key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(ct) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	pt, err := gcm.Open(nil, ct[:gcm.NonceSize()], ct[gcm.NonceSize():], nil)
	return string(pt), err
}

// Resolver resolves hostnames; the caller sets the 2 s timeout (BR-W-06).
type Resolver struct{}

func (Resolver) LookupAddrs(ctx context.Context, host string) ([]netip.Addr, error) {
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if len(addrs) > 16 {
		addrs = addrs[:16]
	}
	return addrs, err
}
