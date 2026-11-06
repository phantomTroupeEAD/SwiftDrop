package domain

import (
	"errors"
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
			t.Errorf("expected valid status for %q, got: %v", s, err)
		}
	}

	invalidStatuses := []ParcelStatus{
		"",
		"PENDING",
		"FAILED",
		"UNKNOWN",
		"created",
	}

	for _, s := range invalidStatuses {
		if err := s.Validate(); !errors.Is(err, ErrInvalidParcelStatus) {
			t.Errorf("expected ErrInvalidParcelStatus for %q, got: %v", s, err)
		}
	}
}

func TestParcelStatus_IsTerminal(t *testing.T) {
	terminalStatuses := []ParcelStatus{
		StatusDelivered,
		StatusReturned,
		StatusCancelled,
	}

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

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		name         string
		from         ParcelStatus
		to           ParcelStatus
		attemptCount int
		wantErr      error
	}{
		// CREATED transitions
		{
			name:    "CREATED to ASSIGNED",
			from:    StatusCreated,
			to:      StatusAssigned,
			wantErr: nil,
		},
		{
			name:    "CREATED to CANCELLED",
			from:    StatusCreated,
			to:      StatusCancelled,
			wantErr: nil,
		},
		{
			name:    "CREATED to DELIVERED (illegal leap)",
			from:    StatusCreated,
			to:      StatusDelivered,
			wantErr: ErrIllegalStateTransition,
		},

		// ASSIGNED transitions
		{
			name:    "ASSIGNED to PICKED_UP",
			from:    StatusAssigned,
			to:      StatusPickedUp,
			wantErr: nil,
		},
		{
			name:    "ASSIGNED to CANCELLED (rejected per rule: cancellation only while CREATED)",
			from:    StatusAssigned,
			to:      StatusCancelled,
			wantErr: ErrCancellationNotAllowed,
		},

		// PICKED_UP transitions
		{
			name:    "PICKED_UP to IN_TRANSIT",
			from:    StatusPickedUp,
			to:      StatusInTransit,
			wantErr: nil,
		},
		{
			name:    "PICKED_UP to CANCELLED (rejected)",
			from:    StatusPickedUp,
			to:      StatusCancelled,
			wantErr: ErrCancellationNotAllowed,
		},

		// IN_TRANSIT transitions
		{
			name:    "IN_TRANSIT to OUT_FOR_DELIVERY",
			from:    StatusInTransit,
			to:      StatusOutForDelivery,
			wantErr: nil,
		},

		// OUT_FOR_DELIVERY transitions
		{
			name:    "OUT_FOR_DELIVERY to DELIVERED",
			from:    StatusOutForDelivery,
			to:      StatusDelivered,
			wantErr: nil,
		},
		{
			name:    "OUT_FOR_DELIVERY to ATTEMPT_FAILED",
			from:    StatusOutForDelivery,
			to:      StatusAttemptFailed,
			wantErr: nil,
		},

		// ATTEMPT_FAILED transitions
		{
			name:         "ATTEMPT_FAILED to OUT_FOR_DELIVERY with 1 attempt",
			from:         StatusAttemptFailed,
			to:           StatusOutForDelivery,
			attemptCount: 1,
			wantErr:      nil,
		},
		{
			name:         "ATTEMPT_FAILED to OUT_FOR_DELIVERY with 2 attempts",
			from:         StatusAttemptFailed,
			to:           StatusOutForDelivery,
			attemptCount: 2,
			wantErr:      nil,
		},
		{
			name:         "ATTEMPT_FAILED to OUT_FOR_DELIVERY with 3 attempts (exceeded)",
			from:         StatusAttemptFailed,
			to:           StatusOutForDelivery,
			attemptCount: 3,
			wantErr:      ErrMaxDeliveryAttemptsExceeded,
		},
		{
			name:         "ATTEMPT_FAILED to RETURNED after 3 attempts",
			from:         StatusAttemptFailed,
			to:           StatusReturned,
			attemptCount: 3,
			wantErr:      nil,
		},
		{
			name:         "ATTEMPT_FAILED to RETURNED early (e.g. refused)",
			from:         StatusAttemptFailed,
			to:           StatusReturned,
			attemptCount: 1,
			wantErr:      nil,
		},

		// Terminal states cannot transition
		{
			name:    "DELIVERED to OUT_FOR_DELIVERY (terminal)",
			from:    StatusDelivered,
			to:      StatusOutForDelivery,
			wantErr: ErrTerminalState,
		},
		{
			name:    "RETURNED to ASSIGNED (terminal)",
			from:    StatusReturned,
			to:      StatusAssigned,
			wantErr: ErrTerminalState,
		},
		{
			name:    "CANCELLED to CREATED (terminal)",
			from:    StatusCancelled,
			to:      StatusCreated,
			wantErr: ErrTerminalState,
		},

		// Self transition
		{
			name:    "CREATED to CREATED (self transition)",
			from:    StatusCreated,
			to:      StatusCreated,
			wantErr: ErrIllegalStateTransition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransition(tt.from, tt.to, tt.attemptCount)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected nil error, got: %v", err)
				}
			} else {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error wrapping %v, got: %v", tt.wantErr, err)
				}
			}
		})
	}
}

