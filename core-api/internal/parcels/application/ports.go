package application

import (
	"context"
	"database/sql"
	"time"

	"github.com/swiftdrop/core-api/internal/parcels/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

// AcceptedQuoteSnapshot represents the authoritative frozen quote and base tariff data
// returned upon successful atomic consumption of an issued pricing quote.
type AcceptedQuoteSnapshot struct {
	QuoteID            string
	TariffID           string
	TariffVersion      int
	RequesterID        *string
	MerchantID         *string
	OriginZoneID       string
	DestinationZoneID  string
	ChargeableWeightKg float64
	ServiceLevel       string
	FeeCents           int64
	Currency           string
	CreatedAt          time.Time
	ExpiresAt          time.Time
	UsedAt             time.Time
}

// QuoteConsumer defines the atomic operation for consuming an unexpired, unused quote.
// In accordance with Decision D8, this operation MUST be executed within the parcel booking transaction.
type QuoteConsumer interface {
	ConsumeQuote(ctx context.Context, q db.Querier, quoteID string, acceptedAt time.Time) (AcceptedQuoteSnapshot, error)
}

// ParcelRepository defines the persistence port for the Parcel aggregate root.
type ParcelRepository interface {
	Save(ctx context.Context, q db.Querier, parcel domain.Parcel) error
	ExistsTrackingNumber(ctx context.Context, q db.Querier, tn domain.TrackingNumber) (bool, error)
	FindByID(ctx context.Context, q db.Querier, id string) (domain.Parcel, error)
	UpdateStatus(ctx context.Context, q db.Querier, parcel domain.Parcel, expectedVersion int) error
}

// StatusHistoryRepository defines the append-only persistence port for status audit entries.
type StatusHistoryRepository interface {
	Save(ctx context.Context, q db.Querier, entry domain.StatusHistoryEntry) error
}

// ParcelLifecycleService defines the application port for requesting parcel lifecycle status transitions.
// Other modules (e.g. Member 3 dispatch and delivery) must interact with parcel status through this interface.
type ParcelLifecycleService interface {
	Execute(ctx context.Context, cmd TransitionParcelStatusCommand) (TransitionParcelStatusResult, error)
	ExecuteTx(ctx context.Context, tx *sql.Tx, cmd TransitionParcelStatusCommand) (TransitionParcelStatusResult, error)
}

// TrackingNumberGenerator defines the port for producing canonical tracking numbers.
type TrackingNumberGenerator interface {
	Generate(ctx context.Context, date time.Time) (domain.TrackingNumber, error)
}

// IDGenerator defines the port for producing unique entity identifiers (UUIDv7).
type IDGenerator interface {
	GenerateID() (string, error)
}

// Clock abstracts evaluation time for deterministic testing.
type Clock interface {
	Now() time.Time
}

// TransactionManager abstracts database transaction lifecycles using platform/db.WithTx.
type TransactionManager interface {
	WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// IdempotencyStore defines persistence and concurrency locking for caller-scoped request idempotency.
type IdempotencyStore interface {
	AcquireLock(ctx context.Context, q db.Querier, key, callerScope string) error
	Find(ctx context.Context, q db.Querier, key, callerScope string) (IdempotencyRecord, bool, error)
	Save(ctx context.Context, q db.Querier, record IdempotencyRecord) error
}
