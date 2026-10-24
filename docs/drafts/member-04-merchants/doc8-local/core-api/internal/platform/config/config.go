// Package config loads and validates environment configuration (Doc 8 §3).
// The process refuses to start on any problem and lists every problem at once,
// naming variables but never printing their values.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv          string
	HTTPAddr        string
	DatabaseURL     string
	LogLevel        slog.Level
	JWTIssuer       string
	JWTPublicKeyPEM []byte

	KeyEnvPrefix  string // sd_live_ or sd_test_
	Pepper        []byte
	PepperVersion int
	EncKey        []byte // 32 bytes for AES-256-GCM
	EncKeyVersion int

	WebhookAllowHTTP           bool
	WebhookAllowedPrivateHosts []string // local only, e.g. "webhook-sink"
	MaxActiveKeys              int
	MaxWebhooks                int

	ShutdownDrain   time.Duration
	ShutdownTimeout time.Duration
}

func (c Config) IsLocal() bool { return c.AppEnv == "local" }

// Load reads configuration through getenv (os.Getenv in production, a map in tests).
func Load(getenv func(string) string) (Config, error) {
	var problems []string
	bad := func(name, why string) { problems = append(problems, name+": "+why) }
	get := func(name, def string) string {
		if v := strings.TrimSpace(getenv(name)); v != "" {
			return v
		}
		return def
	}
	intVar := func(name string, def, min, max int) int {
		s := get(name, strconv.Itoa(def))
		n, err := strconv.Atoi(s)
		if err != nil || n < min || n > max {
			bad(name, fmt.Sprintf("must be an integer %d-%d", min, max))
			return def
		}
		return n
	}

	c := Config{}
	c.AppEnv = get("APP_ENV", "")
	switch c.AppEnv {
	case "local", "development", "staging", "production":
	default:
		bad("APP_ENV", "must be local, development, staging or production")
	}
	c.HTTPAddr = get("HTTP_ADDR", ":8080")
	c.DatabaseURL = get("DATABASE_URL", "")
	if c.DatabaseURL == "" {
		bad("DATABASE_URL", "is required")
	}
	if err := c.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		bad("LOG_LEVEL", "must be debug, info, warn or error")
	}
	c.JWTIssuer = get("JWT_ISSUER", "")
	if c.JWTIssuer == "" {
		bad("JWT_ISSUER", "is required")
	}
	if pemv := get("JWT_PUBLIC_KEY_PEM", ""); pemv == "" {
		bad("JWT_PUBLIC_KEY_PEM", "is required (PEM text or file path)")
	} else if strings.HasPrefix(pemv, "-----BEGIN") {
		c.JWTPublicKeyPEM = []byte(pemv)
	} else if b, err := os.ReadFile(pemv); err != nil {
		bad("JWT_PUBLIC_KEY_PEM", "file cannot be read")
	} else {
		c.JWTPublicKeyPEM = b
	}

	defPrefix := "sd_test_"
	if c.AppEnv == "production" {
		defPrefix = "sd_live_"
	}
	c.KeyEnvPrefix = get("MERCHANT_API_KEY_PREFIX", defPrefix)
	switch {
	case c.KeyEnvPrefix != "sd_live_" && c.KeyEnvPrefix != "sd_test_":
		bad("MERCHANT_API_KEY_PREFIX", "must be sd_live_ or sd_test_")
	case c.AppEnv == "production" && c.KeyEnvPrefix != "sd_live_":
		bad("MERCHANT_API_KEY_PREFIX", "must be sd_live_ in production")
	}

	if p := getenv("MERCHANT_API_KEY_PEPPER"); len(p) < 32 {
		bad("MERCHANT_API_KEY_PEPPER", "is required, at least 32 characters")
	} else {
		c.Pepper = []byte(p)
	}
	c.PepperVersion = intVar("MERCHANT_API_KEY_PEPPER_VERSION", 1, 1, 32767)

	if k := getenv("WEBHOOK_SECRET_ENCRYPTION_KEY"); k == "" {
		bad("WEBHOOK_SECRET_ENCRYPTION_KEY", "is required (base64 of 32 random bytes)")
	} else if b, err := base64.StdEncoding.DecodeString(k); err != nil || len(b) != 32 {
		bad("WEBHOOK_SECRET_ENCRYPTION_KEY", "must be base64 of exactly 32 bytes")
	} else {
		c.EncKey = b
	}
	c.EncKeyVersion = intVar("WEBHOOK_SECRET_KEY_VERSION", 1, 1, 32767)

	c.WebhookAllowHTTP = get("WEBHOOK_ALLOW_HTTP", "false") == "true"
	if c.WebhookAllowHTTP && !c.IsLocal() {
		bad("WEBHOOK_ALLOW_HTTP", "may be true only when APP_ENV=local")
	}
	if hosts := get("WEBHOOK_ALLOWED_PRIVATE_HOSTS", ""); hosts != "" {
		if !c.IsLocal() {
			bad("WEBHOOK_ALLOWED_PRIVATE_HOSTS", "may be set only when APP_ENV=local")
		}
		for _, h := range strings.Split(hosts, ",") {
			if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
				c.WebhookAllowedPrivateHosts = append(c.WebhookAllowedPrivateHosts, h)
			}
		}
	}
	c.MaxActiveKeys = intVar("MERCHANT_MAX_ACTIVE_KEYS", 5, 1, 50)
	c.MaxWebhooks = intVar("WEBHOOK_MAX_ENDPOINTS_PER_MERCHANT", 10, 1, 100)
	c.ShutdownDrain = time.Duration(intVar("SHUTDOWN_DRAIN_SECONDS", 3, 0, 60)) * time.Second
	c.ShutdownTimeout = time.Duration(intVar("SHUTDOWN_TIMEOUT_SECONDS", 15, 1, 120)) * time.Second

	if len(problems) > 0 {
		return Config{}, errors.New("invalid configuration:\n  - " + strings.Join(problems, "\n  - "))
	}
	return c, nil
}
