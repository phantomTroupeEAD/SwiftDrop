package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
	pricingdomain "github.com/swiftdrop/core-api/internal/pricing/domain"
)

// BookMerchantParcelCommand carries the authoritative parameters for B2B merchant parcel booking.
type BookMerchantParcelCommand struct {
	MerchantID          string
	ServiceLevel        string
	RecipientName       string
	RecipientPhone      string
	WeightKg            float64
	Dimensions          DimensionsDTO
	PickupAddress       AddressDTO
	DropoffAddress      AddressDTO
	ItemDescription     string
	SpecialInstructions string
	CorrelationID       string
	Idempotency         *IdempotencyInput
}

// BookMerchantParcelUseCase orchestrates merchant parcel ingestion by calculating delivery price
// dynamically using the authoritative Pricing module, persisting an internal consumed quote,
// and creating the parcel within a single atomic database transaction.
type BookMerchantParcelUseCase struct {
	txManager        TransactionManager
	tariffRepo       TariffRepository
	quoteCreator     InternalQuoteCreator
	bookParcelUC     *BookParcelUseCase
	idGen            IDGenerator
	clock            Clock
	idempotencyStore IdempotencyStore
	idempotencyTTL   time.Duration
}

// NewBookMerchantParcelUseCase constructs a new BookMerchantParcelUseCase.
func NewBookMerchantParcelUseCase(
	txManager TransactionManager,
	tariffRepo TariffRepository,
	quoteCreator InternalQuoteCreator,
	bookParcelUC *BookParcelUseCase,
	idGen IDGenerator,
	clock Clock,
) (*BookMerchantParcelUseCase, error) {
	if txManager == nil {
		return nil, errors.New("book merchant parcel: nil transaction manager")
	}
	if tariffRepo == nil {
		return nil, errors.New("book merchant parcel: nil tariff repository")
	}
	if quoteCreator == nil {
		return nil, errors.New("book merchant parcel: nil quote creator")
	}
	if bookParcelUC == nil {
		return nil, errors.New("book merchant parcel: nil book parcel use case")
	}
	if idGen == nil {
		idGen = NewUUIDv7Generator()
	}
	if clock == nil {
		clock = NewSystemClock()
	}

	return &BookMerchantParcelUseCase{
		txManager:      txManager,
		tariffRepo:     tariffRepo,
		quoteCreator:   quoteCreator,
		bookParcelUC:   bookParcelUC,
		idGen:          idGen,
		clock:          clock,
		idempotencyTTL: DefaultIdempotencyTTL,
	}, nil
}

// WithIdempotency configures persistent request idempotency for BookMerchantParcelUseCase.
func (uc *BookMerchantParcelUseCase) WithIdempotency(store IdempotencyStore, ttl time.Duration) *BookMerchantParcelUseCase {
	uc.idempotencyStore = store
	if ttl > 0 {
		uc.idempotencyTTL = ttl
	} else {
		uc.idempotencyTTL = DefaultIdempotencyTTL
	}
	if uc.bookParcelUC != nil {
		uc.bookParcelUC.WithIdempotency(store, uc.idempotencyTTL)
	}
	return uc
}

// Execute processes an authoritative merchant parcel booking command.
func (uc *BookMerchantParcelUseCase) Execute(ctx context.Context, cmd BookMerchantParcelCommand) (BookParcelResult, error) {
	// 1. Validate Merchant ID
	cleanMerchantID := strings.TrimSpace(cmd.MerchantID)
	if cleanMerchantID == "" || !events.IsValidUUID(cleanMerchantID) {
		return BookParcelResult{}, domain.ErrInvalidMerchantID
	}
	merchantID := cleanMerchantID

	// 2. Validate Service Level (default to STANDARD if blank)
	svcLevelStr := strings.ToUpper(strings.TrimSpace(cmd.ServiceLevel))
	if svcLevelStr == "" {
		svcLevelStr = string(domain.ServiceLevelStandard)
	}
	pricingSvcLevel := pricingdomain.ServiceLevel(svcLevelStr)
	if err := pricingSvcLevel.Validate(); err != nil {
		return BookParcelResult{}, domain.ErrInvalidServiceLevel
	}
	parcelSvcLevel := domain.ServiceLevel(svcLevelStr)

	// 3. Validate Dimensions
	dim, err := domain.NewDimensions(cmd.Dimensions.LengthCm, cmd.Dimensions.WidthCm, cmd.Dimensions.HeightCm)
	if err != nil {
		return BookParcelResult{}, err
	}

	// 4. Calculate Weight using existing Pricing Domain logic (enforcing 5000 divisor and 30 kg limit)
	pricingWeight, err := pricingdomain.CalculateWeight(cmd.WeightKg, cmd.Dimensions.LengthCm, cmd.Dimensions.WidthCm, cmd.Dimensions.HeightCm)
	if err != nil {
		if errors.Is(err, pricingdomain.ErrChargeableWeightExceeded) {
			return BookParcelResult{}, fmt.Errorf("%w: chargeable weight %.2f kg exceeds maximum limit of %.2f kg",
				domain.ErrInvalidWeight, pricingWeight.ChargeableKg, domain.MaxChargeableWeightKg)
		}
		if errors.Is(err, pricingdomain.ErrInvalidDimensions) {
			return BookParcelResult{}, domain.ErrInvalidDimensions
		}
		if errors.Is(err, pricingdomain.ErrInvalidWeight) {
			return BookParcelResult{}, domain.ErrInvalidWeight
		}
		return BookParcelResult{}, err
	}

	parcelWeight, err := domain.NewWeight(pricingWeight.ActualKg, pricingWeight.VolumetricKg, pricingWeight.ChargeableKg)
	if err != nil {
		return BookParcelResult{}, err
	}

	// 5. Validate Addresses
	pickupAddr, err := domain.NewAddress(
		cmd.PickupAddress.ContactName,
		cmd.PickupAddress.ContactPhone,
		cmd.PickupAddress.StreetLine1,
		cmd.PickupAddress.StreetLine2,
		cmd.PickupAddress.City,
		cmd.PickupAddress.PostalCode,
		cmd.PickupAddress.ZoneID,
		cmd.PickupAddress.Instructions,
	)
	if err != nil {
		return BookParcelResult{}, fmt.Errorf("pickup address: %w", err)
	}

	dropoffAddr, err := domain.NewAddress(
		cmd.DropoffAddress.ContactName,
		cmd.DropoffAddress.ContactPhone,
		cmd.DropoffAddress.StreetLine1,
		cmd.DropoffAddress.StreetLine2,
		cmd.DropoffAddress.City,
		cmd.DropoffAddress.PostalCode,
		cmd.DropoffAddress.ZoneID,
		cmd.DropoffAddress.Instructions,
	)
	if err != nil {
		return BookParcelResult{}, fmt.Errorf("dropoff address: %w", err)
	}

	// 6. Recipient Contact Info Fallback
	recipientName := strings.TrimSpace(cmd.RecipientName)
	if recipientName == "" {
		recipientName = dropoffAddr.ContactName
	}
	recipientPhone := strings.TrimSpace(cmd.RecipientPhone)
	if recipientPhone == "" {
		recipientPhone = dropoffAddr.ContactPhone
	}
	if recipientName == "" || recipientPhone == "" {
		return BookParcelResult{}, domain.ErrInvalidRecipient
	}

	// 7. Validate Idempotency
	idemInput, err := uc.validateMerchantIdempotency(cmd.Idempotency, merchantID)
	if err != nil {
		return BookParcelResult{}, err
	}

	now := uc.clock.Now().UTC()
	var result BookParcelResult

	actorID := merchantID
	actorType := domain.ActorTypeMerchantAPI
	actorRole := "MERCHANT"

	// 8. Execute atomic transaction encompassing:
	//    a) Idempotency check before pricing/quote creation
	//    b) Authoritative tariff lookup
	//    c) Authoritative delivery fee calculation
	//    d) Internal quote creation marked consumed immediately
	//    e) Internal quote transactional persistence
	//    f) Shared parcel persistence pipeline (tracking, aggregate, status history, outbox, idempotency)
	err = uc.txManager.WithTx(ctx, func(tx *sql.Tx) error {
		// a. Idempotency replay check BEFORE quote creation or tariff lookup
		if idemInput != nil {
			replayed, hit, err := uc.bookParcelUC.checkIdempotencyReplay(ctx, tx, *idemInput, now)
			if err != nil {
				return err
			}
			if hit {
				result = replayed
				return nil
			}
		}

		// b. Authoritative tariff lookup for route
		tariff, err := uc.tariffRepo.FindApplicable(ctx, pickupAddr.ZoneID, dropoffAddr.ZoneID, now)
		if err != nil {
			return fmt.Errorf("find applicable tariff: %w", err)
		}
		if err := tariff.Validate(); err != nil {
			return fmt.Errorf("invalid tariff: %w", err)
		}
		if !tariff.IsEffectiveAt(now) {
			return domain.ErrInvalidPricingRule
		}

		// c. Authoritative delivery fee calculation
		fee, err := pricingdomain.CalculateDeliveryFee(tariff, pricingWeight, pricingSvcLevel)
		if err != nil {
			return fmt.Errorf("calculate delivery fee: %w", err)
		}

		parcelFee, err := domain.NewMoney(fee.Amount, fee.Currency)
		if err != nil {
			return fmt.Errorf("corrupt delivery fee: %w", err)
		}

		// d. Generate internal Quote and mark used immediately
		quoteID, err := uc.idGen.GenerateID()
		if err != nil {
			return fmt.Errorf("failed to generate quote id: %w", err)
		}

		internalQuote, err := pricingdomain.NewQuote(
			quoteID,
			tariff,
			pricingWeight,
			pricingSvcLevel,
			fee,
			now,
			nil,
			&merchantID,
		)
		if err != nil {
			return fmt.Errorf("construct internal quote: %w", err)
		}

		if err := internalQuote.MarkUsed(now); err != nil {
			return fmt.Errorf("mark internal quote used: %w", err)
		}

		// e. Persist internal quote within this transaction
		if err := uc.quoteCreator.SaveConsumedQuote(ctx, tx, internalQuote); err != nil {
			return fmt.Errorf("persist internal quote: %w", err)
		}

		// f. Call shared parcel booking persistence pipeline
		res, err := uc.bookParcelUC.persistParcelBookingTx(ctx, tx, persistParcelBookingParams{
			Now:                now,
			SenderID:           nil,
			MerchantID:         &merchantID,
			ActorID:            &actorID,
			ActorType:          actorType,
			ActorRole:          actorRole,
			RecipientName:      recipientName,
			RecipientPhone:     recipientPhone,
			QuoteID:            internalQuote.ID,
			PricingRuleID:      tariff.ID,
			PricingRuleVersion: tariff.Version,
			DeliveryFee:        parcelFee,
			OriginZoneID:       pickupAddr.ZoneID,
			DestinationZoneID:  dropoffAddr.ZoneID,
			ServiceLevel:       parcelSvcLevel,
			Weight:             parcelWeight,
			Dimensions:         dim,
			PickupAddress:      pickupAddr,
			DropoffAddress:     dropoffAddr,
			CorrelationID:      cmd.CorrelationID,
			IdemInput:          idemInput,
		})
		if err != nil {
			return err
		}

		result = res
		return nil
	})

	if err != nil {
		return BookParcelResult{}, err
	}

	return result, nil
}

func (uc *BookMerchantParcelUseCase) validateMerchantIdempotency(in *IdempotencyInput, merchantID string) (*IdempotencyInput, error) {
	if in == nil {
		return nil, nil
	}
	if uc.idempotencyStore == nil && (uc.bookParcelUC == nil || uc.bookParcelUC.idempotencyStore == nil) {
		return nil, ErrNilIdempotencyStore
	}
	normInput := *in
	if strings.TrimSpace(normInput.CallerScope) == "" {
		normInput.CallerScope = "MERCHANT:" + merchantID
	}
	if strings.TrimSpace(normInput.Operation) == "" {
		normInput.Operation = DefaultMerchantBookingOperation
	}
	norm, err := normInput.Normalize()
	if err != nil {
		return nil, err
	}
	if err := verifyCallerScopeMatchesOwners(norm.CallerScope, nil, &merchantID); err != nil {
		return nil, err
	}
	return &norm, nil
}
