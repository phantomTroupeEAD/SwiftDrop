package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/dispatch/domain"
)

func TestCourierAssignment_Lifecycle(t *testing.T) {
	parcelID := uuid.New()
	courierID := uuid.New()
	dispatcherID := uuid.New()

	asgn := domain.NewCourierAssignment(parcelID, courierID, dispatcherID, "Initial assignment")

	if asgn.Status != domain.AssignmentStatusActive {
		t.Fatalf("expected status ACTIVE, got %s", asgn.Status)
	}
	if asgn.EndedAt != nil {
		t.Fatalf("expected EndedAt to be nil for active assignment")
	}

	// 1. Complete assignment
	err := asgn.Complete()
	if err != nil {
		t.Fatalf("failed to complete assignment: %v", err)
	}
	if asgn.Status != domain.AssignmentStatusCompleted {
		t.Errorf("expected COMPLETED, got %s", asgn.Status)
	}
	if asgn.EndedAt == nil {
		t.Errorf("expected EndedAt to be set upon completion")
	}

	// 2. Double completion should fail
	err = asgn.Complete()
	if !errors.Is(err, domain.ErrAssignmentAlreadyClosed) {
		t.Errorf("expected ErrAssignmentAlreadyClosed on double complete, got %v", err)
	}
}

func TestCourierAssignment_Cancel(t *testing.T) {
	asgn := domain.NewCourierAssignment(uuid.New(), uuid.New(), uuid.New(), "")

	// 1. Cancel with empty reason should fail
	err := asgn.Cancel("   ")
	if !errors.Is(err, domain.ErrEmptyReason) {
		t.Errorf("expected ErrEmptyReason for blank cancellation reason, got %v", err)
	}

	// 2. Cancel with valid reason
	err = asgn.Cancel("Courier motorcycle mechanical failure")
	if err != nil {
		t.Fatalf("expected successful cancellation, got: %v", err)
	}
	if asgn.Status != domain.AssignmentStatusCancelled {
		t.Errorf("expected CANCELLED status, got %s", asgn.Status)
	}
	if asgn.EndedAt == nil {
		t.Errorf("expected EndedAt to be populated")
	}

	// 3. Mutating cancelled assignment should fail
	err = asgn.Cancel("Another reason")
	if !errors.Is(err, domain.ErrAssignmentAlreadyClosed) {
		t.Errorf("expected ErrAssignmentAlreadyClosed, got %v", err)
	}
}

func TestCourierAssignment_Reassign(t *testing.T) {
	asgn := domain.NewCourierAssignment(uuid.New(), uuid.New(), uuid.New(), "")

	err := asgn.Reassign("Rerouting to relief driver")
	if err != nil {
		t.Fatalf("expected successful reassignment, got %v", err)
	}
	if asgn.Status != domain.AssignmentStatusReassigned {
		t.Errorf("expected REASSIGNED status, got %s", asgn.Status)
	}
}

