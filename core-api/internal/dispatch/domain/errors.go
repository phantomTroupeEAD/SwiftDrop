package domain

import "errors"

var (
	ErrCourierOverloaded       = errors.New("courier has reached maximum active parcel capacity")
	ErrCourierNotOnShift       = errors.New("courier is not currently on shift")
	ErrCourierSuspended        = errors.New("courier account is administratively suspended")
	ErrCourierInactive         = errors.New("courier profile is inactive")
	ErrInvalidVehicleType      = errors.New("invalid motorized vehicle type: must be MOTORCYCLE, VAN, or TRUCK")
	ErrInvalidCapacity         = errors.New("max capacity must be between 1 and 200 parcels")
	ErrInvalidShiftTransition  = errors.New("invalid shift status transition")
	ErrUnauthorizedShiftChange = errors.New("courier cannot perform this administrative shift status change")
	ErrAssignmentAlreadyClosed = errors.New("courier assignment is already closed and cannot be modified")
	ErrEmptyReason             = errors.New("reason cannot be empty for cancellation or reassignment")
	ErrInvalidEmployeeCode     = errors.New("employee code cannot be empty")
	ErrInvalidZoneID           = errors.New("delivery zone ID cannot be empty")
)

