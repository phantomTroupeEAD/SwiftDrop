package tests

import (
	"testing"

	identityapp "github.com/swiftdrop/core-api/internal/identity/application"
	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityinfra "github.com/swiftdrop/core-api/internal/identity/infrastructure"
)

// TestJWTSecret is the shared secret used for integration test token generation and verification.
const TestJWTSecret = "test-integration-secret-key-32chars"

// Standard seeded identities matching canonical DB seeds and E2E testData.ts
const (
	SeedCustomerUserID   = "018f4a7c-1001-7000-8000-000000000001"
	SeedMerchantAlphaUID = "018f4a7c-1002-7000-8000-000000000002"
	SeedMerchantAlphaTID = "mrc_01HMERCHANTTENANT00001"
	SeedMerchantBetaUID  = "018f4a7c-1003-7000-8000-000000000003"
	SeedMerchantBetaTID  = "mrc_02HMERCHANTTENANT00002"
	SeedDispatcherUserID = "018f4a7c-1004-7000-8000-000000000004"
	SeedCourierUserID    = "018f4a7c-1005-7000-8000-000000000005"
	SeedAdminUserID      = "018f4a7c-1006-7000-8000-000000000006"
)

// IssueTestToken generates a signed JWT for testing using canonical identityinfra provider.
func IssueTestToken(t *testing.T, userID, email string, role identitydomain.Role, tenantID *string) string {
	t.Helper()
	prov := identityinfra.NewJWTTokenProvider(TestJWTSecret)
	token, err := prov.GenerateAccessToken(identityapp.Principal{
		UserID:   userID,
		Email:    email,
		Role:     role,
		TenantID: tenantID,
	})
	if err != nil {
		t.Fatalf("failed to generate test token for %s (%s): %v", email, role, err)
	}
	return token
}

// MockCustomerToken returns a signed JWT for the canonical seeded customer.
func MockCustomerToken(t *testing.T) string {
	return IssueTestToken(t, SeedCustomerUserID, "customer@swiftdrop.local", identitydomain.RoleCustomer, nil)
}

// MockMerchantAlphaToken returns a signed JWT for Merchant Tenant Alpha.
func MockMerchantAlphaToken(t *testing.T) string {
	tid := SeedMerchantAlphaTID
	return IssueTestToken(t, SeedMerchantAlphaUID, "merchant@alpha.local", identitydomain.RoleMerchant, &tid)
}

// MockMerchantBetaToken returns a signed JWT for Merchant Tenant Beta.
func MockMerchantBetaToken(t *testing.T) string {
	tid := SeedMerchantBetaTID
	return IssueTestToken(t, SeedMerchantBetaUID, "merchant@beta.local", identitydomain.RoleMerchant, &tid)
}

// MockDispatcherToken returns a signed JWT for the canonical seeded dispatcher.
func MockDispatcherToken(t *testing.T) string {
	return IssueTestToken(t, SeedDispatcherUserID, "dispatcher@swiftdrop.local", identitydomain.RoleDispatcher, nil)
}

// MockCourierToken returns a signed JWT for the canonical seeded courier.
func MockCourierToken(t *testing.T) string {
	return IssueTestToken(t, SeedCourierUserID, "courier@swiftdrop.local", identitydomain.RoleCourier, nil)
}

// MockAdminToken returns a signed JWT for the canonical seeded administrator.
func MockAdminToken(t *testing.T) string {
	return IssueTestToken(t, SeedAdminUserID, "admin@swiftdrop.local", identitydomain.RoleAdmin, nil)
}
