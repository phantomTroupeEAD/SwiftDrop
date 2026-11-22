package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	merchantsdomain "github.com/swiftdrop/core-api/internal/merchants/domain"
)

func TestAcceptance_AT011_MerchantSuspension(t *testing.T) {
	now := time.Now().UTC()
	merchantID := SeedMerchantAlphaTID
	ownerUserID := SeedMerchantAlphaUID

	t.Run("1. Newly Registered Merchant is PENDING and Cannot Issue Credentials or Orders", func(t *testing.T) {
		m, err := merchantsdomain.NewMerchant(merchantID, ownerUserID, "Alpha Logistics Ltd")
		require.NoError(t, err)

		assert.Equal(t, merchantsdomain.StatusPending, m.Status())
		assert.False(t, m.CanIssueCredentials(), "pending merchant must not be permitted to issue API keys or book orders")
	})

	t.Run("2. Administrator Approves Merchant -> Transitions to APPROVED", func(t *testing.T) {
		m, err := merchantsdomain.NewMerchant(merchantID, ownerUserID, "Alpha Logistics Ltd")
		require.NoError(t, err)

		adminID := SeedAdminUserID
		err = m.Do(merchantsdomain.ActionApprove, adminID, "Approved following KYC verification", now)
		require.NoError(t, err)

		assert.Equal(t, merchantsdomain.StatusApproved, m.Status())
		assert.True(t, m.CanIssueCredentials(), "approved merchant must be permitted to operate")
		assert.Equal(t, &adminID, m.ApprovedBy())
	})

	t.Run("3. Administrator Suspends Merchant -> Credentials & Booking Immediate Lockout", func(t *testing.T) {
		m := merchantsdomain.Rehydrate(merchantID, ownerUserID, "Alpha Logistics Ltd", merchantsdomain.StatusApproved, 2)
		require.True(t, m.CanIssueCredentials())

		adminID := SeedAdminUserID
		// Suspending without a reason is rejected
		errNoReason := m.Do(merchantsdomain.ActionSuspend, adminID, "   ", now)
		assert.Error(t, errNoReason, "suspending a merchant requires a documented compliance reason")

		// Suspending with valid reason succeeds
		err := m.Do(merchantsdomain.ActionSuspend, adminID, "Billing default / suspicious activity", now)
		require.NoError(t, err)

		assert.Equal(t, merchantsdomain.StatusSuspended, m.Status())
		assert.False(t, m.CanIssueCredentials(), "suspended merchant must immediately lose credential and booking privileges")
	})

	t.Run("4. Suspended Merchant Reinstated -> Restores Operational Privileges", func(t *testing.T) {
		m := merchantsdomain.Rehydrate(merchantID, ownerUserID, "Alpha Logistics Ltd", merchantsdomain.StatusSuspended, 3)
		assert.False(t, m.CanIssueCredentials())

		adminID := SeedAdminUserID
		err := m.Do(merchantsdomain.ActionReinstate, adminID, "Account settled and verified", now)
		require.NoError(t, err)

		assert.Equal(t, merchantsdomain.StatusApproved, m.Status())
		assert.True(t, m.CanIssueCredentials())
	})

	t.Run("5. Illegal State Transition Rejection", func(t *testing.T) {
		// Cannot suspend a merchant that is already PENDING (must be APPROVED first)
		m, err := merchantsdomain.NewMerchant(merchantID, ownerUserID, "Pending Merchant")
		require.NoError(t, err)

		adminID := SeedAdminUserID
		err = m.Do(merchantsdomain.ActionSuspend, adminID, "Illegal action on pending", now)
		assert.Error(t, err, "illegal transition from PENDING directly to SUSPENDED must be rejected")
	})
}
