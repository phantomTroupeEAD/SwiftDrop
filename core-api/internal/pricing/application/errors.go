package application

import "errors"

var (
	// ErrMerchantNotFound indicates that no merchant tenant record exists for the supplied identifier.
	ErrMerchantNotFound = errors.New("pricing: merchant tenant not found")
)
