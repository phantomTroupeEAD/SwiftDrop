package main

import (
	"errors"
	"os"
	"strings"

	"github.com/swiftdrop/core-api/internal/identity/domain"
	"github.com/swiftdrop/core-api/internal/platform/config"
)

// devJWTSecret is the local-development fallback. It is in the public repo, so production
// must never use it.
// #nosec G101 -- local development fallback only; rejected in production by resolveJWTSecret.
const devJWTSecret = "swiftdrop-development-secret-key-change-in-production-32chars" //nolint:gosec

// minJWTSecretLength matches the tracking-api rule (HS256 needs a long, random key).
const minJWTSecretLength = 32

// resolveJWTSecret returns JWT_SECRET. Outside production a missing value falls back to the dev
// key; in production a missing, short or dev key is an error (core-api must not start).
func resolveJWTSecret() (string, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if !config.IsProduction() {
		if secret == "" {
			return devJWTSecret, nil
		}
		return secret, nil
	}
	switch {
	case secret == "":
		return "", errors.New("JWT_SECRET is required in production")
	case secret == devJWTSecret:
		return "", errors.New("JWT_SECRET must not be the public development key in production")
	case len(secret) < minJWTSecretLength:
		return "", errors.New("JWT_SECRET must be at least 32 characters in production")
	}
	return secret, nil
}

// seedPasswordsFromEnv reads the optional replacement passwords for the demo accounts
// (SEED_PASSWORD_ADMIN, _DISPATCHER, _COURIER, _MERCHANT, _CUSTOMER). Empty values are skipped.
func seedPasswordsFromEnv() map[domain.Role]string {
	roles := []domain.Role{domain.RoleAdmin, domain.RoleDispatcher, domain.RoleCourier, domain.RoleMerchant, domain.RoleCustomer}
	out := make(map[domain.Role]string, len(roles))
	for _, role := range roles {
		if pw := os.Getenv("SEED_PASSWORD_" + string(role)); pw != "" {
			out[role] = pw
		}
	}
	return out
}
