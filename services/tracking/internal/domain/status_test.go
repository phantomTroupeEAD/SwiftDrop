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
