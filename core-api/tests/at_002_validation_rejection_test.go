package tests

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parcelsdomain "github.com/swiftdrop/core-api/internal/parcels/domain"
)

func TestAcceptance_AT002_ValidationRejection(t *testing.T) {
	// 1. Negative or Zero Weight Rejection
	t.Run("Negative Weight Rejection", func(t *testing.T) {
		_, err := parcelsdomain.NewWeight(-2.5, 1.0, 1.0)
		require.Error(t, err)
		assert.True(t, errors.Is(err, parcelsdomain.ErrInvalidWeight), "expected ErrInvalidWeight, got %v", err)
	})

	t.Run("Chargeable Weight Limit Exceeded (> 50kg)", func(t *testing.T) {
		_, err := parcelsdomain.NewWeight(55.0, 55.0, 55.0)
		require.Error(t, err)
		assert.True(t, errors.Is(err, parcelsdomain.ErrInvalidWeight), "expected ErrInvalidWeight, got %v", err)
	})

	// 2. Invalid Address Rejection
	t.Run("Missing Recipient Contact Name", func(t *testing.T) {
		_, err := parcelsdomain.NewAddress("", "+94771234567", "123 Main St", "", "Colombo", "00100", "ZONE_WESTERN_01", "")
		require.Error(t, err)
		assert.True(t, errors.Is(err, parcelsdomain.ErrInvalidAddress), "expected ErrInvalidAddress, got %v", err)
	})

	t.Run("Missing Recipient Contact Phone", func(t *testing.T) {
		_, err := parcelsdomain.NewAddress("Recipient Beta", "", "123 Main St", "", "Colombo", "00100", "ZONE_WESTERN_01", "")
		require.Error(t, err)
		assert.True(t, errors.Is(err, parcelsdomain.ErrInvalidAddress), "expected ErrInvalidAddress, got %v", err)
	})

	t.Run("Missing Street Address", func(t *testing.T) {
		_, err := parcelsdomain.NewAddress("Recipient Beta", "+94771234567", "   ", "", "Colombo", "00100", "ZONE_WESTERN_01", "")
		require.Error(t, err)
		assert.True(t, errors.Is(err, parcelsdomain.ErrInvalidAddress), "expected ErrInvalidAddress, got %v", err)
	})

	t.Run("Invalid Zone Identifier Pattern", func(t *testing.T) {
		_, err := parcelsdomain.NewAddress("Recipient Beta", "+94771234567", "123 Main St", "", "Colombo", "00100", "INVALID-LOWERCASE-ZONE", "")
		require.Error(t, err)
		assert.True(t, errors.Is(err, parcelsdomain.ErrInvalidAddress), "expected ErrInvalidAddress, got %v", err)
	})

	// 3. Invalid Dimensions Rejection
	t.Run("Zero or Negative Dimensions", func(t *testing.T) {
		_, err := parcelsdomain.NewDimensions(0, 20.0, 15.0)
		require.Error(t, err)
		assert.True(t, errors.Is(err, parcelsdomain.ErrInvalidDimensions), "expected ErrInvalidDimensions, got %v", err)

		_, errNegative := parcelsdomain.NewDimensions(-10.0, 20.0, 15.0)
		require.Error(t, errNegative)
		assert.True(t, errors.Is(errNegative, parcelsdomain.ErrInvalidDimensions))
	})

	// 4. Invalid Aggregate Construction Safeguard
	t.Run("Aggregate Construction with Invalid Price Fails", func(t *testing.T) {
		dim, err := parcelsdomain.NewDimensions(30.0, 25.0, 15.0)
		require.NoError(t, err)

		weight, err := parcelsdomain.NewWeight(2.5, 2.25, 2.5)
		require.NoError(t, err)

		pickup, err := parcelsdomain.NewAddress("Sender Alpha", "+94770000001", "123 Main St", "", "Colombo", "00100", "ZONE_WESTERN_01", "")
		require.NoError(t, err)

		dropoff, err := parcelsdomain.NewAddress("Recipient Beta", "+94710000002", "456 Kandy Rd", "", "Peradeniya", "20400", "ZONE_CENTRAL_01", "")
		require.NoError(t, err)

		// Negative money is rejected
		_, errNegativeMoney := parcelsdomain.NewMoney(-500, "LKR")
		require.Error(t, errNegativeMoney)

		// Empty tracking number is rejected
		_, errEmptyTracking := parcelsdomain.NewParcel(parcelsdomain.CreateParcelParams{
			ID:                "parcel-1",
			RecipientName:     "Recipient Beta",
			RecipientPhone:    "+94710000002",
			OriginZoneID:      "ZONE_WESTERN_01",
			DestinationZoneID: "ZONE_CENTRAL_01",
			ServiceLevel:      parcelsdomain.ServiceLevelStandard,
			Weight:            weight,
			Dimensions:        dim,
			PickupAddress:     pickup,
			DropoffAddress:    dropoff,
			CreatedAt:         time.Now().UTC(),
		})
		require.Error(t, errEmptyTracking, "parcel creation without valid tracking number must fail")
	})
}
