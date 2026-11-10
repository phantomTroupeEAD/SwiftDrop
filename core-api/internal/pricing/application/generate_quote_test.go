package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

// fakeTariffRepository simulates tariff retrieval.
type fakeTariffRepository struct {
	tariff            domain.Tariff
	err               error
	called            bool
	originZoneID      string
	destinationZoneID string
}

func (f *fakeTariffRepository) FindApplicable(
	ctx context.Context,
	originZoneID string,
	destinationZoneID string,
	at time.Time,
) (domain.Tariff, error) {
	f.called = true
	f.originZoneID = originZoneID
	f.destinationZoneID = destinationZoneID
	return f.tariff, f.err
}

// fakeQuoteRepository simulates quote persistence.
type fakeQuoteRepository struct {
	savedQuote domain.Quote
	err        error
	called     bool
}

func (f *fakeQuoteRepository) Save(
	ctx context.Context,
	quote domain.Quote,
) error {
	f.called = true
	f.savedQuote = quote
	return f.err
}

// fakeClock provides a predictable current time.
type fakeClock struct {
	now time.Time
}

func (f fakeClock) Now() time.Time {
	return f.now
}

// fakeQuoteIDGenerator provides a predictable quote ID.
type fakeQuoteIDGenerator struct {
	id  string
	err error
}

func (f fakeQuoteIDGenerator) NewID() (string, error) {
	return f.id, f.err
}

// newTestUseCase creates a use case with fake dependencies.
func newTestUseCase(
	tariffRepo *fakeTariffRepository,
	quoteRepo *fakeQuoteRepository,
) *GenerateQuoteUseCase {
	now := time.Date(
		2026, time.November, 6,
		10, 0, 0, 0,
		time.UTC,
	)

	return NewGenerateQuoteUseCase(
		tariffRepo,
		quoteRepo,
		fakeClock{now: now},
		fakeQuoteIDGenerator{id: "quote-test-001"},
	)
}

func ptrStr(s string) *string {
	return &s
}

// validTestTariff creates a valid tariff for testing.
func validTestTariff() domain.Tariff {
	now := time.Date(
		2026, time.November, 1,
		0, 0, 0, 0,
		time.UTC,
	)

	baseFee, _ := domain.NewMoney(35000, domain.CurrencyLKR)
	perKgRate, _ := domain.NewMoney(8000, domain.CurrencyLKR)

	return domain.Tariff{
		ID:                "tariff-001",
		OriginZoneID:      "ZONE_MATARA_01",
		DestinationZoneID: "ZONE_COLOMBO_01",
		BaseFee:           baseFee,
		PerKgRate:         perKgRate,
		EffectiveFrom:     now,
		Version:           1,
		IsActive:          true,
	}
}

// validQuoteCommand creates valid parcel input.
func validQuoteCommand() GenerateQuoteCommand {
	return GenerateQuoteCommand{
		OriginZoneID:      "ZONE_MATARA_01",
		DestinationZoneID: "ZONE_COLOMBO_01",
		ActualWeightKg:    2.5,
		LengthCm:          10,
		WidthCm:           10,
		HeightCm:          10,
		ServiceLevel:      domain.ServiceLevelStandard,
		RequesterID:       ptrStr("018f4a7c-1005-7000-8000-000000000005"),
	}
}

func TestGenerateQuoteUseCase_Success(t *testing.T) {
	tariffRepo := &fakeTariffRepository{
		tariff: validTestTariff(),
	}
	quoteRepo := &fakeQuoteRepository{}

	useCase := newTestUseCase(tariffRepo, quoteRepo)

	quote, err := useCase.Execute(
		context.Background(),
		validQuoteCommand(),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !tariffRepo.called {
		t.Fatal("expected tariff repository to be called")
	}

	if !quoteRepo.called {
		t.Fatal("expected quote repository to be called")
	}

	if quote.ID != "quote-test-001" {
		t.Errorf("unexpected quote ID: %s", quote.ID)
	}

	if quote.TariffID != "tariff-001" {
		t.Errorf("unexpected tariff ID: %s", quote.TariffID)
	}

	if quote.TariffVersion != 1 {
		t.Errorf("unexpected tariff version: %d", quote.TariffVersion)
	}

	if quote.ServiceLevel != domain.ServiceLevelStandard {
		t.Errorf("expected service level %s, got %s", domain.ServiceLevelStandard, quote.ServiceLevel)
	}

	if quote.RequesterID == nil || *quote.RequesterID != "018f4a7c-1005-7000-8000-000000000005" {
		t.Errorf("expected requester ID preserved, got %v", quote.RequesterID)
	}

	if quote.MerchantID != nil {
		t.Errorf("expected nil merchant ID, got %v", quote.MerchantID)
	}

	if tariffRepo.originZoneID != "ZONE_MATARA_01" {
		t.Errorf("expected origin zone ZONE_MATARA_01, got %s", tariffRepo.originZoneID)
	}

	if tariffRepo.destinationZoneID != "ZONE_COLOMBO_01" {
		t.Errorf("expected destination zone ZONE_COLOMBO_01, got %s", tariffRepo.destinationZoneID)
	}

	if quote.ChargeableWeightKg != 2.5 {
		t.Errorf("expected chargeable weight 2.5, got %v", quote.ChargeableWeightKg)
	}

	// Expected: (35000 + (2.5 - 1) * 8000) = 47000 cents.
	if quote.Fee.Amount != 47000 {
		t.Errorf("expected fee 47000 cents, got %d", quote.Fee.Amount)
	}

	if !quote.ExpiresAt.Equal(quote.CreatedAt.Add(domain.QuoteValidity)) {
		t.Error("quote should expire after the configured validity period")
	}

	if quoteRepo.savedQuote.ID != quote.ID {
		t.Error("saved quote does not match returned quote")
	}
}

func TestGenerateQuoteUseCase_InvalidDimensions(t *testing.T) {
	tariffRepo := &fakeTariffRepository{
		tariff: validTestTariff(),
	}
	quoteRepo := &fakeQuoteRepository{}

	useCase := newTestUseCase(tariffRepo, quoteRepo)

	cmd := validQuoteCommand()
	cmd.LengthCm = 0

	_, err := useCase.Execute(context.Background(), cmd)

	if !errors.Is(err, domain.ErrInvalidDimensions) {
		t.Errorf("expected ErrInvalidDimensions, got %v", err)
	}

	if tariffRepo.called {
		t.Error("tariff repository should not be called for invalid dimensions")
	}

	if quoteRepo.called {
		t.Error("quote repository should not be called for invalid dimensions")
	}
}

func TestGenerateQuoteUseCase_TariffNotFound(t *testing.T) {
	tariffRepo := &fakeTariffRepository{
		err: domain.ErrTariffNotFound,
	}
	quoteRepo := &fakeQuoteRepository{}

	useCase := newTestUseCase(tariffRepo, quoteRepo)

	_, err := useCase.Execute(
		context.Background(),
		validQuoteCommand(),
	)

	if !errors.Is(err, domain.ErrTariffNotFound) {
		t.Errorf("expected ErrTariffNotFound, got %v", err)
	}

	if !tariffRepo.called {
		t.Error("expected tariff repository to be called")
	}

	if quoteRepo.called {
		t.Error("quote should not be saved when tariff is not found")
	}
}

func TestGenerateQuoteUseCase_SaveFailure(t *testing.T) {
	saveErr := errors.New("database unavailable")

	tariffRepo := &fakeTariffRepository{
		tariff: validTestTariff(),
	}
	quoteRepo := &fakeQuoteRepository{
		err: saveErr,
	}

	useCase := newTestUseCase(tariffRepo, quoteRepo)

	_, err := useCase.Execute(
		context.Background(),
		validQuoteCommand(),
	)

	if !errors.Is(err, saveErr) {
		t.Errorf("expected save error, got %v", err)
	}

	if !quoteRepo.called {
		t.Error("expected quote repository to be called")
	}
}

func TestGenerateQuoteUseCase_InvalidZone(t *testing.T) {
	tests := []struct {
		name              string
		originZoneID      string
		destinationZoneID string
	}{
		{
			name:              "empty origin zone",
			originZoneID:      "   ",
			destinationZoneID: "ZONE_COLOMBO_01",
		},
		{
			name:              "empty destination zone",
			originZoneID:      "ZONE_MATARA_01",
			destinationZoneID: "   ",
		},
		{
			name:              "invalid origin zone format (missing ZONE_ prefix)",
			originZoneID:      "MATARA",
			destinationZoneID: "ZONE_COLOMBO_01",
		},
		{
			name:              "invalid destination zone format (missing ZONE_ prefix)",
			originZoneID:      "ZONE_MATARA_01",
			destinationZoneID: "COLOMBO",
		},
		{
			name:              "invalid origin zone format (hyphenated legacy)",
			originZoneID:      "ZONE-WESTERN-01",
			destinationZoneID: "ZONE_COLOMBO_01",
		},
		{
			name:              "invalid destination zone format (lowercase)",
			originZoneID:      "ZONE_MATARA_01",
			destinationZoneID: "zone_colombo_01",
		},
		{
			name:              "invalid destination zone format (too short)",
			originZoneID:      "ZONE_MATARA_01",
			destinationZoneID: "ZONE_A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tariffRepo := &fakeTariffRepository{
				tariff: validTestTariff(),
			}
			quoteRepo := &fakeQuoteRepository{}

			useCase := newTestUseCase(tariffRepo, quoteRepo)

			cmd := validQuoteCommand()
			cmd.OriginZoneID = tt.originZoneID
			cmd.DestinationZoneID = tt.destinationZoneID

			_, err := useCase.Execute(context.Background(), cmd)

			if !errors.Is(err, domain.ErrInvalidZoneID) {
				t.Errorf("expected ErrInvalidZoneID, got %v", err)
			}

			if !errors.Is(err, domain.ErrInvalidTariff) {
				t.Errorf("expected ErrInvalidTariff, got %v", err)
			}

			if tariffRepo.called {
				t.Error("tariff repository should not be called for an invalid zone")
			}

			if quoteRepo.called {
				t.Error("quote should not be saved for an invalid zone")
			}
		})
	}
}

func TestGenerateQuoteUseCase_OwnershipModes(t *testing.T) {
	tests := []struct {
		name               string
		requesterID        *string
		merchantID         *string
		expectedRequester  *string
		expectedMerchantID *string
	}{
		{
			name:               "customer-only quote generation",
			requesterID:        ptrStr("  usr-cust-001  "),
			merchantID:         nil,
			expectedRequester:  ptrStr("usr-cust-001"),
			expectedMerchantID: nil,
		},
		{
			name:               "merchant-only quote generation",
			requesterID:        nil,
			merchantID:         ptrStr("  mrc-merch-001  "),
			expectedRequester:  nil,
			expectedMerchantID: ptrStr("mrc-merch-001"),
		},
		{
			name:               "both requester and merchant IDs supplied",
			requesterID:        ptrStr(" usr-cust-001 "),
			merchantID:         ptrStr(" mrc-merch-001 "),
			expectedRequester:  ptrStr("usr-cust-001"),
			expectedMerchantID: ptrStr("mrc-merch-001"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tariffRepo := &fakeTariffRepository{
				tariff: validTestTariff(),
			}
			quoteRepo := &fakeQuoteRepository{}

			useCase := newTestUseCase(tariffRepo, quoteRepo)

			cmd := validQuoteCommand()
			cmd.RequesterID = tc.requesterID
			cmd.MerchantID = tc.merchantID

			quote, err := useCase.Execute(context.Background(), cmd)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !quoteRepo.called {
				t.Fatal("expected quote repository to be called")
			}

			if (quote.RequesterID == nil) != (tc.expectedRequester == nil) ||
				(quote.RequesterID != nil && *quote.RequesterID != *tc.expectedRequester) {
				t.Errorf("expected requester %v, got %v", tc.expectedRequester, quote.RequesterID)
			}

			if (quote.MerchantID == nil) != (tc.expectedMerchantID == nil) ||
				(quote.MerchantID != nil && *quote.MerchantID != *tc.expectedMerchantID) {
				t.Errorf("expected merchant %v, got %v", tc.expectedMerchantID, quote.MerchantID)
			}

			if (quoteRepo.savedQuote.RequesterID == nil) != (tc.expectedRequester == nil) ||
				(quoteRepo.savedQuote.RequesterID != nil && *quoteRepo.savedQuote.RequesterID != *tc.expectedRequester) {
				t.Errorf("saved quote requester mismatch: expected %v, got %v", tc.expectedRequester, quoteRepo.savedQuote.RequesterID)
			}

			if (quoteRepo.savedQuote.MerchantID == nil) != (tc.expectedMerchantID == nil) ||
				(quoteRepo.savedQuote.MerchantID != nil && *quoteRepo.savedQuote.MerchantID != *tc.expectedMerchantID) {
				t.Errorf("saved quote merchant mismatch: expected %v, got %v", tc.expectedMerchantID, quoteRepo.savedQuote.MerchantID)
			}
		})
	}
}

func TestGenerateQuoteUseCase_InvalidOwnership(t *testing.T) {
	tests := []struct {
		name        string
		requesterID *string
		merchantID  *string
	}{
		{
			name:        "both owner IDs nil",
			requesterID: nil,
			merchantID:  nil,
		},
		{
			name:        "empty requester ID with valid merchant ID",
			requesterID: ptrStr(""),
			merchantID:  ptrStr("mrc-001"),
		},
		{
			name:        "whitespace-only requester ID with valid merchant ID",
			requesterID: ptrStr("   \t"),
			merchantID:  ptrStr("mrc-001"),
		},
		{
			name:        "empty merchant ID with valid requester ID",
			requesterID: ptrStr("usr-001"),
			merchantID:  ptrStr(""),
		},
		{
			name:        "whitespace-only merchant ID with valid requester ID",
			requesterID: ptrStr("usr-001"),
			merchantID:  ptrStr("   "),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tariffRepo := &fakeTariffRepository{
				tariff: validTestTariff(),
			}
			quoteRepo := &fakeQuoteRepository{}

			useCase := newTestUseCase(tariffRepo, quoteRepo)

			cmd := validQuoteCommand()
			cmd.RequesterID = tc.requesterID
			cmd.MerchantID = tc.merchantID

			_, err := useCase.Execute(context.Background(), cmd)
			if !errors.Is(err, domain.ErrInvalidQuote) {
				t.Fatalf("expected ErrInvalidQuote, got %v", err)
			}

			if tariffRepo.called {
				t.Error("tariff repository should not be called for invalid ownership")
			}

			if quoteRepo.called {
				t.Error("quote repository should not be called for invalid ownership")
			}
		})
	}
}

func TestGenerateQuoteUseCase_InvalidServiceLevel(t *testing.T) {
	tariffRepo := &fakeTariffRepository{
		tariff: validTestTariff(),
	}
	quoteRepo := &fakeQuoteRepository{}

	useCase := newTestUseCase(tariffRepo, quoteRepo)

	cmd := validQuoteCommand()
	cmd.ServiceLevel = domain.ServiceLevel("INVALID")

	_, err := useCase.Execute(context.Background(), cmd)

	if !errors.Is(err, domain.ErrInvalidServiceLevel) {
		t.Errorf("expected ErrInvalidServiceLevel, got %v", err)
	}

	if tariffRepo.called {
		t.Error("tariff repository should not be called for an invalid service level")
	}

	if quoteRepo.called {
		t.Error("quote repository should not be saved for an invalid service level")
	}
}

func TestGenerateQuoteUseCase_ExpressServiceMultiplier(t *testing.T) {
	tariffRepo := &fakeTariffRepository{
		tariff: validTestTariff(),
	}
	quoteRepo := &fakeQuoteRepository{}

	useCase := newTestUseCase(tariffRepo, quoteRepo)

	cmd := validQuoteCommand()
	cmd.ServiceLevel = domain.ServiceLevelExpress

	quote, err := useCase.Execute(context.Background(), cmd)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if quote.ServiceLevel != domain.ServiceLevelExpress {
		t.Errorf("expected service level EXPRESS, got %s", quote.ServiceLevel)
	}

	// Base 35000 + 1.5 * 8000 = 47000. 47000 * 1.5 = 70500 cents.
	if quote.Fee.Amount != 70500 {
		t.Errorf("expected fee 70500 cents, got %d", quote.Fee.Amount)
	}
}
