package logging

import (
	"testing"
)

func TestSetupLogger(t *testing.T) {
	levels := []string{"debug", "DEBUG", "info", "warn", "warning", "error", "UNKNOWN"}
	for _, lvl := range levels {
		logger := SetupLogger(lvl)
		if logger == nil {
			t.Fatalf("expected non-nil logger for level %s", lvl)
		}
	}
}

func TestMaskURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "postgres://swiftdrop:secret123@localhost:5432/swiftdrop_db?sslmode=disable",
			expected: "postgres://swiftdrop:xxxxx@localhost:5432/swiftdrop_db?sslmode=disable",
		},
		{
			input:    "amqp://guest:secret@rabbitmq:5672/",
			expected: "amqp://guest:xxxxx@rabbitmq:5672/",
		},
		{
			input:    "redis://:redispass@localhost:6379/0",
			expected: "redis://:xxxxx@localhost:6379/0",
		},
		{
			input:    "http://localhost:8081",
			expected: "http://localhost:8081",
		},
		{
			input:    "",
			expected: "",
		},
		{
			input:    "://invalid-url",
			expected: "[unparseable-url]",
		},
	}

	for _, tt := range tests {
		got := MaskURL(tt.input)
		if got != tt.expected {
			t.Errorf("MaskURL(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}
