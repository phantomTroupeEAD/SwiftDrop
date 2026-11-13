package application

import (
	"context"
	"fmt"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// MinSeedPasswordLength is the shortest replacement password SecureSeedAccounts accepts.
const MinSeedPasswordLength = 12

// seedAccount is one of the demo accounts created by migration
// 20261021000002_seed_mock_users (and by the in-memory mock repository).
// Their default passwords are published in that migration, so they are not secrets.
type seedAccount struct {
	email           string
	role            domain.Role
	defaultPassword string
}

var seedAccounts = []seedAccount{
	{email: "admin@swiftdrop.local", role: domain.RoleAdmin, defaultPassword: "AdminPassword123!"},
	{email: "dispatcher@swiftdrop.local", role: domain.RoleDispatcher, defaultPassword: "Dispatcher123!"},
	{email: "courier@swiftdrop.local", role: domain.RoleCourier, defaultPassword: "CourierPassword123!"},
	{email: "merchant@vendor.com", role: domain.RoleMerchant, defaultPassword: "Merchant123!"},
	{email: "customer@swiftdrop.local", role: domain.RoleCustomer, defaultPassword: "CustomerPassword123!"},
}

// SeedReport says what SecureSeedAccounts did. It never contains passwords.
type SeedReport struct {
	Rotated   []domain.Role // default password replaced with the configured one
	Suspended []domain.Role // still on the default and no usable replacement: login blocked
	Unchanged []domain.Role // password was already changed earlier: left alone
	Missing   []domain.Role // account does not exist
}

// SecureSeedAccounts protects the demo accounts in production.
//
// For each demo account that still uses its published default password:
//   - if newPasswords has a password for its role (at least MinSeedPasswordLength characters),
//     the password is replaced and the account is (re)activated;
//   - otherwise the account is suspended, so the public default can no longer log in.
//
// Accounts whose password was already changed are not touched, so the function is safe to run
// on every start. Call it only in production; other environments keep the easy demo logins.
func SecureSeedAccounts(ctx context.Context, users UserRepository, hasher PasswordHasher, newPasswords map[domain.Role]string) (SeedReport, error) {
	var report SeedReport
	for _, acc := range seedAccounts {
		user, err := users.FindByEmail(ctx, acc.email)
		if err != nil || user == nil {
			report.Missing = append(report.Missing, acc.role)
			continue
		}
		if !hasher.VerifyPassword(acc.defaultPassword, user.PasswordHash) {
			report.Unchanged = append(report.Unchanged, acc.role)
			continue
		}

		if pw := newPasswords[acc.role]; len(pw) >= MinSeedPasswordLength {
			hash, hashErr := hasher.HashPassword(pw)
			if hashErr != nil {
				return report, fmt.Errorf("secure seed accounts: hash password for %s: %w", acc.role, hashErr)
			}
			user.PasswordHash = hash
			// Still on the public default, so any suspension was ours (an earlier start without a
			// configured password): reactivate now that a private password is set.
			user.Status = domain.StatusActive
			report.Rotated = append(report.Rotated, acc.role)
		} else {
			user.Status = domain.StatusSuspended
			report.Suspended = append(report.Suspended, acc.role)
		}
		user.UpdatedAt = time.Now().UTC()
		if err := users.Save(ctx, user); err != nil {
			return report, fmt.Errorf("secure seed accounts: save %s: %w", acc.role, err)
		}
	}
	return report, nil
}
