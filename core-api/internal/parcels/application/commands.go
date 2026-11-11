package application

import "github.com/swiftdrop/core-api/internal/parcels/domain"

// AddressDTO encapsulates the address details supplied in a booking command.
type AddressDTO struct {
	ContactName  string
	ContactPhone string
	StreetLine1  string
	StreetLine2  string
	City         string
	PostalCode   string
	ZoneID       string
	Instructions string
}

// DimensionsDTO encapsulates the physical dimensions of a package in centimeters.
type DimensionsDTO struct {
	LengthCm float64
	WidthCm  float64
	HeightCm float64
}

// BookParcelCommand contains only authoritative caller-supplied booking inputs.
// In accordance with security and data-integrity rules, callers CANNOT supply
// delivery fees, tariff versions, chargeable weights, or service-level prices.
type BookParcelCommand struct {
	QuoteID             string
	SenderID            *string
	MerchantID          *string
	ActorType           domain.ActorType
	RecipientName       string
	RecipientPhone      string
	WeightKg            float64
	Dimensions          DimensionsDTO
	PickupAddress       AddressDTO
	DropoffAddress      AddressDTO
	ItemDescription     string
	SpecialInstructions string
	CorrelationID       string
	Idempotency         *IdempotencyInput
}

// BookParcelResult returns the resulting domain aggregate root, initial audit trail,
// and idempotency metadata when request idempotency is enabled.
type BookParcelResult struct {
	Parcel        domain.Parcel
	StatusHistory domain.StatusHistoryEntry
	Replayed      bool
	Idempotency   *IdempotencyRecord
}

// TransitionParcelStatusCommand contains all inputs required to execute a validated parcel lifecycle status transition.
type TransitionParcelStatusCommand struct {
	ParcelID        string
	TargetStatus    domain.ParcelStatus
	ActorID         *string
	ActorType       domain.ActorType
	ActorRole       string
	Reason          string
	ExpectedVersion *int
	CorrelationID   string
}

// TransitionParcelStatusResult contains the updated domain aggregate root and resulting audit log record.
type TransitionParcelStatusResult struct {
	Parcel        domain.Parcel
	StatusHistory domain.StatusHistoryEntry
}
