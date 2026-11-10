package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
)

// BookParcelUseCase orchestrates parcel booking, atomic quote acceptance,
// tracking number generation, aggregate persistence, status audit logging,
// and transactional outbox event publication in one atomic database transaction.
type BookParcelUseCase struct {
	txManager         TransactionManager
	quoteConsumer     QuoteConsumer
	parcelRepo        ParcelRepository
	statusHistoryRepo StatusHistoryRepository
	outboxWriter      events.OutboxWriter
	trackingGen       TrackingNumberGenerator
	idGen             IDGenerator
	clock             Clock
}

// NewBookParcelUseCase constructs a new BookParcelUseCase.
func NewBookParcelUseCase(
	txManager TransactionManager,
	quoteConsumer QuoteConsumer,
	parcelRepo ParcelRepository,
	statusHistoryRepo StatusHistoryRepository,
	outboxWriter events.OutboxWriter,
	trackingGen TrackingNumberGenerator,
	idGen IDGenerator,
	clock Clock,
) (*BookParcelUseCase, error) {
	if txManager == nil {
		return nil, errors.New("book parcel: nil transaction manager")
	}
	if quoteConsumer == nil {
		return nil, errors.New("book parcel: nil quote consumer")
	}
	if parcelRepo == nil {
		return nil, errors.New("book parcel: nil parcel repository")
	}
	if statusHistoryRepo == nil {
		return nil, errors.New("book parcel: nil status history repository")
	}
	if outboxWriter == nil {
		return nil, errors.New("book parcel: nil outbox writer")
	}
	if trackingGen == nil {
		trackingGen = NewCryptoTrackingNumberGenerator()
	}
	if idGen == nil {
		idGen = NewUUIDv7Generator()
	}
	if clock == nil {
		clock = NewSystemClock()
	}

	return &BookParcelUseCase{
		txManager:         txManager,
		quoteConsumer:     quoteConsumer,
		parcelRepo:        parcelRepo,
		statusHistoryRepo: statusHistoryRepo,
		outboxWriter:      outboxWriter,
		trackingGen:       trackingGen,
		idGen:             idGen,
		clock:             clock,
	}, nil
}

// Execute processes a parcel booking command.
func (uc *BookParcelUseCase) Execute(ctx context.Context, cmd BookParcelCommand) (BookParcelResult, error) {
	// 1. Basic command input and ownership validation.
	if strings.TrimSpace(cmd.QuoteID) == "" {
		return BookParcelResult{}, domain.ErrInvalidQuoteID
	}
	senderID, merchantID, err := normalizeBookingOwners(cmd.SenderID, cmd.MerchantID)
	if err != nil {
		return BookParcelResult{}, err
	}
	actorID, actorType, actorRole, err := resolveBookingActor(senderID, merchantID, cmd.ActorType)
	if err != nil {
		return BookParcelResult{}, err
	}
	if strings.TrimSpace(cmd.RecipientName) == "" || strings.TrimSpace(cmd.RecipientPhone) == "" {
		return BookParcelResult{}, domain.ErrInvalidRecipient
	}

	// 2. Physical dimensions and weight validation.
	dim, err := domain.NewDimensions(cmd.Dimensions.LengthCm, cmd.Dimensions.WidthCm, cmd.Dimensions.HeightCm)
	if err != nil {
		return BookParcelResult{}, err
	}

	// Volumetric weight: (L * W * H) / 5000 cm3/kg
	volumetricKg := (cmd.Dimensions.LengthCm * cmd.Dimensions.WidthCm * cmd.Dimensions.HeightCm) / 5000.0
	chargeableKg := math.Max(cmd.WeightKg, volumetricKg)

	weight, err := domain.NewWeight(cmd.WeightKg, volumetricKg, chargeableKg)
	if err != nil {
		return BookParcelResult{}, err
	}

	// 3. Addresses validation (including instructions and ZONE_* format).
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

	now := uc.clock.Now()
	var result BookParcelResult

	// 4. Execute atomic transaction encompassing:
	//    a) Atomic quote consumption and ownership verification
	//    b) Verification of route & weight invariants against the quote
	//    c) Tracking number allocation with bounded collision retry
	//    d) Parcel aggregate construction & persistence
	//    e) Initial CREATED status-history audit record persistence
	//    f) Transactional outbox event publication
	err = uc.txManager.WithTx(ctx, func(tx *sql.Tx) error {
		// a. Atomic quote consumption
		quote, err := uc.quoteConsumer.ConsumeQuote(ctx, tx, cmd.QuoteID, now)
		if err != nil {
			return err
		}

		// a2. Verify quote belongs to the requesting customer/merchant context
		if err := verifyQuoteOwnership(senderID, merchantID, actorType, quote); err != nil {
			return err
		}

		// b. Verify route matches quote
		if pickupAddr.ZoneID != quote.OriginZoneID || dropoffAddr.ZoneID != quote.DestinationZoneID {
			return fmt.Errorf("%w: pickup zone (%s) and dropoff zone (%s) do not match quote origin (%s) and destination (%s)",
				ErrQuoteZoneMismatch, pickupAddr.ZoneID, dropoffAddr.ZoneID, quote.OriginZoneID, quote.DestinationZoneID)
		}

		// c. Verify chargeable weight matches quote
		if math.Abs(weight.ChargeableKg-quote.ChargeableWeightKg) > 1e-4 {
			return fmt.Errorf("%w: calculated chargeable weight (%.2f kg) does not match quote (%.2f kg)",
				ErrQuoteWeightMismatch, weight.ChargeableKg, quote.ChargeableWeightKg)
		}

		// d. Generate unique tracking number with bounded retries
		const maxRetries = 3
		var trackingNumber domain.TrackingNumber
		for i := 0; i < maxRetries; i++ {
			candidate, err := uc.trackingGen.Generate(ctx, now)
			if err != nil {
				return fmt.Errorf("failed to generate tracking number: %w", err)
			}

			exists, err := uc.parcelRepo.ExistsTrackingNumber(ctx, tx, candidate)
			if err != nil {
				return fmt.Errorf("failed to check tracking number uniqueness: %w", err)
			}

			if !exists {
				trackingNumber = candidate
				break
			}
		}

		if trackingNumber == "" {
			return ErrTrackingNumberCollision
		}

		// e. Generate Parcel UUID
		parcelID, err := uc.idGen.GenerateID()
		if err != nil {
			return fmt.Errorf("failed to generate parcel id: %w", err)
		}

		// f. Construct immutable delivery fee and SLA tier from quote snapshot
		fee, err := domain.NewMoney(quote.FeeCents, quote.Currency)
		if err != nil {
			return fmt.Errorf("corrupt quote delivery fee: %w", err)
		}
		serviceLevel := domain.ServiceLevel(quote.ServiceLevel)

		// g. Create Parcel aggregate root
		parcel, err := domain.NewParcel(domain.CreateParcelParams{
			ID:                 parcelID,
			TrackingNumber:     trackingNumber,
			SenderID:           senderID,
			MerchantID:         merchantID,
			RecipientName:      cmd.RecipientName,
			RecipientPhone:     cmd.RecipientPhone,
			QuoteID:            quote.QuoteID,
			PricingRuleID:      quote.TariffID,
			PricingRuleVersion: quote.TariffVersion,
			DeliveryFee:        fee,
			OriginZoneID:       quote.OriginZoneID,
			DestinationZoneID:  quote.DestinationZoneID,
			ServiceLevel:       serviceLevel,
			Weight:             weight,
			Dimensions:         dim,
			PickupAddress:      pickupAddr,
			DropoffAddress:     dropoffAddr,
			CreatedAt:          now,
		})
		if err != nil {
			return fmt.Errorf("failed to construct parcel aggregate: %w", err)
		}

		// h. Persist parcel
		if err := uc.parcelRepo.Save(ctx, tx, parcel); err != nil {
			return fmt.Errorf("failed to persist parcel: %w", err)
		}

		// i. Create and persist initial CREATED status-history entry
		historyID, err := uc.idGen.GenerateID()
		if err != nil {
			return fmt.Errorf("failed to generate status history id: %w", err)
		}

		historyEntry, err := domain.NewStatusHistoryEntry(
			historyID,
			parcel.ID,
			nil, // fromStatus is nil for initial booking creation
			domain.StatusCreated,
			actorID,
			actorType,
			actorRole,
			"Initial parcel booking",
			now,
			0,
		)
		if err != nil {
			return fmt.Errorf("failed to construct status history entry: %w", err)
		}

		if err := uc.statusHistoryRepo.Save(ctx, tx, historyEntry); err != nil {
			return fmt.Errorf("failed to persist status history entry: %w", err)
		}

		// j. Publish parcel.booked.v1 event to transactional outbox
		payload := ParcelBookedPayload{
			ParcelID:           parcel.ID,
			TrackingNumber:     parcel.TrackingNumber.String(),
			SenderID:           parcel.SenderID,
			MerchantID:         parcel.MerchantID,
			RecipientName:      parcel.RecipientName,
			RecipientPhone:     parcel.RecipientPhone,
			QuoteID:            parcel.QuoteID,
			PricingRuleID:      parcel.PricingRuleID,
			PricingRuleVersion: parcel.PricingRuleVersion,
			DeliveryFeeCents:   parcel.DeliveryFee.Amount,
			Currency:           parcel.DeliveryFee.Currency,
			OriginZoneID:       parcel.OriginZoneID,
			DestinationZoneID:  parcel.DestinationZoneID,
			ServiceLevel:       string(parcel.ServiceLevel),
			ActualWeightKg:     parcel.Weight.ActualKg,
			ChargeableWeightKg: parcel.Weight.ChargeableKg,
			Status:             string(parcel.Status),
			CreatedAt:          parcel.CreatedAt,
		}

		corrID := strings.TrimSpace(cmd.CorrelationID)
		if corrID == "" {
			corrID = parcel.ID
		}

		envelope, err := events.NewEnvelope(
			events.ParcelBookedV1,
			"parcel",
			parcel.ID,
			parcel.Version,
			corrID,
			"core-api.parcels",
			payload,
		)
		if err != nil {
			return fmt.Errorf("failed to build outbox envelope: %w", err)
		}

		if err := uc.outboxWriter.Write(ctx, tx, envelope); err != nil {
			return fmt.Errorf("failed to write outbox event: %w", err)
		}

		result = BookParcelResult{
			Parcel:        parcel,
			StatusHistory: historyEntry,
		}

		return nil
	})

	if err != nil {
		return BookParcelResult{}, err
	}

	return result, nil
}

func normalizeOptionalID(id *string, invalidErr error) (*string, error) {
	if id == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*id)
	if trimmed == "" {
		return nil, invalidErr
	}
	return &trimmed, nil
}

func normalizeBookingOwners(senderID, merchantID *string) (*string, *string, error) {
	normSender, err := normalizeOptionalID(senderID, domain.ErrInvalidSenderID)
	if err != nil {
		return nil, nil, err
	}
	normMerchant, err := normalizeOptionalID(merchantID, domain.ErrInvalidMerchantID)
	if err != nil {
		return nil, nil, err
	}
	if normSender == nil && normMerchant == nil {
		return nil, nil, domain.ErrInvalidOwner
	}
	return normSender, normMerchant, nil
}

func resolveBookingActor(senderID, merchantID *string, requestedType domain.ActorType) (*string, domain.ActorType, string, error) {
	var (
		actorID   *string
		actorType domain.ActorType
		actorRole string
	)

	if requestedType == "" {
		if senderID != nil {
			actorType = domain.ActorTypeUser
			actorID = senderID
			actorRole = "CUSTOMER"
			if merchantID != nil {
				actorRole = "MERCHANT"
			}
		} else {
			actorType = domain.ActorTypeMerchantAPI
			actorID = merchantID
			actorRole = "MERCHANT"
		}
	} else {
		if err := requestedType.Validate(); err != nil {
			return nil, "", "", err
		}
		switch requestedType {
		case domain.ActorTypeSystem:
			return nil, "", "", fmt.Errorf("%w: parcel booking cannot use SYSTEM actor type", domain.ErrInvalidActorType)
		case domain.ActorTypeUser:
			if senderID == nil {
				return nil, "", "", fmt.Errorf("%w: USER booking requires sender_id", domain.ErrInvalidSenderID)
			}
			actorType = domain.ActorTypeUser
			actorID = senderID
			actorRole = "CUSTOMER"
			if merchantID != nil {
				actorRole = "MERCHANT"
			}
		case domain.ActorTypeMerchantAPI:
			if merchantID == nil {
				return nil, "", "", fmt.Errorf("%w: MERCHANT_API booking requires merchant_id", domain.ErrInvalidMerchantID)
			}
			actorType = domain.ActorTypeMerchantAPI
			actorID = merchantID
			actorRole = "MERCHANT"
		}
	}

	if actorID != nil && strings.EqualFold(*actorID, string(domain.ActorTypeSystem)) {
		return nil, "", "", fmt.Errorf("%w: literal %q is not a valid actor_id", domain.ErrInvalidActorID, *actorID)
	}

	return actorID, actorType, actorRole, nil
}

func verifyQuoteOwnership(
	senderID, merchantID *string,
	actorType domain.ActorType,
	quote AcceptedQuoteSnapshot,
) error {
	quoteRequester, err := normalizeOptionalID(quote.RequesterID, ErrQuoteOwnershipMismatch)
	if err != nil {
		return fmt.Errorf("%w: quote contains blank requester_id", err)
	}
	quoteMerchant, err := normalizeOptionalID(quote.MerchantID, ErrQuoteOwnershipMismatch)
	if err != nil {
		return fmt.Errorf("%w: quote contains blank merchant_id", err)
	}
	if quoteRequester == nil && quoteMerchant == nil {
		return fmt.Errorf("%w: quote snapshot is missing ownership metadata", ErrQuoteOwnershipMismatch)
	}

	switch actorType {
	case domain.ActorTypeMerchantAPI:
		if merchantID == nil || quoteMerchant == nil || *quoteMerchant != *merchantID {
			return fmt.Errorf("%w: quote does not belong to requesting merchant", ErrQuoteOwnershipMismatch)
		}
		if senderID != nil && quoteRequester != nil && *quoteRequester != *senderID {
			return fmt.Errorf("%w: quote requester does not match booking sender", ErrQuoteOwnershipMismatch)
		}
		return nil

	case domain.ActorTypeUser:
		if merchantID == nil {
			if quoteMerchant != nil {
				return fmt.Errorf("%w: merchant-scoped quote cannot be booked without matching merchant", ErrQuoteOwnershipMismatch)
			}
			if senderID == nil || quoteRequester == nil || *quoteRequester != *senderID {
				return fmt.Errorf("%w: quote does not belong to requesting customer", ErrQuoteOwnershipMismatch)
			}
			return nil
		}

		if quoteMerchant == nil || *quoteMerchant != *merchantID {
			return fmt.Errorf("%w: quote does not belong to requesting merchant", ErrQuoteOwnershipMismatch)
		}
		if quoteRequester != nil && (senderID == nil || *quoteRequester != *senderID) {
			return fmt.Errorf("%w: quote requester does not match requesting user", ErrQuoteOwnershipMismatch)
		}
		return nil

	default:
		return ErrQuoteOwnershipMismatch
	}
}
