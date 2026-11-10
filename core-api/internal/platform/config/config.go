package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrMissingConfig is returned when a required environment variable is not defined or empty.
	ErrMissingConfig = errors.New("config: required environment variable is missing or empty")
)

// String retrieves an environment variable by key or returns def if unset or whitespace-only.
func String(key, def string) string {
	val, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(val) == "" {
		return def
	}
	return strings.TrimSpace(val)
}

// Int retrieves an environment variable as an integer or returns def if unset or unparseable.
func Int(key string, def int) int {
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

// Bool retrieves an environment variable as a boolean (1, t, true, TRUE, etc.) or returns def.
func Bool(key string, def bool) bool {
	val, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(val) == "" {
		return def
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(val))
	if err != nil {
		return def
	}
	return parsed
}

// Duration retrieves an environment variable as a time.Duration (e.g. "5s", "30m") or returns def.
func Duration(key string, def time.Duration) time.Duration {
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

// Require retrieves a required environment variable. It returns ErrMissingConfig if unset or empty.
func Require(key string) (string, error) {
	val, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(val) == "" {
		return "", fmt.Errorf("%w: %s", ErrMissingConfig, key)
	}
	return strings.TrimSpace(val), nil
}

// AppEnv returns the normalized application runtime environment: "local", "test", "staging", or "production".
// Defaults to "local" if unset.
func AppEnv() string {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	switch env {
	case "test", "staging", "production":
		return env
	case "local", "":
		return "local"
	default:
		return env
	}
}

// IsProduction returns true if APP_ENV is set to "production".
func IsProduction() bool {
	return AppEnv() == "production"
}

// IsLocal returns true if APP_ENV is set to "local".
func IsLocal() bool {
	return AppEnv() == "local"
}

// LoadEnv reads one or more .env files and sets environment variables that are not already set.
// It skips non-existent files silently and ignores comments (#) and blank lines.
func LoadEnv(files ...string) {
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			// Strip surrounding quotes if present
			if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
				v = v[1 : len(v)-1]
			}
			if _, exists := os.LookupEnv(k); !exists {
				_ = os.Setenv(k, v)
			}
		}
	}
}
