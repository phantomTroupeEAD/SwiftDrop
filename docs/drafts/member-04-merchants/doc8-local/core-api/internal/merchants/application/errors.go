package application

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrMerchantNotFound  = errors.New("merchant not found")
	ErrMerchantExists    = errors.New("merchant already exists for this user")
	ErrNotApproved       = errors.New("merchant not approved")
	ErrVersionConflict   = errors.New("version conflict")
	ErrForbidden         = errors.New("forbidden")
	ErrNotAdmin          = errors.New("actor is not ADMIN")
	ErrKeyLimit          = errors.New("api key limit reached")
	ErrLabelDuplicate    = errors.New("api key label already used")
	ErrPrefixCollision   = errors.New("api key prefix collision")
	ErrWebhookLimit      = errors.New("webhook limit reached")
	ErrWebhookDuplicate  = errors.New("webhook url already registered")
	ErrEventUnsupported  = errors.New("unsupported event type")
	ErrScopeDenied       = errors.New("scope denied")
	ErrAPIKeyRevoked     = errors.New("api key revoked")
	ErrAPIKeyExpired     = errors.New("api key expired")
	ErrIdempotencyReused = errors.New("idempotency key reused with different payload")
)

// ValidationError is a 422 with a field detail.
type ValidationError struct{ Field, Issue string }

func (e ValidationError) Error() string { return e.Field + ": " + e.Issue }
