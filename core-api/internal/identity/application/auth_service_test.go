package application_test

import (
	"context"
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
	"github.com/swiftdrop/core-api/internal/identity/infrastructure"
)

func TestAuthService_Login(t *testing.T) {
	hasher := infrastructure.NewStandardPasswordHasher()
	tokenProvider := infrastructure.NewJWTTokenProvider("test-secret-key-at-least-32-chars-long")
	userRepo := infrastructure.NewMockUserRepository(hasher)
	tokenRepo := infrastructure.NewMockTokenRepository()

	svc := application.NewAuthService(userRepo, tokenRepo, hasher, tokenProvider)
	ctx := context.Background()

	t.Run("Login success for all 5 mock roles", func(t *testing.T) {
		accounts := []struct {
			email    string
			password string
			role     domain.Role
		}{
			{"admin@swiftdrop.local", "AdminPassword123!", domain.RoleAdmin},
			{"dispatcher@swiftdrop.local", "Dispatcher123!", domain.RoleDispatcher},
			{"courier@swiftdrop.local", "CourierPassword123!", domain.RoleCourier},
			{"merchant@vendor.com", "Merchant123!", domain.RoleMerchant},
			{"customer@swiftdrop.local", "CustomerPassword123!", domain.RoleCustomer},
		}

		for _, acc := range accounts {
			res, err := svc.Login(ctx, application.LoginCommand{
				Email:    acc.email,
				Password: acc.password,
			})
			if err != nil {
				t.Fatalf("Login(%s) failed: %v", acc.email, err)
			}

			if res.AccessToken == "" {
				t.Errorf("Expected non-empty access token for %s", acc.email)
			}
			if res.RefreshToken == "" {
				t.Errorf("Expected non-empty refresh token for %s", acc.email)
			}
			if res.User.Role != acc.role {
				t.Errorf("Expected role %s, got %s", acc.role, res.User.Role)
			}

			// Validate token contains correct claims
			principal, err := tokenProvider.ValidateAccessToken(res.AccessToken)
			if err != nil {
				t.Fatalf("ValidateAccessToken failed for %s: %v", acc.email, err)
			}
			if principal.Role != acc.role {
				t.Errorf("Principal role = %s; want %s", principal.Role, acc.role)
			}
		}
	})

	t.Run("Login failure on incorrect password", func(t *testing.T) {
		_, err := svc.Login(ctx, application.LoginCommand{
			Email:    "admin@swiftdrop.local",
			Password: "WrongPassword123!",
		})
		if err != domain.ErrInvalidCredentials {
			t.Errorf("Expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("Login failure on non-existent email", func(t *testing.T) {
		_, err := svc.Login(ctx, application.LoginCommand{
			Email:    "unknown@swiftdrop.local",
			Password: "AdminPassword123!",
		})
		if err != domain.ErrInvalidCredentials {
			t.Errorf("Expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("Login failure on suspended account", func(t *testing.T) {
		// Suspend courier account
		courier, _ := userRepo.FindByEmail(ctx, "courier@swiftdrop.local")
		courier.Suspend()
		_ = userRepo.Save(ctx, courier)

		_, err := svc.Login(ctx, application.LoginCommand{
			Email:    "courier@swiftdrop.local",
			Password: "CourierPassword123!",
		})
		if err != domain.ErrAccountSuspended {
			t.Errorf("Expected ErrAccountSuspended, got %v", err)
		}
	})
}
