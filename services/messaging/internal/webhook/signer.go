package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Sign computes the canonical HMAC-SHA256 signature header for an outbound webhook payload.
// Format: t=<unix>,v1=<hex_signature>
// Signed String: "{timestamp}.{body}"
func Sign(timestamp int64, body []byte, secret []byte) string {
	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(body))

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signedPayload))
	sigHex := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("t=%d,v1=%s", timestamp, sigHex)
}

// VerifySignature validates a received webhook signature header against the payload and secret.
// Supports multi-v1 headers during secret rotation (e.g. t=...,v1=sig1,v1=sig2).
func VerifySignature(body []byte, signatureHeader string, secret []byte, tolerance time.Duration) bool {
	if signatureHeader == "" || len(secret) == 0 {
		return false
	}

	parts := strings.Split(signatureHeader, ",")
	var timestampStr string
	var signatures []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "t=") {
			timestampStr = strings.TrimPrefix(part, "t=")
		} else if strings.HasPrefix(part, "v1=") {
			signatures = append(signatures, strings.TrimPrefix(part, "v1="))
		}
	}

	if timestampStr == "" || len(signatures) == 0 {
		return false
	}

	ts, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return false
	}

	// Verify timestamp tolerance if specified
	if tolerance > 0 {
		msgTime := time.Unix(ts, 0)
		diff := time.Since(msgTime)
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			return false
		}
	}

	// Calculate expected signature
	signedPayload := fmt.Sprintf("%d.%s", ts, string(body))
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signedPayload))
	expectedSig := mac.Sum(nil)

	// Check if any v1 signature matches
	for _, sigHex := range signatures {
		sigBytes, err := hex.DecodeString(sigHex)
		if err != nil {
			continue
		}
		if hmac.Equal(expectedSig, sigBytes) {
			return true
		}
	}

	return false
}
