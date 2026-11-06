package application

import (
	"context"
	"time"

	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

// TariffRepository defines how the application retrieves
// an applicable pricing tariff.
type TariffRepository interface {
	FindApplicable(
		ctx context.Context,
		originZoneID string,
		destinationZoneID string,
		at time.Time,
	) (domain.Tariff, error)
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
