package application

import (
	"context"
	"time"

	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

// ListTariffsFilter specifies filtering criteria when listing pricing rules / tariffs.
type ListTariffsFilter struct {
	OriginZoneID      string
	DestinationZoneID string
	ActiveOnly        *bool
}

// TariffRepository defines how the application retrieves
// pricing tariffs.
type TariffRepository interface {
	FindApplicable(
		ctx context.Context,
		originZoneID string,
		destinationZoneID string,
		at time.Time,
	) (domain.Tariff, error)

	List(
		ctx context.Context,
		filter ListTariffsFilter,
		offset int,
		limit int,
	) ([]domain.Tariff, int, error)
}

// QuoteRepository defines how generated pricing quotes are persisted.
type QuoteRepository interface {
	Save(ctx context.Context, quote domain.Quote) error
}

// Clock provides the current time to application use cases.
type Clock interface {
	Now() time.Time
}

// QuoteIDGenerator generates unique identifiers for pricing quotes.
type QuoteIDGenerator interface {
	NewID() (string, error)
}

// MerchantStatusApproved indicates an approved merchant permitted to create merchant-scoped quotes.
const MerchantStatusApproved = "APPROVED"

// MerchantProfile holds authoritative merchant identity and approval metadata needed
// to verify merchant-scoped pricing requests without coupling pricing to merchant internals.
type MerchantProfile struct {
	ID          string
	OwnerUserID string
	Status      string
}

// MerchantReader retrieves authoritative merchant records for tenant ownership verification.
type MerchantReader interface {
	FindByID(ctx context.Context, merchantID string) (MerchantProfile, error)
}
