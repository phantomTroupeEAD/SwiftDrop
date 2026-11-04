package domain

import "errors"

var (
	ErrMissingDeliveryProof     = errors.New("proof of delivery is required for successful delivery completion")
	ErrRecipientNameRequired    = errors.New("received_by_name is required for signature and recipient confirmation proof")
	ErrInvalidSignatureData     = errors.New("signature stroke data is invalid or too short (minimum 50 characters required)")
	ErrMissingProofPhotoURL     = errors.New("proof photo URL or storage key is required")
	ErrInvalidFailureReasonCode = errors.New("invalid delivery failure reason code")
	ErrFailureNoteTooShort      = errors.New("failure note must be at least 15 characters when reason code is OTHER")
	ErrAttemptLimitExceeded     = errors.New("delivery attempt number must be between 1 and 3")
	ErrInvalidAttemptOutcome    = errors.New("attempt outcome must be SUCCESS or FAILED")
	ErrOutcomeConflict          = errors.New("successful delivery attempt cannot have failure reason code")
	ErrFailureConflict          = errors.New("failed delivery attempt cannot contain proof of delivery")
	ErrInvalidCoordinates       = errors.New("latitude/longitude coordinates are out of valid geographic range")
	ErrNotAssignedCourier       = errors.New("authenticated courier is not assigned to this parcel")
)

