package application

import "errors"

// Error kinds the admin use cases return. Transport maps each one to an HTTP
// status and error code; anything else becomes a generic 500.
var (
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
