package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"kasun.perera@example.lk", "k***@example.lk"},
		{"jane.doe@vendor.com", "j***@vendor.com"},
		{"a@example.com", "*@example.com"},
		{"", ""},
		{"invalid-email", "***"},
		{"@domain.com", "***"},
		{"user@", "***"},
	}

	for _, tc := range tests {
		got := MaskEmail(tc.input)
		if got != tc.expected {
			t.Errorf("MaskEmail(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestHostOnly(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://merchant-api.com/webhooks/orders", "merchant-api.com"},
		{"http://localhost:8080/hook?secret=sensitive_token&v=1", "localhost:8080"},
		{"https://user:password@gateway.acme.lk:443/api", "gateway.acme.lk:443"},
		{"", ""},
		{"not-a-valid-url-with-no-scheme", "not-a-valid-url-with-no-scheme"},
	}

	for _, tc := range tests {
		got := HostOnly(tc.input)
		if got != tc.expected {
			t.Errorf("HostOnly(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestLogger_NoLeakage(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(handler)

	rawURL := "https://merchant.com/hook?token=SUPER_SECRET_TOKEN"
	rawEmail := "customer.private@example.com"

	logger.Info(
		"outbound webhook dispatched",
		"host", HostOnly(rawURL),
		"recipient", MaskEmail(rawEmail),
		"delivery_id", "0192e4a1-0001-7abc-8def-0123456789ab",
		"event_id", "0192e4a1-0002-7abc-8def-0123456789ab",
	)

	logOutput := buf.String()

	// Verify logged data
	if !strings.Contains(logOutput, `"host":"merchant.com"`) {
		t.Errorf("expected host 'merchant.com' in log, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"recipient":"c***@example.com"`) {
		t.Errorf("expected masked email 'c***@example.com' in log, got: %s", logOutput)
	}

	// Verify no secrets or sensitive data leaked
	if strings.Contains(logOutput, "SUPER_SECRET_TOKEN") {
		t.Errorf("leak detected: secret token found in log output: %s", logOutput)
	}
	if strings.Contains(logOutput, "customer.private@example.com") {
		t.Errorf("leak detected: raw unmasked email found in log output: %s", logOutput)
	}
}
