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
	k, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Now()
	good := Claims{Iss: "iss", Sub: "u1", Role: "MERCHANT", TenantID: "m1", Exp: now.Add(time.Minute).Unix(), Nbf: now.Unix()}
	v := NewVerifier(&k.PublicKey, "iss")

	tok, _ := Sign(k, good)
	if c, err := v.Verify(tok); err != nil || c.TenantID != "m1" {
		t.Fatalf("good token rejected: %v", err)
	}
	expired := good
	expired.Exp = now.Add(-2 * time.Minute).Unix()
	wrongIss := good
	wrongIss.Iss = "evil"
	noRole := good
	noRole.Role = ""
	for name, c := range map[string]Claims{"expired": expired, "wrong iss": wrongIss, "no role": noRole} {
		tk, _ := Sign(k, c)
		if _, err := v.Verify(tk); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	forged, _ := Sign(other, good)
	if _, err := v.Verify(forged); err == nil {
		t.Error("token signed by other key accepted")
	}
	parts := strings.Split(tok, ".")
	none := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." + parts[1] + "."
	if _, err := v.Verify(none); err == nil {
		t.Error("alg=none accepted")
	}
}
