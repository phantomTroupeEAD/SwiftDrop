// Package auth verifies RS256 access tokens (ADR-003 section 3). This is a stand-in
// for Member 1's middleware so the admin module can run on its own; the real one
// lives in core-api/internal/platform/auth/ and replaces this file.
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
	"net/http"
	"strings"
	"time"

	"swiftdrop.local/m7/core-api/internal/platform/web"
)

const clockSkew = 60 * time.Second // ADR-003 3.1

var errToken = errors.New("invalid token")

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

type Verifier struct {
	key    *rsa.PublicKey
	issuer string
	now    func() time.Time
}

func ParsePublicKey(pemBytes []byte) (*rsa.PublicKey, error) {
	b, _ := pem.Decode(pemBytes)
	if b == nil {
		return nil, errors.New("JWT_PUBLIC_KEY_PEM is not PEM")
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

func NewVerifier(key *rsa.PublicKey, issuer string) *Verifier {
	return &Verifier{key: key, issuer: issuer, now: time.Now}
}

// Verify checks signature, alg, iss, exp, nbf and required claims.
func (v *Verifier) Verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errToken
	}
	var hdr struct {
		Alg string `json:"alg"`
	}
	if err := decode(parts[0], &hdr); err != nil || hdr.Alg != "RS256" {
		return Claims{}, errToken // refuses "none" and HS256 key-confusion tricks
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, errToken
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(v.key, crypto.SHA256, sum[:], sig) != nil {
		return Claims{}, errToken
	}
	var c Claims
	if err := decode(parts[1], &c); err != nil {
		return Claims{}, errToken
	}
	now := v.now()
	if c.Iss != v.issuer || c.Sub == "" || c.Role == "" ||
		now.After(time.Unix(c.Exp, 0).Add(clockSkew)) || now.Add(clockSkew).Before(time.Unix(c.Nbf, 0)) {
		return Claims{}, errToken
	}
	return c, nil
}

func decode(seg string, out any) error {
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// Middleware puts the verified principal into the context. A missing or bad token
// leaves the context empty; RequireRole then answers 401.
func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			if c, err := v.Verify(strings.TrimPrefix(h, "Bearer ")); err == nil {
				r = r.WithContext(web.WithPrincipal(r.Context(), web.Principal{UserID: c.Sub, Role: c.Role, TenantID: c.TenantID}))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Sign makes a token. Used only by cmd/devtoken and tests, never by the server.
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
