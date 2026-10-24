// Package domain is reference code for core-api/internal/merchants/domain.
// Standard library only (AGENTS.md §4.2): no net/http, no database/sql.
package domain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidBusinessName = errors.New("merchants: business name must be 2-150 characters")
	ErrReasonRequired      = errors.New("merchants: reason required")
	ErrInvalidReason       = errors.New("merchants: reason must be 5-500 characters")

	ErrAPIKeyInvalid = errors.New("merchants: api key invalid")
	ErrInvalidScope  = errors.New("merchants: invalid scope")
	ErrInvalidLabel  = errors.New("merchants: label must be 1-60 characters")
	ErrInvalidExpiry = errors.New("merchants: expires_at must be 1-365 days in the future")

	ErrURLInvalid          = errors.New("merchants: webhook url invalid")
	ErrURLSchemeNotAllowed = errors.New("merchants: webhook url scheme not allowed")
	ErrURLUnresolvable     = errors.New("merchants: webhook url unresolvable")
	ErrSSRFProhibited      = errors.New("merchants: webhook url targets a prohibited destination")
)

// InvalidTransitionError is returned when an action is not allowed from the current status.
type InvalidTransitionError struct {
	From   MerchantStatus
	Action StatusAction
}

func (e InvalidTransitionError) Error() string {
	return fmt.Sprintf("merchants: cannot %s a merchant in status %s", e.Action, e.From)
}
