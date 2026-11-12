package config

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidConfig = errors.New("config: invalid configuration parameter")
)

// Config encapsulates runtime configuration for messaging daemons (Relay, Worker, Reconciler).
type Config struct {
	AppEnv                     string
	DatabaseURL                string
	ReconcilerDatabaseURL      string
	RabbitMQURL                string
	OutboxPollInterval         time.Duration
	OutboxBatchSize            int
	OutboxStaleSeconds         int
	OutboxLeaseSeconds         int
	WorkerConcurrency          int
	WebhookHTTPTimeout         time.Duration
	WebhookRetrySchedule       []time.Duration
	WebhookAllowPrivateHosts   bool
	WebhookSecretEncryptionKey string // 64 hex characters (32 bytes AES-256-GCM)
	DeliveryLeaseSeconds       int
	SMTPHost                   string
	SMTPPort                   int
	SMTPSecurity               string // "starttls", "tls", or "none"
	SMTPUsername               string
	SMTPPassword               string
	SMTPFromEmail              string
	SMTPFromName               string
	ProbePort                  int
	SwaggerEnabled             bool
	LogLevel                   string
}

// loadDotEnv attempts to read key-value pairs from local .env files if not already set in the environment.
func loadDotEnv() {
	candidates := []string{".env", "../.env", "../../.env"}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					val = strings.Trim(val, `"'`)
					if os.Getenv(key) == "" {
						_ = os.Setenv(key, val)
					}
				}
			}
			return
		}
	}
}

// LoadConfig parses environment variables into Config with fail-fast validation.
func LoadConfig(defaultProbePort int) (*Config, error) {
	loadDotEnv()
	appEnv := strings.ToLower(strings.TrimSpace(getEnv("APP_ENV", "local")))
	if appEnv == "" {
		appEnv = "local"
	}

	dbURL := getEnv("DATABASE_URL", "postgres://swiftdrop_user:swiftdrop_password@localhost:5432/swiftdrop_db?sslmode=disable")
	if _, err := url.Parse(dbURL); err != nil {
		return nil, fmt.Errorf("%w: invalid DATABASE_URL: %v", ErrInvalidConfig, err)
	}

	reconcilerDBURL := getEnv("RECONCILER_DATABASE_URL", dbURL)
	if _, err := url.Parse(reconcilerDBURL); err != nil {
		return nil, fmt.Errorf("%w: invalid RECONCILER_DATABASE_URL: %v", ErrInvalidConfig, err)
	}

	rabbitURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	if !strings.HasPrefix(rabbitURL, "amqp://") && !strings.HasPrefix(rabbitURL, "amqps://") {
		return nil, fmt.Errorf("%w: RABBITMQ_URL must begin with amqp:// or amqps://", ErrInvalidConfig)
	}

	pollIntervalMS := getEnvInt("OUTBOX_POLL_INTERVAL_MS", 500)
	if pollIntervalMS <= 0 {
		pollIntervalMS = 500
	}

	batchSize := getEnvInt("OUTBOX_BATCH_SIZE", 50)
	if batchSize <= 0 {
		batchSize = 50
	}

	staleSeconds := getEnvInt("OUTBOX_STALE_SECONDS", 300)
	if staleSeconds <= 0 {
		staleSeconds = 300
	}

	leaseSeconds := getEnvInt("OUTBOX_LEASE_SECONDS", 60)
	if leaseSeconds <= 0 {
		leaseSeconds = 60
	}

	workerConcurrency := getEnvInt("WORKER_CONCURRENCY", 10)
	if workerConcurrency <= 0 {
		workerConcurrency = 10
	}

	webhookTimeoutSec := getEnvInt("WEBHOOK_HTTP_TIMEOUT_SECONDS", 5)
	if webhookTimeoutSec <= 0 {
		webhookTimeoutSec = 5
	}

	scheduleStr := getEnv("WEBHOOK_RETRY_SCHEDULE", "0s,30s,2m,10m,30m")
	retrySchedule, err := parseSchedule(scheduleStr)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid WEBHOOK_RETRY_SCHEDULE: %v", ErrInvalidConfig, err)
	}

	allowPrivateHosts := getEnvBool("WEBHOOK_ALLOW_PRIVATE_HOSTS", false)

	// Encryption key validation: accepts 64 hex characters or base64 (32 bytes AES-256 key)
	encKey := getEnv("WEBHOOK_SECRET_ENCRYPTION_KEY", "")
	if encKey == "" {
		if appEnv == "production" {
			return nil, fmt.Errorf("%w: WEBHOOK_SECRET_ENCRYPTION_KEY is required in production", ErrInvalidConfig)
		}
		// Default development fallback key: 64 hex characters (32 bytes of 0x01)
		encKey = "0101010101010101010101010101010101010101010101010101010101010101"
	}
	if _, err := DecodeAES256Key(encKey); err != nil {
		return nil, fmt.Errorf("%w: invalid WEBHOOK_SECRET_ENCRYPTION_KEY: %v", ErrInvalidConfig, err)
	}

	deliveryLeaseSeconds := getEnvInt("DELIVERY_LEASE_SECONDS", 120)
	if deliveryLeaseSeconds <= 0 {
		deliveryLeaseSeconds = 120
	}

	smtpHost := getEnv("SMTP_HOST", "localhost")
	smtpPort := getEnvInt("SMTP_PORT", 1025)
	smtpSecurity := strings.ToLower(getEnv("SMTP_SECURITY", "none"))
	if smtpSecurity != "starttls" && smtpSecurity != "tls" && smtpSecurity != "none" {
		return nil, fmt.Errorf("%w: invalid SMTP_SECURITY (must be starttls, tls, or none): %s", ErrInvalidConfig, smtpSecurity)
	}
	if smtpSecurity == "none" && appEnv != "local" && appEnv != "test" {
		return nil, fmt.Errorf("%w: SMTP_SECURITY 'none' is only permitted in local/test environments", ErrInvalidConfig)
	}

	probePort := getEnvInt("PROBE_PORT", defaultProbePort)
	if probePort <= 0 {
		probePort = defaultProbePort
	}

	swaggerDefault := (appEnv == "local")
	swaggerEnabled := getEnvBool("SWAGGER_ENABLED", swaggerDefault)

	return &Config{
		AppEnv:                     appEnv,
		DatabaseURL:                dbURL,
		ReconcilerDatabaseURL:      reconcilerDBURL,
		RabbitMQURL:                rabbitURL,
		OutboxPollInterval:         time.Duration(pollIntervalMS) * time.Millisecond,
		OutboxBatchSize:            batchSize,
		OutboxStaleSeconds:         staleSeconds,
		OutboxLeaseSeconds:         leaseSeconds,
		WorkerConcurrency:          workerConcurrency,
		WebhookHTTPTimeout:         time.Duration(webhookTimeoutSec) * time.Second,
		WebhookRetrySchedule:       retrySchedule,
		WebhookAllowPrivateHosts:   allowPrivateHosts,
		WebhookSecretEncryptionKey: encKey,
		DeliveryLeaseSeconds:       deliveryLeaseSeconds,
		SMTPHost:                   smtpHost,
		SMTPPort:                   smtpPort,
		SMTPSecurity:               smtpSecurity,
		SMTPUsername:               getEnv("SMTP_USERNAME", ""),
		SMTPPassword:               getEnv("SMTP_PASSWORD", ""),
		SMTPFromEmail:              getEnv("SMTP_FROM_EMAIL", "notifications@swiftdrop.io"),
		SMTPFromName:               getEnv("SMTP_FROM_NAME", "SwiftDrop Deliveries"),
		ProbePort:                  probePort,
		SwaggerEnabled:             swaggerEnabled,
		LogLevel:                   getEnv("LOG_LEVEL", "INFO"),
	}, nil
}

func parseSchedule(s string) ([]time.Duration, error) {
	parts := strings.Split(s, ",")
	res := make([]time.Duration, len(parts))
	for i, part := range parts {
		d, err := time.ParseDuration(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		res[i] = d
	}
	return res, nil
}

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return def
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return parsed
}

func getEnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return def
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return parsed
}

// DecodeAES256Key decodes an AES-256 key from a 64-character hex string or a base64 string.
// It returns the 32-byte binary key or an error if invalid.
func DecodeAES256Key(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty encryption key")
	}

	// 1. Try 64-character Hex
	if len(raw) == 64 {
		if b, err := hex.DecodeString(raw); err == nil && len(b) == 32 {
			return b, nil
		}
	}

	// 2. Try Base64 variants (StdEncoding, RawStdEncoding, URLEncoding, RawURLEncoding)
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}

	// 3. Fallback hex decode (if hex was valid hex but not exactly 64 length)
	if b, err := hex.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}

	return nil, fmt.Errorf("encryption key must decode to exactly 32 bytes (AES-256) via 64-character hex or base64 (got length %d)", len(raw))
}

