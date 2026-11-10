package domain

import "errors"

var (
	// ErrInvalidTrackingNumber indicates that a tracking number string violates canonical format rules.
	ErrInvalidTrackingNumber = errors.New("invalid tracking number format")

	// ErrInvalidParcelStatus indicates that an unrecognized or unagreed parcel status was encountered.
	ErrInvalidParcelStatus = errors.New("invalid parcel status")

	// ErrTrackingNotFound indicates that no parcel was found matching the requested tracking number.
	ErrTrackingNotFound = errors.New("tracking not found")

	// ErrDatabaseUnavailable indicates a transient or persistent failure connecting to PostgreSQL.
	ErrDatabaseUnavailable = errors.New("database unavailable")

	// ErrUnauthenticated indicates that a request lacks valid authentication credentials.
	ErrUnauthenticated = errors.New("unauthenticated")

	// ErrAccessDenied indicates that an authenticated caller lacks authorization to access the parcel.
	ErrAccessDenied = errors.New("access denied")
)
