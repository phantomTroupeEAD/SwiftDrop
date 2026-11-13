package config

import (
	"bufio"
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

// parseEnvFile reads a file line by line and sets environment variables if unset.
func parseEnvFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Strip quotes if wrapped in "..." or '...'
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}

		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return true
}

// LoadEnv loads environment variables from a .env file into the process environment
// if the variable is not already defined in the environment.
// It searches for .env files in the provided paths, or defaults to:
// 1. ".env" in current directory
// 2. "core-api/.env" (e.g. if running from repository root)
// 3. "../.env" (e.g. if running from core-api/cmd/api)
func LoadEnv(customPaths ...string) {
	paths := customPaths
	if len(paths) == 0 {
		paths = []string{".env", "core-api/.env", "../.env"}
	}

	for _, p := range paths {
		if parseEnvFile(p) {
			break
		}
	}
}

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
