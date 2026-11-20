package tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// computeWebhookSignature produces the canonical SwiftDrop signature format:
// "t=<unix_timestamp>,v1=<hex_hmac>" signing "{timestamp}.{body}".
func computeWebhookSignature(timestamp int64, body []byte, secret []byte) string {
	payload := fmt.Sprintf("%d.%s", timestamp, string(body))
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	sigHex := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, sigHex)
}

// verifyWebhookSignature parses and validates the signature header against the payload and secret.
func verifyWebhookSignature(body []byte, sigHeader string, secret []byte, tolerance time.Duration) bool {
	if sigHeader == "" || len(secret) == 0 {
		return false
	}

	parts := strings.Split(sigHeader, ",")
	var tsStr string
	var signatures []string

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "t=") {
			tsStr = strings.TrimPrefix(p, "t=")
		} else if strings.HasPrefix(p, "v1=") {
			signatures = append(signatures, strings.TrimPrefix(p, "v1="))
		}
	}

	if tsStr == "" || len(signatures) == 0 {
		return false
	}

	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return false
	}

	// Check tolerance against replay attacks
	if tolerance > 0 {
		now := time.Now().Unix()
		diff := now - ts
		if diff < 0 {
			diff = -diff
		}
		if time.Duration(diff)*time.Second > tolerance {
			return false
		}
	}

	expectedSig := computeWebhookSignature(ts, body, secret)
	expectedHex := strings.TrimPrefix(strings.Split(expectedSig, ",")[1], "v1=")

	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expectedHex)) {
			return true
		}
	}
	return false
}

// mockWebhookDispatcher simulates the dispatch loop with retry backoff and DLQ routing.
type mockWebhookDispatcher struct {
	maxAttempts int
	backoffs    []time.Duration
}

type deliveryAttemptResult struct {
	AttemptNumber int
	StatusCode    int
	Status        string // SUCCESS, RETRYING, DLQ_EXHAUSTED
}

func (d *mockWebhookDispatcher) Dispatch(targetURL string, body []byte, secret []byte, client *http.Client) []deliveryAttemptResult {
	var history []deliveryAttemptResult

	for attempt := 1; attempt <= d.maxAttempts; attempt++ {
		req, _ := http.NewRequest(http.MethodPost, targetURL, strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-SwiftDrop-Signature", computeWebhookSignature(time.Now().Unix(), body, secret))

		resp, err := client.Do(req)
		statusCode := 0
		if err == nil {
			statusCode = resp.StatusCode
			_ = resp.Body.Close()
		}

		if statusCode == http.StatusOK || statusCode == http.StatusNoContent {
			history = append(history, deliveryAttemptResult{
				AttemptNumber: attempt,
				StatusCode:    statusCode,
				Status:        "SUCCESS",
			})
			return history
		}

		// Failure: calculate exponential backoff (10ms * 2^(attempt-1))
		backoff := time.Duration(10*(1<<(attempt-1))) * time.Millisecond
		d.backoffs = append(d.backoffs, backoff)

		status := "RETRYING"
		if attempt == d.maxAttempts {
			status = "DLQ_EXHAUSTED"
		}
		history = append(history, deliveryAttemptResult{
			AttemptNumber: attempt,
			StatusCode:    statusCode,
			Status:        status,
		})
	}
	return history
}

func TestAcceptance_AT012_WebhookRetryAndChaos(t *testing.T) {
	secret := []byte("whsec_01HMERCHANTTESTSECRETKEY32CH")
	payload := []byte(`{"event":"parcel.delivered.v1","tracking_number":"SD-20261114-X9K1","status":"DELIVERED"}`)

	t.Run("1. Canonical HMAC-SHA256 Signature Verification", func(t *testing.T) {
		ts := time.Now().Unix()
		sig := computeWebhookSignature(ts, payload, secret)

		require.True(t, strings.HasPrefix(sig, "t="))
		require.Contains(t, sig, ",v1=")

		// Valid signature passes
		assert.True(t, verifyWebhookSignature(payload, sig, secret, 5*time.Minute))

		// Tampered body fails
		tamperedBody := []byte(`{"event":"parcel.delivered.v1","status":"FAILED"}`)
		assert.False(t, verifyWebhookSignature(tamperedBody, sig, secret, 5*time.Minute))

		// Incorrect secret fails
		wrongSecret := []byte("whsec_wrong_secret_key_32chars_long")
		assert.False(t, verifyWebhookSignature(payload, sig, wrongSecret, 5*time.Minute))

		// Expired timestamp fails tolerance check
		expiredSig := computeWebhookSignature(time.Now().Add(-10*time.Minute).Unix(), payload, secret)
		assert.False(t, verifyWebhookSignature(payload, expiredSig, secret, 5*time.Minute))
	})

	t.Run("2. Chaos Mode: Transient 500 Error Retried with Backoff and Succeeds", func(t *testing.T) {
		attemptsReceived := 0

		// Receiver simulates 2 failures then 1 success
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attemptsReceived++
			sig := r.Header.Get("X-SwiftDrop-Signature")
			if !verifyWebhookSignature(payload, sig, secret, 5*time.Minute) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if attemptsReceived < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		dispatcher := &mockWebhookDispatcher{maxAttempts: 5}
		history := dispatcher.Dispatch(srv.URL, payload, secret, srv.Client())

		require.Len(t, history, 3, "expected exactly 3 attempts before success")
		assert.Equal(t, "RETRYING", history[0].Status)
		assert.Equal(t, http.StatusInternalServerError, history[0].StatusCode)

		assert.Equal(t, "RETRYING", history[1].Status)
		assert.Equal(t, http.StatusInternalServerError, history[1].StatusCode)

		assert.Equal(t, "SUCCESS", history[2].Status)
		assert.Equal(t, http.StatusOK, history[2].StatusCode)

		// Verify exponential backoff intervals
		require.Len(t, dispatcher.backoffs, 2)
		assert.Equal(t, 10*time.Millisecond, dispatcher.backoffs[0])
		assert.Equal(t, 20*time.Millisecond, dispatcher.backoffs[1])
	})

	t.Run("3. Persistent Failure Escalates to Dead-Letter Queue (DLQ)", func(t *testing.T) {
		// Receiver persistently fails with 500 Internal Server Error
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		dispatcher := &mockWebhookDispatcher{maxAttempts: 5}
		history := dispatcher.Dispatch(srv.URL, payload, secret, srv.Client())

		require.Len(t, history, 5, "expected 5 attempts before exhaustion")
		lastAttempt := history[4]

		assert.Equal(t, 5, lastAttempt.AttemptNumber)
		assert.Equal(t, http.StatusInternalServerError, lastAttempt.StatusCode)
		assert.Equal(t, "DLQ_EXHAUSTED", lastAttempt.Status, "final failed attempt must route to DLQ")
	})
}
