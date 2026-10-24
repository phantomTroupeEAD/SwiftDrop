package keycrypto

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/swiftdrop/merchants-draft/domain"
)

var hexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

var adrKeyRe = regexp.MustCompile(`^sd_(live|test)_[0-9A-Za-z]{8}_[0-9A-Za-z]{32}$`)

func TestGeneratedKeyMatchesADRFormat(t *testing.T) {
	for _, env := range []string{domain.EnvPrefixLive, domain.EnvPrefixTest} {
		k, err := NewAPIKey(env)
		if err != nil {
			t.Fatal(err)
		}
		if !adrKeyRe.MatchString(k.Reveal()) {
			t.Fatalf("generated key %q does not match ADR-003 format", k)
		}
		if _, err := domain.ParseAPIKey(k.Reveal(), env); err != nil {
			t.Fatalf("generated key rejected by domain: %v", err)
		}
	}
}

func TestGeneratedKeysAreUniqueAndUseFullAlphabet(t *testing.T) {
	seen := map[string]bool{}
	prefixes := map[string]bool{}
	chars := map[rune]bool{}
	for i := 0; i < 2000; i++ {
		k, err := NewAPIKey(domain.EnvPrefixLive)
		if err != nil {
			t.Fatal(err)
		}
		full := k.Reveal()
		if seen[full] {
			t.Fatal("duplicate key generated")
		}
		seen[full] = true
		prefixes[string(k.Prefix())] = true
		for _, r := range full[len(domain.EnvPrefixLive):] {
			if r != '_' {
				chars[r] = true
			}
		}
	}
	if len(prefixes) != 2000 {
		t.Fatalf("prefix collision in 2000 keys (%d unique) — unexpected at 62^8", len(prefixes))
	}
	if len(chars) != 62 {
		t.Fatalf("expected all 62 base62 chars to appear, saw %d", len(chars))
	}
}

func TestHash(t *testing.T) {
	key := domain.EnvPrefixLive + strings.Repeat("A", 8) + "_" + strings.Repeat("A", 32)
	h1 := Hash([]byte("pepper-v1"), key)
	h2 := Hash([]byte("pepper-v1"), key)
	h3 := Hash([]byte("pepper-v2"), key)
	h4 := Hash([]byte("pepper-v1"), key+"x")
	if !hexRe.MatchString(h1) {
		t.Fatalf("hash format %q does not match DB check constraint", h1)
	}
	if h1 != h2 {
		t.Fatal("hash must be deterministic")
	}
	if h1 == h3 {
		t.Fatal("different pepper must give different hash")
	}
	if h1 == h4 {
		t.Fatal("different key must give different hash")
	}
	if strings.Contains(h1, "AAAA") {
		t.Fatal("hash must not contain key material")
	}
}

// FM-08: a failing random source must produce an error, never a weak key.
func TestRandomSourceFailure(t *testing.T) {
	orig := randRead
	t.Cleanup(func() { randRead = orig })

	randRead = func([]byte) (int, error) { return 0, errors.New("entropy unavailable") }
	if _, err := GenerateBody(8); err == nil {
		t.Fatal("GenerateBody must fail when random source fails")
	}
	if _, err := NewAPIKey(domain.EnvPrefixLive); err == nil {
		t.Fatal("NewAPIKey must fail when prefix generation fails")
	}

	calls := 0
	randRead = func(b []byte) (int, error) { // prefix succeeds, secret fails
		calls++
		if calls > 1 {
			return 0, errors.New("entropy unavailable")
		}
		return orig(b)
	}
	if _, err := NewAPIKey(domain.EnvPrefixLive); err == nil {
		t.Fatal("NewAPIKey must fail when secret generation fails")
	}
}

func TestEqual(t *testing.T) {
	a := Hash([]byte("p"), "k1")
	b := Hash([]byte("p"), "k2")
	if !Equal(a, a) || Equal(a, b) || Equal(a, "") {
		t.Fatal("Equal wrong")
	}
}
