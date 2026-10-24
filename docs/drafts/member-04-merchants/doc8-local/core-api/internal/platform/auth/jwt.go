// Package auth verifies RS256 access tokens (ADR-003 §3). STAND-IN for Member 1's
// middleware so the merchants module can run on its own; replaced at integration.
package auth

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"time"
)

const clockSkew = 60 * time.Second // ADR-003 §3.1

var ErrToken = errors.New("invalid token")

type Claims struct {
	Iss      string `json:"iss"`
	Sub      string `json:"sub"`
	Role     string `json:"role"`
	TenantID string `json:"tenant_id,omitempty"`
	Exp      int64  `json:"exp"`
	Nbf      int64  `json:"nbf"`
	Iat      int64  `json:"iat"`
	Jti      string `json:"jti"`
}

func ParsePublicKey(pemBytes []byte) (*rsa.PublicKey, error) {
	b, _ := pem.Decode(pemBytes)
	if b == nil {
		return nil, errors.New("JWT public key is not PEM")
	}
	k, err := x509.ParsePKIXPublicKey(b.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := k.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("JWT public key is not RSA")
	}
	return rk, nil
}

type Verifier struct {
	key    *rsa.PublicKey
	issuer string
	now    func() time.Time
}

func NewVerifier(key *rsa.PublicKey, issuer string) *Verifier {
	return &Verifier{key: key, issuer: issuer, now: time.Now}
}

// Verify checks alg=RS256 (refuses "none"/HS256), signature, iss, exp, nbf, sub, role.
func (v *Verifier) Verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrToken
	}
	var hdr struct {
		Alg string `json:"alg"`
	}
	if err := decode(parts[0], &hdr); err != nil || hdr.Alg != "RS256" {
		return Claims{}, ErrToken
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrToken
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(v.key, crypto.SHA256, sum[:], sig) != nil {
		return Claims{}, ErrToken
	}
	var c Claims
	if err := decode(parts[1], &c); err != nil {
		return Claims{}, ErrToken
	}
	now := v.now()
	switch {
	case c.Iss != v.issuer,
		c.Sub == "" || c.Role == "",
		now.After(time.Unix(c.Exp, 0).Add(clockSkew)),
		now.Add(clockSkew).Before(time.Unix(c.Nbf, 0)):
		return Claims{}, ErrToken
	}
	return c, nil
}

// Sign creates an RS256 token. Used only by cmd/devtoken and tests.
func Sign(key *rsa.PrivateKey, c Claims) (string, error) {
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	p, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	in := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(p)
	sum := sha256.Sum256([]byte(in))
	sig, err := rsa.SignPKCS1v15(nil, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return in + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func decode(seg string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
