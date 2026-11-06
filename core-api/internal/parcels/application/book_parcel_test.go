package application

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/events/eventstest"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

// In-memory SQL driver for standard library *sql.DB and *sql.Tx instantiation.
var (
	mockAppDriverOnce sync.Once
	testTxMu          sync.Mutex
	lastTxCommitted   bool
	lastTxRolledBack  bool
)

type appTestDriver struct{}

func (d *appTestDriver) Open(name string) (driver.Conn, error) {
	return &appTestConn{}, nil
}

type appTestConn struct{}

func (c *appTestConn) Prepare(query string) (driver.Stmt, error) {
	return &appTestStmt{}, nil
}
func (c *appTestConn) Close() error { return nil }
func (c *appTestConn) Begin() (driver.Tx, error) {
	testTxMu.Lock()
	lastTxCommitted = false
	lastTxRolledBack = false
	testTxMu.Unlock()
	return &appTestTx{}, nil
}

type appTestTx struct{}

func (t *appTestTx) Commit() error {
	testTxMu.Lock()
	lastTxCommitted = true
	testTxMu.Unlock()
	return nil
}

func (t *appTestTx) Rollback() error {
	testTxMu.Lock()
	lastTxRolledBack = true
	testTxMu.Unlock()
	return nil
}

type appTestStmt struct{}

func (s *appTestStmt) Close() error  { return nil }
func (s *appTestStmt) NumInput() int { return -1 }
func (s *appTestStmt) Exec(args []driver.Value) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}
func (s *appTestStmt) Query(args []driver.Value) (driver.Rows, error) {
	return nil, nil
}

func getTestDB(t *testing.T) *sql.DB {
	t.Helper()
	mockAppDriverOnce.Do(func() {
		sql.Register("mock_app_driver", &appTestDriver{})
	})
	database, err := sql.Open("mock_app_driver", "test")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	return database
}

// Fixed Clock for deterministic tests.
type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

// Sequential Tracking Number Generator for deterministic tests.
type mockTrackingGen struct {
	candidates []domain.TrackingNumber
	idx        int
}

func (m *mockTrackingGen) Generate(ctx context.Context, date time.Time) (domain.TrackingNumber, error) {
	if len(m.candidates) == 0 {
		return domain.FormatTrackingNumber(date, "000001")
	}
	if m.idx >= len(m.candidates) {
		return "", errors.New("exhausted mock tracking candidates")
	}
	tn := m.candidates[m.idx]
	m.idx++
	return tn, nil
}

// Mock ID Generator.
type mockIDGen struct {
	ids []string
	idx int
}

func (m *mockIDGen) GenerateID() (string, error) {
	if len(m.ids) == 0 {
		return events.GenerateUUIDv7()
	}
	if m.idx >= len(m.ids) {
		return events.GenerateUUIDv7()
	}
	id := m.ids[m.idx]
	m.idx++
	return id, nil
}

// Mock Quote Consumer.
type mockQuoteConsumer struct {
	quote   AcceptedQuoteSnapshot
	err     error
	invoked bool
}

func (m *mockQuoteConsumer) ConsumeQuote(ctx context.Context, q db.Querier, quoteID string, acceptedAt time.Time) (AcceptedQuoteSnapshot, error) {
	m.invoked = true
	if m.err != nil {
		return AcceptedQuoteSnapshot{}, m.err
	}
	return m.quote, nil
}

// Mock Parcel Repository.
type mockParcelRepo struct {
	parcels     map[string]domain.Parcel
	existingTNs map[string]bool
	saveErr     error
	saveCalled  bool
	existsErr   error
}

func newMockParcelRepo() *mockParcelRepo {
	return &mockParcelRepo{
		parcels:     make(map[string]domain.Parcel),
		existingTNs: make(map[string]bool),
	}
}

func (m *mockParcelRepo) Save(ctx context.Context, q db.Querier, parcel domain.Parcel) error {
	m.saveCalled = true
	if m.saveErr != nil {
		return m.saveErr
	}
	m.parcels[parcel.ID] = parcel
	m.existingTNs[parcel.TrackingNumber.String()] = true
	return nil
}

func (m *mockParcelRepo) ExistsTrackingNumber(ctx context.Context, q db.Querier, tn domain.TrackingNumber) (bool, error) {
	if m.existsErr != nil {
		return false, m.existsErr
	}
	return m.existingTNs[tn.String()], nil
}

func (m *mockParcelRepo) FindByID(ctx context.Context, q db.Querier, id string) (domain.Parcel, error) {
	p, ok := m.parcels[id]
	if !ok {
		return domain.Parcel{}, domain.ErrInvalidParcelID
	}
	return p, nil
}

// Mock Status History Repository.
type mockStatusHistoryRepo struct {
	entries    []domain.StatusHistoryEntry
	saveErr    error
	saveCalled bool
}

func newMockStatusHistoryRepo() *mockStatusHistoryRepo {
	return &mockStatusHistoryRepo{
		entries: make([]domain.StatusHistoryEntry, 0),
	}
}

func (m *mockStatusHistoryRepo) Save(ctx context.Context, q db.Querier, entry domain.StatusHistoryEntry) error {
	m.saveCalled = true
	if m.saveErr != nil {
		return m.saveErr
	}
	m.entries = append(m.entries, entry)
	return nil
}

// Helper to construct a valid test booking command.
func validTestBookingCommand() BookParcelCommand {
	return BookParcelCommand{
		QuoteID:        "quote-uuid-001",
		SenderID:       "usr-sender-001",
		MerchantID:     nil,
		RecipientName:  "Sunil Perera",
		RecipientPhone: "+94771234567",
		WeightKg:       2.5,
		Dimensions: DimensionsDTO{
			LengthCm: 30.0,
			WidthCm:  20.0,
			HeightCm: 15.0, // Volumetric = (30*20*15)/5000 = 1.8 kg -> Chargeable = 2.5 kg
		},
		PickupAddress: AddressDTO{
			ContactName:  "Kasun Perera",
			ContactPhone: "+94771234567",
			StreetLine1:  "45 Galle Road",
			City:         "Colombo",
			PostalCode:   "00300",
			ZoneID:       "ZONE-WESTERN-01",
		},
		DropoffAddress: AddressDTO{
			ContactName:  "Sunil Perera",
			ContactPhone: "+94719876543",
			StreetLine1:  "12 Kandy Road",
			City:         "Kandy",
			PostalCode:   "20000",
			ZoneID:       "ZONE-CENTRAL-01",
		},
		ItemDescription:     "Electronic Parts",
		SpecialInstructions: "Fragile",
		CorrelationID:       "corr-trace-001",
	}
}

// Helper to construct a valid matching quote snapshot.
func validTestQuoteSnapshot(now time.Time) AcceptedQuoteSnapshot {
	return AcceptedQuoteSnapshot{
		QuoteID:            "quote-uuid-001",
		TariffID:           "tariff-rule-001",
		TariffVersion:      2,
		OriginZoneID:       "ZONE-WESTERN-01",
		DestinationZoneID:  "ZONE-CENTRAL-01",
		ChargeableWeightKg: 2.5,
		ServiceLevel:       "STANDARD",
		FeeCents:           55000,
		Currency:           domain.CurrencyLKR,
		CreatedAt:          now.Add(-5 * time.Minute),
		ExpiresAt:          now.Add(10 * time.Minute),
		UsedAt:             now,
	}
}

// ---------------------------------------------------------------------------
// TEST SUITES
// ---------------------------------------------------------------------------

func TestBookParcelUseCase_Success(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	database := getTestDB(t)
	defer database.Close()

	txManager := NewDBTransactionManager(database)
	quoteConsumer := &mockQuoteConsumer{quote: validTestQuoteSnapshot(now)}
	parcelRepo := newMockParcelRepo()
	statusHistoryRepo := newMockStatusHistoryRepo()
	outboxWriter := eventstest.NewInMemoryWriter()
	trackingGen := &mockTrackingGen{}
	idGen := &mockIDGen{}
	clock := fixedClock{now: now}

	uc, err := NewBookParcelUseCase(
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
		t.Fatalf("failed to create use case: %v", err)
	}

	cmd := validTestBookingCommand()
	result, err := uc.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error during booking: %v", err)
	}

	// 1. Verify Parcel aggregate properties
	parcel := result.Parcel
	if parcel.ID == "" {
		t.Error("expected non-empty parcel ID")
	}
	if parcel.QuoteID != cmd.QuoteID {
		t.Errorf("expected quote ID %s, got %s", cmd.QuoteID, parcel.QuoteID)
	}
	if parcel.PricingRuleID != "tariff-rule-001" {
		t.Errorf("expected rule ID tariff-rule-001, got %s", parcel.PricingRuleID)
	}
	if parcel.PricingRuleVersion != 2 {
		t.Errorf("expected rule version 2, got %d", parcel.PricingRuleVersion)
	}
	if parcel.DeliveryFee.Amount != 55000 {
		t.Errorf("expected fee 55000, got %d", parcel.DeliveryFee.Amount)
	}
	if parcel.DeliveryFee.Currency != domain.CurrencyLKR {
		t.Errorf("expected currency LKR, got %s", parcel.DeliveryFee.Currency)
	}
	if parcel.ServiceLevel != domain.ServiceLevelStandard {
		t.Errorf("expected service level STANDARD, got %s", parcel.ServiceLevel)
	}
	if parcel.Status != domain.StatusCreated {
		t.Errorf("expected status CREATED, got %s", parcel.Status)
	}
	if parcel.DeliveryAttempts != 0 {
		t.Errorf("expected delivery attempts 0, got %d", parcel.DeliveryAttempts)
	}
	if parcel.Version != 1 {
		t.Errorf("expected version 1, got %d", parcel.Version)
	}

	// 2. Verify StatusHistory audit record
	if len(statusHistoryRepo.entries) != 1 {
		t.Fatalf("expected 1 status history entry, got %d", len(statusHistoryRepo.entries))
	}
	history := statusHistoryRepo.entries[0]
	if history.ParcelID != parcel.ID {
		t.Errorf("expected history parcel ID %s, got %s", parcel.ID, history.ParcelID)
	}
	if history.FromStatus != nil {
		t.Errorf("expected nil FromStatus for initial creation, got %v", history.FromStatus)
	}
	if history.ToStatus != domain.StatusCreated {
		t.Errorf("expected ToStatus CREATED, got %s", history.ToStatus)
	}
	if history.ActorRole != "CUSTOMER" {
		t.Errorf("expected ActorRole CUSTOMER, got %s", history.ActorRole)
	}

	// 3. Verify Outbox Event
	if outboxWriter.Count() != 1 {
		t.Fatalf("expected 1 outbox event, got %d", outboxWriter.Count())
	}
	eventsList := outboxWriter.FindByEventType(events.ParcelBookedV1)
	if len(eventsList) != 1 {
		t.Fatalf("expected 1 parcel.booked.v1 event, got %d", len(eventsList))
	}
	env := eventsList[0]
	if env.GetAggregateID() != parcel.ID {
		t.Errorf("expected aggregate ID %s, got %s", parcel.ID, env.GetAggregateID())
	}
	if env.GetAggregateType() != "parcel" {
		t.Errorf("expected aggregate type parcel, got %s", env.GetAggregateType())
	}

	// 4. Verify Transaction was committed
	testTxMu.Lock()
	committed := lastTxCommitted
	testTxMu.Unlock()
	if !committed {
		t.Error("expected transaction to be committed")
	}
}

func TestBookParcelUseCase_MissingQuote(t *testing.T) {
	now := time.Now().UTC()
	database := getTestDB(t)
	defer database.Close()

	txManager := NewDBTransactionManager(database)
	quoteConsumer := &mockQuoteConsumer{err: ErrQuoteNotFound}
	parcelRepo := newMockParcelRepo()
	statusHistoryRepo := newMockStatusHistoryRepo()
	outboxWriter := eventstest.NewInMemoryWriter()

	uc, _ := NewBookParcelUseCase(
		txManager, quoteConsumer, parcelRepo, statusHistoryRepo, outboxWriter, nil, nil, fixedClock{now: now},
	)

	_, err := uc.Execute(context.Background(), validTestBookingCommand())
	if err == nil || !errors.Is(err, ErrQuoteNotFound) {
		t.Fatalf("expected ErrQuoteNotFound, got: %v", err)
	}

	if parcelRepo.saveCalled {
		t.Error("parcel repo save should not have been called")
	}
	if statusHistoryRepo.saveCalled {
		t.Error("status history save should not have been called")
	}
	if outboxWriter.Count() > 0 {
		t.Error("no outbox events should be written")
	}
}

func TestBookParcelUseCase_ExpiredQuote(t *testing.T) {
	now := time.Now().UTC()
	database := getTestDB(t)
	defer database.Close()

	txManager := NewDBTransactionManager(database)
	quoteConsumer := &mockQuoteConsumer{err: ErrQuoteExpired}
	parcelRepo := newMockParcelRepo()
	statusHistoryRepo := newMockStatusHistoryRepo()
	outboxWriter := eventstest.NewInMemoryWriter()

	uc, _ := NewBookParcelUseCase(
		txManager, quoteConsumer, parcelRepo, statusHistoryRepo, outboxWriter, nil, nil, fixedClock{now: now},
	)

	_, err := uc.Execute(context.Background(), validTestBookingCommand())
	if err == nil || !errors.Is(err, ErrQuoteExpired) {
		t.Fatalf("expected ErrQuoteExpired, got: %v", err)
	}

	if parcelRepo.saveCalled {
		t.Error("parcel repo save should not have been called")
	}
}

func TestBookParcelUseCase_AlreadyConsumedQuote(t *testing.T) {
	now := time.Now().UTC()
	database := getTestDB(t)
	defer database.Close()

	txManager := NewDBTransactionManager(database)
	quoteConsumer := &mockQuoteConsumer{err: ErrQuoteAlreadyUsed}
	parcelRepo := newMockParcelRepo()
	statusHistoryRepo := newMockStatusHistoryRepo()
	outboxWriter := eventstest.NewInMemoryWriter()

	uc, _ := NewBookParcelUseCase(
		txManager, quoteConsumer, parcelRepo, statusHistoryRepo, outboxWriter, nil, nil, fixedClock{now: now},
	)

	_, err := uc.Execute(context.Background(), validTestBookingCommand())
	if err == nil || !errors.Is(err, ErrQuoteAlreadyUsed) {
		t.Fatalf("expected ErrQuoteAlreadyUsed, got: %v", err)
	}

	if parcelRepo.saveCalled {
		t.Error("parcel repo save should not have been called")
	}
}

func TestBookParcelUseCase_InvalidBookingDetails(t *testing.T) {
	now := time.Now().UTC()
	database := getTestDB(t)
	defer database.Close()

	txManager := NewDBTransactionManager(database)
	quoteConsumer := &mockQuoteConsumer{quote: validTestQuoteSnapshot(now)}
	parcelRepo := newMockParcelRepo()
	statusHistoryRepo := newMockStatusHistoryRepo()
	outboxWriter := eventstest.NewInMemoryWriter()

	uc, _ := NewBookParcelUseCase(
		txManager, quoteConsumer, parcelRepo, statusHistoryRepo, outboxWriter, nil, nil, fixedClock{now: now},
	)

	// 1. Missing Quote ID
	cmd1 := validTestBookingCommand()
	cmd1.QuoteID = "  "
	if _, err := uc.Execute(context.Background(), cmd1); !errors.Is(err, domain.ErrInvalidQuoteID) {
		t.Errorf("expected ErrInvalidQuoteID, got: %v", err)
	}

	// 2. Missing Sender ID
	cmd2 := validTestBookingCommand()
	cmd2.SenderID = ""
	if _, err := uc.Execute(context.Background(), cmd2); !errors.Is(err, domain.ErrInvalidSenderID) {
		t.Errorf("expected ErrInvalidSenderID, got: %v", err)
	}

	// 3. Missing Recipient Name
	cmd3 := validTestBookingCommand()
	cmd3.RecipientName = ""
	if _, err := uc.Execute(context.Background(), cmd3); !errors.Is(err, domain.ErrInvalidRecipient) {
		t.Errorf("expected ErrInvalidRecipient, got: %v", err)
	}

	// 4. Zero Weight
	cmd4 := validTestBookingCommand()
	cmd4.WeightKg = 0
	if _, err := uc.Execute(context.Background(), cmd4); !errors.Is(err, domain.ErrInvalidWeight) {
		t.Errorf("expected ErrInvalidWeight, got: %v", err)
	}

	// 5. Excessive Weight (> 30 kg)
	cmd5 := validTestBookingCommand()
	cmd5.WeightKg = 35.0
	if _, err := uc.Execute(context.Background(), cmd5); !errors.Is(err, domain.ErrInvalidWeight) {
		t.Errorf("expected ErrInvalidWeight, got: %v", err)
	}

	// 6. Zero Dimensions
	cmd6 := validTestBookingCommand()
	cmd6.Dimensions.HeightCm = 0
	if _, err := uc.Execute(context.Background(), cmd6); !errors.Is(err, domain.ErrInvalidDimensions) {
		t.Errorf("expected ErrInvalidDimensions, got: %v", err)
	}

	// 7. Missing Address field
	cmd7 := validTestBookingCommand()
	cmd7.PickupAddress.City = ""
	if _, err := uc.Execute(context.Background(), cmd7); !errors.Is(err, domain.ErrInvalidAddress) {
		t.Errorf("expected ErrInvalidAddress, got: %v", err)
	}
}

func TestBookParcelUseCase_QuoteZoneMismatch(t *testing.T) {
	now := time.Now().UTC()
	database := getTestDB(t)
	defer database.Close()

	txManager := NewDBTransactionManager(database)
	quoteConsumer := &mockQuoteConsumer{quote: validTestQuoteSnapshot(now)}
	parcelRepo := newMockParcelRepo()
	statusHistoryRepo := newMockStatusHistoryRepo()
	outboxWriter := eventstest.NewInMemoryWriter()

	uc, _ := NewBookParcelUseCase(
		txManager, quoteConsumer, parcelRepo, statusHistoryRepo, outboxWriter, nil, nil, fixedClock{now: now},
	)

	// Command with mismatched pickup zone
	cmd := validTestBookingCommand()
	cmd.PickupAddress.ZoneID = "ZONE-SOUTHERN-01" // Quote specifies ZONE-WESTERN-01

	_, err := uc.Execute(context.Background(), cmd)
	if err == nil || !errors.Is(err, ErrQuoteZoneMismatch) {
		t.Fatalf("expected ErrQuoteZoneMismatch, got: %v", err)
	}
}

func TestBookParcelUseCase_QuoteWeightMismatch(t *testing.T) {
	now := time.Now().UTC()
	database := getTestDB(t)
	defer database.Close()

	txManager := NewDBTransactionManager(database)
	quoteConsumer := &mockQuoteConsumer{quote: validTestQuoteSnapshot(now)} // Quoted for 2.5 kg
	parcelRepo := newMockParcelRepo()
	statusHistoryRepo := newMockStatusHistoryRepo()
	outboxWriter := eventstest.NewInMemoryWriter()

	uc, _ := NewBookParcelUseCase(
		txManager, quoteConsumer, parcelRepo, statusHistoryRepo, outboxWriter, nil, nil, fixedClock{now: now},
	)

	// Command supplying physical weight that calculates to 5.0 kg chargeable
	cmd := validTestBookingCommand()
	cmd.WeightKg = 5.0

	_, err := uc.Execute(context.Background(), cmd)
	if err == nil || !errors.Is(err, ErrQuoteWeightMismatch) {
		t.Fatalf("expected ErrQuoteWeightMismatch, got: %v", err)
	}
}

func TestBookParcelUseCase_AcceptedFeeAndTariffSnapshotPreserved(t *testing.T) {
	now := time.Now().UTC()
	database := getTestDB(t)
	defer database.Close()

	txManager := NewDBTransactionManager(database)
	customQuote := validTestQuoteSnapshot(now)
	customQuote.FeeCents = 87500
	customQuote.TariffID = "tariff-express-tier-4"
	customQuote.TariffVersion = 7
	customQuote.ServiceLevel = "EXPRESS"

	quoteConsumer := &mockQuoteConsumer{quote: customQuote}
	parcelRepo := newMockParcelRepo()
	statusHistoryRepo := newMockStatusHistoryRepo()
	outboxWriter := eventstest.NewInMemoryWriter()

	uc, _ := NewBookParcelUseCase(
		txManager, quoteConsumer, parcelRepo, statusHistoryRepo, outboxWriter, nil, nil, fixedClock{now: now},
	)

	res, err := uc.Execute(context.Background(), validTestBookingCommand())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that the fee and tariff snapshot on the parcel match the quote snapshot exactly
	if res.Parcel.DeliveryFee.Amount != 87500 {
		t.Errorf("expected fee 87500, got %d", res.Parcel.DeliveryFee.Amount)
	}
	if res.Parcel.PricingRuleID != "tariff-express-tier-4" {
		t.Errorf("expected tariff ID tariff-express-tier-4, got %s", res.Parcel.PricingRuleID)
	}
	if res.Parcel.PricingRuleVersion != 7 {
		t.Errorf("expected tariff version 7, got %d", res.Parcel.PricingRuleVersion)
	}
	if res.Parcel.ServiceLevel != domain.ServiceLevelExpress {
		t.Errorf("expected service level EXPRESS, got %s", res.Parcel.ServiceLevel)
	}
}

func TestBookParcelUseCase_TrackingNumberCollision_RetriesAndSucceeds(t *testing.T) {
	now := time.Now().UTC()
	database := getTestDB(t)
	defer database.Close()

	collidingTN, err := domain.FormatTrackingNumber(now, "AA0001")
	if err != nil {
		t.Fatalf("failed to format colliding TN: %v", err)
	}
	uniqueTN, err := domain.FormatTrackingNumber(now, "BB0002")
	if err != nil {
		t.Fatalf("failed to format unique TN: %v", err)
	}

	txManager := NewDBTransactionManager(database)
	quoteConsumer := &mockQuoteConsumer{quote: validTestQuoteSnapshot(now)}
	parcelRepo := newMockParcelRepo()
	// Pre-seed colliding tracking number
	parcelRepo.existingTNs[collidingTN.String()] = true

	statusHistoryRepo := newMockStatusHistoryRepo()
	outboxWriter := eventstest.NewInMemoryWriter()
	trackingGen := &mockTrackingGen{
		candidates: []domain.TrackingNumber{collidingTN, uniqueTN},
	}

	uc, _ := NewBookParcelUseCase(
		txManager, quoteConsumer, parcelRepo, statusHistoryRepo, outboxWriter, trackingGen, nil, fixedClock{now: now},
	)

	res, err := uc.Execute(context.Background(), validTestBookingCommand())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Parcel.TrackingNumber != uniqueTN {
		t.Errorf("expected tracking number %s after retry, got %s", uniqueTN, res.Parcel.TrackingNumber)
	}
}

func TestBookParcelUseCase_TrackingNumberCollision_ExhaustsRetries(t *testing.T) {
	now := time.Now().UTC()
	database := getTestDB(t)
	defer database.Close()

	colliding1, _ := domain.FormatTrackingNumber(now, "AA0001")
	colliding2, _ := domain.FormatTrackingNumber(now, "BB0002")
	colliding3, _ := domain.FormatTrackingNumber(now, "CC0003")

	txManager := NewDBTransactionManager(database)
	quoteConsumer := &mockQuoteConsumer{quote: validTestQuoteSnapshot(now)}
	parcelRepo := newMockParcelRepo()
	parcelRepo.existingTNs[colliding1.String()] = true
	parcelRepo.existingTNs[colliding2.String()] = true
	parcelRepo.existingTNs[colliding3.String()] = true

	statusHistoryRepo := newMockStatusHistoryRepo()
	outboxWriter := eventstest.NewInMemoryWriter()
	trackingGen := &mockTrackingGen{
		candidates: []domain.TrackingNumber{colliding1, colliding2, colliding3},
	}

	uc, _ := NewBookParcelUseCase(
		txManager, quoteConsumer, parcelRepo, statusHistoryRepo, outboxWriter, trackingGen, nil, fixedClock{now: now},
	)

	_, err := uc.Execute(context.Background(), validTestBookingCommand())
	if err == nil || !errors.Is(err, ErrTrackingNumberCollision) {
		t.Fatalf("expected ErrTrackingNumberCollision, got: %v", err)
	}
}

func TestBookParcelUseCase_ParcelPersistenceFailure_TriggersRollback(t *testing.T) {
	now := time.Now().UTC()
	database := getTestDB(t)
	defer database.Close()

	dbErr := errors.New("disk full: failed to insert parcel")

	txManager := NewDBTransactionManager(database)
	quoteConsumer := &mockQuoteConsumer{quote: validTestQuoteSnapshot(now)}
	parcelRepo := newMockParcelRepo()
	parcelRepo.saveErr = dbErr
	statusHistoryRepo := newMockStatusHistoryRepo()
	outboxWriter := eventstest.NewInMemoryWriter()

	uc, _ := NewBookParcelUseCase(
		txManager, quoteConsumer, parcelRepo, statusHistoryRepo, outboxWriter, nil, nil, fixedClock{now: now},
	)

	_, err := uc.Execute(context.Background(), validTestBookingCommand())
	if err == nil || !errors.Is(err, dbErr) {
		t.Fatalf("expected dbErr, got: %v", err)
	}

	testTxMu.Lock()
	rolledBack := lastTxRolledBack
	testTxMu.Unlock()
	if !rolledBack {
		t.Error("expected transaction rollback on parcel persistence failure")
	}
}

func TestBookParcelUseCase_StatusHistoryPersistenceFailure_TriggersRollback(t *testing.T) {
	now := time.Now().UTC()
	database := getTestDB(t)
	defer database.Close()

	dbErr := errors.New("foreign key violation: failed to insert status history")

	txManager := NewDBTransactionManager(database)
	quoteConsumer := &mockQuoteConsumer{quote: validTestQuoteSnapshot(now)}
	parcelRepo := newMockParcelRepo()
	statusHistoryRepo := newMockStatusHistoryRepo()
	statusHistoryRepo.saveErr = dbErr
	outboxWriter := eventstest.NewInMemoryWriter()

	uc, _ := NewBookParcelUseCase(
		txManager, quoteConsumer, parcelRepo, statusHistoryRepo, outboxWriter, nil, nil, fixedClock{now: now},
	)

	_, err := uc.Execute(context.Background(), validTestBookingCommand())
	if err == nil || !errors.Is(err, dbErr) {
		t.Fatalf("expected dbErr, got: %v", err)
	}

	testTxMu.Lock()
	rolledBack := lastTxRolledBack
	testTxMu.Unlock()
	if !rolledBack {
		t.Error("expected transaction rollback on status history persistence failure")
	}
}

func TestBookParcelUseCase_OutboxFailure_TriggersRollback(t *testing.T) {
	now := time.Now().UTC()
	database := getTestDB(t)
	defer database.Close()

	txManager := NewDBTransactionManager(database)
	quoteConsumer := &mockQuoteConsumer{quote: validTestQuoteSnapshot(now)}
	parcelRepo := newMockParcelRepo()
	statusHistoryRepo := newMockStatusHistoryRepo()

	// Outbox writer that simulates failure
	failingOutbox := &failingOutboxWriter{err: errors.New("outbox table write error")}

	uc, _ := NewBookParcelUseCase(
		txManager, quoteConsumer, parcelRepo, statusHistoryRepo, failingOutbox, nil, nil, fixedClock{now: now},
	)

	_, err := uc.Execute(context.Background(), validTestBookingCommand())
	if err == nil {
		t.Fatal("expected outbox failure error, got nil")
	}

	testTxMu.Lock()
	rolledBack := lastTxRolledBack
	testTxMu.Unlock()
	if !rolledBack {
		t.Error("expected transaction rollback on outbox write failure")
	}
}

func TestBookParcelUseCase_MerchantRoleActor(t *testing.T) {
	now := time.Now().UTC()
	database := getTestDB(t)
	defer database.Close()

	txManager := NewDBTransactionManager(database)
	quoteConsumer := &mockQuoteConsumer{quote: validTestQuoteSnapshot(now)}
	parcelRepo := newMockParcelRepo()
	statusHistoryRepo := newMockStatusHistoryRepo()
	outboxWriter := eventstest.NewInMemoryWriter()

	uc, _ := NewBookParcelUseCase(
		txManager, quoteConsumer, parcelRepo, statusHistoryRepo, outboxWriter, nil, nil, fixedClock{now: now},
	)

	cmd := validTestBookingCommand()
	merchantID := "mrc_01HMERCHANT0001"
	cmd.MerchantID = &merchantID

	res, err := uc.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Parcel.MerchantID == nil || *res.Parcel.MerchantID != merchantID {
		t.Errorf("expected merchant ID %s, got %v", merchantID, res.Parcel.MerchantID)
	}
	if res.StatusHistory.ActorRole != "MERCHANT" {
		t.Errorf("expected actor role MERCHANT, got %s", res.StatusHistory.ActorRole)
	}
}

type failingOutboxWriter struct {
	err error
}

func (f *failingOutboxWriter) Write(ctx context.Context, tx *sql.Tx, envelopes ...events.AnyEnvelope) error {
	return f.err
}
