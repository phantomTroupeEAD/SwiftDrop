package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityinfra "github.com/swiftdrop/core-api/internal/identity/infrastructure"
)

func TestAuthHelpers_TokenGenerationAndValidation(t *testing.T) {
	validator := identityinfra.NewJWTTokenProvider(TestJWTSecret)

	t.Run("Customer Token", func(t *testing.T) {
		token := MockCustomerToken(t)
		require.NotEmpty(t, token)

		principal, err := validator.ValidateAccessToken(token)
		require.NoError(t, err)
		assert.Equal(t, SeedCustomerUserID, principal.UserID)
		assert.Equal(t, identitydomain.RoleCustomer, principal.Role)
		assert.Nil(t, principal.TenantID)
	})

	t.Run("Merchant Alpha Token", func(t *testing.T) {
		token := MockMerchantAlphaToken(t)
		require.NotEmpty(t, token)

		principal, err := validator.ValidateAccessToken(token)
		require.NoError(t, err)
		assert.Equal(t, SeedMerchantAlphaUID, principal.UserID)
		assert.Equal(t, identitydomain.RoleMerchant, principal.Role)
		require.NotNil(t, principal.TenantID)
		assert.Equal(t, SeedMerchantAlphaTID, *principal.TenantID)
	})

	t.Run("Merchant Beta Token (Tenant Isolation)", func(t *testing.T) {
		token := MockMerchantBetaToken(t)
		require.NotEmpty(t, token)

		principal, err := validator.ValidateAccessToken(token)
		require.NoError(t, err)
		assert.Equal(t, SeedMerchantBetaUID, principal.UserID)
		assert.Equal(t, identitydomain.RoleMerchant, principal.Role)
		require.NotNil(t, principal.TenantID)
		assert.Equal(t, SeedMerchantBetaTID, *principal.TenantID)
		assert.NotEqual(t, SeedMerchantAlphaTID, *principal.TenantID)
	})

	t.Run("Dispatcher Token", func(t *testing.T) {
		token := MockDispatcherToken(t)
		require.NotEmpty(t, token)

		principal, err := validator.ValidateAccessToken(token)
		require.NoError(t, err)
		assert.Equal(t, SeedDispatcherUserID, principal.UserID)
		assert.Equal(t, identitydomain.RoleDispatcher, principal.Role)
	})

	t.Run("Courier Token", func(t *testing.T) {
		token := MockCourierToken(t)
		require.NotEmpty(t, token)

		principal, err := validator.ValidateAccessToken(token)
		require.NoError(t, err)
		assert.Equal(t, SeedCourierUserID, principal.UserID)
		assert.Equal(t, identitydomain.RoleCourier, principal.Role)
	})

	t.Run("Admin Token", func(t *testing.T) {
		token := MockAdminToken(t)
		require.NotEmpty(t, token)

		principal, err := validator.ValidateAccessToken(token)
		require.NoError(t, err)
		assert.Equal(t, SeedAdminUserID, principal.UserID)
		assert.Equal(t, identitydomain.RoleAdmin, principal.Role)
	})

	t.Run("Tampered / Invalid Secret Rejection", func(t *testing.T) {
		token := MockCustomerToken(t)
		wrongSecretValidator := identityinfra.NewJWTTokenProvider("wrong-secret-key-that-does-not-match")

		_, err := wrongSecretValidator.ValidateAccessToken(token)
		assert.Error(t, err, "token validated with wrong secret should fail")
	})
}
