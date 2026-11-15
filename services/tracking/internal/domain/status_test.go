package domain

import (
	"testing"
)

func TestParcelStatus_Validate(t *testing.T) {
	validStatuses := []ParcelStatus{
		StatusCreated,
		StatusAssigned,
		StatusPickedUp,
		StatusInTransit,
		StatusOutForDelivery,
		StatusDelivered,
		StatusAttemptFailed,
		StatusReturned,
		StatusCancelled,
	}

	for _, s := range validStatuses {
		if err := s.Validate(); err != nil {
			t.Errorf("expected status %s to be valid, got err: %v", s, err)
		}
	}

	invalid := ParcelStatus("UNKNOWN_STATUS")
	if err := invalid.Validate(); err == nil {
		t.Error("expected invalid status to return error")
	}
}

func TestSafeStatusDescription_SEC009(t *testing.T) {
	cases := []struct {
		status   string
		expected string
	}{
		{string(StatusCreated), "Shipment booked and delivery label generated."},
		{string(StatusAssigned), "Courier assigned and dispatched for pickup."},
		{string(StatusPickedUp), "Parcel collected by courier from sender."},
		{string(StatusInTransit), "Parcel sorted and in transit between hubs."},
		{string(StatusOutForDelivery), "Courier is out for final doorstep delivery."},
		{string(StatusDelivered), "Parcel successfully delivered to recipient."},
		{string(StatusAttemptFailed), "Delivery attempt was unsuccessful; reattempt will be scheduled."},
		{string(StatusReturned), "Parcel returned to sender."},
		{string(StatusCancelled), "Shipment cancelled by sender or system."},
		{"CUSTOM", "Parcel status updated."},
		{"MALICIOUS_INJECTION", "Parcel status updated."},
	}

	for _, tc := range cases {
		desc := SafeStatusDescription(tc.status)
		if desc != tc.expected {
			t.Errorf("status %s: expected %q, got %q", tc.status, tc.expected, desc)
		}
	}
}

func TestParcelStatus_IsTerminal(t *testing.T) {
	terminalStatuses := []ParcelStatus{StatusDelivered, StatusReturned, StatusCancelled}
	for _, s := range terminalStatuses {
		if !s.IsTerminal() {
			t.Errorf("expected status %s to be terminal", s)
		}
	}

	nonTerminalStatuses := []ParcelStatus{
		StatusCreated,
		StatusAssigned,
		StatusPickedUp,
		StatusInTransit,
		StatusOutForDelivery,
		StatusAttemptFailed,
	}
	for _, s := range nonTerminalStatuses {
		if s.IsTerminal() {
			t.Errorf("expected status %s to be non-terminal", s)
		}
	}
}

func TestIsValidTransition(t *testing.T) {
	validCases := []struct {
		from ParcelStatus
		to   ParcelStatus
	}{
		{StatusCreated, StatusAssigned},
		{StatusCreated, StatusCancelled},
		{StatusAssigned, StatusPickedUp},
		{StatusPickedUp, StatusInTransit},
		{StatusInTransit, StatusOutForDelivery},
		{StatusOutForDelivery, StatusDelivered},
		{StatusOutForDelivery, StatusAttemptFailed},
		{StatusAttemptFailed, StatusOutForDelivery}, // Valid rescheduling!
		{StatusAttemptFailed, StatusReturned},       // Valid return to sender!
	}

	for _, tc := range validCases {
		if !IsValidTransition(tc.from, tc.to) {
			t.Errorf("expected valid transition from %s to %s", tc.from, tc.to)
		}
	}

	invalidCases := []struct {
		from ParcelStatus
		to   ParcelStatus
		desc string
	}{
		// Self transitions
		{StatusCreated, StatusCreated, "identical self-transition"},
		{StatusInTransit, StatusInTransit, "identical self-transition"},
		{StatusOutForDelivery, StatusOutForDelivery, "identical self-transition"},
		// Backward regressions
		{StatusAssigned, StatusCreated, "regression from ASSIGNED to CREATED"},
		{StatusPickedUp, StatusAssigned, "regression from PICKED_UP to ASSIGNED"},
		{StatusInTransit, StatusPickedUp, "regression from IN_TRANSIT to PICKED_UP"},
		{StatusOutForDelivery, StatusInTransit, "regression from OUT_FOR_DELIVERY to IN_TRANSIT"},
		{StatusAttemptFailed, StatusInTransit, "regression from ATTEMPT_FAILED to IN_TRANSIT"},
		{StatusAttemptFailed, StatusPickedUp, "regression from ATTEMPT_FAILED to PICKED_UP"},
		// Invalid cancellation
		{StatusAssigned, StatusCancelled, "cancellation after assignment"},
		{StatusInTransit, StatusCancelled, "cancellation while in transit"},
		{StatusOutForDelivery, StatusCancelled, "cancellation while out for delivery"},
		// Terminal states cannot transition out
		{StatusDelivered, StatusOutForDelivery, "transition from terminal DELIVERED"},
		{StatusReturned, StatusOutForDelivery, "transition from terminal RETURNED"},
		{StatusCancelled, StatusCreated, "transition from terminal CANCELLED"},
	}

	for _, tc := range invalidCases {
		if IsValidTransition(tc.from, tc.to) {
			t.Errorf("expected invalid transition for %s (%s -> %s)", tc.desc, tc.from, tc.to)
		}
	}
}

func TestCanReach(t *testing.T) {
	// Valid forward progressions and multi-step reachable transitions
	validCases := []struct {
		from ParcelStatus
		to   ParcelStatus
	}{
		{StatusCreated, StatusAssigned},
		{StatusCreated, StatusInTransit},
		{StatusCreated, StatusDelivered},
		{StatusCreated, StatusCancelled},
		{StatusAssigned, StatusDelivered},
		{StatusPickedUp, StatusOutForDelivery},
		{StatusInTransit, StatusAttemptFailed},
		{StatusOutForDelivery, StatusAttemptFailed},
		{StatusAttemptFailed, StatusOutForDelivery}, // Valid rescheduling
		{StatusAttemptFailed, StatusReturned},
		{StatusAttemptFailed, StatusDelivered}, // Reattempt succeeded
	}

	for _, tc := range validCases {
		if !CanReach(tc.from, tc.to) {
			t.Errorf("expected CanReach(%s, %s) to be true", tc.from, tc.to)
		}
	}

	// Invalid regressive or illegal transitions
	invalidCases := []struct {
		from ParcelStatus
		to   ParcelStatus
	}{
		{StatusOutForDelivery, StatusInTransit},
		{StatusOutForDelivery, StatusPickedUp},
		{StatusOutForDelivery, StatusAssigned},
		{StatusOutForDelivery, StatusCreated},
		{StatusOutForDelivery, StatusCancelled},
		{StatusAttemptFailed, StatusInTransit},
		{StatusAttemptFailed, StatusPickedUp},
		{StatusAttemptFailed, StatusCancelled},
		{StatusDelivered, StatusOutForDelivery},
		{StatusReturned, StatusAttemptFailed},
		{StatusCancelled, StatusCreated},
	}

	for _, tc := range invalidCases {
		if CanReach(tc.from, tc.to) {
			t.Errorf("expected CanReach(%s, %s) to be false", tc.from, tc.to)
		}
	}
}
