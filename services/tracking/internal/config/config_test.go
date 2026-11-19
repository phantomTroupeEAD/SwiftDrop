package config

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestConfigLoadDefaults(t *testing.T) {
	// Clear relevant env vars to test defaults
	envKeys := []string{
		"TRACKING_HTTP_ADDR", "HTTP_ADDR", "PORT",
		"APP_ENV", "SHUTDOWN_TIMEOUT", "READ_TIMEOUT",
		"WRITE_TIMEOUT", "IDLE_TIMEOUT", "LOG_LEVEL",
		"TRACKING_DATABASE_URL", "DATABASE_URL",
		"REDIS_URL", "RABBITMQ_URL", "SSE_HEARTBEAT_SECONDS",
	}
	for _, k := range envKeys {
		os.Unsetenv(k)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading default config: %v", err)
	}

	if cfg.HTTPAddr != ":8081" {
		t.Errorf("expected default HTTPAddr :8081, got %s", cfg.HTTPAddr)
	}
	if cfg.AppEnv != "local" {
		t.Errorf("expected default AppEnv local, got %s", cfg.AppEnv)
	}
	if !cfg.IsLocal() {
		t.Error("expected IsLocal() to be true for local environment")
	}
	if cfg.IsProduction() {
		t.Error("expected IsProduction() to be false for local environment")
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("expected default ShutdownTimeout 15s, got %v", cfg.ShutdownTimeout)
	}
	if cfg.ReadTimeout != 15*time.Second {
		t.Errorf("expected default ReadTimeout 15s, got %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 30*time.Second {
		t.Errorf("expected default WriteTimeout 30s, got %v", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 60*time.Second {
		t.Errorf("expected default IdleTimeout 60s, got %v", cfg.IdleTimeout)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default LogLevel info, got %s", cfg.LogLevel)
	}
	if cfg.SSEHeartbeatSeconds != 15 {
		t.Errorf("expected default SSEHeartbeatSeconds 15, got %d", cfg.SSEHeartbeatSeconds)
	}
	if cfg.DBMaxOpenConns != 5 {
		t.Errorf("expected default DBMaxOpenConns 5 (Agreement D8), got %d", cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns != 2 {
		t.Errorf("expected default DBMaxIdleConns 2 (Agreement D8), got %d", cfg.DBMaxIdleConns)
	}
	if cfg.DBConnMaxLifetime != 15*time.Minute {
		t.Errorf("expected default DBConnMaxLifetime 15m, got %v", cfg.DBConnMaxLifetime)
	}
	if cfg.DBConnMaxIdleTime != 5*time.Minute {
		t.Errorf("expected default DBConnMaxIdleTime 5m, got %v", cfg.DBConnMaxIdleTime)
	}
	if cfg.DBQueryTimeout != 3*time.Second {
		t.Errorf("expected default DBQueryTimeout 3s, got %v", cfg.DBQueryTimeout)
	}
	if cfg.RabbitMQQueue != "q.tracking.updates" {
		t.Errorf("expected default RabbitMQQueue q.tracking.updates, got %s", cfg.RabbitMQQueue)
	}
	if cfg.RabbitMQExchange != "swiftdrop.events.topic" {
		t.Errorf("expected default RabbitMQExchange swiftdrop.events.topic, got %s", cfg.RabbitMQExchange)
	}
	if cfg.RabbitMQPrefetch != 10 {
		t.Errorf("expected default RabbitMQPrefetch 10, got %d", cfg.RabbitMQPrefetch)
	}
	if cfg.RabbitMQReconnectDelay != 2*time.Second {
		t.Errorf("expected default RabbitMQReconnectDelay 2s, got %v", cfg.RabbitMQReconnectDelay)
	}
	if cfg.RedisChannel != "tracking:updates" {
		t.Errorf("expected default RedisChannel tracking:updates, got %s", cfg.RedisChannel)
	}
	if cfg.RedisDialTimeout != 5*time.Second {
		t.Errorf("expected default RedisDialTimeout 5s, got %v", cfg.RedisDialTimeout)
	}
	if cfg.RedisReadTimeout != 3*time.Second {
		t.Errorf("expected default RedisReadTimeout 3s, got %v", cfg.RedisReadTimeout)
	}
	if cfg.RedisWriteTimeout != 3*time.Second {
		t.Errorf("expected default RedisWriteTimeout 3s, got %v", cfg.RedisWriteTimeout)
	}
}

func TestConfigAddrPrecedence(t *testing.T) {
	defer func() {
		os.Unsetenv("TRACKING_HTTP_ADDR")
		os.Unsetenv("HTTP_ADDR")
		os.Unsetenv("PORT")
	}()

	// 1. PORT only (numeric)
	os.Unsetenv("TRACKING_HTTP_ADDR")
	os.Unsetenv("HTTP_ADDR")
	os.Setenv("PORT", "9090")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Errorf("expected :9090 from PORT, got %s", cfg.HTTPAddr)
	}

	// 2. HTTP_ADDR overrides PORT
	os.Setenv("HTTP_ADDR", ":9091")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":9091" {
		t.Errorf("expected :9091 from HTTP_ADDR, got %s", cfg.HTTPAddr)
	}

	// 3. TRACKING_HTTP_ADDR overrides HTTP_ADDR
	os.Setenv("TRACKING_HTTP_ADDR", "0.0.0.0:8081")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != "0.0.0.0:8081" {
		t.Errorf("expected 0.0.0.0:8081 from TRACKING_HTTP_ADDR, got %s", cfg.HTTPAddr)
	}
}

func TestConfigValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(c *Config)
		wantErr string
	}{
		{
			name: "empty HTTPAddr",
			mutate: func(c *Config) {
				c.HTTPAddr = ""
			},
			wantErr: "HTTPAddr cannot be empty",
		},
		{
			name: "invalid port in :port",
			mutate: func(c *Config) {
				c.HTTPAddr = ":99999"
			},
			wantErr: "out of range",
		},
		{
			name: "non-numeric port",
			mutate: func(c *Config) {
				c.HTTPAddr = ":invalid"
			},
			wantErr: "out of range",
		},
		{
			name: "invalid host:port format",
			mutate: func(c *Config) {
				c.HTTPAddr = "invalid-address-without-port"
			},
			wantErr: "malformed",
		},
		{
			name: "invalid AppEnv",
			mutate: func(c *Config) {
				c.AppEnv = "unknown-environment"
			},
			wantErr: "unrecognized AppEnv",
		},
		{
			name: "non-positive ShutdownTimeout",
			mutate: func(c *Config) {
				c.ShutdownTimeout = 0
			},
			wantErr: "ShutdownTimeout must be greater than zero",
		},
		{
			name: "non-positive ReadTimeout",
			mutate: func(c *Config) {
				c.ReadTimeout = -1 * time.Second
			},
			wantErr: "ReadTimeout must be greater than zero",
		},
		{
			name: "non-positive WriteTimeout",
			mutate: func(c *Config) {
				c.WriteTimeout = 0
			},
			wantErr: "WriteTimeout must be greater than zero",
		},
		{
			name: "non-positive IdleTimeout",
			mutate: func(c *Config) {
				c.IdleTimeout = 0
			},
			wantErr: "IdleTimeout must be greater than zero",
		},
		{
			name: "unrecognized LogLevel",
			mutate: func(c *Config) {
				c.LogLevel = "verbose"
			},
			wantErr: "unrecognized LogLevel",
		},
		{
			name: "non-positive SSEHeartbeatSeconds",
			mutate: func(c *Config) {
				c.SSEHeartbeatSeconds = 0
			},
			wantErr: "SSEHeartbeatSeconds must be greater than zero",
		},
		{
			name: "non-positive DBMaxOpenConns",
			mutate: func(c *Config) {
				c.DBMaxOpenConns = 0
			},
			wantErr: "DBMaxOpenConns must be greater than zero",
		},
		{
			name: "non-positive DBMaxIdleConns",
			mutate: func(c *Config) {
				c.DBMaxIdleConns = 0
			},
			wantErr: "DBMaxIdleConns must be greater than zero",
		},
		{
			name: "non-positive DBConnMaxLifetime",
			mutate: func(c *Config) {
				c.DBConnMaxLifetime = 0
			},
			wantErr: "DBConnMaxLifetime must be greater than zero",
		},
		{
			name: "non-positive DBConnMaxIdleTime",
			mutate: func(c *Config) {
				c.DBConnMaxIdleTime = 0
			},
			wantErr: "DBConnMaxIdleTime must be greater than zero",
		},
		{
			name: "non-positive DBQueryTimeout",
			mutate: func(c *Config) {
				c.DBQueryTimeout = 0
			},
			wantErr: "DBQueryTimeout must be greater than zero",
		},
		{
			name: "DBMaxIdleConns exceeds DBMaxOpenConns",
			mutate: func(c *Config) {
				c.DBMaxOpenConns = 5
				c.DBMaxIdleConns = 6
			},
			wantErr: "cannot exceed DBMaxOpenConns",
		},
		{
			name: "empty JWTSecret in production",
			mutate: func(c *Config) {
				c.AppEnv = "production"
				c.JWTSecret = ""
			},
			wantErr: "JWTSecret must be explicitly configured in production and cannot use default dev secret",
		},
		{
			name: "default dev JWTSecret in production",
			mutate: func(c *Config) {
				c.AppEnv = "production"
				c.JWTSecret = DefaultDevJWTSecret
			},
			wantErr: "JWTSecret must be explicitly configured in production and cannot use default dev secret",
		},
		{
			name: "short JWTSecret in production",
			mutate: func(c *Config) {
				c.AppEnv = "production"
				c.JWTSecret = "too-short"
			},
			wantErr: "JWTSecret must be at least 32 characters in production",
		},
		{
			name: "empty JWTIssuer in production",
			mutate: func(c *Config) {
				c.AppEnv = "production"
				c.JWTSecret = "valid-production-secret-key-that-is-32-chars-long"
				c.JWTIssuer = ""
			},
			wantErr: "JWTIssuer must be explicitly configured in production",
		},
		{
			name: "empty RabbitMQQueue",
			mutate: func(c *Config) {
				c.RabbitMQQueue = ""
			},
			wantErr: "RabbitMQQueue cannot be empty",
		},
		{
			name: "empty RabbitMQExchange",
			mutate: func(c *Config) {
				c.RabbitMQExchange = ""
			},
			wantErr: "RabbitMQExchange cannot be empty",
		},
		{
			name: "non-positive RabbitMQPrefetch",
			mutate: func(c *Config) {
				c.RabbitMQPrefetch = 0
			},
			wantErr: "RabbitMQPrefetch must be greater than zero",
		},
		{
			name: "non-positive RabbitMQReconnectDelay",
			mutate: func(c *Config) {
				c.RabbitMQReconnectDelay = 0
			},
			wantErr: "RabbitMQReconnectDelay must be greater than zero",
		},
		{
			name: "empty RedisChannel",
			mutate: func(c *Config) {
				c.RedisChannel = ""
			},
			wantErr: "RedisChannel cannot be empty",
		},
		{
			name: "non-positive RedisDialTimeout",
			mutate: func(c *Config) {
				c.RedisDialTimeout = 0
			},
			wantErr: "RedisDialTimeout must be greater than zero",
		},
		{
			name: "non-positive RedisReadTimeout",
			mutate: func(c *Config) {
				c.RedisReadTimeout = 0
			},
			wantErr: "RedisReadTimeout must be greater than zero",
		},
		{
			name: "non-positive RedisWriteTimeout",
			mutate: func(c *Config) {
				c.RedisWriteTimeout = 0
			},
			wantErr: "RedisWriteTimeout must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				HTTPAddr:               ":8081",
				AppEnv:                 "local",
				ShutdownTimeout:        15 * time.Second,
				ReadTimeout:            15 * time.Second,
				WriteTimeout:           30 * time.Second,
				IdleTimeout:            60 * time.Second,
				LogLevel:               "info",
				SSEHeartbeatSeconds:    15,
				DBMaxOpenConns:         5,
				DBMaxIdleConns:         2,
				DBConnMaxLifetime:      15 * time.Minute,
				DBConnMaxIdleTime:      5 * time.Minute,
				DBQueryTimeout:         3 * time.Second,
				RabbitMQQueue:          "q.tracking.updates",
				RabbitMQExchange:       "swiftdrop.events.topic",
				RabbitMQPrefetch:       10,
				RabbitMQReconnectDelay: 2 * time.Second,
				RedisChannel:           "tracking:updates",
				RedisDialTimeout:       5 * time.Second,
				RedisReadTimeout:       3 * time.Second,
				RedisWriteTimeout:      3 * time.Second,
			}
			tt.mutate(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("expected error to wrap ErrInvalidConfig, got %v", err)
			}
		})
	}
}

func TestConfigEnvOverrides(t *testing.T) {
	os.Setenv("APP_ENV", "production")
	os.Setenv("LOG_LEVEL", "DEBUG")
	os.Setenv("SHUTDOWN_TIMEOUT", "25s")
	os.Setenv("READ_TIMEOUT", "10s")
	os.Setenv("WRITE_TIMEOUT", "20s")
	os.Setenv("IDLE_TIMEOUT", "45s")
	os.Setenv("TRACKING_DATABASE_URL", "postgres://user:pass@host:5432/db")
	os.Setenv("REDIS_URL", "redis://localhost:6379/1")
	os.Setenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	os.Setenv("SSE_HEARTBEAT_SECONDS", "30")
	os.Setenv("TRACKING_JWT_SECRET", "super-secret-production-key-at-least-32-chars-long!")

	defer func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("SHUTDOWN_TIMEOUT")
		os.Unsetenv("READ_TIMEOUT")
		os.Unsetenv("WRITE_TIMEOUT")
		os.Unsetenv("IDLE_TIMEOUT")
		os.Unsetenv("TRACKING_DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("RABBITMQ_URL")
		os.Unsetenv("SSE_HEARTBEAT_SECONDS")
		os.Unsetenv("TRACKING_JWT_SECRET")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AppEnv != "production" || !cfg.IsProduction() {
		t.Errorf("expected AppEnv production, got %s", cfg.AppEnv)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel debug, got %s", cfg.LogLevel)
	}
	if cfg.ShutdownTimeout != 25*time.Second {
		t.Errorf("expected ShutdownTimeout 25s, got %v", cfg.ShutdownTimeout)
	}
	if cfg.DatabaseURL != "postgres://user:pass@host:5432/db" {
		t.Errorf("expected DatabaseURL override, got %s", cfg.DatabaseURL)
	}
	if cfg.RedisURL != "redis://localhost:6379/1" {
		t.Errorf("expected RedisURL override, got %s", cfg.RedisURL)
	}
	if cfg.RabbitMQURL != "amqp://guest:guest@localhost:5672/" {
		t.Errorf("expected RabbitMQURL override, got %s", cfg.RabbitMQURL)
	}
	if cfg.SSEHeartbeatSeconds != 30 {
		t.Errorf("expected SSEHeartbeatSeconds 30, got %d", cfg.SSEHeartbeatSeconds)
	}
}
