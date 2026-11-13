package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/swiftdrop/core-api/internal/events"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
	merchantsinfra "github.com/swiftdrop/core-api/internal/merchants/infrastructure"
	parcelsapp "github.com/swiftdrop/core-api/internal/parcels/application"
	parcelsinfra "github.com/swiftdrop/core-api/internal/parcels/infrastructure"
	parcelstransport "github.com/swiftdrop/core-api/internal/parcels/transport"
	pricingapp "github.com/swiftdrop/core-api/internal/pricing/application"
	pricinginfra "github.com/swiftdrop/core-api/internal/pricing/infrastructure"
	pricingtransport "github.com/swiftdrop/core-api/internal/pricing/transport"
)

// registerPricingModule wires Member 02 Pricing Management operations into the router.
// Returns an error if dependencies fail validation or handler initialization fails.
func registerPricingModule(mux *http.ServeMux, db *sql.DB, sec *identitytransport.SecurityMiddleware) error {
	if mux == nil {
		return errors.New("pricing: http.ServeMux is required")
	}
	if db == nil {
		return errors.New("pricing: PostgreSQL connection (*sql.DB) is required")
	}
	if sec == nil {
		return errors.New("pricing: security middleware is required")
	}

	// 1. Repositories & Infrastructure
	tariffRepo := pricinginfra.NewPostgresTariffRepository(db)
	quoteRepo := pricinginfra.NewPostgresQuoteRepository(db)
	clock := pricingapp.NewSystemClock()
	idGen := pricinginfra.NewUUIDv7QuoteIDGenerator()
	merchantReader := pricinginfra.NewMerchantRepoAdapter(merchantsinfra.MerchantRepo{}, db)

	// 2. Application Use Cases
	quoteUC := pricingapp.NewGenerateQuoteUseCase(tariffRepo, quoteRepo, clock, idGen)

	// 3. Transport Handler & Routes
	pricingHandler, err := pricingtransport.NewHandler(quoteUC, merchantReader)
	if err != nil {
		return fmt.Errorf("pricing: initialize handler: %w", err)
	}

	pricingtransport.RegisterRoutes(mux, pricingHandler, sec)
	return nil
}

// registerParcelsModule wires Member 02 Parcel Management operations into the router.
// Returns an error if dependencies fail validation or use cases/handlers fail initialization.
func registerParcelsModule(mux *http.ServeMux, db *sql.DB, sec *identitytransport.SecurityMiddleware) error {
	if mux == nil {
		return errors.New("parcels: http.ServeMux is required")
	}
	if db == nil {
		return errors.New("parcels: PostgreSQL connection (*sql.DB) is required")
	}
	if sec == nil {
		return errors.New("parcels: security middleware is required")
	}

	// 1. Repositories & Infrastructure
	txManager := parcelsapp.NewDBTransactionManager(db)
	quoteConsumer := parcelsinfra.NewPostgresQuoteConsumer(db)
	parcelRepo := parcelsinfra.NewPostgresParcelRepository(db)
	statusHistoryRepo := parcelsinfra.NewPostgresStatusHistoryRepository(db)
	idempotencyRepo := parcelsinfra.NewPostgresIdempotencyRepository(db)
	outboxWriter := events.NewPostgresOutboxWriter()
	trackingGen := parcelsapp.NewCryptoTrackingNumberGenerator()
	idGen := parcelsapp.NewUUIDv7Generator()
	clock := parcelsapp.NewSystemClock()
	merchantReader := parcelsinfra.NewMerchantRepoAdapter(merchantsinfra.MerchantRepo{}, db)

	// 2. Application Use Cases
	bookUC, err := parcelsapp.NewBookParcelUseCase(
		txManager,
		quoteConsumer,
		parcelRepo,
		statusHistoryRepo,
		outboxWriter,
		trackingGen,
		idGen,
		clock,
	)
	if err != nil {
		return fmt.Errorf("parcels: initialize book parcel use case: %w", err)
	}
	bookUC = bookUC.WithIdempotency(idempotencyRepo, parcelsapp.DefaultIdempotencyTTL)

	getParcelUC, err := parcelsapp.NewGetParcelUseCase(parcelRepo, merchantReader)
	if err != nil {
		return fmt.Errorf("parcels: initialize get parcel use case: %w", err)
	}

	getHistoryUC, err := parcelsapp.NewGetParcelHistoryUseCase(parcelRepo, statusHistoryRepo, merchantReader)
	if err != nil {
		return fmt.Errorf("parcels: initialize get parcel history use case: %w", err)
	}

	// 3. Transport Handler & Routes
	parcelHandler, err := parcelstransport.NewHandler(bookUC, getParcelUC, getHistoryUC, merchantReader)
	if err != nil {
		return fmt.Errorf("parcels: initialize handler: %w", err)
	}

	parcelstransport.RegisterRoutes(mux, parcelHandler, sec)
	return nil
}

