// Package domain holds the admin module's zone rules. It has no imports of
// net/http, database/sql or messaging packages (depguard rule domain_purity).
package domain

import "errors"

var (
	ErrInvalidZoneID         = errors.New("invalid zone id")
	ErrInvalidZoneCode       = errors.New("invalid zone code")
	ErrInvalidZoneName       = errors.New("invalid zone name")
	ErrInvalidDescription    = errors.New("invalid zone description")
	ErrInvalidPrefix         = errors.New("invalid postal code prefix")
	ErrNoPrefixes            = errors.New("zone needs at least one postal code prefix")
	ErrDuplicatePrefixInZone = errors.New("postal code prefix listed twice in the same zone")
	ErrAlreadyEnabled        = errors.New("zone is already enabled")
	ErrAlreadyDisabled       = errors.New("zone is already disabled")
)
