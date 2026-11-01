package config

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear relevant env vars
	os.Unsetenv("APP_ENV")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("RABBITMQ_URL")
	os.Unsetenv("WEBHOOK_SECRET_ENCRYPTION_KEY")
	os.Unsetenv("SMTP_SECURITY")

	cfg, err := LoadConfig(8082)
	if err != nil {
		t.Fatalf("expected valid default config, got: %v", err)
	}

	if cfg.AppEnv != "local" {
		t.Errorf("expected AppEnv local, got %s", cfg.AppEnv)
	}
	if cfg.OutboxPollInterval != 500*time.Millisecond {
		t.Errorf("expected 500ms poll interval, got %v", cfg.OutboxPollInterval)
	}
	if cfg.OutboxBatchSize != 50 {
		t.Errorf("expected batch size 50, got %d", cfg.OutboxBatchSize)
	}
	if cfg.WorkerConcurrency != 10 {
		t.Errorf("expected worker concurrency 10, got %d", cfg.WorkerConcurrency)
	}
	if cfg.WebhookHTTPTimeout != 5*time.Second {
		t.Errorf("expected 5s webhook timeout, got %v", cfg.WebhookHTTPTimeout)
	}
	if len(cfg.WebhookRetrySchedule) != 5 {
		t.Errorf("expected 5 retry schedule steps, got %d", len(cfg.WebhookRetrySchedule))
	}
	if cfg.ProbePort != 8082 {
		t.Errorf("expected probe port 8082, got %d", cfg.ProbePort)
	}
}

func TestLoadConfig_BadRabbitURL(t *testing.T) {
	os.Setenv("RABBITMQ_URL", "http://bad-protocol:5672")
	defer os.Unsetenv("RABBITMQ_URL")

	_, err := LoadConfig(8082)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for bad rabbit URL, got: %v", err)
	}
}

func TestLoadConfig_EncryptionKeyValidation(t *testing.T) {
	// 1. Invalid length (less than 64 hex chars)
	os.Setenv("WEBHOOK_SECRET_ENCRYPTION_KEY", "abc123")
	_, err := LoadConfig(8082)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for short key, got: %v", err)
	}

	// 2. Non-hex characters
	badHex := "01010101010101010101010101010101010101010101010101010101010101zz"
	os.Setenv("WEBHOOK_SECRET_ENCRYPTION_KEY", badHex)
	_, err = LoadConfig(8082)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for non-hex key, got: %v", err)
	}

	// 3. Valid 64-hex key
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	os.Setenv("WEBHOOK_SECRET_ENCRYPTION_KEY", validKey)
	cfg, err := LoadConfig(8082)
	if err != nil {
		t.Fatalf("expected success with valid key, got: %v", err)
	}
	if cfg.WebhookSecretEncryptionKey != validKey {
		t.Errorf("expected key %s, got %s", validKey, cfg.WebhookSecretEncryptionKey)
	}
}

func TestLoadConfig_SMTPSecurityRules(t *testing.T) {
	// SMTP none forbidden in production
	os.Setenv("APP_ENV", "production")
	os.Setenv("WEBHOOK_SECRET_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	os.Setenv("SMTP_SECURITY", "none")
	defer func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("WEBHOOK_SECRET_ENCRYPTION_KEY")
		os.Unsetenv("SMTP_SECURITY")
	}()

	_, err := LoadConfig(8082)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for SMTP none in production, got: %v", err)
	}
}
