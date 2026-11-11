package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

// GenerateQuoteCommand contains the information required
// to generate a delivery pricing quote.
type GenerateQuoteCommand struct {
	OriginZoneID      string
	DestinationZoneID string
	ActualWeightKg    float64
	LengthCm          float64
	WidthCm           float64
	HeightCm          float64
	ServiceLevel      domain.ServiceLevel
	RequesterID       *string
	MerchantID        *string
}

// GenerateQuoteUseCase coordinates the quote generation workflow.
type GenerateQuoteUseCase struct {
	tariffRepo  TariffRepository
	quoteRepo   QuoteRepository
	clock       Clock
	idGenerator QuoteIDGenerator
}

// NewGenerateQuoteUseCase creates a new quote generation use case.
func NewGenerateQuoteUseCase(
	tariffRepo TariffRepository,
	quoteRepo QuoteRepository,
	clock Clock,
	idGenerator QuoteIDGenerator,
) *GenerateQuoteUseCase {
	return &GenerateQuoteUseCase{
		tariffRepo:  tariffRepo,
		quoteRepo:   quoteRepo,
		clock:       clock,
		idGenerator: idGenerator,
	}
}

// Execute generates a delivery quote for the supplied parcel details.
func (uc *GenerateQuoteUseCase) Execute(
	ctx context.Context,
	cmd GenerateQuoteCommand,
) (domain.Quote, error) {

	// 1. Validate the requested service level.
	if err := cmd.ServiceLevel.Validate(); err != nil {
		return domain.Quote{}, err
	}

	// 2. Validate the required location information.
	if err := domain.ValidateZoneID(cmd.OriginZoneID); err != nil {
		return domain.Quote{}, fmt.Errorf("%w: origin zone %w", domain.ErrInvalidTariff, err)
	}
	if err := domain.ValidateZoneID(cmd.DestinationZoneID); err != nil {
		return domain.Quote{}, fmt.Errorf("%w: destination zone %w", domain.ErrInvalidTariff, err)
	}

	// 2b. Validate quote requester/merchant ownership before querying persistence.
	if (cmd.RequesterID == nil && cmd.MerchantID == nil) ||
		(cmd.RequesterID != nil && strings.TrimSpace(*cmd.RequesterID) == "") ||
		(cmd.MerchantID != nil && strings.TrimSpace(*cmd.MerchantID) == "") {
		return domain.Quote{}, domain.ErrInvalidQuote
	}

	// 3. Calculate the parcel's chargeable weight.
	weight, err := domain.CalculateWeight(
		cmd.ActualWeightKg,
		cmd.LengthCm,
		cmd.WidthCm,
		cmd.HeightCm,
	)
	if err != nil {
		return domain.Quote{}, err
	}

	// 4. Get the current time.
	now := uc.clock.Now()

	// 5. Find the applicable tariff.
	tariff, err := uc.tariffRepo.FindApplicable(
		ctx,
		cmd.OriginZoneID,
		cmd.DestinationZoneID,
		now,
	)
	if err != nil {
		return domain.Quote{}, fmt.Errorf(
			"find applicable tariff: %w",
			err,
		)
	}

	// 6. Validate the retrieved tariff.
	if err := tariff.Validate(); err != nil {
		return domain.Quote{}, err
	}

	if tariff.OriginZoneID != cmd.OriginZoneID ||
		tariff.DestinationZoneID != cmd.DestinationZoneID ||
		!tariff.IsEffectiveAt(now) {
		return domain.Quote{}, domain.ErrInvalidTariff
	}

	// 7. Calculate the delivery fee using domain rules.
	fee, err := domain.CalculateDeliveryFee(tariff, weight, cmd.ServiceLevel)
	if err != nil {
		return domain.Quote{}, err
	}

	// 8. Generate a unique quote ID.
	quoteID, err := uc.idGenerator.NewID()
	if err != nil {
		return domain.Quote{}, fmt.Errorf(
			"generate quote ID: %w",
			err,
		)
	}

	// 9. Create the quote using the domain constructor.
	quote, err := domain.NewQuote(
		quoteID,
		tariff,
		weight,
		cmd.ServiceLevel,
		fee,
		now,
		cmd.RequesterID,
		cmd.MerchantID,
	)
	if err != nil {
		return domain.Quote{}, err
	}

	// 10. Persist the generated quote.
	if err := uc.quoteRepo.Save(ctx, quote); err != nil {
		return domain.Quote{}, fmt.Errorf(
			"save pricing quote: %w",
			err,
		)
	}

	// 11. Return the generated quote.
	return quote, nil
}
