package application

import "errors"

var (
	// Quote validation and consumption errors.
	ErrQuoteNotFound          = errors.New("quote not found")
	ErrQuoteExpired           = errors.New("quote has expired")
	ErrQuoteAlreadyUsed       = errors.New("quote has already been used")
	ErrQuoteWeightMismatch    = errors.New("package physical weight/dimensions do not match quoted chargeable weight")
	ErrQuoteZoneMismatch      = errors.New("booking addresses do not match quoted origin/destination zones")
	ErrQuoteOwnershipMismatch = errors.New("quote does not belong to the requesting customer or merchant")

	// Booking and tracking errors.
	ErrTrackingNumberCollision = errors.New("unable to allocate unique tracking number after retries")
	ErrInvalidBookingCommand   = errors.New("invalid booking command")
	ErrNilTransactionManager   = errors.New("nil transaction manager provided")

	// Parcel lifecycle transition errors.
	ErrParcelNotFound             = errors.New("parcel not found")
	ErrConcurrencyConflict        = errors.New("concurrency conflict: parcel was modified by another transaction")
	ErrInvalidTransitionCommand   = errors.New("invalid transition command")
	ErrNilParcelRepository        = errors.New("nil parcel repository provided")
	ErrNilStatusHistoryRepository = errors.New("nil status history repository provided")
	ErrNilOutboxWriter            = errors.New("nil outbox writer provided")

	// Idempotency errors.
	ErrIdempotencyPayloadMismatch  = errors.New("idempotency key reused with different request payload")
	ErrIdempotencyConflict         = errors.New("idempotency key conflict")
	ErrInvalidIdempotencyKey       = errors.New("invalid idempotency key")
	ErrInvalidCallerScope          = errors.New("invalid idempotency caller scope")
	ErrInvalidIdempotencyOperation = errors.New("invalid idempotency operation")
	ErrInvalidRequestHash          = errors.New("invalid idempotency request hash")
	ErrInvalidIdempotencyRecord    = errors.New("invalid idempotency record")
	ErrNilIdempotencyStore         = errors.New("nil idempotency store provided")
)
