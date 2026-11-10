package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrInvalidConfig indicates a configuration validation failure.
	ErrInvalidConfig = errors.New("config: invalid configuration value")

	// DefaultDevJWTSecret is the default secret key used in local and development environments.
	DefaultDevJWTSecret = "swiftdrop-development-secret-key-change-in-production-32chars"
)

// Config encapsulates runtime configuration for the Tracking service.
type Config struct {
	// Server settings
	HTTPAddr        string
	AppEnv          string
	ShutdownTimeout time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration

	// Logging
	LogLevel string

	// Future Integration Contracts (Phase 2+)
	DatabaseURL         string
	RedisURL            string
	RabbitMQURL         string
	SSEHeartbeatSeconds int

	// Database Connection Pool (Phase 2)
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	DBConnMaxIdleTime time.Duration
	DBQueryTimeout    time.Duration

	// Identity & JWT Verification (Phase 3)
	JWTSecret string
	JWTIssuer string
}

// Load reads configuration from environment variables with sensible defaults
// and validates the resulting configuration.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:        resolveHTTPAddr(),
		AppEnv:          resolveAppEnv(),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		ReadTimeout:     getDuration("READ_TIMEOUT", 15*time.Second),
		WriteTimeout:    getDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:     getDuration("IDLE_TIMEOUT", 60*time.Second),
		LogLevel:        strings.ToLower(getString("LOG_LEVEL", "info")),

		// Integration contract fields
		DatabaseURL:         getString("TRACKING_DATABASE_URL", getString("DATABASE_URL", "")),
		RedisURL:            getString("REDIS_URL", ""),
		RabbitMQURL:         getString("RABBITMQ_URL", ""),
		SSEHeartbeatSeconds: getInt("SSE_HEARTBEAT_SECONDS", 15),

		// Database Connection Pool (Agreement D8: max 5 connections per pod replica)
		DBMaxOpenConns:    getInt("TRACKING_DB_MAX_OPEN_CONNS", getInt("DATABASE_MAX_OPEN_CONNS", 5)),
		DBMaxIdleConns:    getInt("TRACKING_DB_MAX_IDLE_CONNS", getInt("DATABASE_MAX_IDLE_CONNS", 2)),
		DBConnMaxLifetime: getDuration("TRACKING_DB_CONN_MAX_LIFETIME", 15*time.Minute),
		DBConnMaxIdleTime: getDuration("TRACKING_DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		DBQueryTimeout:    getDuration("TRACKING_DB_QUERY_TIMEOUT", 3*time.Second),

		// Identity & JWT Verification (Phase 3)
		JWTSecret: getString("TRACKING_JWT_SECRET", getString("JWT_SECRET", DefaultDevJWTSecret)),
		JWTIssuer: getString("TRACKING_JWT_ISSUER", getString("JWT_ISSUER", "https://api.swiftdrop.io")),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate ensures all configuration invariants are satisfied.
func (c *Config) Validate() error {
	// Validate HTTP address
	if c.HTTPAddr == "" {
		return fmt.Errorf("%w: HTTPAddr cannot be empty", ErrInvalidConfig)
	}

	// Verify HTTPAddr is parsable as a valid host:port or :port
	addr := c.HTTPAddr
	if strings.HasPrefix(addr, ":") {
		portStr := strings.TrimPrefix(addr, ":")
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("%w: HTTPAddr port '%s' is out of range (1-65535)", ErrInvalidConfig, portStr)
		}
	} else {
		host, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			return fmt.Errorf("%w: HTTPAddr '%s' is malformed: %v", ErrInvalidConfig, addr, err)
		}
		if host == "" && portStr == "" {
			return fmt.Errorf("%w: HTTPAddr '%s' has neither host nor port", ErrInvalidConfig, addr)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("%w: HTTPAddr port '%s' is out of range (1-65535)", ErrInvalidConfig, portStr)
		}
	}

	// Validate environment
	switch c.AppEnv {
	case "local", "development", "test", "staging", "production":
		// Valid environments
	default:
		return fmt.Errorf("%w: unrecognized AppEnv '%s' (allowed: local, development, test, staging, production)", ErrInvalidConfig, c.AppEnv)
	}

	// Validate timeouts
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("%w: ShutdownTimeout must be greater than zero", ErrInvalidConfig)
	}
	if c.ReadTimeout <= 0 {
		return fmt.Errorf("%w: ReadTimeout must be greater than zero", ErrInvalidConfig)
	}
	if c.WriteTimeout <= 0 {
		return fmt.Errorf("%w: WriteTimeout must be greater than zero", ErrInvalidConfig)
	}
	if c.IdleTimeout <= 0 {
		return fmt.Errorf("%w: IdleTimeout must be greater than zero", ErrInvalidConfig)
	}

	// Validate LogLevel
	switch c.LogLevel {
	case "debug", "info", "warn", "warning", "error":
		// Valid levels
	default:
		return fmt.Errorf("%w: unrecognized LogLevel '%s' (allowed: debug, info, warn, error)", ErrInvalidConfig, c.LogLevel)
	}

	// Validate SSE Heartbeat
	if c.SSEHeartbeatSeconds <= 0 {
		return fmt.Errorf("%w: SSEHeartbeatSeconds must be greater than zero", ErrInvalidConfig)
	}

	// Validate DB Pool settings
	if c.DBMaxOpenConns <= 0 {
		return fmt.Errorf("%w: DBMaxOpenConns must be greater than zero", ErrInvalidConfig)
	}
	if c.DBMaxIdleConns <= 0 {
		return fmt.Errorf("%w: DBMaxIdleConns must be greater than zero", ErrInvalidConfig)
	}
	if c.DBMaxIdleConns > c.DBMaxOpenConns {
		return fmt.Errorf("%w: DBMaxIdleConns (%d) cannot exceed DBMaxOpenConns (%d)", ErrInvalidConfig, c.DBMaxIdleConns, c.DBMaxOpenConns)
	}
	if c.DBConnMaxLifetime <= 0 {
		return fmt.Errorf("%w: DBConnMaxLifetime must be greater than zero", ErrInvalidConfig)
	}
	if c.DBConnMaxIdleTime <= 0 {
		return fmt.Errorf("%w: DBConnMaxIdleTime must be greater than zero", ErrInvalidConfig)
	}
	if c.DBQueryTimeout <= 0 {
		return fmt.Errorf("%w: DBQueryTimeout must be greater than zero", ErrInvalidConfig)
	}

	// Validate JWT settings in production
	if c.IsProduction() {
		if c.JWTSecret == "" || c.JWTSecret == DefaultDevJWTSecret {
			return fmt.Errorf("%w: JWTSecret must be explicitly configured in production and cannot use default dev secret", ErrInvalidConfig)
		}
		if len(c.JWTSecret) < 32 {
			return fmt.Errorf("%w: JWTSecret must be at least 32 characters in production", ErrInvalidConfig)
		}
	}

	return nil
}

// IsProduction returns true if the service environment is set to production.
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

// IsLocal returns true if the service environment is set to local.
func (c *Config) IsLocal() bool {
	return c.AppEnv == "local"
}

func resolveHTTPAddr() string {
	// Precedence: TRACKING_HTTP_ADDR -> HTTP_ADDR -> PORT -> :8081
	if addr := strings.TrimSpace(os.Getenv("TRACKING_HTTP_ADDR")); addr != "" {
		return normalizeAddr(addr)
	}
	if addr := strings.TrimSpace(os.Getenv("HTTP_ADDR")); addr != "" {
		return normalizeAddr(addr)
	}
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		return normalizeAddr(port)
	}
	return ":8081"
}

func normalizeAddr(addr string) string {
	// If only a port number was provided (e.g. "8081"), prefix with ":"
	if _, err := strconv.Atoi(addr); err == nil {
		return ":" + addr
	}
	return addr
}

func resolveAppEnv() string {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if env == "" {
		return "local"
	}
	return env
}

func getString(key, def string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return def
}

func getInt(key string, def int) int {
	val, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(val) == "" {
		return def
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		return def
	}
	return parsed
}

func getDuration(key string, def time.Duration) time.Duration {
	val, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(val) == "" {
		return def
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(val))
	if err != nil {
		return def
	}
	return parsed
}
