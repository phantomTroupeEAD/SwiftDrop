package application_test

import (
	"context"
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
	"github.com/swiftdrop/core-api/internal/identity/infrastructure"
)

// â”€â”€â”€ helpers â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func buildAuthService(t *testing.T) *application.AuthService {
	t.Helper()
	hasher := infrastructure.NewStandardPasswordHasher()
	tokenProvider := infrastructure.NewJWTTokenProvider("test-secret-key-at-least-32-chars-long")
	userRepo := infrastructure.NewMockUserRepository(hasher)
	tokenRepo := infrastructure.NewMockTokenRepository()
	otpRepo := infrastructure.NewMemoryOTPRepository()
	emailSender := infrastructure.NewDevEmailSender()
	oauthRepo := infrastructure.NewMockOAuthRepository()
	oidcClient := infrastructure.NewGoogleOIDCClient("")

	return application.NewAuthService(userRepo, tokenRepo, hasher, tokenProvider).
		WithOTP(otpRepo, emailSender).
		WithOAuth(oauthRepo, oidcClient)
}

// â”€â”€â”€ Existing Login Tests â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

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

// â”€â”€â”€ Registration Tests â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestAuthService_SendRegistrationOTP(t *testing.T) {
	ctx := context.Background()
	svc := buildAuthService(t)

	t.Run("OTP sent for new available email", func(t *testing.T) {
		res, err := svc.SendRegistrationOTP(ctx, application.SendRegistrationOTPCommand{
			Email: "newuser@example.com",
		})
		if err != nil {
			t.Fatalf("Expected success, got error: %v", err)
		}
		if res.Email != "newuser@example.com" {
			t.Errorf("Expected email newuser@example.com, got %s", res.Email)
		}
		if res.ExpiresIn != 600 {
			t.Errorf("Expected ExpiresIn=600, got %d", res.ExpiresIn)
		}
		if res.CooldownSeconds != 60 {
			t.Errorf("Expected CooldownSeconds=60, got %d", res.CooldownSeconds)
		}
	})

	t.Run("Returns ErrEmailAlreadyExists for registered email", func(t *testing.T) {
		_, err := svc.SendRegistrationOTP(ctx, application.SendRegistrationOTPCommand{
			Email: "admin@swiftdrop.local", // pre-seeded in mock repo
		})
		if err != domain.ErrEmailAlreadyExists {
			t.Errorf("Expected ErrEmailAlreadyExists, got %v", err)
		}
	})

	t.Run("Returns ErrInvalidEmail for malformed email", func(t *testing.T) {
		_, err := svc.SendRegistrationOTP(ctx, application.SendRegistrationOTPCommand{
			Email: "not-a-valid-email",
		})
		if err != domain.ErrInvalidEmail {
			t.Errorf("Expected ErrInvalidEmail, got %v", err)
		}
	})

	t.Run("Returns ErrOTPCooldownActive on rapid resend", func(t *testing.T) {
		email := "cooldown@example.com"
		// First send â€” should succeed
		if _, err := svc.SendRegistrationOTP(ctx, application.SendRegistrationOTPCommand{Email: email}); err != nil {
			t.Fatalf("First OTP send failed: %v", err)
		}
		// Immediate resend â€” should hit cooldown
		_, err := svc.SendRegistrationOTP(ctx, application.SendRegistrationOTPCommand{Email: email})
		if err != domain.ErrOTPCooldownActive {
			t.Errorf("Expected ErrOTPCooldownActive, got %v", err)
		}
	})
}

func TestAuthService_Register(t *testing.T) {
	ctx := context.Background()

	validCmd := func(email, otp string) application.RegisterCustomerCommand {
		return application.RegisterCustomerCommand{
			Email:         email,
			OTP:           otp,
			FirstName:     "Alice",
			LastName:      "Johnson",
			Phone:         "+94771234567",
			Password:      "SecurePassword123!",
			TermsAccepted: true,
		}
	}

	// Helper: request an OTP and extract the printed code via a stub
	sendAndGetOTP := func(svc *application.AuthService, email string) string {
		_, err := svc.SendRegistrationOTP(ctx, application.SendRegistrationOTPCommand{Email: email})
		if err != nil {
			t.Fatalf("SendRegistrationOTP(%s) failed: %v", email, err)
		}
		// In tests we bypass the email sender and read from OTP repo directly via
		// a known-code override â€” here we simulate by using a fresh svc with a
		// deterministic repo (see TestAuthService_Register_KnownOTP below).
		return ""
	}
	_ = sendAndGetOTP // used in sub-test below

	t.Run("Returns ErrTermsNotAccepted when terms_accepted is false", func(t *testing.T) {
		svc := buildAuthService(t)
		cmd := validCmd("alice@example.com", "000000")
		cmd.TermsAccepted = false
		_, err := svc.Register(ctx, cmd)
		if err != domain.ErrTermsNotAccepted {
			t.Errorf("Expected ErrTermsNotAccepted, got %v", err)
		}
	})

	t.Run("Returns ErrWeakPassword for simple password", func(t *testing.T) {
		svc := buildAuthService(t)
		cmd := validCmd("alice@example.com", "000000")
		cmd.Password = "weak"
		_, err := svc.Register(ctx, cmd)
		if err != domain.ErrWeakPassword {
			t.Errorf("Expected ErrWeakPassword, got %v", err)
		}
	})

	t.Run("Returns ErrInvalidPhone for bad phone", func(t *testing.T) {
		svc := buildAuthService(t)
		cmd := validCmd("alice@example.com", "000000")
		cmd.Phone = "077123" // invalid
		_, err := svc.Register(ctx, cmd)
		if err != domain.ErrInvalidPhone {
			t.Errorf("Expected ErrInvalidPhone, got %v", err)
		}
	})

	t.Run("Returns ErrOTPInvalid for wrong OTP code", func(t *testing.T) {
		svc := buildAuthService(t)
		email := "alice2@example.com"
		if _, err := svc.SendRegistrationOTP(ctx, application.SendRegistrationOTPCommand{Email: email}); err != nil {
			t.Fatalf("OTP send failed: %v", err)
		}
		_, err := svc.Register(ctx, validCmd(email, "000000")) // wrong code
		if err != domain.ErrOTPInvalid && err != domain.ErrOTPMaxAttempts {
			t.Errorf("Expected ErrOTPInvalid or ErrOTPMaxAttempts, got %v", err)
		}
	})

	t.Run("Successful full registration flow with correct OTP", func(t *testing.T) {
		// Use a deterministic OTP repository so we know the code
		hasher := infrastructure.NewStandardPasswordHasher()
		tokenProvider := infrastructure.NewJWTTokenProvider("test-secret-key-at-least-32-chars-long")
		userRepo := infrastructure.NewMockUserRepository(hasher)
		tokenRepo := infrastructure.NewMockTokenRepository()
		otpRepo := infrastructure.NewMemoryOTPRepository()
		emailSender := infrastructure.NewDevEmailSender()
		svc := application.NewAuthService(userRepo, tokenRepo, hasher, tokenProvider).
			WithOTP(otpRepo, emailSender)

		email := "success@example.com"

		// Step 1: send OTP â€” saves code internally
		if _, err := svc.SendRegistrationOTP(ctx, application.SendRegistrationOTPCommand{Email: email}); err != nil {
			t.Fatalf("OTP send failed: %v", err)
		}

		// Step 2: inject a known code directly (white-box test shortcut)
		knownCode := "123456"
		_ = otpRepo.SaveOTP(ctx, email, knownCode) // override with known code

		// Step 3: register
		cmd := validCmd(email, knownCode)
		result, err := svc.Register(ctx, cmd)
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		// Assertions
		if result.User.Email.String() != email {
			t.Errorf("Expected email %s, got %s", email, result.User.Email.String())
		}
		if result.User.Role != domain.RoleCustomer {
			t.Errorf("Expected role CUSTOMER, got %s", result.User.Role)
		}
		if result.User.FirstName != "Alice" {
			t.Errorf("Expected FirstName Alice, got %s", result.User.FirstName)
		}
		if result.User.LastName != "Johnson" {
			t.Errorf("Expected LastName Johnson, got %s", result.User.LastName)
		}
		if result.User.Phone != "+94771234567" {
			t.Errorf("Expected phone +94771234567, got %s", result.User.Phone)
		}
		if !result.User.EmailVerified {
			t.Error("Expected EmailVerified to be true")
		}
		if !result.IsNewUser {
			t.Error("Expected IsNewUser to be true")
		}

		// Step 4: OTP must be consumed â€” reuse must fail
		_, err = svc.Register(ctx, cmd)
		if err != domain.ErrOTPInvalid && err != domain.ErrEmailAlreadyExists {
			t.Errorf("Expected OTP reuse or duplicate email error, got %v", err)
		}
	})
}

func TestAuthService_RefreshSession_and_Logout(t *testing.T) {
	ctx := context.Background()
	svc := buildAuthService(t)

	t.Run("Successful token rotation returns fresh access token and new refresh token", func(t *testing.T) {
		loginRes, err := svc.Login(ctx, application.LoginCommand{
			Email:    "customer@swiftdrop.local",
			Password: "CustomerPassword123!",
		})
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		rawToken1 := loginRes.RefreshToken

		refRes, err := svc.RefreshSession(ctx, application.RefreshSessionCommand{
			RawRefreshToken: rawToken1,
		})
		if err != nil {
			t.Fatalf("RefreshSession failed: %v", err)
		}

		if refRes.AccessToken == "" {
			t.Error("Expected non-empty access token")
		}
		if refRes.NewRefreshToken == "" || refRes.NewRefreshToken == rawToken1 {
			t.Error("Expected rotated new refresh token different from rawToken1")
		}

		// Replay attack detection (ADR-003): reusing rawToken1 must trigger ErrTokenReused
		_, err = svc.RefreshSession(ctx, application.RefreshSessionCommand{
			RawRefreshToken: rawToken1,
		})
		if err != domain.ErrTokenReused {
			t.Errorf("Expected ErrTokenReused for replay attack, got %v", err)
		}

		// And the newly rotated token (which belongs to the same family) must now also be revoked
		_, err = svc.RefreshSession(ctx, application.RefreshSessionCommand{
			RawRefreshToken: refRes.NewRefreshToken,
		})
		if err != domain.ErrTokenRevoked {
			t.Errorf("Expected ErrTokenRevoked because family was compromised, got %v", err)
		}
	})

	t.Run("Logout revokes the session family", func(t *testing.T) {
		res, err := svc.Login(ctx, application.LoginCommand{
			Email:    "admin@swiftdrop.local",
			Password: "AdminPassword123!",
		})
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		err = svc.Logout(ctx, application.LogoutCommand{
			RawRefreshToken: res.RefreshToken,
		})
		if err != nil {
			t.Fatalf("Logout failed: %v", err)
		}

		_, err = svc.RefreshSession(ctx, application.RefreshSessionCommand{
			RawRefreshToken: res.RefreshToken,
		})
		if err != domain.ErrTokenRevoked {
			t.Errorf("Expected ErrTokenRevoked after logout, got %v", err)
		}
	})
}

func TestAuthService_LoginWithGoogle(t *testing.T) {
	hasher := infrastructure.NewStandardPasswordHasher()
	tokenProvider := infrastructure.NewJWTTokenProvider("test-secret-key-at-least-32-chars-long")
	userRepo := infrastructure.NewMockUserRepository(hasher)
	tokenRepo := infrastructure.NewMockTokenRepository()
	oauthRepo := infrastructure.NewMockOAuthRepository()
	oidcClient := infrastructure.NewGoogleOIDCClient("")

	svc := application.NewAuthService(userRepo, tokenRepo, hasher, tokenProvider).
		WithOAuth(oauthRepo, oidcClient)
	ctx := context.Background()

	t.Run("New Google User registration creates customer with phone onboarding required", func(t *testing.T) {
		res, err := svc.LoginWithGoogle(ctx, application.GoogleLoginCommand{
			IDToken: "mock_newgoogleuser@gmail.com",
		})
		if err != nil {
			t.Fatalf("LoginWithGoogle failed: %v", err)
		}

		if !res.IsNewUser {
			t.Error("Expected IsNewUser to be true")
		}
		if !res.RequiresPhone {
			t.Error("Expected RequiresPhone to be true for new Google user")
		}
		if res.User.Role != domain.RoleCustomer {
			t.Errorf("Expected role CUSTOMER, got %s", res.User.Role)
		}
		if !res.User.EmailVerified {
			t.Error("Expected EmailVerified to be true")
		}
		if res.AccessToken == "" || res.RefreshToken == "" {
			t.Error("Expected access and refresh tokens")
		}

		// Re-login with the same Google identity should find the existing link
		res2, err := svc.LoginWithGoogle(ctx, application.GoogleLoginCommand{
			IDToken: "mock_newgoogleuser@gmail.com",
		})
		if err != nil {
			t.Fatalf("Second LoginWithGoogle failed: %v", err)
		}
		if res2.IsNewUser {
			t.Error("Expected IsNewUser to be false on second login")
		}
		if res2.User.ID != res.User.ID {
			t.Errorf("Expected same user ID %s, got %s", res.User.ID, res2.User.ID)
		}
	})

	t.Run("Existing password-based user links Google account on first Google login", func(t *testing.T) {
		// customer@swiftdrop.local is preloaded in MockUserRepository
		res, err := svc.LoginWithGoogle(ctx, application.GoogleLoginCommand{
			IDToken: "mock_customer@swiftdrop.local",
		})
		if err != nil {
			t.Fatalf("LoginWithGoogle on existing email failed: %v", err)
		}

		if res.IsNewUser {
			t.Error("Expected IsNewUser to be false when linking to existing account")
		}
		if res.User.Email.String() != "customer@swiftdrop.local" {
			t.Errorf("Expected customer@swiftdrop.local, got %s", res.User.Email.String())
		}
		// Preloaded customer has phone set, so RequiresPhone should be false
		if res.RequiresPhone {
			t.Error("Expected RequiresPhone to be false for preloaded customer with phone")
		}
	})

	t.Run("Invalid Google ID token returns ErrInvalidToken", func(t *testing.T) {
		_, err := svc.LoginWithGoogle(ctx, application.GoogleLoginCommand{
			IDToken: "mock_invalid_token",
		})
		if err != domain.ErrInvalidToken {
			t.Errorf("Expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("Suspended user cannot log in via Google", func(t *testing.T) {
		// Register a user then suspend them
		res, err := svc.LoginWithGoogle(ctx, application.GoogleLoginCommand{
			IDToken: "mock_suspendedgoogle@gmail.com",
		})
		if err != nil {
			t.Fatalf("Initial signup failed: %v", err)
		}

		user, _ := userRepo.FindByID(ctx, res.User.ID)
		user.Suspend()
		_ = userRepo.Save(ctx, user)

		_, err = svc.LoginWithGoogle(ctx, application.GoogleLoginCommand{
			IDToken: "mock_suspendedgoogle@gmail.com",
		})
		if err != domain.ErrAccountSuspended {
			t.Errorf("Expected ErrAccountSuspended, got %v", err)
		}
	})
}
