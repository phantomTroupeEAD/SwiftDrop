package domain

import (
	"fmt"
	"strings"
	"time"
)

// CreateParcelParams contains the complete set of authoritative inputs
// required to construct a new Parcel aggregate root at booking time.
type CreateParcelParams struct {
	ID                 string
	TrackingNumber     TrackingNumber
	SenderID           *string
	MerchantID         *string
	RecipientName      string
	RecipientPhone     string
	QuoteID            string
	PricingRuleID      string
	PricingRuleVersion int
	DeliveryFee        Money
	OriginZoneID       string
	DestinationZoneID  string
	ServiceLevel       ServiceLevel
	Weight             Weight
	Dimensions         Dimensions
	PickupAddress      Address
	DropoffAddress     Address
	CreatedAt          time.Time
}

// ReconstituteParcelParams contains all fields required to hydrate
// a persisted Parcel aggregate root from the database.
type ReconstituteParcelParams struct {
	ID                 string
	TrackingNumber     TrackingNumber
	SenderID           *string
	MerchantID         *string
	RecipientName      string
	RecipientPhone     string
	QuoteID            string
	PricingRuleID      string
	PricingRuleVersion int
	DeliveryFee        Money
	OriginZoneID       string
	DestinationZoneID  string
	ServiceLevel       ServiceLevel
	Weight             Weight
	Dimensions         Dimensions
	PickupAddress      Address
	DropoffAddress     Address
	Status             ParcelStatus
	AttemptCount       int
	Version            int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Parcel is the authoritative Domain Aggregate Root governing courier parcel operations,
// status transitions, accepted pricing snapshots, and regulatory delivery attempt limits.
type Parcel struct {
	ID             string
	TrackingNumber TrackingNumber
	SenderID       *string
	MerchantID     *string
	RecipientName  string
	RecipientPhone string

	// Immutable pricing and quote snapshot established at booking time.
	QuoteID            string
	PricingRuleID      string
	PricingRuleVersion int
	DeliveryFee        Money

	// Geographic zone routing.
	OriginZoneID      string
	DestinationZoneID string

	// SLA tier and physical package characteristics.
	ServiceLevel ServiceLevel
	Weight       Weight
	Dimensions   Dimensions

	// Immutable address snapshots captured at booking time.
	PickupAddress  Address
	DropoffAddress Address

	// Lifecycle state machine and operational counters.
	Status       ParcelStatus
	AttemptCount int

	// Optimistic concurrency control and audit timestamps.
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func normalizeOptionalID(id *string, invalidErr error) (*string, error) {
	if id == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*id)
	if trimmed == "" {
		return nil, invalidErr
	}
	return &trimmed, nil
}

func normalizeParcelOwners(senderID, merchantID *string) (*string, *string, error) {
	normalizedSenderID, err := normalizeOptionalID(senderID, ErrInvalidSenderID)
	if err != nil {
		return nil, nil, err
	}
	normalizedMerchantID, err := normalizeOptionalID(merchantID, ErrInvalidMerchantID)
	if err != nil {
		return nil, nil, err
	}
	if normalizedSenderID == nil && normalizedMerchantID == nil {
		return nil, nil, ErrInvalidOwner
	}
	return normalizedSenderID, normalizedMerchantID, nil
}

// NewParcel creates a new Parcel aggregate root in the CREATED status.
// All accepted quote details and fees are frozen immutably into the parcel aggregate.
func NewParcel(params CreateParcelParams) (Parcel, error) {
	if strings.TrimSpace(params.ID) == "" {
		return Parcel{}, ErrInvalidParcelID
	}
	if err := params.TrackingNumber.Validate(); err != nil {
		return Parcel{}, err
	}
	senderID, merchantID, err := normalizeParcelOwners(params.SenderID, params.MerchantID)
	if err != nil {
		return Parcel{}, err
	}
	if strings.TrimSpace(params.RecipientName) == "" || strings.TrimSpace(params.RecipientPhone) == "" {
		return Parcel{}, ErrInvalidRecipient
	}
	if strings.TrimSpace(params.QuoteID) == "" {
		return Parcel{}, ErrInvalidQuoteID
	}
	if strings.TrimSpace(params.PricingRuleID) == "" || params.PricingRuleVersion <= 0 {
		return Parcel{}, ErrInvalidPricingRule
	}
	if err := params.DeliveryFee.Validate(); err != nil {
		return Parcel{}, err
	}
	if err := ValidateZoneID(params.OriginZoneID); err != nil {
		return Parcel{}, err
	}
	if err := ValidateZoneID(params.DestinationZoneID); err != nil {
		return Parcel{}, err
	}
	if err := params.ServiceLevel.Validate(); err != nil {
		return Parcel{}, err
	}
	if err := params.Weight.Validate(); err != nil {
		return Parcel{}, err
	}
	if err := params.Dimensions.Validate(); err != nil {
		return Parcel{}, err
	}
	if err := params.PickupAddress.Validate(); err != nil {
		return Parcel{}, fmt.Errorf("pickup address: %w", err)
	}
	if err := params.DropoffAddress.Validate(); err != nil {
		return Parcel{}, fmt.Errorf("dropoff address: %w", err)
	}

	// Address zone references must match the aggregate's origin and destination routing zones.
	if params.PickupAddress.ZoneID != params.OriginZoneID {
		return Parcel{}, fmt.Errorf("%w: pickup address zone %q does not match origin zone %q",
			ErrZoneMismatch, params.PickupAddress.ZoneID, params.OriginZoneID)
	}
	if params.DropoffAddress.ZoneID != params.DestinationZoneID {
		return Parcel{}, fmt.Errorf("%w: dropoff address zone %q does not match destination zone %q",
			ErrZoneMismatch, params.DropoffAddress.ZoneID, params.DestinationZoneID)
	}

	if params.CreatedAt.IsZero() {
		return Parcel{}, fmt.Errorf("%w: created_at timestamp is required", ErrInvalidTimestamp)
	}

	return Parcel{
		ID:                 strings.TrimSpace(params.ID),
		TrackingNumber:     params.TrackingNumber,
		SenderID:           senderID,
		MerchantID:         merchantID,
		RecipientName:      strings.TrimSpace(params.RecipientName),
		RecipientPhone:     strings.TrimSpace(params.RecipientPhone),
		QuoteID:            strings.TrimSpace(params.QuoteID),
		PricingRuleID:      strings.TrimSpace(params.PricingRuleID),
		PricingRuleVersion: params.PricingRuleVersion,
		DeliveryFee:        params.DeliveryFee,
		OriginZoneID:       params.OriginZoneID,
		DestinationZoneID:  params.DestinationZoneID,
		ServiceLevel:       params.ServiceLevel,
		Weight:             params.Weight,
		Dimensions:         params.Dimensions,
		PickupAddress:      params.PickupAddress,
		DropoffAddress:     params.DropoffAddress,
		Status:             StatusCreated,
		AttemptCount:       0,
		Version:            1,
		CreatedAt:          params.CreatedAt,
		UpdatedAt:          params.CreatedAt,
	}, nil
}

// ReconstituteParcel hydrates a Parcel aggregate root loaded from persistence.
func ReconstituteParcel(params ReconstituteParcelParams) (Parcel, error) {
	if strings.TrimSpace(params.ID) == "" {
		return Parcel{}, ErrInvalidParcelID
	}
	if err := params.TrackingNumber.Validate(); err != nil {
		return Parcel{}, err
	}
	senderID, merchantID, err := normalizeParcelOwners(params.SenderID, params.MerchantID)
	if err != nil {
		return Parcel{}, err
	}
	if strings.TrimSpace(params.RecipientName) == "" || strings.TrimSpace(params.RecipientPhone) == "" {
		return Parcel{}, ErrInvalidRecipient
	}
	if strings.TrimSpace(params.QuoteID) == "" {
		return Parcel{}, ErrInvalidQuoteID
	}
	if strings.TrimSpace(params.PricingRuleID) == "" || params.PricingRuleVersion <= 0 {
		return Parcel{}, ErrInvalidPricingRule
	}
	if err := params.DeliveryFee.Validate(); err != nil {
		return Parcel{}, err
	}
	if err := ValidateZoneID(params.OriginZoneID); err != nil {
		return Parcel{}, err
	}
	if err := ValidateZoneID(params.DestinationZoneID); err != nil {
		return Parcel{}, err
	}
	if err := params.ServiceLevel.Validate(); err != nil {
		return Parcel{}, err
	}
	if err := params.Weight.Validate(); err != nil {
		return Parcel{}, err
	}
	if err := params.Dimensions.Validate(); err != nil {
		return Parcel{}, err
	}
	if err := params.PickupAddress.Validate(); err != nil {
		return Parcel{}, fmt.Errorf("pickup address: %w", err)
	}
	if err := params.DropoffAddress.Validate(); err != nil {
		return Parcel{}, fmt.Errorf("dropoff address: %w", err)
	}
	if params.PickupAddress.ZoneID != params.OriginZoneID {
		return Parcel{}, fmt.Errorf("%w: pickup address zone %q does not match origin zone %q",
			ErrZoneMismatch, params.PickupAddress.ZoneID, params.OriginZoneID)
	}
	if params.DropoffAddress.ZoneID != params.DestinationZoneID {
		return Parcel{}, fmt.Errorf("%w: dropoff address zone %q does not match destination zone %q",
			ErrZoneMismatch, params.DropoffAddress.ZoneID, params.DestinationZoneID)
	}
	if err := params.Status.Validate(); err != nil {
		return Parcel{}, err
	}
	if params.AttemptCount < 0 {
		return Parcel{}, fmt.Errorf("%w: attempt count cannot be negative (%d)", ErrInvalidParcel, params.AttemptCount)
	}
	if params.Version <= 0 {
		return Parcel{}, fmt.Errorf("%w: version must be greater than zero (%d)", ErrInvalidParcel, params.Version)
	}
	if params.CreatedAt.IsZero() || params.UpdatedAt.IsZero() {
		return Parcel{}, fmt.Errorf("%w: timestamps cannot be zero", ErrInvalidTimestamp)
	}
	if params.UpdatedAt.Before(params.CreatedAt) {
		return Parcel{}, fmt.Errorf("%w: updated_at cannot precede created_at", ErrInvalidTimestamp)
	}

	return Parcel{
		ID:                 strings.TrimSpace(params.ID),
		TrackingNumber:     params.TrackingNumber,
		SenderID:           senderID,
		MerchantID:         merchantID,
		RecipientName:      strings.TrimSpace(params.RecipientName),
		RecipientPhone:     strings.TrimSpace(params.RecipientPhone),
		QuoteID:            strings.TrimSpace(params.QuoteID),
		PricingRuleID:      strings.TrimSpace(params.PricingRuleID),
		PricingRuleVersion: params.PricingRuleVersion,
		DeliveryFee:        params.DeliveryFee,
		OriginZoneID:       params.OriginZoneID,
		DestinationZoneID:  params.DestinationZoneID,
		ServiceLevel:       params.ServiceLevel,
		Weight:             params.Weight,
		Dimensions:         params.Dimensions,
		PickupAddress:      params.PickupAddress,
		DropoffAddress:     params.DropoffAddress,
		Status:             params.Status,
		AttemptCount:       params.AttemptCount,
		Version:            params.Version,
		CreatedAt:          params.CreatedAt,
		UpdatedAt:          params.UpdatedAt,
	}, nil
}

// CanTransitionTo reports whether the parcel can transition to the given target status
// under the current aggregate state and attempt count.
func (p *Parcel) CanTransitionTo(target ParcelStatus) error {
	return ValidateTransition(p.Status, target, p.AttemptCount)
}

// TransitionTo validates and executes a legal status transition on the parcel aggregate.
// It mutates aggregate state, increments Version and UpdatedAt, and returns an immutable
// StatusHistoryEntry recording the event.
func (p *Parcel) TransitionTo(target ParcelStatus, actorID *string, actorType ActorType, actorRole, reason string, at time.Time) (StatusHistoryEntry, error) {
	if at.IsZero() {
		return StatusHistoryEntry{}, fmt.Errorf("%w: event timestamp cannot be zero", ErrInvalidTimestamp)
	}
	if at.Before(p.UpdatedAt) {
		return StatusHistoryEntry{}, fmt.Errorf("%w: event timestamp %v precedes current updated_at %v",
			ErrInvalidTimestamp, at, p.UpdatedAt)
	}

	if err := p.CanTransitionTo(target); err != nil {
		return StatusHistoryEntry{}, err
	}

	normalizedActorID, err := normalizeStatusActor(actorID, actorType)
	if err != nil {
		return StatusHistoryEntry{}, fmt.Errorf("%w: %w", ErrInvalidStatusHistory, err)
	}

	fromStatus := p.Status

	// Increment attempt counter upon recording a delivery failure.
	if target == StatusAttemptFailed {
		p.AttemptCount++
	}

	p.Status = target
	p.Version++
	p.UpdatedAt = at

	entry := StatusHistoryEntry{
		ParcelID:   p.ID,
		FromStatus: &fromStatus,
		ToStatus:   target,
		ActorID:    normalizedActorID,
		ActorType:  actorType,
		ActorRole:  strings.TrimSpace(actorRole),
		Reason:     strings.TrimSpace(reason),
		OccurredAt: at,
	}

	return entry, nil
}

// Assign transitions the parcel from CREATED to ASSIGNED upon courier dispatch.
func (p *Parcel) Assign(courierID string, at time.Time) (StatusHistoryEntry, error) {
	return p.TransitionTo(StatusAssigned, &courierID, ActorTypeUser, "DISPATCHER", "Courier assigned for pickup", at)
}

// Cancel cancels the parcel. Cancellation is permitted ONLY while the parcel is in CREATED status.
func (p *Parcel) Cancel(actorID *string, actorType ActorType, actorRole, reason string, at time.Time) (StatusHistoryEntry, error) {
	return p.TransitionTo(StatusCancelled, actorID, actorType, actorRole, reason, at)
}

// PickUp transitions the parcel from ASSIGNED to PICKED_UP upon courier collection.
func (p *Parcel) PickUp(courierID string, at time.Time) (StatusHistoryEntry, error) {
	return p.TransitionTo(StatusPickedUp, &courierID, ActorTypeUser, "COURIER", "Parcel collected from sender", at)
}

// MarkInTransit transitions the parcel from PICKED_UP to IN_TRANSIT.
func (p *Parcel) MarkInTransit(actorID *string, actorType ActorType, actorRole string, at time.Time) (StatusHistoryEntry, error) {
	return p.TransitionTo(StatusInTransit, actorID, actorType, actorRole, "Parcel arrived at hub / vehicle in transit", at)
}

// DispatchForDelivery transitions the parcel to OUT_FOR_DELIVERY.
func (p *Parcel) DispatchForDelivery(courierID string, at time.Time) (StatusHistoryEntry, error) {
	return p.TransitionTo(StatusOutForDelivery, &courierID, ActorTypeUser, "DISPATCHER", "Dispatched for final-mile delivery", at)
}

// ConfirmDelivered transitions the parcel from OUT_FOR_DELIVERY to the terminal DELIVERED status.
func (p *Parcel) ConfirmDelivered(courierID, reason string, at time.Time) (StatusHistoryEntry, error) {
	return p.TransitionTo(StatusDelivered, &courierID, ActorTypeUser, "COURIER", reason, at)
}

// RecordFailedAttempt records an unsuccessful delivery attempt, incrementing attempt count
// and setting the status to ATTEMPT_FAILED.
func (p *Parcel) RecordFailedAttempt(courierID, reasonCode string, at time.Time) (StatusHistoryEntry, error) {
	return p.TransitionTo(StatusAttemptFailed, &courierID, ActorTypeUser, "COURIER", reasonCode, at)
}

// ReturnToSender transitions the parcel from ATTEMPT_FAILED to the terminal RETURNED status.
func (p *Parcel) ReturnToSender(actorID *string, actorType ActorType, actorRole, reason string, at time.Time) (StatusHistoryEntry, error) {
	return p.TransitionTo(StatusReturned, actorID, actorType, actorRole, reason, at)
}
