package testutil

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/swiftdrop/tracking/internal/auth"
)

// GenerateTestToken creates a signed JWT for testing purposes only.
// This function is strictly scoped to automated test doubles and unit/integration test suites.
// It is located in internal/testutil to ensure it is never invoked or referenced by production runtime code.
func GenerateTestToken(secret string, claims auth.JWTClaims, overrideAlg ...string) (string, error) {
	alg := "HS256"
	if len(overrideAlg) > 0 && overrideAlg[0] != "" {
		alg = overrideAlg[0]
	}

	header := map[string]string{
		"alg": alg,
		"typ": "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64

	if strings.EqualFold(alg, "none") {
		return signingInput + ".", nil
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := mac.Sum(nil)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + sigB64, nil
}
