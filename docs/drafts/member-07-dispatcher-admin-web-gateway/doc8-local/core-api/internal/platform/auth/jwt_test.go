package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestVerify(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_800_000_000, 0)
	v := NewVerifier(&key.PublicKey, "http://localhost")
	v.now = func() time.Time { return now }
	good := Claims{Iss: "http://localhost", Sub: "u1", Role: "ADMIN", Exp: now.Add(15 * time.Minute).Unix(), Nbf: now.Unix()}
	tok, _ := Sign(key, good)
	if c, err := v.Verify(tok); err != nil || c.Role != "ADMIN" {
		t.Fatalf("good token refused: %v", err)
	}
	bad := map[string]func() string{
		"wrong key":    func() string { s, _ := Sign(other, good); return s },
		"expired":      func() string { c := good; c.Exp = now.Add(-2 * time.Minute).Unix(); s, _ := Sign(key, c); return s },
		"not yet":      func() string { c := good; c.Nbf = now.Add(5 * time.Minute).Unix(); s, _ := Sign(key, c); return s },
		"wrong issuer": func() string { c := good; c.Iss = "https://evil"; s, _ := Sign(key, c); return s },
		"no role":      func() string { c := good; c.Role = ""; s, _ := Sign(key, c); return s },
		"alg none": func() string {
			p := strings.Split(tok, ".")
			return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." + p[1] + "."
		},
		"edited payload": func() string {
			p := strings.Split(tok, ".")
			c := strings.Replace(string(mustDecode(p[1])), `"ADMIN"`, `"ADMIM"`, 1)
			return p[0] + "." + base64.RawURLEncoding.EncodeToString([]byte(c)) + "." + p[2]
		},
		"garbage": func() string { return "abc" },
	}
	for name, mk := range bad {
		if _, err := v.Verify(mk()); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// 30 s past exp is still inside the 60 s clock skew.
	c := good
	c.Exp = now.Add(-30 * time.Second).Unix()
	s, _ := Sign(key, c)
	if _, err := v.Verify(s); err != nil {
		t.Errorf("skew not applied: %v", err)
	}
}

func mustDecode(s string) []byte { b, _ := base64.RawURLEncoding.DecodeString(s); return b }
