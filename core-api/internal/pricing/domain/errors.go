package domain

import "errors"

var (
	// Weight-related errors.
	ErrInvalidWeight            = errors.New("invalid parcel weight")
	ErrInvalidDimensions        = errors.New("invalid parcel dimensions")
	ErrChargeableWeightExceeded = errors.New("chargeable weight exceeds maximum allowed weight")

	// Pricing rule errors.
	ErrTariffNotFound      = errors.New("no applicable pricing tariff found")
	ErrInvalidTariff       = errors.New("invalid pricing tariff")
	ErrInvalidServiceLevel = errors.New("invalid service level")

	// Quote-related errors.
	ErrQuoteExpired     = errors.New("pricing quote has expired")
	ErrInvalidQuote     = errors.New("invalid pricing quote")
	ErrQuoteAlreadyUsed = errors.New("pricing quote has already been used")

	// Money-related errors.
	ErrInvalidMoney     = errors.New("invalid monetary amount")
	ErrInvalidCurrency  = errors.New("unsupported currency")
	ErrCurrencyMismatch = errors.New("cannot operate on different currencies")
	ErrMoneyOverflow    = errors.New("monetary calculation overflow")
)
