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

// MaskURL masks sensitive credentials in a URL before logging.
// It uses standard Go url.Redacted() to strip passwords from connection strings.
// Example: "postgres://user:secret@localhost:5432/db" -> "postgres://user:xxxxx@localhost:5432/db"
func MaskURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "[unparseable-url]"
	}

	return parsed.Redacted()
}
