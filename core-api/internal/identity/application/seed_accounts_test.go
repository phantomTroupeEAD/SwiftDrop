package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
	"github.com/swiftdrop/core-api/internal/identity/infrastructure"
)

// The mock repository is pre-seeded with the same 5 demo accounts and published default
// passwords as migration 20261021000002_seed_mock_users.
func seedFixture(t *testing.T) (*application.AuthService, application.UserRepository, application.PasswordHasher) {
	t.Helper()
	hasher := infrastructure.NewStandardPasswordHasher()
	repo := infrastructure.NewMockUserRepository(hasher)
	svc := application.NewAuthService(repo, infrastructure.NewMockTokenRepository(), hasher,
		infrastructure.NewJWTTokenProvider("test-secret-key-at-least-32-chars-long"))
	return svc, repo, hasher
}

func login(svc *application.AuthService, email, password string) error {
	_, err := svc.Login(context.Background(), application.LoginCommand{Email: email, Password: password})
	return err
}

func TestSecureSeedAccounts_SuspendsDefaultsWithoutReplacement(t *testing.T) {
	svc, repo, hasher := seedFixture(t)

	report, err := application.SecureSeedAccounts(context.Background(), repo, hasher, nil)
	if err != nil {
		t.Fatalf("SecureSeedAccounts: %v", err)
	}
	if len(report.Suspended) != 5 || len(report.Rotated) != 0 {
		t.Fatalf("want 5 suspended, 0 rotated; got %+v", report)
	}
	if err := login(svc, "admin@swiftdrop.local", "AdminPassword123!"); !errors.Is(err, domain.ErrAccountSuspended) {
		t.Fatalf("published admin password must no longer log in, got err=%v", err)
	}
}

func TestSecureSeedAccounts_RotatesWhenPasswordConfigured(t *testing.T) {
	svc, repo, hasher := seedFixture(t)
	const newPw = "a-private-admin-password"

	report, err := application.SecureSeedAccounts(context.Background(), repo, hasher,
		map[domain.Role]string{domain.RoleAdmin: newPw})
	if err != nil {
		t.Fatalf("SecureSeedAccounts: %v", err)
	}
	if len(report.Rotated) != 1 || report.Rotated[0] != domain.RoleAdmin || len(report.Suspended) != 4 {
		t.Fatalf("want admin rotated and 4 suspended; got %+v", report)
	}
	if err := login(svc, "admin@swiftdrop.local", newPw); err != nil {
		t.Fatalf("admin must log in with the configured password: %v", err)
	}
	if err := login(svc, "admin@swiftdrop.local", "AdminPassword123!"); err == nil {
		t.Fatal("published default must be rejected after rotation")
	}
}

func TestSecureSeedAccounts_ShortPasswordIsNotUsed(t *testing.T) {
	_, repo, hasher := seedFixture(t)

	report, err := application.SecureSeedAccounts(context.Background(), repo, hasher,
		map[domain.Role]string{domain.RoleCourier: "short"})
	if err != nil {
		t.Fatalf("SecureSeedAccounts: %v", err)
	}
	if len(report.Rotated) != 0 || len(report.Suspended) != 5 {
		t.Fatalf("a password shorter than %d chars must not be used; got %+v", application.MinSeedPasswordLength, report)
	}
}

func TestSecureSeedAccounts_LeavesChangedPasswordsAlone(t *testing.T) {
	svc, repo, hasher := seedFixture(t)
	ctx := context.Background()

	// Simulate an operator who already changed the customer password.
	user, err := repo.FindByEmail(ctx, "customer@swiftdrop.local")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	hash, _ := hasher.HashPassword("already-changed-by-ops")
	user.PasswordHash = hash
	if err := repo.Save(ctx, user); err != nil {
		t.Fatalf("Save: %v", err)
	}

	report, err := application.SecureSeedAccounts(ctx, repo, hasher, nil)
	if err != nil {
		t.Fatalf("SecureSeedAccounts: %v", err)
	}
	if len(report.Unchanged) != 1 || report.Unchanged[0] != domain.RoleCustomer {
		t.Fatalf("customer must be left unchanged; got %+v", report)
	}
	if err := login(svc, "customer@swiftdrop.local", "already-changed-by-ops"); err != nil {
		t.Fatalf("changed customer account must keep working: %v", err)
	}
}

func TestSecureSeedAccounts_ReactivatesWhenPasswordAddedLater(t *testing.T) {
	svc, repo, hasher := seedFixture(t)
	ctx := context.Background()
	const newPw = "dispatcher-private-pw"

	// First start: no password configured -> suspended. Second start: password configured.
	if _, err := application.SecureSeedAccounts(ctx, repo, hasher, nil); err != nil {
		t.Fatalf("first run: %v", err)
	}
	report, err := application.SecureSeedAccounts(ctx, repo, hasher, map[domain.Role]string{domain.RoleDispatcher: newPw})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(report.Rotated) != 1 {
		t.Fatalf("dispatcher must be rotated on the second run; got %+v", report)
	}
	if err := login(svc, "dispatcher@swiftdrop.local", newPw); err != nil {
		t.Fatalf("dispatcher must be active again with the new password: %v", err)
	}
}

func TestSecureSeedAccounts_IsIdempotent(t *testing.T) {
	_, repo, hasher := seedFixture(t)
	ctx := context.Background()
	pw := map[domain.Role]string{domain.RoleMerchant: "merchant-private-password"}

	if _, err := application.SecureSeedAccounts(ctx, repo, hasher, pw); err != nil {
		t.Fatalf("first run: %v", err)
	}
	report, err := application.SecureSeedAccounts(ctx, repo, hasher, pw)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	// Second run: merchant no longer on the default -> unchanged; the other 4 are already
	// suspended but still on the default password, so they are suspended again (no-op).
	if len(report.Rotated) != 0 || len(report.Unchanged) != 1 {
		t.Fatalf("second run must not rotate again; got %+v", report)
	}
}
