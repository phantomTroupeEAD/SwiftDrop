package main

import (
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

func TestResolveJWTSecret(t *testing.T) {
	long := "a-very-long-random-production-jwt-secret-value"
	cases := []struct {
		name    string
		env     string
		secret  string
		want    string
		wantErr bool
	}{
		{"local: missing falls back to dev key", "local", "", devJWTSecret, false},
		{"local: explicit value used", "local", long, long, false},
		{"production: missing is an error", "production", "", "", true},
		{"production: dev key is an error", "production", devJWTSecret, "", true},
		{"production: short key is an error", "production", "short-secret", "", true},
		{"production: long key accepted", "production", long, long, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.env)
			t.Setenv("JWT_SECRET", tc.secret)
			got, err := resolveJWTSecret()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSeedPasswordsFromEnv(t *testing.T) {
	t.Setenv("SEED_PASSWORD_ADMIN", "admin-private-password")
	t.Setenv("SEED_PASSWORD_COURIER", "")
	got := seedPasswordsFromEnv()
	if got[domain.RoleAdmin] != "admin-private-password" {
		t.Fatalf("admin password not read: %v", got)
	}
	if _, ok := got[domain.RoleCourier]; ok {
		t.Fatal("empty values must be skipped")
	}
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
}
