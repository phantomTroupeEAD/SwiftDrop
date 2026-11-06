package domain

import "errors"

var (
	// Parcel validation errors.
	ErrInvalidParcelID       = errors.New("invalid parcel id")
	ErrInvalidTrackingNumber = errors.New("invalid tracking number")
	ErrInvalidSenderID       = errors.New("invalid sender id")
	ErrInvalidRecipient      = errors.New("invalid recipient details")
	ErrInvalidQuoteID        = errors.New("invalid quote id")
	ErrInvalidPricingRule    = errors.New("invalid pricing rule reference")
	ErrInvalidDeliveryFee    = errors.New("invalid delivery fee")
	ErrInvalidZone           = errors.New("invalid zone reference")
	ErrZoneMismatch          = errors.New("address zone does not match parcel zone")
	ErrInvalidServiceLevel   = errors.New("invalid service level")
	ErrInvalidWeight         = errors.New("invalid weight")
	ErrInvalidDimensions     = errors.New("invalid dimensions")
	ErrInvalidAddress        = errors.New("invalid address")
	ErrInvalidParcel         = errors.New("invalid parcel")
	ErrInvalidTimestamp      = errors.New("invalid timestamp")

	// Lifecycle and status transition errors.
	ErrInvalidParcelStatus         = errors.New("invalid parcel status")
	ErrIllegalStateTransition      = errors.New("illegal parcel state transition")
	ErrCancellationNotAllowed      = errors.New("cancellation is only permitted while parcel is in CREATED status")
	ErrMaxDeliveryAttemptsExceeded = errors.New("maximum delivery attempts (3) exceeded")
	ErrTerminalState               = errors.New("cannot transition from terminal state")
	ErrInvalidStatusHistory        = errors.New("invalid status history entry")
)

