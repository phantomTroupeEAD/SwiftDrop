package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type AssignmentStatus string

const (
	AssignmentStatusActive     AssignmentStatus = "ACTIVE"
	AssignmentStatusCompleted  AssignmentStatus = "COMPLETED"
	AssignmentStatusReassigned AssignmentStatus = "REASSIGNED"
	AssignmentStatusCancelled  AssignmentStatus = "CANCELLED"
)

// Validate checks whether the status string is an approved assignment status.
func (a AssignmentStatus) Validate() error {
	switch a {
	case AssignmentStatusActive, AssignmentStatusCompleted, AssignmentStatusReassigned, AssignmentStatusCancelled:
		return nil
	default:
		return ErrAssignmentAlreadyClosed
	}
}

func (a AssignmentStatus) String() string {
	return string(a)
}

// CourierAssignment represents an operational assignment link between a parcel and an assigned courier.
type CourierAssignment struct {
	ID         uuid.UUID
	ParcelID   uuid.UUID
	CourierID  uuid.UUID
	AssignedBy uuid.UUID
	Status     AssignmentStatus
	AssignedAt time.Time
	EndedAt    *time.Time
	Notes      string
}

// NewCourierAssignment creates a new active assignment aggregate.
func NewCourierAssignment(parcelID, courierID, assignedBy uuid.UUID, notes string) *CourierAssignment {
	now := time.Now().UTC()
	return &CourierAssignment{
		ID:         uuid.New(),
		ParcelID:   parcelID,
		CourierID:  courierID,
		AssignedBy: assignedBy,
		Status:     AssignmentStatusActive,
		AssignedAt: now,
		EndedAt:    nil,
		Notes:      strings.TrimSpace(notes),
	}
}

// Complete transitions the assignment to terminal COMPLETED status when a delivery attempt succeeds.
func (a *CourierAssignment) Complete() error {
	if a.Status != AssignmentStatusActive {
		return ErrAssignmentAlreadyClosed
	}
	now := time.Now().UTC()
	a.Status = AssignmentStatusCompleted
	a.EndedAt = &now
	return nil
}

// Cancel terminates an active assignment prior to courier departure, restoring courier capacity.
func (a *CourierAssignment) Cancel(reason string) error {
	if a.Status != AssignmentStatusActive {
		return ErrAssignmentAlreadyClosed
	}
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return ErrEmptyReason
	}
	now := time.Now().UTC()
	a.Status = AssignmentStatusCancelled
	a.EndedAt = &now
	if a.Notes != "" {
		a.Notes = a.Notes + " | Cancellation Reason: " + trimmed
	} else {
		a.Notes = trimmed
	}
	return nil
}

// Reassign marks the assignment as REASSIGNED when shifting a package to a relief courier.
func (a *CourierAssignment) Reassign(reason string) error {
	if a.Status != AssignmentStatusActive {
		return ErrAssignmentAlreadyClosed
	}
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return ErrEmptyReason
	}
	now := time.Now().UTC()
	a.Status = AssignmentStatusReassigned
	a.EndedAt = &now
	if a.Notes != "" {
		a.Notes = a.Notes + " | Reassignment Reason: " + trimmed
	} else {
		a.Notes = trimmed
	}
	return nil
}
