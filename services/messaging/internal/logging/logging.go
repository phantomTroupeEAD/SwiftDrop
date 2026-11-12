package logging

import (
	"log/slog"
	"net/url"
	"os"
	"strings"
)

// SetupLogger initializes a structured JSON slog.Logger writing to stdout.
func SetupLogger(levelStr string) *slog.Logger {
	var level slog.Level
	switch strings.ToUpper(strings.TrimSpace(levelStr)) {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN", "WARNING":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler)
}

// NewLogger is an alias for SetupLogger.
func NewLogger(levelStr string) *slog.Logger {
	return SetupLogger(levelStr)
}

// MaskEmail masks the local part of an email address to protect customer PII in logs.
// Example: "kasun.perera@example.lk" -> "k***@example.lk".
func MaskEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "***"
	}

	user := parts[0]
	domain := parts[1]

	if len(user) <= 1 {
		return "*@" + domain
	}

	return string(user[0]) + "***@" + domain
}

// HostOnly extracts the hostname/IP from a URL string, stripping all credentials,
// paths, query parameters, and fragments to prevent secret and URL leakage in logs.
// Example: "https://admin:secret@merchant.com:8443/webhook?token=xyz#hash" -> "merchant.com:8443".
func HostOnly(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		// Fallback: strip scheme if raw parse failed
		stripped := rawURL
		if idx := strings.Index(stripped, "://"); idx != -1 {
			stripped = stripped[idx+3:]
		}
		if idx := strings.IndexAny(stripped, "/?#"); idx != -1 {
			stripped = stripped[:idx]
		}
		if stripped == "" {
			return "[invalid-url]"
		}
		return stripped
	}

	return parsed.Host
}
