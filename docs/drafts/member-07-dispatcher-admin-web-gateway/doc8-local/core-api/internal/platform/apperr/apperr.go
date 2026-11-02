// Package apperr holds the error kinds that use cases return and handlers map to HTTP.
package apperr

import "errors"

var (
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrForbidden          = errors.New("forbidden")
	ErrBadRequest         = errors.New("bad request")
	ErrNotFound           = errors.New("not found")
	ErrValidation         = errors.New("validation failed")
	ErrUnknownSetting     = errors.New("unknown setting")
	ErrConcurrentConflict = errors.New("concurrent modification")
	ErrPrefixConflict     = errors.New("postal prefix conflict")
	ErrZoneCodeTaken      = errors.New("zone code taken")
	ErrZoneNotFound       = errors.New("zone not found")
	ErrReasonRequired     = errors.New("reason required")
	ErrBusy               = errors.New("busy, try again")
	ErrUnavailable        = errors.New("dependency unavailable")
)
