// Package config reads all settings from environment variables once, at start-up.
// A missing or bad value stops the process with a clear message (fail fast).
// Secret values are never printed.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv           string        // APP_ENV
	HTTPAddr         string        // HTTP_ADDR
	LogLevel         string        // LOG_LEVEL
	DatabaseURL      string        // DATABASE_URL (secret)
	DBMaxOpenConns   int32         // DB_MAX_OPEN_CONNS
	DBMinIdleConns   int32         // DB_MAX_IDLE_CONNS (kept warm)
	DBStatementTO    time.Duration // DB_STATEMENT_TIMEOUT_MS
	AdminLockTO      time.Duration // ADMIN_LOCK_TIMEOUT_MS
	JWTPublicKeyPEM  string        // JWT_PUBLIC_KEY_PEM (PEM text or file path)
	JWTIssuer        string        // JWT_ISSUER
	ShutdownGrace    time.Duration // SHUTDOWN_GRACE_SECONDS
	ReadinessDrain   time.Duration // READINESS_DRAIN_SECONDS
	RequestTimeout   time.Duration // HTTP_REQUEST_TIMEOUT_SECONDS
	TrustProxyHeader bool          // TRUST_PROXY_HEADERS
}

var validEnvs = map[string]bool{"local": true, "development": true, "staging": true, "production": true}

// Load reads the environment. getenv is os.Getenv in main and a map in tests.
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	str := func(k, def string, required bool) string {
		v := strings.TrimSpace(getenv(k))
		if v == "" {
			if required {
				errs = append(errs, fmt.Errorf("%s is required", k))
			}
			return def
		}
		return v
	}
	num := func(k string, def, lo, hi int) int {
		v := strings.TrimSpace(getenv(k))
		if v == "" {
			return def
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < lo || n > hi {
			errs = append(errs, fmt.Errorf("%s must be a whole number from %d to %d", k, lo, hi))
			return def
		}
		return n
	}
	c := Config{
		AppEnv:          str("APP_ENV", "", true),
		HTTPAddr:        str("HTTP_ADDR", ":8080", false),
		LogLevel:        strings.ToLower(str("LOG_LEVEL", "info", false)),
		DatabaseURL:     str("DATABASE_URL", "", true),
		DBMaxOpenConns:  int32(num("DB_MAX_OPEN_CONNS", 25, 1, 200)),
		DBMinIdleConns:  int32(num("DB_MAX_IDLE_CONNS", 10, 0, 200)),
		DBStatementTO:   time.Duration(num("DB_STATEMENT_TIMEOUT_MS", 3000, 100, 60000)) * time.Millisecond,
		AdminLockTO:     time.Duration(num("ADMIN_LOCK_TIMEOUT_MS", 2000, 100, 30000)) * time.Millisecond,
		JWTPublicKeyPEM: str("JWT_PUBLIC_KEY_PEM", "", true),
		JWTIssuer:       str("JWT_ISSUER", "http://localhost", false),
		ShutdownGrace:   time.Duration(num("SHUTDOWN_GRACE_SECONDS", 20, 1, 120)) * time.Second,
		ReadinessDrain:  time.Duration(num("READINESS_DRAIN_SECONDS", 5, 0, 60)) * time.Second,
		RequestTimeout:  time.Duration(num("HTTP_REQUEST_TIMEOUT_SECONDS", 25, 1, 120)) * time.Second,
	}
	c.TrustProxyHeader = str("TRUST_PROXY_HEADERS", "true", false) == "true"
	if c.AppEnv != "" && !validEnvs[c.AppEnv] {
		errs = append(errs, fmt.Errorf("APP_ENV must be local, development, staging or production"))
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("LOG_LEVEL must be debug, info, warn or error"))
	}
	if c.DBMinIdleConns > c.DBMaxOpenConns {
		errs = append(errs, fmt.Errorf("DB_MAX_IDLE_CONNS cannot be more than DB_MAX_OPEN_CONNS"))
	}
	// sslmode=disable is allowed: inside the cluster the database is on the private
	// network (alignment guide 3.5). A database outside the cluster needs sslmode=require.
	if c.RequestTimeout >= 30*time.Second {
		errs = append(errs, fmt.Errorf("HTTP_REQUEST_TIMEOUT_SECONDS must be below the gateway's 30 s"))
	}
	return c, errors.Join(errs...)
}

// PublicKeyPEM returns the PEM text, reading the file when the value is a path.
func (c Config) PublicKeyPEM() ([]byte, error) {
	if strings.HasPrefix(c.JWTPublicKeyPEM, "-----BEGIN") {
		return []byte(c.JWTPublicKeyPEM), nil
	}
	return os.ReadFile(c.JWTPublicKeyPEM)
}

// Safe returns settings that may be logged at start-up. Secrets are left out.
func (c Config) Safe() map[string]any {
	return map[string]any{
		"app_env": c.AppEnv, "http_addr": c.HTTPAddr, "log_level": c.LogLevel,
		"db_max_open_conns": c.DBMaxOpenConns, "db_statement_timeout": c.DBStatementTO.String(),
		"admin_lock_timeout": c.AdminLockTO.String(), "jwt_issuer": c.JWTIssuer,
		"shutdown_grace": c.ShutdownGrace.String(), "readiness_drain": c.ReadinessDrain.String(),
		"request_timeout": c.RequestTimeout.String(), "database_url": "[set, hidden]",
	}
}
