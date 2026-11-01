package webhook

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/messaging/internal/config"
	"github.com/swiftdrop/messaging/internal/delivery"
)

// TestSignature_KnownAnswerVector verifies deterministic HMAC-SHA256 signature computation.
func TestSignature_KnownAnswerVector(t *testing.T) {
	secret := []byte("whsec_test_secret_for_local_merchant_123")
	timestamp := int64(1729512000)
	body := []byte(`{"event_id":"0192e2b3-7c8a-7890-a123-000000000001","status":"DELIVERED"}`)

	sig := Sign(timestamp, body, secret)

	// Verify header format: t=1729512000,v1=<hex>
	if !strings.HasPrefix(sig, "t=1729512000,v1=") {
		t.Fatalf("unexpected signature header format: %s", sig)
	}

	// Verify signature validates
	if !VerifySignature(body, sig, secret, 0) {
		t.Fatalf("signature verification failed for valid header: %s", sig)
	}

	// Tampered body must fail
	tamperedBody := []byte(`{"event_id":"0192e2b3-7c8a-7890-a123-000000000001","status":"CANCELLED"}`)
	if VerifySignature(tamperedBody, sig, secret, 0) {
		t.Fatalf("expected verification to fail for tampered body")
	}

	// Wrong secret must fail
	wrongSecret := []byte("whsec_wrong_secret_1234567890123456")
	if VerifySignature(body, sig, wrongSecret, 0) {
		t.Fatalf("expected verification to fail for wrong secret")
	}
}

// TestDecryptor tests AES-256-GCM decryption, tampering detection, and AAD checks.
func TestDecryptor(t *testing.T) {
	// 32-byte key (64 hex characters)
	keyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	decryptor, err := NewDecryptor(keyHex)
	if err != nil {
		t.Fatalf("failed to create decryptor: %v", err)
	}

	endpointID := "0192e2b3-7c8a-7890-a123-000000000088"
	plaintextSecret := []byte("whsec_super_secret_merchant_key_2026")

	// Helper to encrypt
	keyBytes, _ := hex.DecodeString(keyHex)
	block, _ := aes.NewCipher(keyBytes)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	_, _ = rand.Read(nonce)
	ciphertextAndTag := gcm.Seal(nil, nonce, plaintextSecret, []byte(endpointID))
	validEncrypted := append(nonce, ciphertextAndTag...)

	t.Run("Valid decryption succeeds", func(t *testing.T) {
		decrypted, err := decryptor.Decrypt(validEncrypted, endpointID, 1)
		if err != nil {
			t.Fatalf("unexpected decryption error: %v", err)
		}
		if string(decrypted) != string(plaintextSecret) {
			t.Errorf("expected %s, got %s", plaintextSecret, decrypted)
		}
	})

	t.Run("Ciphertext too short (< 28 bytes) fails", func(t *testing.T) {
		shortData := []byte("short-less-than-28-bytes")
		_, err := decryptor.Decrypt(shortData, endpointID, 1)
		if err != ErrCiphertextTooShort {
			t.Errorf("expected ErrCiphertextTooShort, got %v", err)
		}
	})

	t.Run("Wrong AAD (different endpoint_id) fails", func(t *testing.T) {
		_, err := decryptor.Decrypt(validEncrypted, "wrong-endpoint-id", 1)
		if err != ErrDecryptFailed {
			t.Errorf("expected ErrDecryptFailed on wrong AAD, got %v", err)
		}
	})

	t.Run("Tampered tag fails", func(t *testing.T) {
		tampered := make([]byte, len(validEncrypted))
		copy(tampered, validEncrypted)
		tampered[len(tampered)-1] ^= 0xff // flip last byte of tag
		_, err := decryptor.Decrypt(tampered, endpointID, 1)
		if err != ErrDecryptFailed {
			t.Errorf("expected ErrDecryptFailed on tampered tag, got %v", err)
		}
	})

	t.Run("Unavailable key version fails", func(t *testing.T) {
		_, err := decryptor.Decrypt(validEncrypted, endpointID, 99)
		if err == nil || !strings.Contains(err.Error(), "encryption key unavailable") {
			t.Errorf("expected key unavailable error, got %v", err)
		}
	})
}

// fakeResolver mocks DNS resolution for testing SSRF hostname blocking.
type fakeResolver struct {
	mapping map[string][]net.IPAddr
}

func (f *fakeResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	if ips, ok := f.mapping[host]; ok {
		return ips, nil
	}
	return nil, fmt.Errorf("no such host: %s", host)
}

func TestSSRFTable(t *testing.T) {
	testCases := []struct {
		name      string
		targetIP  string
		isBlocked bool
	}{
		{"IPv4 Loopback 127.0.0.1", "127.0.0.1", true},
		{"IPv4 Loopback 127.1.2.3", "127.1.2.3", true},
		{"RFC 1918 10.0.0.1", "10.0.0.1", true},
		{"RFC 1918 172.16.0.1", "172.16.0.1", true},
		{"RFC 1918 192.168.1.1", "192.168.1.1", true},
		{"Cloud Metadata 169.254.169.254", "169.254.169.254", true},
		{"Carrier Grade NAT 100.64.0.1", "100.64.0.1", true},
		{"Current Network 0.0.0.0", "0.0.0.0", true},
		{"IPv6 Loopback ::1", "::1", true},
		{"IPv6 ULA fc00::1", "fc00::1", true},
		{"IPv6 Link Local fe80::1", "fe80::1", true},
		{"IPv4-mapped IPv6 ::ffff:10.0.0.1", "::ffff:10.0.0.1", true},
		{"Public IP 8.8.8.8", "8.8.8.8", false},
		{"Public IP 1.1.1.1", "1.1.1.1", false},
		{"Public IP 93.184.216.34", "93.184.216.34", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.targetIP)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tc.targetIP)
			}
			blocked := IsPrivateOrBlockedIP(ip)
			if blocked != tc.isBlocked {
				t.Errorf("for IP %s expected blocked=%v, got %v", tc.targetIP, tc.isBlocked, blocked)
			}
		})
	}
}

func TestSSRF_HostnameResolvingToPrivateBlocked(t *testing.T) {
	resolver := &fakeResolver{
		mapping: map[string][]net.IPAddr{
			"internal.merchant.com": {
				{IP: net.ParseIP("10.0.0.5")},
			},
		},
	}

	client := NewSafeClient(2*time.Second, false, resolver)
	res := client.Do(context.Background(), "https://internal.merchant.com/webhook", nil, []byte(`{}`))

	if !res.IsTerminal {
		t.Errorf("expected terminal failure for SSRF blocked host")
	}
	if res.ErrorClass != "SSRF_BLOCKED" {
		t.Errorf("expected SSRF_BLOCKED error class, got %s", res.ErrorClass)
	}
}

func TestSafeClient_RedirectNotFollowed(t *testing.T) {
	// Create server that returns 302 redirect
	redirectSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer redirectSrv.Close()

	// allowPrivateHosts = true so it connects to local test server
	client := NewSafeClient(2*time.Second, true, nil)
	res := client.Do(context.Background(), redirectSrv.URL, nil, []byte(`{}`))

	// Should stop at 302 and NOT follow to metadata IP
	if res.StatusCode != http.StatusFound {
		t.Errorf("expected status code 302 Found without following, got %d", res.StatusCode)
	}
	if !res.IsTerminal {
		t.Errorf("expected 302 to be classified as terminal")
	}
}

func TestSafeClient_OutcomeMatrix(t *testing.T) {
	cases := []struct {
		statusCode  int
		isSuccess   bool
		isRetryable bool
		isTerminal  bool
	}{
		{200, true, false, false},
		{204, true, false, false},
		{301, false, false, true},
		{400, false, false, true},
		{401, false, false, true},
		{403, false, false, true},
		{404, false, false, true},
		{408, false, true, false},
		{410, false, false, true}, // 410 Gone explicitly terminal
		{422, false, false, true},
		{429, false, true, false}, // Rate limit retryable
		{500, false, true, false}, // 500 retryable
		{502, false, true, false},
		{503, false, true, false},
		{504, false, true, false},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("HTTP_%d", tc.statusCode), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(`{"test":"outcome"}`))
			}))
			defer srv.Close()

			client := NewSafeClient(2*time.Second, true, nil)
			res := client.Do(context.Background(), srv.URL, nil, []byte(`{}`))

			if res.IsSuccess != tc.isSuccess {
				t.Errorf("status %d: expected isSuccess=%v, got %v", tc.statusCode, tc.isSuccess, res.IsSuccess)
			}
			if res.IsRetryable != tc.isRetryable {
				t.Errorf("status %d: expected isRetryable=%v, got %v", tc.statusCode, tc.isRetryable, res.IsRetryable)
			}
			if res.IsTerminal != tc.isTerminal {
				t.Errorf("status %d: expected isTerminal=%v, got %v", tc.statusCode, tc.isTerminal, res.IsTerminal)
			}
		})
	}
}

func TestWebhookDispatcher_Success(t *testing.T) {
	secret := []byte("test_secret_for_dispatcher")
	keyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	decryptor, _ := NewDecryptor(keyHex)

	endpointID := "0192e2b3-7c8a-7890-a123-000000000088"
	keyBytes, _ := hex.DecodeString(keyHex)
	block, _ := aes.NewCipher(keyBytes)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	_, _ = rand.Read(nonce)
	cipherAndTag := gcm.Seal(nil, nonce, secret, []byte(endpointID))
	encryptedSecret := append(nonce, cipherAndTag...)

	repo := newMockWebhookRepo()
	repo.target = &delivery.MerchantWebhookTarget{
		EndpointID:       endpointID,
		MerchantID:       "0192e2b3-7c8a-7890-a123-000000000042",
		URL:              "https://merchant.example.com/webhook",
		SecretCiphertext: encryptedSecret,
		SecretKeyVersion: 1,
	}

	repo.tasks = append(repo.tasks, delivery.WebhookTask{
		ID:               "del-1",
		EndpointID:       endpointID,
		EventID:          "0192e2b3-7c8a-7890-a123-000000000001",
		EventType:        "parcel.booked.v1",
		AggregateID:      "0192e2b3-7c8a-7890-a123-000000000010",
		AggregateVersion: 1,
		OccurredAt:       time.Now(),
		Payload:          []byte(`{"tracking_number":"SD-001","status":"BOOKED"}`),
		AttemptCount:     1,
	})

	mockClient := &mockHTTPClient{
		result: DispatchResult{
			StatusCode: 200,
			Snippet:    `{"status":"ok"}`,
			IsSuccess:  true,
		},
	}

	cfg := &config.Config{
		DeliveryLeaseSeconds: 60,
		WebhookHTTPTimeout:   5 * time.Second,
	}

	calc := delivery.NewBackoffCalculator(delivery.DefaultRetrySchedule, nil)
	dispatcher := NewDispatcher(repo, mockClient, decryptor, calc, cfg, nil)

	count, err := dispatcher.PollAndDispatch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 task dispatched, got %d", count)
	}

	if repo.successID != "del-1" {
		t.Errorf("expected success recorded for del-1, got %s", repo.successID)
	}
}

type mockHTTPClient struct {
	result DispatchResult
}

func (m *mockHTTPClient) Do(ctx context.Context, targetURL string, headers map[string]string, body []byte) DispatchResult {
	return m.result
}

type mockWebhookRepo struct {
	delivery.Repository
	tasks     []delivery.WebhookTask
	target    *delivery.MerchantWebhookTarget
	successID string
	retryID   string
	failID    string
}

func newMockWebhookRepo() *mockWebhookRepo {
	return &mockWebhookRepo{}
}

func (m *mockWebhookRepo) ClaimWebhooks(ctx context.Context, limit int, leaseDuration time.Duration) ([]delivery.WebhookTask, error) {
	res := m.tasks
	m.tasks = nil
	return res, nil
}

func (m *mockWebhookRepo) GetEndpointByID(ctx context.Context, endpointID string) (*delivery.MerchantWebhookTarget, error) {
	return m.target, nil
}

func (m *mockWebhookRepo) RecordWebhookSuccess(ctx context.Context, deliveryID, endpointID, eventID string, attemptNo int, statusCode int, snippet string) error {
	m.successID = deliveryID
	return nil
}

func (m *mockWebhookRepo) RecordWebhookRetry(ctx context.Context, deliveryID string, attemptNo int, statusCode int, snippet, errorClass, errorMsg string, nextAttemptAt time.Time) error {
	m.retryID = deliveryID
	return nil
}

func (m *mockWebhookRepo) RecordWebhookTerminalFailure(ctx context.Context, deliveryID, endpointID, eventID string, attemptNo int, statusCode int, snippet, errorClass, errorMsg string) error {
	m.failID = deliveryID
	return nil
}
