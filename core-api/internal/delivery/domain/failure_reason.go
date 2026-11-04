package domain

import "strings"

type FailureReasonCode string

const (
	FailureReasonRecipientUnavailable        FailureReasonCode = "RECIPIENT_UNAVAILABLE"
	FailureReasonCustomerRequestedReschedule FailureReasonCode = "CUSTOMER_REQUESTED_RESCHEDULE"
	FailureReasonAccessRestricted            FailureReasonCode = "ACCESS_RESTRICTED"
	FailureReasonInvalidAddress              FailureReasonCode = "INVALID_ADDRESS"
	FailureReasonRefusedByRecipient          FailureReasonCode = "REFUSED_BY_RECIPIENT"
	FailureReasonDamagedInTransit            FailureReasonCode = "DAMAGED_IN_TRANSIT"
	FailureReasonOther                       FailureReasonCode = "OTHER"
)

// Validate checks whether the reason code matches the 7 approved ADR-004 codes
// and validates note length when code is OTHER.
func (f FailureReasonCode) Validate(notes string) error {
	switch f {
	case FailureReasonRecipientUnavailable,
		FailureReasonCustomerRequestedReschedule,
		FailureReasonAccessRestricted,
		FailureReasonInvalidAddress,
		FailureReasonRefusedByRecipient,
		FailureReasonDamagedInTransit:
		return nil
	case FailureReasonOther:
		if len(strings.TrimSpace(notes)) < 15 {
			return ErrFailureNoteTooShort
		}
		return nil
	default:
		return ErrInvalidFailureReasonCode
	}
}

// IsImmediateReturn indicates whether this failure reason triggers an immediate terminal return to hub/sender.
func (f FailureReasonCode) IsImmediateReturn() bool {
	return f == FailureReasonRefusedByRecipient
}

// AllowsImmediateReattempt indicates whether the parcel is eligible for immediate reattempt review without data corrections.
func (f FailureReasonCode) AllowsImmediateReattempt() bool {
	switch f {
	case FailureReasonRecipientUnavailable,
		FailureReasonCustomerRequestedReschedule,
		FailureReasonAccessRestricted:
		return true
	default:
		return false
	}
}

func (f FailureReasonCode) String() string {
	return string(f)
}

