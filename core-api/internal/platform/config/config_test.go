package config

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestConfigHelpers(t *testing.T) {
	// String
	os.Setenv("TEST_STR_KEY", "  custom_value  ")
	defer os.Unsetenv("TEST_STR_KEY")
	if val := String("TEST_STR_KEY", "default"); val != "custom_value" {
		t.Errorf("expected 'custom_value', got %q", val)
	}
	if val := String("UNSET_STR_KEY", "default"); val != "default" {
		t.Errorf("expected 'default', got %q", val)
	}

	// Int
	os.Setenv("TEST_INT_KEY", "42")
	defer os.Unsetenv("TEST_INT_KEY")
	if val := Int("TEST_INT_KEY", 10); val != 42 {
		t.Errorf("expected 42, got %d", val)
	}
	os.Setenv("TEST_BAD_INT_KEY", "not-a-number")
	defer os.Unsetenv("TEST_BAD_INT_KEY")
	if val := Int("TEST_BAD_INT_KEY", 10); val != 10 {
		t.Errorf("expected fallback 10, got %d", val)
	}
	if val := Int("UNSET_INT_KEY", 10); val != 10 {
		t.Errorf("expected default 10, got %d", val)
	}

	// Bool
	os.Setenv("TEST_BOOL_KEY", "true")
	defer os.Unsetenv("TEST_BOOL_KEY")
	if val := Bool("TEST_BOOL_KEY", false); !val {
		t.Error("expected true, got false")
	}
	os.Setenv("TEST_BAD_BOOL_KEY", "invalid")
	defer os.Unsetenv("TEST_BAD_BOOL_KEY")
	if val := Bool("TEST_BAD_BOOL_KEY", true); !val {
		t.Error("expected fallback true, got false")
	}

	// Duration
	os.Setenv("TEST_DUR_KEY", "25m")
	defer os.Unsetenv("TEST_DUR_KEY")
	if val := Duration("TEST_DUR_KEY", 5*time.Second); val != 25*time.Minute {
		t.Errorf("expected 25m, got %v", val)
	}
	os.Setenv("TEST_BAD_DUR_KEY", "invalid")
	defer os.Unsetenv("TEST_BAD_DUR_KEY")
	if val := Duration("TEST_BAD_DUR_KEY", 5*time.Second); val != 5*time.Second {
		t.Errorf("expected default 5s, got %v", val)
	}

	// Require
	os.Setenv("TEST_REQ_KEY", "present")
	defer os.Unsetenv("TEST_REQ_KEY")
	val, err := Require("TEST_REQ_KEY")
	if err != nil || val != "present" {
		t.Errorf("expected 'present', got %q (err: %v)", val, err)
	}
	_, err = Require("UNSET_REQ_KEY")
	if !errors.Is(err, ErrMissingConfig) {
		t.Errorf("expected ErrMissingConfig, got %v", err)
	}

	// AppEnv
	os.Setenv("APP_ENV", "production")
	defer os.Unsetenv("APP_ENV")
	if env := AppEnv(); env != "production" {
		t.Errorf("expected 'production', got %q", env)
	}
	if !IsProduction() {
		t.Error("expected IsProduction() to be true")
	}
	if IsLocal() {
		t.Error("expected IsLocal() to be false")
	}

	os.Unsetenv("APP_ENV")
	if env := AppEnv(); env != "local" {
		t.Errorf("expected default 'local', got %q", env)
	}
	if !IsLocal() {
		t.Error("expected IsLocal() to be true")
	}
}

func TestLoadEnv(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_env_*.env")
	if err != nil {
		t.Fatalf("failed to create temp env file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `
# Comment line
TEST_LOAD_ENV_A=hello_world
TEST_LOAD_ENV_B="quoted_string"
TEST_LOAD_ENV_C='single_quoted'
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write temp env file: %v", err)
	}
	_ = tmpFile.Close()

	os.Setenv("TEST_LOAD_ENV_EXISTING", "original")
	defer os.Unsetenv("TEST_LOAD_ENV_EXISTING")
	defer os.Unsetenv("TEST_LOAD_ENV_A")
	defer os.Unsetenv("TEST_LOAD_ENV_B")
	defer os.Unsetenv("TEST_LOAD_ENV_C")

	LoadEnv(tmpFile.Name())

	if val := os.Getenv("TEST_LOAD_ENV_A"); val != "hello_world" {
		t.Errorf("expected 'hello_world', got %q", val)
	}
	if val := os.Getenv("TEST_LOAD_ENV_B"); val != "quoted_string" {
		t.Errorf("expected 'quoted_string', got %q", val)
	}
	if val := os.Getenv("TEST_LOAD_ENV_C"); val != "single_quoted" {
		t.Errorf("expected 'single_quoted', got %q", val)
	}
}
