package domain_test

import (
	"testing"

	"github.com/swiftdrop/core-api/internal/dispatch/domain"
)

func TestShiftStatus_Validate(t *testing.T) {
	valid := []domain.ShiftStatus{
		domain.ShiftStatusOffShift,
		domain.ShiftStatusOnShift,
		domain.ShiftStatusOnBreak,
		domain.ShiftStatusSuspended,
	}

	for _, s := range valid {
		if err := s.Validate(); err != nil {
			t.Errorf("expected valid shift status for %s, got: %v", s, err)
		}
	}

	invalid := []domain.ShiftStatus{"VACATION", "ACTIVE", "SICK", ""}
	for _, s := range invalid {
		if err := s.Validate(); err == nil {
			t.Errorf("expected error for invalid shift status %s, got nil", s)
		}
	}
}

func TestShiftStatus_CanTransitionTo_SelfService(t *testing.T) {
	// Courier self-service transitions
	assertTransition := func(from, to domain.ShiftStatus, expected bool) {
		t.Helper()
		actual := from.CanTransitionTo(to, true)
		if actual != expected {
			t.Errorf("self-service from %s to %s: expected %v, got %v", from, to, expected, actual)
		}
	}

	// Allowed
	assertTransition(domain.ShiftStatusOffShift, domain.ShiftStatusOnShift, true)
	assertTransition(domain.ShiftStatusOnShift, domain.ShiftStatusOffShift, true)
	assertTransition(domain.ShiftStatusOnShift, domain.ShiftStatusOnBreak, true)
	assertTransition(domain.ShiftStatusOnBreak, domain.ShiftStatusOnShift, true)
	assertTransition(domain.ShiftStatusOnBreak, domain.ShiftStatusOffShift, true)

	// Blocked
	assertTransition(domain.ShiftStatusOffShift, domain.ShiftStatusOnBreak, false)
	assertTransition(domain.ShiftStatusOffShift, domain.ShiftStatusSuspended, false)
	assertTransition(domain.ShiftStatusOnShift, domain.ShiftStatusSuspended, false)
	assertTransition(domain.ShiftStatusSuspended, domain.ShiftStatusOnShift, false)
	assertTransition(domain.ShiftStatusOnShift, domain.ShiftStatusOnShift, false) // same state
}

func TestShiftStatus_CanTransitionTo_Admin(t *testing.T) {
	// Admin transitions
	assertTransition := func(from, to domain.ShiftStatus, expected bool) {
		t.Helper()
		actual := from.CanTransitionTo(to, false)
		if actual != expected {
			t.Errorf("admin from %s to %s: expected %v, got %v", from, to, expected, actual)
		}
	}

	// Admin can suspend from any state
	assertTransition(domain.ShiftStatusOffShift, domain.ShiftStatusSuspended, true)
	assertTransition(domain.ShiftStatusOnShift, domain.ShiftStatusSuspended, true)
	assertTransition(domain.ShiftStatusOnBreak, domain.ShiftStatusSuspended, true)

	// Admin can reinstate suspended courier to OFF_SHIFT
	assertTransition(domain.ShiftStatusSuspended, domain.ShiftStatusOffShift, true)
	// Direct reinstatement to ON_SHIFT from suspended is barred; must go through OFF_SHIFT
	assertTransition(domain.ShiftStatusSuspended, domain.ShiftStatusOnShift, false)
}

