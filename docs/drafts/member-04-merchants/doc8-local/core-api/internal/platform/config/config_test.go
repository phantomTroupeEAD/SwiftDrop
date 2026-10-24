package config

import (
	"strings"
	"testing"
)

const testPEM = "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----\n"

func env(over map[string]string) func(string) string {
	base := map[string]string{
		"APP_ENV":                       "local",
		"DATABASE_URL":                  "postgres://u:p@db:5432/x",
		"JWT_ISSUER":                    "http://localhost",
		"JWT_PUBLIC_KEY_PEM":            testPEM,
		"MERCHANT_API_KEY_PEPPER":       "local-dev-pepper-change-me-0123456789",
		"WEBHOOK_SECRET_ENCRYPTION_KEY": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=",
	}
	for k, v := range over {
		base[k] = v
	}
	return func(k string) string { return base[k] }
}

func TestLoadValidLocal(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.KeyEnvPrefix != "sd_test_" || len(c.EncKey) != 32 || c.MaxActiveKeys != 5 {
		t.Fatalf("defaults wrong: %+v", c)
	}
}

func TestLoadListsAllProblemsWithoutValues(t *testing.T) {
	secret := "short-pepper-SECRET"
	_, err := Load(env(map[string]string{
		"APP_ENV":                       "production",
		"DATABASE_URL":                  "",
		"MERCHANT_API_KEY_PEPPER":       secret,
		"WEBHOOK_SECRET_ENCRYPTION_KEY": "not-base64!",
		"WEBHOOK_ALLOW_HTTP":            "true",
		"WEBHOOK_ALLOWED_PRIVATE_HOSTS": "webhook-sink",
		"MERCHANT_API_KEY_PREFIX":       "sd_test_",
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"DATABASE_URL", "MERCHANT_API_KEY_PEPPER", "WEBHOOK_SECRET_ENCRYPTION_KEY",
		"WEBHOOK_ALLOW_HTTP", "WEBHOOK_ALLOWED_PRIVATE_HOSTS", "MERCHANT_API_KEY_PREFIX"} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing problem for %s in:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, secret) {
		t.Fatal("error message leaked a secret value")
	}
}

func TestProductionDefaultsToLivePrefix(t *testing.T) {
	c, err := Load(env(map[string]string{"APP_ENV": "production"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.KeyEnvPrefix != "sd_live_" {
		t.Fatalf("prefix = %s", c.KeyEnvPrefix)
	}
}
