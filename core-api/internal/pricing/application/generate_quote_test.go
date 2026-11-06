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
	tariff domain.Tariff
	err    error
	called bool
}

func (f *fakeTariffRepository) FindApplicable(
	ctx context.Context,
	pickupZone string,
	destinationZone string,
	serviceLevel domain.ServiceLevel,
	at time.Time,
) (domain.Tariff, error) {
	f.called = true
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
		ID:              "tariff-001",
		PickupZone:      "MATARA",
		DestinationZone: "COLOMBO",
		BaseFee:         baseFee,
		PerKgRate:       perKgRate,
		ServiceLevel:    domain.ServiceLevelStandard,
		EffectiveFrom:   now,
		Version:         1,
	}
}

// validQuoteCommand creates valid parcel input.
func validQuoteCommand() GenerateQuoteCommand {
	return GenerateQuoteCommand{
		PickupZone:      "MATARA",
		DestinationZone: "COLOMBO",
		ActualWeightKg:  2.5,
		LengthCm:        10,
		WidthCm:         10,
		HeightCm:        10,
		ServiceLevel:    domain.ServiceLevelStandard,
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
	tariffRepo := &fakeTariffRepository{
		tariff: validTestTariff(),
	}
	quoteRepo := &fakeQuoteRepository{}

	useCase := newTestUseCase(tariffRepo, quoteRepo)

	cmd := validQuoteCommand()
	cmd.PickupZone = "   "

	_, err := useCase.Execute(context.Background(), cmd)

	if !errors.Is(err, domain.ErrInvalidTariff) {
		t.Errorf("expected ErrInvalidTariff, got %v", err)
	}

	if tariffRepo.called {
		t.Error("tariff repository should not be called for an invalid zone")
	}

	if quoteRepo.called {
		t.Error("quote should not be saved for an invalid zone")
	}
}
