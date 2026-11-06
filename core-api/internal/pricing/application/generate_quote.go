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
	PickupZone      string
	DestinationZone string
	ActualWeightKg  float64
	LengthCm        float64
	WidthCm         float64
	HeightCm        float64
	ServiceLevel    domain.ServiceLevel
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

	// 1. Validate the required location information.
	if strings.TrimSpace(cmd.PickupZone) == "" ||
		strings.TrimSpace(cmd.DestinationZone) == "" {
		return domain.Quote{}, domain.ErrInvalidTariff
	}

	// 2. Calculate the parcel's chargeable weight.
	weight, err := domain.CalculateWeight(
		cmd.ActualWeightKg,
		cmd.LengthCm,
		cmd.WidthCm,
		cmd.HeightCm,
	)
	if err != nil {
		return domain.Quote{}, err
	}

	// 3. Get the current time.
	now := uc.clock.Now()

	// 4. Find the applicable tariff.
	tariff, err := uc.tariffRepo.FindApplicable(
		ctx,
		cmd.PickupZone,
		cmd.DestinationZone,
		cmd.ServiceLevel,
		now,
	)
	if err != nil {
		return domain.Quote{}, fmt.Errorf(
			"find applicable tariff: %w",
			err,
		)
	}

	// 5. Validate the retrieved tariff.
	if err := tariff.Validate(); err != nil {
		return domain.Quote{}, err
	}

	if tariff.PickupZone != cmd.PickupZone ||
		tariff.DestinationZone != cmd.DestinationZone ||
		tariff.ServiceLevel != cmd.ServiceLevel ||
		!tariff.IsEffectiveAt(now) {
		return domain.Quote{}, domain.ErrInvalidTariff
	}

	// 6. Calculate the delivery fee using domain rules.
	fee, err := domain.CalculateDeliveryFee(tariff, weight)
	if err != nil {
		return domain.Quote{}, err
	}

	// 7. Generate a unique quote ID.
	quoteID, err := uc.idGenerator.NewID()
	if err != nil {
		return domain.Quote{}, fmt.Errorf(
			"generate quote ID: %w",
			err,
		)
	}

	// 8. Create the quote using the domain constructor.
	quote, err := domain.NewQuote(
		quoteID,
		tariff,
		weight,
		fee,
		now,
	)
	if err != nil {
		return domain.Quote{}, err
	}

	// 9. Persist the generated quote.
	if err := uc.quoteRepo.Save(ctx, quote); err != nil {
		return domain.Quote{}, fmt.Errorf(
			"save pricing quote: %w",
			err,
		)
	}

	// 10. Return the generated quote.
	return quote, nil
}
