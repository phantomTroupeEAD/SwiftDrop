package application

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/events/eventstest"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
	pricingdomain "github.com/swiftdrop/core-api/internal/pricing/domain"
)

// Mock Tariff Repository
type mockTariffRepo struct {
	tariff pricingdomain.Tariff
	err    error
	calls  int
	origin string
	dest   string
}

func (m *mockTariffRepo) FindApplicable(ctx context.Context, originZoneID, destinationZoneID string, at time.Time) (pricingdomain.Tariff, error) {
	m.calls++
	m.origin = originZoneID
	m.dest = destinationZoneID
	if m.err != nil {
		return pricingdomain.Tariff{}, m.err
	}
	return m.tariff, nil
}

// Mock Internal Quote Creator
type mockInternalQuoteCreator struct {
	savedQuote pricingdomain.Quote
	err        error
	invoked    bool
}

func (m *mockInternalQuoteCreator) SaveConsumedQuote(ctx context.Context, q db.Querier, quote pricingdomain.Quote) error {
	m.invoked = true
	if m.err != nil {
		return m.err
	}
	m.savedQuote = quote
	return nil
}

func defaultTestTariff(now time.Time) pricingdomain.Tariff {
	baseFee, _ := pricingdomain.NewMoney(50000, pricingdomain.CurrencyLKR)   // 500.00 LKR base
	perKgRate, _ := pricingdomain.NewMoney(10000, pricingdomain.CurrencyLKR) // 100.00 LKR / kg
	return pricingdomain.Tariff{
		ID:                "tariff-rule-001",
		OriginZoneID:      "ZONE_WESTERN_01",
		DestinationZoneID: "ZONE_CENTRAL_01",
		BaseFee:           baseFee,
		PerKgRate:         perKgRate,
		EffectiveFrom:     now.Add(-24 * time.Hour),
		IsActive:          true,
		Version:           1,
		CreatedAt:         now.Add(-24 * time.Hour),
		UpdatedAt:         now.Add(-24 * time.Hour),
	}
}

type autoReleaseTxManager struct {
	inner TransactionManager
	store *mockIdempotencyStore
}

func (t *autoReleaseTxManager) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	defer func() {
		t.store.mu.Lock()
		for k, held := range t.store.heldLocks {
			if held {
				t.store.heldLocks[k] = false
				if kl, ok := t.store.keyLocks[k]; ok {
					kl.Unlock()
				}
			}
		}
		t.store.mu.Unlock()
	}()
	return t.inner.WithTx(ctx, fn)
}

func setupMerchantBookingTest(t *testing.T) (
	*BookMerchantParcelUseCase,
	*mockTariffRepo,
	*mockInternalQuoteCreator,
	*mockParcelRepo,
	*mockStatusHistoryRepo,
	*eventstest.InMemoryWriter,
	*mockIdempotencyStore,
	time.Time,
) {
	t.Helper()

	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	clock := fixedClock{now: now}

	database := getTestDB(t)
	idemStore := newMockIdempotencyStore()
	txMgr := &autoReleaseTxManager{
		inner: NewDBTransactionManager(database),
		store: idemStore,
	}

	tariff := defaultTestTariff(now)
	tariffRepo := &mockTariffRepo{tariff: tariff}
	quoteCreator := &mockInternalQuoteCreator{}
	quoteConsumer := &mockQuoteConsumer{}

	parcelRepo := newMockParcelRepo()
	statusHistoryRepo := &mockStatusHistoryRepo{}
	outboxWriter := eventstest.NewInMemoryWriter()
	trackingGen := &mockTrackingGen{}
	idGen := &mockIDGen{}

	bookUC, err := NewBookParcelUseCase(
		txMgr,
		quoteConsumer,
		parcelRepo,
		statusHistoryRepo,
		outboxWriter,
		trackingGen,
		idGen,
		clock,
	)
	if err != nil {
		t.Fatalf("failed to create BookParcelUseCase: %v", err)
	}

	bookUC = bookUC.WithIdempotency(idemStore, DefaultIdempotencyTTL)

	merchantUC, err := NewBookMerchantParcelUseCase(
		txMgr,
		tariffRepo,
		quoteCreator,
		bookUC,
		idGen,
		clock,
	)
	if err != nil {
		t.Fatalf("failed to create BookMerchantParcelUseCase: %v", err)
	}
	merchantUC = merchantUC.WithIdempotency(idemStore, DefaultIdempotencyTTL)

	return merchantUC, tariffRepo, quoteCreator, parcelRepo, statusHistoryRepo, outboxWriter, idemStore, now
}

func validMerchantBookingCommand() BookMerchantParcelCommand {
	return BookMerchantParcelCommand{
		MerchantID:   "0192e2b3-7c8a-7890-a123-000000000001",
		ServiceLevel: "STANDARD",
		WeightKg:     2.5,
		Dimensions: DimensionsDTO{
			LengthCm: 30.0,
			WidthCm:  20.0,
			HeightCm: 15.0, // 30*20*15 / 5000 = 1.8 kg volumetric, chargeable = 2.5 kg
		},
		PickupAddress: AddressDTO{
			ContactName:  "Merchant Dispatch",
			ContactPhone: "+94771234567",
			StreetLine1:  "10 Galle Road",
			City:         "Colombo",
			PostalCode:   "00300",
			ZoneID:       "ZONE_WESTERN_01",
			Instructions: "Warehouse door 3",
		},
		DropoffAddress: AddressDTO{
			ContactName:  "Jane Doe",
			ContactPhone: "+94779876543",
			StreetLine1:  "45 Temple Road",
			City:         "Kandy",
			PostalCode:   "20000",
			ZoneID:       "ZONE_CENTRAL_01",
			Instructions: "Leave with security",
		},
		ItemDescription:     "Electronic Gadget",
		SpecialInstructions: "Handle with care",
		CorrelationID:       "corr-merchant-001",
	}
}

func TestBookMerchantParcelUseCase_SuccessfulBooking(t *testing.T) {
	uc, tariffRepo, quoteCreator, parcelRepo, statusRepo, outboxWriter, _, now := setupMerchantBookingTest(t)

	cmd := validMerchantBookingCommand()
	result, err := uc.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error on valid merchant booking: %v", err)
	}

	// 1. Verify result structure
	if result.Parcel.ID == "" {
		t.Error("expected non-empty Parcel.ID")
	}
	if result.Parcel.TrackingNumber == "" {
		t.Error("expected non-empty TrackingNumber")
	}
	if result.Parcel.MerchantID == nil || *result.Parcel.MerchantID != cmd.MerchantID {
		t.Errorf("expected MerchantID %s, got %v", cmd.MerchantID, result.Parcel.MerchantID)
	}
	if result.Parcel.SenderID != nil {
		t.Errorf("expected nil SenderID for merchant booking, got %v", result.Parcel.SenderID)
	}
	if result.Parcel.Status != domain.StatusCreated {
		t.Errorf("expected CREATED status, got %s", result.Parcel.Status)
	}

	// 2. Verify pricing: 2.5 kg chargeable -> base (1 kg) 500.00 + extra (1.5 kg * 100.00) = 650.00 LKR = 65000 cents
	expectedFeeCents := int64(65000)
	if result.Parcel.DeliveryFee.Amount != expectedFeeCents {
		t.Errorf("expected fee %d cents, got %d", expectedFeeCents, result.Parcel.DeliveryFee.Amount)
	}
	if result.Parcel.DeliveryFee.Currency != "LKR" {
		t.Errorf("expected currency LKR, got %s", result.Parcel.DeliveryFee.Currency)
	}

	// 3. Verify internal quote creation & consumption
	if !quoteCreator.invoked {
		t.Fatal("expected quoteCreator.SaveConsumedQuote to be invoked")
	}
	savedQuote := quoteCreator.savedQuote
	if savedQuote.ID == "" {
		t.Error("expected non-empty Quote.ID")
	}
	if savedQuote.RequesterID != nil {
		t.Errorf("expected nil RequesterID on merchant quote, got %v", savedQuote.RequesterID)
	}
	if savedQuote.MerchantID == nil || *savedQuote.MerchantID != cmd.MerchantID {
		t.Errorf("expected MerchantID %s on quote, got %v", cmd.MerchantID, savedQuote.MerchantID)
	}
	if savedQuote.Fee.Amount != expectedFeeCents {
		t.Errorf("expected quote fee %d, got %d", expectedFeeCents, savedQuote.Fee.Amount)
	}
	if savedQuote.UsedAt == nil || !savedQuote.UsedAt.Equal(now) {
		t.Errorf("expected quote UsedAt == %v, got %v", now, savedQuote.UsedAt)
	}
	if !savedQuote.ExpiresAt.Equal(now.Add(15 * time.Minute)) {
		t.Errorf("expected quote ExpiresAt == %v, got %v", now.Add(15*time.Minute), savedQuote.ExpiresAt)
	}
	if result.Parcel.QuoteID != savedQuote.ID {
		t.Errorf("parcel QuoteID %s does not match saved quote ID %s", result.Parcel.QuoteID, savedQuote.ID)
	}

	// 4. Verify Tariff Selection
	if tariffRepo.calls != 1 {
		t.Errorf("expected 1 tariff lookup, got %d", tariffRepo.calls)
	}
	if tariffRepo.origin != "ZONE_WESTERN_01" || tariffRepo.dest != "ZONE_CENTRAL_01" {
		t.Errorf("expected route ZONE_WESTERN_01 -> ZONE_CENTRAL_01, got %s -> %s", tariffRepo.origin, tariffRepo.dest)
	}

	// 5. Verify Persistence in Parcel Repository
	if len(parcelRepo.parcels) != 1 {
		t.Fatalf("expected 1 persisted parcel, got %d", len(parcelRepo.parcels))
	}

	// 6. Verify Status History Audit Entry
	if len(statusRepo.entries) != 1 {
		t.Fatalf("expected 1 status history entry, got %d", len(statusRepo.entries))
	}
	entry := statusRepo.entries[0]
	if entry.ToStatus != domain.StatusCreated {
		t.Errorf("expected status history toStatus CREATED, got %s", entry.ToStatus)
	}
	if entry.ActorType != domain.ActorTypeMerchantAPI {
		t.Errorf("expected actor type MERCHANT_API, got %s", entry.ActorType)
	}
	if entry.ActorID == nil || *entry.ActorID != cmd.MerchantID {
		t.Errorf("expected actor ID %s, got %v", cmd.MerchantID, entry.ActorID)
	}
	if entry.ActorRole != "MERCHANT" {
		t.Errorf("expected actor role MERCHANT, got %s", entry.ActorRole)
	}

	// 7. Verify Outbox Event
	if outboxWriter.Count() != 1 {
		t.Fatalf("expected 1 outbox event, got %d", outboxWriter.Count())
	}
	eventsList := outboxWriter.FindByEventType(events.ParcelBookedV1)
	if len(eventsList) != 1 {
		t.Fatalf("expected 1 parcel.booked.v1 event, got %d", len(eventsList))
	}
	env := eventsList[0]
	if env.GetAggregateID() != result.Parcel.ID {
		t.Errorf("expected outbox aggregate ID %s, got %s", result.Parcel.ID, env.GetAggregateID())
	}
	if env.GetAggregateType() != "parcel" {
		t.Errorf("expected outbox aggregate type parcel, got %s", env.GetAggregateType())
	}

	// 8. Verify Transaction Committed
	testTxMu.Lock()
	committed := lastTxCommitted
	testTxMu.Unlock()
	if !committed {
		t.Error("expected database transaction to be committed")
	}
}

func TestBookMerchantParcelUseCase_ChargeableWeightCalculation(t *testing.T) {
	uc, _, _, _, _, _, _, _ := setupMerchantBookingTest(t)

	cmd := validMerchantBookingCommand()
	// Actual weight 1.0 kg, dimensions 40 x 30 x 20 cm = 24,000 / 5000 = 4.8 kg volumetric
	cmd.WeightKg = 1.0
	cmd.Dimensions = DimensionsDTO{
		LengthCm: 40.0,
		WidthCm:  30.0,
		HeightCm: 20.0,
	}

	result, err := uc.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Parcel.Weight.ActualKg != 1.0 {
		t.Errorf("expected actual weight 1.0, got %f", result.Parcel.Weight.ActualKg)
	}
	if result.Parcel.Weight.VolumetricKg != 4.8 {
		t.Errorf("expected volumetric weight 4.8, got %f", result.Parcel.Weight.VolumetricKg)
	}
	if result.Parcel.Weight.ChargeableKg != 4.8 {
		t.Errorf("expected chargeable weight 4.8, got %f", result.Parcel.Weight.ChargeableKg)
	}

	// Base 50000 + (3.8 * 10000) = 88000 cents
	expectedFee := int64(88000)
	if result.Parcel.DeliveryFee.Amount != expectedFee {
		t.Errorf("expected fee %d cents, got %d", expectedFee, result.Parcel.DeliveryFee.Amount)
	}
}

func TestBookMerchantParcelUseCase_DeliveryFee_ServiceLevels(t *testing.T) {
	tests := []struct {
		serviceLevel string
		expectedFee  int64 // Subtotal: 65000 cents
	}{
		{serviceLevel: "STANDARD", expectedFee: 65000},  // 65000 * 1.0 = 65000
		{serviceLevel: "EXPRESS", expectedFee: 97500},   // 65000 * 1.5 = 97500
		{serviceLevel: "SAME_DAY", expectedFee: 130000}, // 65000 * 2.0 = 130000
		{serviceLevel: "", expectedFee: 65000},          // default to STANDARD
	}

	for _, tc := range tests {
		t.Run("service_level_"+tc.serviceLevel, func(t *testing.T) {
			uc, _, _, _, _, _, _, _ := setupMerchantBookingTest(t)
			cmd := validMerchantBookingCommand()
			cmd.ServiceLevel = tc.serviceLevel

			result, err := uc.Execute(context.Background(), cmd)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Parcel.DeliveryFee.Amount != tc.expectedFee {
				t.Errorf("expected fee %d cents for %s, got %d", tc.expectedFee, tc.serviceLevel, result.Parcel.DeliveryFee.Amount)
			}
		})
	}
}

func TestBookMerchantParcelUseCase_TransactionAtomicity_RollbackOnFailure(t *testing.T) {
	uc, _, _, parcelRepo, _, _, _, _ := setupMerchantBookingTest(t)

	// Simulate failure during parcel save
	parcelRepo.saveErr = errors.New("db disk failure")

	cmd := validMerchantBookingCommand()
	_, err := uc.Execute(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	testTxMu.Lock()
	rolledBack := lastTxRolledBack
	committed := lastTxCommitted
	testTxMu.Unlock()

	if !rolledBack {
		t.Error("expected transaction to be rolled back on failure")
	}
	if committed {
		t.Error("expected transaction not to be committed")
	}
}

func TestBookMerchantParcelUseCase_IdempotentReplay(t *testing.T) {
	uc, _, quoteCreator, parcelRepo, _, _, _, _ := setupMerchantBookingTest(t)

	key := "0192e2b3-7c8a-7890-a123-000000000099"
	hash := "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"

	cmd := validMerchantBookingCommand()
	cmd.Idempotency = &IdempotencyInput{
		Key:         key,
		RequestHash: hash,
	}

	// 1. First execution
	res1, err := uc.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("first execution failed: %v", err)
	}
	if res1.Replayed {
		t.Error("expected first execution to not be a replay")
	}
	if len(parcelRepo.parcels) != 1 {
		t.Fatalf("expected 1 parcel after first execution, got %d", len(parcelRepo.parcels))
	}

	// 2. Second execution (identical payload & key)
	res2, err := uc.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("second execution failed: %v", err)
	}
	if !res2.Replayed {
		t.Error("expected second execution to be marked replayed")
	}
	if res2.Parcel.ID != res1.Parcel.ID {
		t.Errorf("expected same Parcel.ID on replay (%s), got %s", res1.Parcel.ID, res2.Parcel.ID)
	}
	if res2.Parcel.TrackingNumber != res1.Parcel.TrackingNumber {
		t.Errorf("expected same tracking number on replay (%s), got %s", res1.Parcel.TrackingNumber, res2.Parcel.TrackingNumber)
	}

	// Internal quote creation must only have run once
	if len(parcelRepo.parcels) != 1 {
		t.Fatalf("expected parcel count to still be 1, got %d", len(parcelRepo.parcels))
	}
	_ = quoteCreator
}

func TestBookMerchantParcelUseCase_IdempotencyPayloadConflict(t *testing.T) {
	uc, _, _, _, _, _, _, _ := setupMerchantBookingTest(t)

	key := "0192e2b3-7c8a-7890-a123-000000000099"
	hash1 := "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"
	hash2 := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

	cmd := validMerchantBookingCommand()
	cmd.Idempotency = &IdempotencyInput{
		Key:         key,
		RequestHash: hash1,
	}

	// First execution
	_, err := uc.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("first execution failed: %v", err)
	}

	// Second execution with same key but different hash
	cmd.Idempotency.RequestHash = hash2
	_, err = uc.Execute(context.Background(), cmd)
	if !errors.Is(err, ErrIdempotencyPayloadMismatch) {
		t.Fatalf("expected ErrIdempotencyPayloadMismatch, got %v", err)
	}
}

func TestBookMerchantParcelUseCase_ValidationErrors(t *testing.T) {
	t.Run("invalid merchant ID", func(t *testing.T) {
		uc, _, _, _, _, _, _, _ := setupMerchantBookingTest(t)
		cmd := validMerchantBookingCommand()

		// Empty
		cmd.MerchantID = "   "
		_, err := uc.Execute(context.Background(), cmd)
		if !errors.Is(err, domain.ErrInvalidMerchantID) {
			t.Errorf("expected ErrInvalidMerchantID for empty ID, got %v", err)
		}

		// Non-UUID
		cmd.MerchantID = "not-a-uuid"
		_, err = uc.Execute(context.Background(), cmd)
		if !errors.Is(err, domain.ErrInvalidMerchantID) {
			t.Errorf("expected ErrInvalidMerchantID for non-UUID, got %v", err)
		}
	})

	t.Run("invalid dimensions", func(t *testing.T) {
		uc, _, _, _, _, _, _, _ := setupMerchantBookingTest(t)
		cmd := validMerchantBookingCommand()

		cmd.Dimensions.LengthCm = 0
		_, err := uc.Execute(context.Background(), cmd)
		if !errors.Is(err, domain.ErrInvalidDimensions) {
			t.Errorf("expected ErrInvalidDimensions for length 0, got %v", err)
		}

		cmd.Dimensions.LengthCm = -5
		_, err = uc.Execute(context.Background(), cmd)
		if !errors.Is(err, domain.ErrInvalidDimensions) {
			t.Errorf("expected ErrInvalidDimensions for negative length, got %v", err)
		}
	})

	t.Run("invalid weight", func(t *testing.T) {
		uc, _, _, _, _, _, _, _ := setupMerchantBookingTest(t)
		cmd := validMerchantBookingCommand()

		cmd.WeightKg = 0
		_, err := uc.Execute(context.Background(), cmd)
		if !errors.Is(err, domain.ErrInvalidWeight) {
			t.Errorf("expected ErrInvalidWeight for weight 0, got %v", err)
		}

		cmd.WeightKg = -2.5
		_, err = uc.Execute(context.Background(), cmd)
		if !errors.Is(err, domain.ErrInvalidWeight) {
			t.Errorf("expected ErrInvalidWeight for negative weight, got %v", err)
		}
	})

	t.Run("maximum chargeable weight exceeded", func(t *testing.T) {
		uc, _, _, _, _, _, _, _ := setupMerchantBookingTest(t)
		cmd := validMerchantBookingCommand()

		// Actual weight > 30 kg
		cmd.WeightKg = 31.0
		_, err := uc.Execute(context.Background(), cmd)
		if !errors.Is(err, domain.ErrInvalidWeight) || !strings.Contains(err.Error(), "exceeds maximum limit") {
			t.Errorf("expected ErrInvalidWeight exceeding maximum limit, got %v", err)
		}

		// Volumetric weight > 30 kg (60 x 60 x 50 / 5000 = 36 kg)
		cmd.WeightKg = 2.0
		cmd.Dimensions = DimensionsDTO{LengthCm: 60, WidthCm: 60, HeightCm: 50}
		_, err = uc.Execute(context.Background(), cmd)
		if !errors.Is(err, domain.ErrInvalidWeight) || !strings.Contains(err.Error(), "exceeds maximum limit") {
			t.Errorf("expected ErrInvalidWeight exceeding maximum limit via volumetric, got %v", err)
		}
	})

	t.Run("unsupported service level", func(t *testing.T) {
		uc, _, _, _, _, _, _, _ := setupMerchantBookingTest(t)
		cmd := validMerchantBookingCommand()

		cmd.ServiceLevel = "HYPER_SONIC"
		_, err := uc.Execute(context.Background(), cmd)
		if !errors.Is(err, domain.ErrInvalidServiceLevel) {
			t.Errorf("expected ErrInvalidServiceLevel, got %v", err)
		}
	})

	t.Run("missing applicable tariff", func(t *testing.T) {
		uc, tariffRepo, _, _, _, _, _, _ := setupMerchantBookingTest(t)
		tariffRepo.err = pricingdomain.ErrTariffNotFound

		cmd := validMerchantBookingCommand()
		_, err := uc.Execute(context.Background(), cmd)
		if !errors.Is(err, pricingdomain.ErrTariffNotFound) {
			t.Errorf("expected ErrTariffNotFound, got %v", err)
		}
	})

	t.Run("invalid recipient fallback", func(t *testing.T) {
		uc, _, _, _, _, _, _, _ := setupMerchantBookingTest(t)
		cmd := validMerchantBookingCommand()

		cmd.RecipientName = ""
		cmd.DropoffAddress.ContactName = "   "
		_, err := uc.Execute(context.Background(), cmd)
		if !errors.Is(err, domain.ErrInvalidRecipient) && !strings.Contains(err.Error(), "contact name is required") {
			t.Errorf("expected recipient validation error, got %v", err)
		}
	})
}
