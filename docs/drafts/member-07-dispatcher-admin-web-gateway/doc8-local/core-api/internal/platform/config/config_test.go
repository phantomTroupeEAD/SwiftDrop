package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoad_DefaultsAndRequired(t *testing.T) {
	c, err := Load(env(map[string]string{"APP_ENV": "local", "DATABASE_URL": "postgres://x", "JWT_PUBLIC_KEY_PEM": "/k.pem"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.DBMaxOpenConns != 25 || c.ShutdownGrace.Seconds() != 20 {
		t.Fatalf("defaults wrong: %+v", c)
	}
	_, err = Load(env(map[string]string{}))
	for _, k := range []string{"APP_ENV", "DATABASE_URL", "JWT_PUBLIC_KEY_PEM"} {
		if err == nil || !strings.Contains(err.Error(), k) {
			t.Errorf("missing %s not reported: %v", k, err)
		}
	}
}

func TestLoad_BadValues(t *testing.T) {
	base := map[string]string{"APP_ENV": "production", "DATABASE_URL": "postgres://u:p@h/db?sslmode=disable", "JWT_PUBLIC_KEY_PEM": "/k",
		"LOG_LEVEL": "loud", "DB_MAX_OPEN_CONNS": "abc", "HTTP_REQUEST_TIMEOUT_SECONDS": "40"}
	_, err := Load(env(base))
	for _, want := range []string{"LOG_LEVEL", "DB_MAX_OPEN_CONNS", "HTTP_REQUEST_TIMEOUT_SECONDS"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("expected error about %s, got %v", want, err)
		}
	}
}

func TestSafe_HidesSecrets(t *testing.T) {
	c, _ := Load(env(map[string]string{"APP_ENV": "local", "DATABASE_URL": "postgres://u:hunter2@h/db", "JWT_PUBLIC_KEY_PEM": "/k"}))
	for _, v := range c.Safe() {
		if s, ok := v.(string); ok && strings.Contains(s, "hunter2") {
			t.Fatal("secret leaked in Safe()")
		}
	}
}
