package domain_test

import (
	"errors"
	"testing"

	"github.com/swiftdrop/core-api/internal/delivery/domain"
)

func TestFailureReasonCode_Validate(t *testing.T) {
	standardCodes := []domain.FailureReasonCode{
		domain.FailureReasonRecipientUnavailable,
		domain.FailureReasonCustomerRequestedReschedule,
		domain.FailureReasonAccessRestricted,
		domain.FailureReasonInvalidAddress,
		domain.FailureReasonRefusedByRecipient,
		domain.FailureReasonDamagedInTransit,
	}

	for _, code := range standardCodes {
		if err := code.Validate(""); err != nil {
			t.Errorf("expected valid for %s, got: %v", code, err)
		}
	}

	// Reason OTHER requires at least 15 characters in note
	other := domain.FailureReasonOther
	err := other.Validate("Too short")
	if !errors.Is(err, domain.ErrFailureNoteTooShort) {
		t.Errorf("expected ErrFailureNoteTooShort for short note, got: %v", err)
	}

	err = other.Validate("Building intercom broken, security refused gate entry")
	if err != nil {
		t.Errorf("expected valid for long note on OTHER, got error: %v", err)
	}

	// Arbitrary unapproved code
	invalid := domain.FailureReasonCode("TRAFFIC_JAM")
	err = invalid.Validate("Heavy traffic delay on highway")
	if !errors.Is(err, domain.ErrInvalidFailureReasonCode) {
		t.Errorf("expected ErrInvalidFailureReasonCode, got: %v", err)
	}
}

func TestFailureReasonCode_ImmediateReturn(t *testing.T) {
	if !domain.FailureReasonRefusedByRecipient.IsImmediateReturn() {
		t.Errorf("expected REFUSED_BY_RECIPIENT to trigger immediate return")
	}

	if domain.FailureReasonRecipientUnavailable.IsImmediateReturn() {
		t.Errorf("expected RECIPIENT_UNAVAILABLE not to trigger immediate return")
	}
}

