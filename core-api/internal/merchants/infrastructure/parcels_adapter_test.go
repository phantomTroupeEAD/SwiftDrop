package infrastructure

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/events/eventstest"
	"github.com/swiftdrop/core-api/internal/merchants/application"
	parcelsapp "github.com/swiftdrop/core-api/internal/parcels/application"
	parcelsdomain "github.com/swiftdrop/core-api/internal/parcels/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
	pricingdomain "github.com/swiftdrop/core-api/internal/pricing/domain"
)

var (
	adapterMockDriverOnce sync.Once
)

type adapterTestDriver struct{}

func (d *adapterTestDriver) Open(name string) (driver.Conn, error) {
	return &adapterTestConn{}, nil
}

type adapterTestConn struct{}

func (c *adapterTestConn) Prepare(query string) (driver.Stmt, error) {
	return &adapterTestStmt{}, nil
}
func (c *adapterTestConn) Close() error              { return nil }
func (c *adapterTestConn) Begin() (driver.Tx, error) { return &adapterTestTx{}, nil }

type adapterTestTx struct{}

func (t *adapterTestTx) Commit() error   { return nil }
func (t *adapterTestTx) Rollback() error { return nil }

type adapterTestStmt struct{}

func (s *adapterTestStmt) Close() error  { return nil }
func (s *adapterTestStmt) NumInput() int { return -1 }
func (s *adapterTestStmt) Exec(args []driver.Value) (driver.Result, error) {
	return adapterTestResult{}, nil
}
func (s *adapterTestStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &adapterTestRows{}, nil
}

type adapterTestResult struct{}

func (adapterTestResult) LastInsertId() (int64, error) { return 0, nil }
func (adapterTestResult) RowsAffected() (int64, error) { return 1, nil }

type adapterTestRows struct{}

func (r *adapterTestRows) Columns() []string              { return []string{} }
func (r *adapterTestRows) Close() error                   { return nil }
func (r *adapterTestRows) Next(dest []driver.Value) error { return errors.New("no rows") }

func getAdapterTestDB(t *testing.T) *sql.DB {
	t.Helper()
	adapterMockDriverOnce.Do(func() {
		sql.Register("adapter_mock_driver", &adapterTestDriver{})
	})
	database, err := sql.Open("adapter_mock_driver", "test")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	return database
}

// In-Memory Test Repositories & Mocks
type mockAdapterParcelRepo struct {
	mu         sync.Mutex
	parcels    map[string]parcelsdomain.Parcel
	byMerchant map[string][]parcelsdomain.Parcel
}

func newMockAdapterParcelRepo() *mockAdapterParcelRepo {
	return &mockAdapterParcelRepo{
		parcels:    make(map[string]parcelsdomain.Parcel),
		byMerchant: make(map[string][]parcelsdomain.Parcel),
	}
}

func (m *mockAdapterParcelRepo) Save(ctx context.Context, q db.Querier, parcel parcelsdomain.Parcel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parcels[parcel.ID] = parcel
	if parcel.MerchantID != nil {
		m.byMerchant[*parcel.MerchantID] = append(m.byMerchant[*parcel.MerchantID], parcel)
	}
	return nil
}

func (m *mockAdapterParcelRepo) FindByID(ctx context.Context, q db.Querier, id string) (parcelsdomain.Parcel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.parcels[id]
	if !ok {
		return parcelsdomain.Parcel{}, parcelsdomain.ErrInvalidParcelID
	}
	return p, nil
}

func (m *mockAdapterParcelRepo) FindByTrackingNumber(ctx context.Context, q db.Querier, tn parcelsdomain.TrackingNumber) (parcelsdomain.Parcel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.parcels {
		if p.TrackingNumber == tn {
			return p, nil
		}
	}
	return parcelsdomain.Parcel{}, parcelsdomain.ErrInvalidTrackingNumber
}

func (m *mockAdapterParcelRepo) UpdateStatus(ctx context.Context, q db.Querier, parcel parcelsdomain.Parcel, expectedVersion int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parcels[parcel.ID] = parcel
	return nil
}

func (m *mockAdapterParcelRepo) ExistsTrackingNumber(ctx context.Context, q db.Querier, tn parcelsdomain.TrackingNumber) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.parcels {
		if p.TrackingNumber == tn {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockAdapterParcelRepo) ListByMerchantID(ctx context.Context, q db.Querier, merchantID string) ([]parcelsdomain.Parcel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.byMerchant[merchantID]
	// Return newest first (reverse of insertion order for test)
	out := make([]parcelsdomain.Parcel, len(list))
	for i, p := range list {
		out[len(list)-1-i] = p
	}
	return out, nil
}

type mockAdapterStatusHistoryRepo struct{}

func (m *mockAdapterStatusHistoryRepo) Save(ctx context.Context, q db.Querier, entry parcelsdomain.StatusHistoryEntry) error {
	return nil
}
func (m *mockAdapterStatusHistoryRepo) FindByParcelID(ctx context.Context, q db.Querier, parcelID string) ([]parcelsdomain.StatusHistoryEntry, error) {
	return nil, nil
}

type mockAdapterTariffRepo struct {
	tariff pricingdomain.Tariff
	err    error
}

func (m *mockAdapterTariffRepo) FindApplicable(ctx context.Context, originZoneID, destinationZoneID string, at time.Time) (pricingdomain.Tariff, error) {
	if m.err != nil {
		return pricingdomain.Tariff{}, m.err
	}
	return m.tariff, nil
}

type mockAdapterQuoteCreator struct{}

func (m *mockAdapterQuoteCreator) SaveConsumedQuote(ctx context.Context, q db.Querier, quote pricingdomain.Quote) error {
	return nil
}

type mockAdapterQuoteConsumer struct{}

func (m *mockAdapterQuoteConsumer) ConsumeQuote(ctx context.Context, q db.Querier, quoteID string, acceptedAt time.Time) (parcelsapp.AcceptedQuoteSnapshot, error) {
	return parcelsapp.AcceptedQuoteSnapshot{}, nil
}

type mockAdapterClock struct{ now time.Time }

func (c mockAdapterClock) Now() time.Time { return c.now }

type mockAdapterTrackingGen struct {
	mu  sync.Mutex
	seq int
}

func (g *mockAdapterTrackingGen) Generate(ctx context.Context, at time.Time) (parcelsdomain.TrackingNumber, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	return parcelsdomain.FormatTrackingNumber(at, "000001")
}

type mockAdapterIDGen struct {
	mu  sync.Mutex
	seq int
}

func (g *mockAdapterIDGen) GenerateID() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	return fmt.Sprintf("0192e2b3-7c8a-7890-a123-%012d", g.seq), nil
}

type adapterTxWrapper struct {
	inner parcelsapp.TransactionManager
	store *mockAdapterIdemStore
}

func (w *adapterTxWrapper) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	defer func() {
		w.store.mu.Lock()
		for k, held := range w.store.held {
			if held {
				w.store.held[k] = false
				if kl, ok := w.store.locks[k]; ok {
					kl.Unlock()
				}
			}
		}
		w.store.mu.Unlock()
	}()
	return w.inner.WithTx(ctx, fn)
}

type mockAdapterIdemStore struct {
	mu      sync.Mutex
	locks   map[string]*sync.Mutex
	held    map[string]bool
	records map[string]parcelsapp.IdempotencyRecord
}

func newMockAdapterIdemStore() *mockAdapterIdemStore {
	return &mockAdapterIdemStore{
		locks:   make(map[string]*sync.Mutex),
		held:    make(map[string]bool),
		records: make(map[string]parcelsapp.IdempotencyRecord),
	}
}

func (m *mockAdapterIdemStore) AcquireLock(_ context.Context, _ db.Querier, key, callerScope string) error {
	k := callerScope + "\x00" + key
	m.mu.Lock()
	kl, ok := m.locks[k]
	if !ok {
		kl = &sync.Mutex{}
		m.locks[k] = kl
	}
	m.mu.Unlock()

	kl.Lock()
	m.mu.Lock()
	m.held[k] = true
	m.mu.Unlock()
	return nil
}

func (m *mockAdapterIdemStore) Find(_ context.Context, _ db.Querier, key, callerScope string) (parcelsapp.IdempotencyRecord, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[callerScope+"\x00"+key]
	return rec, ok, nil
}

func (m *mockAdapterIdemStore) Save(_ context.Context, _ db.Querier, rec parcelsapp.IdempotencyRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[rec.CallerScope+"\x00"+rec.Key] = rec
	return nil
}

func setupTestAdapter(t *testing.T) (*ParcelsBookingAdapter, *mockAdapterParcelRepo, *mockAdapterTariffRepo, time.Time) {
	t.Helper()
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	clock := mockAdapterClock{now: now}

	database := getAdapterTestDB(t)
	idemStore := newMockAdapterIdemStore()
	rawTxMgr := parcelsapp.NewDBTransactionManager(database)
	txMgr := &adapterTxWrapper{inner: rawTxMgr, store: idemStore}

	baseFee, _ := pricingdomain.NewMoney(50000, pricingdomain.CurrencyLKR)
	perKgRate, _ := pricingdomain.NewMoney(10000, pricingdomain.CurrencyLKR)
	tariff := pricingdomain.Tariff{
		ID:                "0192e2b3-7c8a-7890-a123-000000000010",
		Version:           1,
		OriginZoneID:      "ZONE_WESTERN_01",
		DestinationZoneID: "ZONE_CENTRAL_01",
		BaseFee:           baseFee,
		PerKgRate:         perKgRate,
		IsActive:          true,
		EffectiveFrom:     now.Add(-24 * time.Hour),
		CreatedAt:         now.Add(-24 * time.Hour),
	}
	tariffRepo := &mockAdapterTariffRepo{tariff: tariff}
	quoteCreator := &mockAdapterQuoteCreator{}
	quoteConsumer := &mockAdapterQuoteConsumer{}
	parcelRepo := newMockAdapterParcelRepo()
	statusRepo := &mockAdapterStatusHistoryRepo{}
	outboxWriter := eventstest.NewInMemoryWriter()
	trackingGen := &mockAdapterTrackingGen{}
	idGen := &mockAdapterIDGen{}

	bookUC, err := parcelsapp.NewBookParcelUseCase(
		txMgr,
		quoteConsumer,
		parcelRepo,
		statusRepo,
		outboxWriter,
		trackingGen,
		idGen,
		clock,
	)
	if err != nil {
		t.Fatalf("failed to create BookParcelUseCase: %v", err)
	}
	bookUC = bookUC.WithIdempotency(idemStore, parcelsapp.DefaultIdempotencyTTL)

	merchantUC, err := parcelsapp.NewBookMerchantParcelUseCase(
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
	merchantUC = merchantUC.WithIdempotency(idemStore, parcelsapp.DefaultIdempotencyTTL)

	adapter := NewParcelsBookingAdapter(merchantUC, parcelRepo)
	return adapter, parcelRepo, tariffRepo, now
}

func validTestOrderRequest() application.OrderRequest {
	return application.OrderRequest{
		MerchantReference: "ORD-2026-9988",
		ServiceLevel:      "STANDARD",
		Pickup: map[string]any{
			"contact_name":  "Warehouse Dispatch",
			"contact_phone": "+94771234567",
			"street_line_1": "10 Galle Road",
			"city":          "Colombo",
			"postal_code":   "00300",
			"zone_id":       "ZONE_WESTERN_01",
			"instructions":  "Loading Bay 2",
		},
		Dropoff: map[string]any{
			"contact_name":  "Nimal Perera",
			"contact_phone": "+94779876543",
			"street_line_1": "12 Temple Road",
			"city":          "Kandy",
			"postal_code":   "20000",
			"zone_id":       "ZONE_CENTRAL_01",
			"instructions":  "Deliver to door",
		},
		Parcel: application.ParcelRequest{
			WeightKG: 2.5,
			LengthCM: 30.0,
			WidthCM:  20.0,
			HeightCM: 15.0,
		},
	}
}

func TestParcelsBookingAdapter_CreateForMerchant_Success(t *testing.T) {
	adapter, parcelRepo, _, _ := setupTestAdapter(t)
	merchantID := "0192e2b3-7c8a-7890-a123-000000000001"
	req := validTestOrderRequest()

	hash := strings.Repeat("a", 64)
	result, replayed, err := adapter.CreateForMerchant(context.Background(), merchantID, "0192e2b3-7c8a-7890-a123-000000000099", hash, req)
	if err != nil {
		t.Fatalf("unexpected error from CreateForMerchant: %v", err)
	}
	if replayed {
		t.Error("expected fresh request, got replayed=true")
	}
	if result.ParcelID == "" {
		t.Error("expected non-empty ParcelID")
	}
	if !strings.HasPrefix(result.TrackingNumber, "SD-") {
		t.Errorf("expected tracking number starting with SD-, got %s", result.TrackingNumber)
	}
	if result.MerchantReference != req.MerchantReference {
		t.Errorf("expected merchant reference %s, got %s", req.MerchantReference, result.MerchantReference)
	}
	if result.Status != "CREATED" {
		t.Errorf("expected status CREATED, got %s", result.Status)
	}
	if result.Fee.Currency != "LKR" {
		t.Errorf("expected currency LKR, got %s", result.Fee.Currency)
	}
	if result.Fee.AmountMinor != 65000 { // 50000 + (1.5 * 10000) = 65000 cents
		t.Errorf("expected fee 65000, got %d", result.Fee.AmountMinor)
	}
	if len(parcelRepo.parcels) != 1 {
		t.Fatalf("expected 1 persisted parcel, got %d", len(parcelRepo.parcels))
	}
}

func TestParcelsBookingAdapter_CreateForMerchant_IdempotentReplay(t *testing.T) {
	adapter, parcelRepo, _, _ := setupTestAdapter(t)
	merchantID := "0192e2b3-7c8a-7890-a123-000000000001"
	idemKey := "0192e2b3-7c8a-7890-a123-000000000099"
	hash := "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"
	req := validTestOrderRequest()

	// 1. Initial execution
	res1, replayed1, err := adapter.CreateForMerchant(context.Background(), merchantID, idemKey, hash, req)
	if err != nil {
		t.Fatalf("initial execution failed: %v", err)
	}
	if replayed1 {
		t.Error("expected initial request replayed=false")
	}

	// 2. Replay execution
	res2, replayed2, err := adapter.CreateForMerchant(context.Background(), merchantID, idemKey, hash, req)
	if err != nil {
		t.Fatalf("replay execution failed: %v", err)
	}
	if !replayed2 {
		t.Error("expected replayed=true on second call")
	}
	if res2.ParcelID != res1.ParcelID {
		t.Errorf("expected same ParcelID %s, got %s", res1.ParcelID, res2.ParcelID)
	}
	if res2.TrackingNumber != res1.TrackingNumber {
		t.Errorf("expected same TrackingNumber %s, got %s", res1.TrackingNumber, res2.TrackingNumber)
	}
	if len(parcelRepo.parcels) != 1 {
		t.Errorf("expected parcel count to remain 1 on replay, got %d", len(parcelRepo.parcels))
	}
}

func TestParcelsBookingAdapter_CreateForMerchant_IdempotencyConflict(t *testing.T) {
	adapter, _, _, _ := setupTestAdapter(t)
	merchantID := "0192e2b3-7c8a-7890-a123-000000000001"
	idemKey := "0192e2b3-7c8a-7890-a123-000000000099"
	hash1 := "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"
	hash2 := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	req := validTestOrderRequest()

	_, _, err := adapter.CreateForMerchant(context.Background(), merchantID, idemKey, hash1, req)
	if err != nil {
		t.Fatalf("first execution failed: %v", err)
	}

	// Reusing same key with different hash -> ErrIdempotencyReused
	_, _, err = adapter.CreateForMerchant(context.Background(), merchantID, idemKey, hash2, req)
	if !errors.Is(err, application.ErrIdempotencyReused) {
		t.Fatalf("expected ErrIdempotencyReused on payload mismatch, got: %v", err)
	}
}

func TestParcelsBookingAdapter_CreateForMerchant_DimensionsVariations(t *testing.T) {
	merchantID := "0192e2b3-7c8a-7890-a123-000000000001"

	t.Run("dimensions via dimensions_cm object", func(t *testing.T) {
		adapter, _, _, _ := setupTestAdapter(t)
		req := validTestOrderRequest()
		req.Parcel.LengthCM = 0
		req.Parcel.WidthCM = 0
		req.Parcel.HeightCM = 0
		req.Parcel.DimensionsCM = &application.DimensionsRequest{
			Length: 40.0,
			Width:  30.0,
			Height: 20.0,
		}

		res, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		if err != nil {
			t.Fatalf("unexpected error with dimensions_cm: %v", err)
		}
		if res.ParcelID == "" {
			t.Error("expected valid ParcelID")
		}
	})

	t.Run("dimensions via dimensions object", func(t *testing.T) {
		adapter, _, _, _ := setupTestAdapter(t)
		req := validTestOrderRequest()
		req.Parcel.LengthCM = 0
		req.Parcel.WidthCM = 0
		req.Parcel.HeightCM = 0
		req.Parcel.Dimensions = &application.DimensionsRequest{
			LengthCM: 35.0,
			WidthCM:  25.0,
			HeightCM: 15.0,
		}

		res, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		if err != nil {
			t.Fatalf("unexpected error with dimensions object: %v", err)
		}
		if res.ParcelID == "" {
			t.Error("expected valid ParcelID")
		}
	})

	t.Run("missing dimensions returns validation error", func(t *testing.T) {
		adapter, _, _, _ := setupTestAdapter(t)
		req := validTestOrderRequest()
		req.Parcel.LengthCM = 0
		req.Parcel.WidthCM = 0
		req.Parcel.HeightCM = 0
		req.Parcel.DimensionsCM = nil
		req.Parcel.Dimensions = nil

		_, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		var ve application.ValidationError
		if !errors.As(err, &ve) || ve.Field != "parcel.dimensions" {
			t.Fatalf("expected ValidationError on parcel.dimensions, got: %v", err)
		}
	})

	t.Run("negative dimensions returns validation error", func(t *testing.T) {
		adapter, _, _, _ := setupTestAdapter(t)
		req := validTestOrderRequest()
		req.Parcel.LengthCM = -10.0

		_, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		var ve application.ValidationError
		if !errors.As(err, &ve) || ve.Field != "parcel.dimensions" {
			t.Fatalf("expected ValidationError on parcel.dimensions, got: %v", err)
		}
	})
}

func TestParcelsBookingAdapter_CreateForMerchant_WeightValidation(t *testing.T) {
	adapter, _, _, _ := setupTestAdapter(t)
	merchantID := "0192e2b3-7c8a-7890-a123-000000000001"

	t.Run("missing or zero weight", func(t *testing.T) {
		req := validTestOrderRequest()
		req.Parcel.WeightKG = 0
		req.Parcel.WeightGrams = 0

		_, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		var ve application.ValidationError
		if !errors.As(err, &ve) || ve.Field != "parcel.weight_kg" {
			t.Fatalf("expected ValidationError on parcel.weight_kg, got: %v", err)
		}
	})

	t.Run("weight_grams conversion", func(t *testing.T) {
		req := validTestOrderRequest()
		req.Parcel.WeightKG = 0
		req.Parcel.WeightGrams = 3000 // 3.0 kg

		res, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		if err != nil {
			t.Fatalf("unexpected error converting weight_grams: %v", err)
		}
		if res.ParcelID == "" {
			t.Error("expected valid ParcelID")
		}
	})

	t.Run("chargeable weight exceeds 30kg", func(t *testing.T) {
		req := validTestOrderRequest()
		req.Parcel.WeightKG = 35.0

		_, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		var ve application.ValidationError
		if !errors.As(err, &ve) || ve.Field != "parcel.weight_kg" {
			t.Fatalf("expected ValidationError on parcel.weight_kg exceeding limit, got: %v", err)
		}
	})
}

func TestParcelsBookingAdapter_CreateForMerchant_AddressValidation(t *testing.T) {
	merchantID := "0192e2b3-7c8a-7890-a123-000000000001"

	t.Run("missing pickup address", func(t *testing.T) {
		adapter, _, _, _ := setupTestAdapter(t)
		req := validTestOrderRequest()
		req.Pickup = nil

		_, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		var ve application.ValidationError
		if !errors.As(err, &ve) || ve.Field != "pickup" {
			t.Fatalf("expected ValidationError on pickup, got: %v", err)
		}
	})

	t.Run("missing dropoff address and recipient", func(t *testing.T) {
		adapter, _, _, _ := setupTestAdapter(t)
		req := validTestOrderRequest()
		req.Dropoff = nil
		req.Recipient = nil

		_, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		var ve application.ValidationError
		if !errors.As(err, &ve) || ve.Field != "dropoff" {
			t.Fatalf("expected ValidationError on dropoff, got: %v", err)
		}
	})

	t.Run("fallback to recipient map", func(t *testing.T) {
		adapter, _, _, _ := setupTestAdapter(t)
		req := validTestOrderRequest()
		req.Recipient = req.Dropoff
		req.Dropoff = nil

		res, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		if err != nil {
			t.Fatalf("unexpected error falling back to recipient: %v", err)
		}
		if res.ParcelID == "" {
			t.Error("expected valid ParcelID")
		}
	})

	t.Run("nested address structure", func(t *testing.T) {
		adapter, _, _, _ := setupTestAdapter(t)
		req := validTestOrderRequest()
		req.Dropoff = map[string]any{
			"contact_name":  "Alice Perera",
			"contact_phone": "+94770001122",
			"address": map[string]string{
				"line1":       "250 Kandy Road",
				"city":        "Kadawatha",
				"postal_code": "11850",
				"zone_id":     "ZONE_CENTRAL_01",
			},
		}

		res, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		if err != nil {
			t.Fatalf("unexpected error with nested address: %v", err)
		}
		if res.ParcelID == "" {
			t.Error("expected valid ParcelID")
		}
	})

	t.Run("address_lines array structure", func(t *testing.T) {
		adapter, _, _, _ := setupTestAdapter(t)
		req := validTestOrderRequest()
		req.Dropoff = map[string]any{
			"name":          "Alice Perera",
			"phone":         "+94770001122",
			"address_lines": []string{"250 Kandy Road", "Suite 4B"},
			"city":          "Kadawatha",
			"postal_code":   "11850",
			"zone_id":       "ZONE_CENTRAL_01",
		}

		res, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		if err != nil {
			t.Fatalf("unexpected error with address_lines: %v", err)
		}
		if res.ParcelID == "" {
			t.Error("expected valid ParcelID")
		}
	})

	t.Run("invalid zone_id format", func(t *testing.T) {
		adapter, _, _, _ := setupTestAdapter(t)
		req := validTestOrderRequest()
		req.Dropoff["zone_id"] = "not_a_valid_zone"

		_, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		var ve application.ValidationError
		if !errors.As(err, &ve) || ve.Field != "dropoff.zone_id" {
			t.Fatalf("expected ValidationError on dropoff.zone_id, got: %v", err)
		}
	})
}

func TestParcelsBookingAdapter_CreateForMerchant_ServiceLevel(t *testing.T) {
	merchantID := "0192e2b3-7c8a-7890-a123-000000000001"

	t.Run("invalid service level", func(t *testing.T) {
		adapter, _, _, _ := setupTestAdapter(t)
		req := validTestOrderRequest()
		req.ServiceLevel = "WARP_SPEED"

		_, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		var ve application.ValidationError
		if !errors.As(err, &ve) || ve.Field != "service_level" {
			t.Fatalf("expected ValidationError on service_level, got: %v", err)
		}
	})

	t.Run("empty service level defaults to STANDARD", func(t *testing.T) {
		adapter, _, _, _ := setupTestAdapter(t)
		req := validTestOrderRequest()
		req.ServiceLevel = ""

		res, _, err := adapter.CreateForMerchant(context.Background(), merchantID, "", "", req)
		if err != nil {
			t.Fatalf("unexpected error with blank service level: %v", err)
		}
		if res.ParcelID == "" {
			t.Error("expected valid ParcelID")
		}
	})
}

func TestParcelsBookingAdapter_CreateForMerchant_MerchantSecurity(t *testing.T) {
	adapter, _, _, _ := setupTestAdapter(t)
	req := validTestOrderRequest()

	t.Run("empty merchant ID rejected", func(t *testing.T) {
		_, _, err := adapter.CreateForMerchant(context.Background(), "   ", "", "", req)
		var ve application.ValidationError
		if !errors.As(err, &ve) || ve.Field != "merchant_id" {
			t.Fatalf("expected ValidationError on merchant_id, got: %v", err)
		}
	})

	t.Run("non-UUID merchant ID rejected", func(t *testing.T) {
		_, _, err := adapter.CreateForMerchant(context.Background(), "merchant-001", "", "", req)
		var ve application.ValidationError
		if !errors.As(err, &ve) || ve.Field != "merchant_id" {
			t.Fatalf("expected ValidationError on merchant_id, got: %v", err)
		}
	})
}

func TestParcelsBookingAdapter_ListForMerchant(t *testing.T) {
	adapter, parcelRepo, _, now := setupTestAdapter(t)
	merchantAID := "0192e2b3-7c8a-7890-a123-000000000001"
	merchantBID := "0192e2b3-7c8a-7890-a123-000000000002"

	// Seed parcels for merchant A
	p1 := parcelsdomain.Parcel{
		ID:             "0192e2b3-7c8a-7890-a123-000000000101",
		TrackingNumber: "SD-20261119-00001",
		MerchantID:     &merchantAID,
		Status:         parcelsdomain.StatusCreated,
		DeliveryFee:    parcelsdomain.Money{Amount: 50000, Currency: "LKR"},
		CreatedAt:      now.Add(-2 * time.Hour),
	}
	p2 := parcelsdomain.Parcel{
		ID:             "0192e2b3-7c8a-7890-a123-000000000102",
		TrackingNumber: "SD-20261119-00002",
		MerchantID:     &merchantAID,
		Status:         parcelsdomain.StatusInTransit,
		DeliveryFee:    parcelsdomain.Money{Amount: 75000, Currency: "LKR"},
		CreatedAt:      now.Add(-1 * time.Hour),
	}

	// Seed parcel for merchant B (tenant isolation check)
	p3 := parcelsdomain.Parcel{
		ID:             "0192e2b3-7c8a-7890-a123-000000000201",
		TrackingNumber: "SD-20261119-00003",
		MerchantID:     &merchantBID,
		Status:         parcelsdomain.StatusCreated,
		DeliveryFee:    parcelsdomain.Money{Amount: 60000, Currency: "LKR"},
		CreatedAt:      now,
	}

	_ = parcelRepo.Save(context.Background(), nil, p1)
	_ = parcelRepo.Save(context.Background(), nil, p2)
	_ = parcelRepo.Save(context.Background(), nil, p3)

	t.Run("lists parcels for merchant A newest first", func(t *testing.T) {
		orders, err := adapter.ListForMerchant(context.Background(), merchantAID)
		if err != nil {
			t.Fatalf("unexpected error listing orders: %v", err)
		}
		if len(orders) != 2 {
			t.Fatalf("expected 2 orders for merchant A, got %d", len(orders))
		}
		// Newest first
		if orders[0].ParcelID != p2.ID {
			t.Errorf("expected newest order %s first, got %s", p2.ID, orders[0].ParcelID)
		}
		if orders[1].ParcelID != p1.ID {
			t.Errorf("expected older order %s second, got %s", p1.ID, orders[1].ParcelID)
		}
		if orders[0].TrackingNumber != "SD-20261119-00002" {
			t.Errorf("expected tracking number SD-20261119-00002, got %s", orders[0].TrackingNumber)
		}
		if orders[0].Status != "IN_TRANSIT" {
			t.Errorf("expected status IN_TRANSIT, got %s", orders[0].Status)
		}
		if orders[0].Fee.AmountMinor != 75000 {
			t.Errorf("expected fee 75000, got %d", orders[0].Fee.AmountMinor)
		}
	})

	t.Run("tenant isolation: merchant A does not see merchant B parcels", func(t *testing.T) {
		ordersA, err := adapter.ListForMerchant(context.Background(), merchantAID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, o := range ordersA {
			if o.ParcelID == p3.ID {
				t.Fatalf("tenant isolation violation: merchant A saw merchant B parcel %s", p3.ID)
			}
		}

		ordersB, err := adapter.ListForMerchant(context.Background(), merchantBID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ordersB) != 1 || ordersB[0].ParcelID != p3.ID {
			t.Fatalf("expected 1 order for merchant B (%s), got %v", p3.ID, ordersB)
		}
	})

	t.Run("empty list for merchant with no orders returns empty non-nil slice", func(t *testing.T) {
		orders, err := adapter.ListForMerchant(context.Background(), "0192e2b3-7c8a-7890-a123-000000000999")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if orders == nil {
			t.Error("expected non-nil empty slice")
		}
		if len(orders) != 0 {
			t.Errorf("expected 0 orders, got %d", len(orders))
		}
	})

	t.Run("invalid merchant ID returns validation error", func(t *testing.T) {
		_, err := adapter.ListForMerchant(context.Background(), "invalid-uuid")
		var ve application.ValidationError
		if !errors.As(err, &ve) || ve.Field != "merchant_id" {
			t.Fatalf("expected ValidationError on merchant_id, got: %v", err)
		}
	})
}
