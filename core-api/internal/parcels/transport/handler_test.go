package transport

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/events/eventstest"
	identityapp "github.com/swiftdrop/core-api/internal/identity/application"
	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityinfra "github.com/swiftdrop/core-api/internal/identity/infrastructure"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/parcels/application"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
	"github.com/swiftdrop/core-api/internal/platform/middleware"
)

const (
	testCustomerUserID     = "018f4a7c-1005-7000-8000-000000000005"
	testOtherCustomerID    = "018f4a7c-9999-7000-8000-000000000099"
	testAdminUserID        = "018f4a7c-1001-7000-8000-000000000001"
	testCourierUserID      = "018f4a7c-1003-7000-8000-000000000003"
	testDispatcherUserID   = "018f4a7c-1002-7000-8000-000000000002"
	testMerchantOwnerID    = "018f4a7c-1004-7000-8000-000000000004"
	testOtherMerchantOwner = "018f4a7c-1006-7000-8000-000000000006"
	testMerchantID         = "0192e2b3-7c8a-7890-a123-000000000010"
	testOtherMerchantID    = "0192e2b3-7c8a-7890-a123-000000000020"
	testCustomerQuoteID    = "0192e2b3-7c8a-7890-a123-000000000777"
	testCustomerQuote2ID   = "0192e2b3-7c8a-7890-a123-000000000778"
	testMerchantQuoteID    = "0192e2b3-7c8a-7890-a123-000000000779"
	testExpiredQuoteID     = "0192e2b3-7c8a-7890-a123-000000000780"
	testAdminQuoteID       = "0192e2b3-7c8a-7890-a123-000000000781"
	testIdemKey1           = "11111111-1111-4111-8111-111111111111"
	testIdemKey2           = "22222222-2222-4222-8222-222222222222"
	testMissingParcelID    = "0192e2b3-7c8a-7890-a123-000000000404"
)

type fakeClock struct {
	now time.Time
}

func (f *fakeClock) Now() time.Time {
	return f.now
}

type fakeIDGen struct{}

func (fakeIDGen) GenerateID() (string, error) {
	return events.GenerateUUIDv7()
}

type fakeTrackingGen struct {
	mu  sync.Mutex
	seq int
}

func (f *fakeTrackingGen) Generate(_ context.Context, date time.Time) (domain.TrackingNumber, error) {
	f.mu.Lock()
	f.seq++
	n := f.seq
	f.mu.Unlock()

	alphabet := domain.CrockfordBase32Alphabet
	var suffix [6]byte
	for i := 5; i >= 0; i-- {
		suffix[i] = alphabet[n%len(alphabet)]
		n /= len(alphabet)
	}
	return domain.FormatTrackingNumber(date, string(suffix[:]))
}

type storedQuote struct {
	snapshot application.AcceptedQuoteSnapshot
	used     bool
}

type fakeQuoteConsumer struct {
	mu           sync.Mutex
	quotes       map[string]storedQuote
	consumeCount int
}

func newFakeQuoteConsumer() *fakeQuoteConsumer {
	return &fakeQuoteConsumer{
		quotes: make(map[string]storedQuote),
	}
}

func (f *fakeQuoteConsumer) ConsumeQuote(_ context.Context, _ db.Querier, quoteID string, acceptedAt time.Time) (application.AcceptedQuoteSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.consumeCount++
	rec, ok := f.quotes[quoteID]
	if !ok {
		return application.AcceptedQuoteSnapshot{}, application.ErrQuoteNotFound
	}
	if rec.used {
		return application.AcceptedQuoteSnapshot{}, application.ErrQuoteAlreadyUsed
	}
	if !acceptedAt.Before(rec.snapshot.ExpiresAt) {
		return application.AcceptedQuoteSnapshot{}, application.ErrQuoteExpired
	}

	rec.used = true
	rec.snapshot.UsedAt = acceptedAt
	f.quotes[quoteID] = rec
	return rec.snapshot, nil
}

type fakeParcelRepo struct {
	mu         sync.Mutex
	parcels    map[string]domain.Parcel
	tracking   map[string]bool
	saveCalled int
}

func newFakeParcelRepo() *fakeParcelRepo {
	return &fakeParcelRepo{
		parcels:  make(map[string]domain.Parcel),
		tracking: make(map[string]bool),
	}
}

func (f *fakeParcelRepo) Save(_ context.Context, _ db.Querier, parcel domain.Parcel) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saveCalled++
	f.parcels[parcel.ID] = parcel
	f.tracking[parcel.TrackingNumber.String()] = true
	return nil
}

func (f *fakeParcelRepo) ExistsTrackingNumber(_ context.Context, _ db.Querier, tn domain.TrackingNumber) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tracking[tn.String()], nil
}

func (f *fakeParcelRepo) FindByID(_ context.Context, _ db.Querier, id string) (domain.Parcel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.parcels[id]
	if !ok {
		return domain.Parcel{}, domain.ErrInvalidParcelID
	}
	return p, nil
}

func (f *fakeParcelRepo) UpdateStatus(_ context.Context, _ db.Querier, parcel domain.Parcel, expectedVersion int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.parcels[parcel.ID]
	if !ok {
		return domain.ErrInvalidParcelID
	}
	if existing.Version != expectedVersion {
		return application.ErrConcurrencyConflict
	}
	f.parcels[parcel.ID] = parcel
	return nil
}

type fakeStatusHistoryRepo struct {
	mu             sync.Mutex
	entries        []domain.StatusHistoryEntry
	findByParcelFn int
}

func newFakeStatusHistoryRepo() *fakeStatusHistoryRepo {
	return &fakeStatusHistoryRepo{
		entries: make([]domain.StatusHistoryEntry, 0),
	}
}

func (f *fakeStatusHistoryRepo) Save(_ context.Context, _ db.Querier, entry domain.StatusHistoryEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeStatusHistoryRepo) FindByParcelID(_ context.Context, _ db.Querier, parcelID string) ([]domain.StatusHistoryEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.findByParcelFn++
	out := make([]domain.StatusHistoryEntry, 0)
	for _, entry := range f.entries {
		if entry.ParcelID == parcelID {
			out = append(out, entry)
		}
	}
	return out, nil
}

type fakeIdempotencyStore struct {
	mu      sync.Mutex
	records map[string]application.IdempotencyRecord
}

func newFakeIdempotencyStore() *fakeIdempotencyStore {
	return &fakeIdempotencyStore{
		records: make(map[string]application.IdempotencyRecord),
	}
}

func idemCompositeKey(key, callerScope string) string {
	return callerScope + "|" + key
}

func (f *fakeIdempotencyStore) AcquireLock(_ context.Context, _ db.Querier, _, _ string) error {
	return nil
}

func (f *fakeIdempotencyStore) Find(_ context.Context, _ db.Querier, key, callerScope string) (application.IdempotencyRecord, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.records[idemCompositeKey(key, callerScope)]
	return rec, ok, nil
}

func (f *fakeIdempotencyStore) Save(_ context.Context, _ db.Querier, record application.IdempotencyRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records[idemCompositeKey(record.Key, record.CallerScope)] = record
	return nil
}

var registerTransportMockDriverOnce sync.Once

type transportMockDriver struct{}

func (transportMockDriver) Open(_ string) (driver.Conn, error) { return transportMockConn{}, nil }

type transportMockConn struct{}

func (transportMockConn) Prepare(_ string) (driver.Stmt, error) { return nil, errors.New("unused") }
func (transportMockConn) Close() error                          { return nil }
func (transportMockConn) Begin() (driver.Tx, error)             { return transportMockTx{}, nil }

type transportMockTx struct{}

func (transportMockTx) Commit() error   { return nil }
func (transportMockTx) Rollback() error { return nil }

func newTransportMockDB(t *testing.T) *sql.DB {
	t.Helper()
	registerTransportMockDriverOnce.Do(func() {
		sql.Register("parcel_transport_mock_driver", transportMockDriver{})
	})
	conn, err := sql.Open("parcel_transport_mock_driver", "test")
	if err != nil {
		t.Fatalf("sql.Open mock driver failed: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// atomicFakeTxManager snapshots and restores in-memory stores if the transactional callback fails,
// mirroring PostgreSQL transaction rollback semantics during transport tests.
type atomicFakeTxManager struct {
	sqlDB   *sql.DB
	quotes  *fakeQuoteConsumer
	parcels *fakeParcelRepo
	history *fakeStatusHistoryRepo
	idem    *fakeIdempotencyStore
}

func (m *atomicFakeTxManager) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := m.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	m.quotes.mu.Lock()
	quoteSnap := make(map[string]storedQuote, len(m.quotes.quotes))
	for k, v := range m.quotes.quotes {
		quoteSnap[k] = v
	}
	m.quotes.mu.Unlock()

	m.parcels.mu.Lock()
	parcelSnap := make(map[string]domain.Parcel, len(m.parcels.parcels))
	for k, v := range m.parcels.parcels {
		parcelSnap[k] = v
	}
	tnSnap := make(map[string]bool, len(m.parcels.tracking))
	for k, v := range m.parcels.tracking {
		tnSnap[k] = v
	}
	m.parcels.mu.Unlock()

	m.history.mu.Lock()
	histSnap := make([]domain.StatusHistoryEntry, len(m.history.entries))
	copy(histSnap, m.history.entries)
	m.history.mu.Unlock()

	m.idem.mu.Lock()
	idemSnap := make(map[string]application.IdempotencyRecord, len(m.idem.records))
	for k, v := range m.idem.records {
		idemSnap[k] = v
	}
	m.idem.mu.Unlock()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()

		m.quotes.mu.Lock()
		m.quotes.quotes = quoteSnap
		m.quotes.mu.Unlock()

		m.parcels.mu.Lock()
		m.parcels.parcels = parcelSnap
		m.parcels.tracking = tnSnap
		m.parcels.mu.Unlock()

		m.history.mu.Lock()
		m.history.entries = histSnap
		m.history.mu.Unlock()

		m.idem.mu.Lock()
		m.idem.records = idemSnap
		m.idem.mu.Unlock()

		return err
	}

	return tx.Commit()
}

type fakeMerchantReader struct {
	profiles map[string]application.MerchantProfile
	err      error
	called   bool
	lastID   string
}

func (f *fakeMerchantReader) FindByID(_ context.Context, merchantID string) (application.MerchantProfile, error) {
	f.called = true
	f.lastID = merchantID
	if f.err != nil {
		return application.MerchantProfile{}, f.err
	}
	p, ok := f.profiles[merchantID]
	if !ok {
		return application.MerchantProfile{}, application.ErrMerchantNotFound
	}
	return p, nil
}

type parcelTestHarness struct {
	handler        *Handler
	mux            http.Handler
	tokenProvider  *identityinfra.JWTTokenProvider
	bookUC         *application.BookParcelUseCase
	getParcelUC    *application.GetParcelUseCase
	getHistoryUC   *application.GetParcelHistoryUseCase
	quoteConsumer  *fakeQuoteConsumer
	parcelRepo     *fakeParcelRepo
	historyRepo    *fakeStatusHistoryRepo
	idemStore      *fakeIdempotencyStore
	outboxWriter   *eventstest.InMemoryWriter
	merchantReader *fakeMerchantReader
	clock          *fakeClock
}

func newParcelTestHarness(t *testing.T) *parcelTestHarness {
	t.Helper()

	now := time.Date(2026, time.November, 11, 10, 0, 0, 0, time.UTC)
	clk := &fakeClock{now: now}
	quotes := newFakeQuoteConsumer()
	quotes.quotes[testCustomerQuoteID] = storedQuote{
		snapshot: application.AcceptedQuoteSnapshot{
			QuoteID:            testCustomerQuoteID,
			TariffID:           "rule_west_central_v2",
			TariffVersion:      2,
			RequesterID:        strPtr(testCustomerUserID),
			MerchantID:         nil,
			OriginZoneID:       "ZONE_WESTERN_01",
			DestinationZoneID:  "ZONE_CENTRAL_01",
			ChargeableWeightKg: 2.5,
			ServiceLevel:       "STANDARD",
			FeeCents:           47000,
			Currency:           domain.CurrencyLKR,
			CreatedAt:          now.Add(-5 * time.Minute),
			ExpiresAt:          now.Add(10 * time.Minute),
		},
	}
	quotes.quotes[testCustomerQuote2ID] = storedQuote{
		snapshot: application.AcceptedQuoteSnapshot{
			QuoteID:            testCustomerQuote2ID,
			TariffID:           "rule_west_central_v2",
			TariffVersion:      2,
			RequesterID:        strPtr(testOtherCustomerID),
			MerchantID:         nil,
			OriginZoneID:       "ZONE_WESTERN_01",
			DestinationZoneID:  "ZONE_CENTRAL_01",
			ChargeableWeightKg: 2.5,
			ServiceLevel:       "EXPRESS",
			FeeCents:           70500,
			Currency:           domain.CurrencyLKR,
			CreatedAt:          now.Add(-5 * time.Minute),
			ExpiresAt:          now.Add(10 * time.Minute),
		},
	}
	quotes.quotes[testMerchantQuoteID] = storedQuote{
		snapshot: application.AcceptedQuoteSnapshot{
			QuoteID:            testMerchantQuoteID,
			TariffID:           "rule_west_central_v2",
			TariffVersion:      2,
			RequesterID:        strPtr(testMerchantOwnerID),
			MerchantID:         strPtr(testMerchantID),
			OriginZoneID:       "ZONE_WESTERN_01",
			DestinationZoneID:  "ZONE_CENTRAL_01",
			ChargeableWeightKg: 2.5,
			ServiceLevel:       "STANDARD",
			FeeCents:           47000,
			Currency:           domain.CurrencyLKR,
			CreatedAt:          now.Add(-5 * time.Minute),
			ExpiresAt:          now.Add(10 * time.Minute),
		},
	}
	quotes.quotes[testExpiredQuoteID] = storedQuote{
		snapshot: application.AcceptedQuoteSnapshot{
			QuoteID:            testExpiredQuoteID,
			TariffID:           "rule_west_central_v2",
			TariffVersion:      2,
			RequesterID:        strPtr(testCustomerUserID),
			MerchantID:         nil,
			OriginZoneID:       "ZONE_WESTERN_01",
			DestinationZoneID:  "ZONE_CENTRAL_01",
			ChargeableWeightKg: 2.5,
			ServiceLevel:       "STANDARD",
			FeeCents:           47000,
			Currency:           domain.CurrencyLKR,
			CreatedAt:          now.Add(-20 * time.Minute),
			ExpiresAt:          now.Add(-5 * time.Minute),
		},
	}
	quotes.quotes[testAdminQuoteID] = storedQuote{
		snapshot: application.AcceptedQuoteSnapshot{
			QuoteID:            testAdminQuoteID,
			TariffID:           "rule_west_central_v2",
			TariffVersion:      2,
			RequesterID:        strPtr(testAdminUserID),
			MerchantID:         nil,
			OriginZoneID:       "ZONE_WESTERN_01",
			DestinationZoneID:  "ZONE_CENTRAL_01",
			ChargeableWeightKg: 2.5,
			ServiceLevel:       "STANDARD",
			FeeCents:           47000,
			Currency:           domain.CurrencyLKR,
			CreatedAt:          now.Add(-5 * time.Minute),
			ExpiresAt:          now.Add(10 * time.Minute),
		},
	}

	parcelRepo := newFakeParcelRepo()
	historyRepo := newFakeStatusHistoryRepo()
	idemStore := newFakeIdempotencyStore()
	outbox := eventstest.NewInMemoryWriter()
	txMgr := &atomicFakeTxManager{
		sqlDB:   newTransportMockDB(t),
		quotes:  quotes,
		parcels: parcelRepo,
		history: historyRepo,
		idem:    idemStore,
	}

	merchantReader := &fakeMerchantReader{
		profiles: map[string]application.MerchantProfile{
			testMerchantID: {
				ID:          testMerchantID,
				OwnerUserID: testMerchantOwnerID,
				Status:      application.MerchantStatusApproved,
			},
			testOtherMerchantID: {
				ID:          testOtherMerchantID,
				OwnerUserID: testOtherMerchantOwner,
				Status:      application.MerchantStatusApproved,
			},
		},
	}

	bookUC, err := application.NewBookParcelUseCase(
		txMgr,
		quotes,
		parcelRepo,
		historyRepo,
		outbox,
		&fakeTrackingGen{},
		fakeIDGen{},
		clk,
	)
	if err != nil {
		t.Fatalf("NewBookParcelUseCase failed: %v", err)
	}
	bookUC.WithIdempotency(idemStore, application.DefaultIdempotencyTTL)

	getParcelUC, err := application.NewGetParcelUseCase(parcelRepo, merchantReader)
	if err != nil {
		t.Fatalf("NewGetParcelUseCase failed: %v", err)
	}
	getHistoryUC, err := application.NewGetParcelHistoryUseCase(parcelRepo, historyRepo, merchantReader)
	if err != nil {
		t.Fatalf("NewGetParcelHistoryUseCase failed: %v", err)
	}

	h, err := NewHandler(bookUC, getParcelUC, getHistoryUC, merchantReader)
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	tokenProv := identityinfra.NewJWTTokenProvider("test-jwt-secret-key-at-least-32-chars")
	sec := identitytransport.NewSecurityMiddleware(tokenProv)

	mux := http.NewServeMux()
	RegisterRoutes(mux, h, sec)

	return &parcelTestHarness{
		handler:        h,
		mux:            middleware.RequestID(mux),
		tokenProvider:  tokenProv,
		bookUC:         bookUC,
		getParcelUC:    getParcelUC,
		getHistoryUC:   getHistoryUC,
		quoteConsumer:  quotes,
		parcelRepo:     parcelRepo,
		historyRepo:    historyRepo,
		idemStore:      idemStore,
		outboxWriter:   outbox,
		merchantReader: merchantReader,
		clock:          clk,
	}
}

func (th *parcelTestHarness) issueToken(t *testing.T, p identityapp.Principal) string {
	t.Helper()
	tok, err := th.tokenProvider.GenerateAccessToken(p)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}
	return tok
}

func (th *parcelTestHarness) doJSON(t *testing.T, method, path, token, idemKey, body string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	rec := httptest.NewRecorder()
	th.mux.ServeHTTP(rec, req)
	return rec
}

func strPtr(s string) *string {
	return &s
}

func validBookingJSON(quoteID string) string {
	return fmt.Sprintf(`{
		"quote_id": %q,
		"recipient_name": "Nimal Fernando",
		"recipient_phone": "+94719876543",
		"weight_kg": 2.5,
		"dimensions": {
			"length_cm": 30.0,
			"width_cm": 25.0,
			"height_cm": 15.0
		},
		"pickup": {
			"contact_name": "Kasun Perera",
			"contact_phone": "+94771234567",
			"address_line1": "45 Galle Road",
			"address_line2": "Suite 3B",
			"city": "Colombo 03",
			"postal_code": "00300",
			"zone_id": "ZONE_WESTERN_01",
			"instructions": "Ring intercom 3B"
		},
		"dropoff": {
			"contact_name": "Nimal Fernando",
			"contact_phone": "+94719876543",
			"address_line1": "12 Kandy Road",
			"address_line2": "",
			"city": "Peradeniya",
			"postal_code": "20400",
			"zone_id": "ZONE_CENTRAL_01",
			"instructions": "Leave at front desk"
		},
		"item_description": "Laptop battery pack",
		"special_instructions": "Handle with care"
	}`, quoteID)
}

func assertParcelErrorEnvelope(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int, expectedCode string) map[string]any {
	t.Helper()
	if rec.Code != expectedStatus {
		t.Fatalf("expected HTTP %d, got %d; body=%s", expectedStatus, rec.Code, rec.Body.String())
	}

	var root map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&root); err != nil {
		t.Fatalf("failed to decode JSON error response: %v", err)
	}
	errObj, ok := root["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected canonical {\"error\": ...} envelope, got %v", root)
	}
	if errObj["code"] != expectedCode {
		t.Fatalf("expected error code %q, got %v", expectedCode, errObj["code"])
	}
	if _, hasMsg := errObj["message"].(string); !hasMsg {
		t.Fatalf("expected error message string, got %v", errObj)
	}
	return errObj
}

func assertParcelDataEnvelope(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int) map[string]any {
	t.Helper()
	if rec.Code != expectedStatus {
		t.Fatalf("expected HTTP %d, got %d; body=%s", expectedStatus, rec.Code, rec.Body.String())
	}
	var root map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&root); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if len(root) != 1 {
		t.Fatalf("expected top-level keys strictly containing {\"data\": ...}, got %v", root)
	}
	data, ok := root["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected canonical {\"data\": {...}} envelope, got %v", root)
	}
	return data
}

func TestNewHandler_NilDependencies(t *testing.T) {
	th := newParcelTestHarness(t)

	if _, err := NewHandler(nil, th.getParcelUC, th.getHistoryUC, th.merchantReader); err == nil {
		t.Fatal("expected error for nil bookUseCase")
	}
	if _, err := NewHandler(th.bookUC, nil, th.getHistoryUC, th.merchantReader); err == nil {
		t.Fatal("expected error for nil getParcelUseCase")
	}
	if _, err := NewHandler(th.bookUC, th.getParcelUC, nil, th.merchantReader); err == nil {
		t.Fatal("expected error for nil getHistoryUseCase")
	}
	if _, err := NewHandler(th.bookUC, th.getParcelUC, th.getHistoryUC, nil); err == nil {
		t.Fatal("expected error for nil merchantReader")
	}

	// RegisterRoutes must safely no-op when passed nil inputs.
	RegisterRoutes(nil, th.handler, nil)
}

func TestParcelEndpoints_UnauthenticatedRejected(t *testing.T) {
	th := newParcelTestHarness(t)

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/parcels", validBookingJSON(testCustomerQuoteID)},
		{http.MethodGet, "/api/v1/parcels/" + testMissingParcelID, ""},
		{http.MethodGet, "/api/v1/parcels/" + testMissingParcelID + "/history", ""},
	}

	for _, tc := range cases {
		rec := th.doJSON(t, tc.method, tc.path, "", testIdemKey1, tc.body)
		assertParcelErrorEnvelope(t, rec, http.StatusUnauthorized, "AUTH_UNAUTHORIZED")
	}
}

func TestBookParcel_IdempotencyKeyValidation(t *testing.T) {
	th := newParcelTestHarness(t)
	token := th.issueToken(t, identityapp.Principal{UserID: testCustomerUserID, Role: identitydomain.RoleCustomer})

	recMissing := th.doJSON(t, http.MethodPost, "/api/v1/parcels", token, "", validBookingJSON(testCustomerQuoteID))
	assertParcelErrorEnvelope(t, recMissing, http.StatusBadRequest, "MISSING_IDEMPOTENCY_KEY")

	recInvalid := th.doJSON(t, http.MethodPost, "/api/v1/parcels", token, "not-a-valid-uuid", validBookingJSON(testCustomerQuoteID))
	assertParcelErrorEnvelope(t, recInvalid, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY")

	if th.quoteConsumer.consumeCount != 0 {
		t.Fatalf("expected quote consumer never invoked on invalid Idempotency-Key, got %d", th.quoteConsumer.consumeCount)
	}
	if len(th.parcelRepo.parcels) != 0 {
		t.Fatalf("expected zero parcels persisted, got %d", len(th.parcelRepo.parcels))
	}
}

func TestBookParcel_UnknownFieldsAndBodyLimitsRejected(t *testing.T) {
	th := newParcelTestHarness(t)
	token := th.issueToken(t, identityapp.Principal{UserID: testCustomerUserID, Role: identitydomain.RoleCustomer})

	disallowedPayloads := []struct {
		name string
		body string
	}{
		{"sender_id", `{"quote_id":"` + testCustomerQuoteID + `","sender_id":"` + testOtherCustomerID + `","recipient_name":"Nimal","recipient_phone":"+94719876543","weight_kg":2.5,"dimensions":{"length_cm":30,"width_cm":25,"height_cm":15},"pickup":{"contact_name":"K","contact_phone":"+94771234567","address_line1":"45 Galle Rd","city":"Colombo","postal_code":"00300","zone_id":"ZONE_WESTERN_01"},"dropoff":{"contact_name":"N","contact_phone":"+94719876543","address_line1":"12 Kandy Rd","city":"Kandy","postal_code":"20400","zone_id":"ZONE_CENTRAL_01"}}`},
		{"merchant_id", `{"quote_id":"` + testCustomerQuoteID + `","merchant_id":"` + testMerchantID + `","recipient_name":"Nimal","recipient_phone":"+94719876543","weight_kg":2.5,"dimensions":{"length_cm":30,"width_cm":25,"height_cm":15},"pickup":{"contact_name":"K","contact_phone":"+94771234567","address_line1":"45 Galle Rd","city":"Colombo","postal_code":"00300","zone_id":"ZONE_WESTERN_01"},"dropoff":{"contact_name":"N","contact_phone":"+94719876543","address_line1":"12 Kandy Rd","city":"Kandy","postal_code":"20400","zone_id":"ZONE_CENTRAL_01"}}`},
		{"delivery_fee_cents", `{"quote_id":"` + testCustomerQuoteID + `","delivery_fee_cents":100,"recipient_name":"Nimal","recipient_phone":"+94719876543","weight_kg":2.5,"dimensions":{"length_cm":30,"width_cm":25,"height_cm":15},"pickup":{"contact_name":"K","contact_phone":"+94771234567","address_line1":"45 Galle Rd","city":"Colombo","postal_code":"00300","zone_id":"ZONE_WESTERN_01"},"dropoff":{"contact_name":"N","contact_phone":"+94719876543","address_line1":"12 Kandy Rd","city":"Kandy","postal_code":"20400","zone_id":"ZONE_CENTRAL_01"}}`},
		{"pricing_rule_id", `{"quote_id":"` + testCustomerQuoteID + `","pricing_rule_id":"hacked","recipient_name":"Nimal","recipient_phone":"+94719876543","weight_kg":2.5,"dimensions":{"length_cm":30,"width_cm":25,"height_cm":15},"pickup":{"contact_name":"K","contact_phone":"+94771234567","address_line1":"45 Galle Rd","city":"Colombo","postal_code":"00300","zone_id":"ZONE_WESTERN_01"},"dropoff":{"contact_name":"N","contact_phone":"+94719876543","address_line1":"12 Kandy Rd","city":"Kandy","postal_code":"20400","zone_id":"ZONE_CENTRAL_01"}}`},
		{"status", `{"quote_id":"` + testCustomerQuoteID + `","status":"DELIVERED","recipient_name":"Nimal","recipient_phone":"+94719876543","weight_kg":2.5,"dimensions":{"length_cm":30,"width_cm":25,"height_cm":15},"pickup":{"contact_name":"K","contact_phone":"+94771234567","address_line1":"45 Galle Rd","city":"Colombo","postal_code":"00300","zone_id":"ZONE_WESTERN_01"},"dropoff":{"contact_name":"N","contact_phone":"+94719876543","address_line1":"12 Kandy Rd","city":"Kandy","postal_code":"20400","zone_id":"ZONE_CENTRAL_01"}}`},
		{"trailing_json", validBookingJSON(testCustomerQuoteID) + `{}`},
		{"empty_body", "   "},
		{"malformed_json", `{"quote_id": `},
	}

	for _, tc := range disallowedPayloads {
		t.Run(tc.name, func(t *testing.T) {
			rec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", token, testIdemKey1, tc.body)
			assertParcelErrorEnvelope(t, rec, http.StatusBadRequest, "INVALID_REQUEST_PAYLOAD")
		})
	}

	t.Run("unsupported_media_type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/parcels", strings.NewReader(validBookingJSON(testCustomerQuoteID)))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Idempotency-Key", testIdemKey1)
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()
		th.mux.ServeHTTP(rec, req)
		assertParcelErrorEnvelope(t, rec, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE")
	})

	t.Run("payload_too_large_over_64kb", func(t *testing.T) {
		oversized := strings.Repeat("a", int(maxParcelBodyBytes)+512)
		body := fmt.Sprintf(`{"quote_id":%q,"item_description":%q}`, testCustomerQuoteID, oversized)
		rec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", token, testIdemKey1, body)
		assertParcelErrorEnvelope(t, rec, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE")
	})
}

func TestBookParcel_CustomerSuccessAndCorrelationID(t *testing.T) {
	th := newParcelTestHarness(t)
	token := th.issueToken(t, identityapp.Principal{UserID: testCustomerUserID, Role: identitydomain.RoleCustomer})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parcels", strings.NewReader(validBookingJSON(testCustomerQuoteID)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", testIdemKey1)
	req.Header.Set("X-Request-ID", "corr-book-customer-001")
	rec := httptest.NewRecorder()
	th.mux.ServeHTTP(rec, req)

	if rec.Header().Get("Idempotent-Replay") != "" {
		t.Fatalf("did not expect Idempotent-Replay header on initial booking")
	}

	data := assertParcelDataEnvelope(t, rec, http.StatusCreated)
	parcelID, _ := data["id"].(string)
	if !events.IsValidUUID(parcelID) {
		t.Fatalf("expected valid parcel UUID, got %v", data["id"])
	}
	if tn, _ := data["tracking_number"].(string); !strings.HasPrefix(tn, "SD-20261111-") {
		t.Fatalf("expected canonical tracking number SD-20261111-*, got %v", data["tracking_number"])
	}
	if data["sender_id"] != testCustomerUserID {
		t.Fatalf("expected server-resolved sender_id %s, got %v", testCustomerUserID, data["sender_id"])
	}
	if _, hasMerchant := data["merchant_id"]; hasMerchant {
		t.Fatalf("did not expect merchant_id for customer booking, got %v", data["merchant_id"])
	}
	if data["status"] != string(domain.StatusCreated) {
		t.Fatalf("expected status CREATED, got %v", data["status"])
	}
	if int(data["delivery_fee_cents"].(float64)) != 47000 || data["currency"] != "LKR" {
		t.Fatalf("unexpected fee metadata in response: %v", data)
	}
	if int(data["max_attempts"].(float64)) != domain.MaxDeliveryAttempts {
		t.Fatalf("expected max_attempts %d, got %v", domain.MaxDeliveryAttempts, data["max_attempts"])
	}
	if data["pricing_rule_id"] != "rule_west_central_v2" || int(data["pricing_rule_version"].(float64)) != 2 {
		t.Fatalf("unexpected pricing rule metadata: %v", data)
	}
	if th.outboxWriter.Count() != 1 {
		t.Fatalf("expected 1 outbox event written, got %d", th.outboxWriter.Count())
	}
	evts := th.outboxWriter.FindByEventType(events.ParcelBookedV1)
	if len(evts) != 1 || evts[0].GetCorrelationID() != "corr-book-customer-001" {
		t.Fatalf("expected parcel.booked.v1 event with correlation ID corr-book-customer-001, got %+v", evts)
	}
}

func TestBookParcel_ApprovedMerchantSuccess(t *testing.T) {
	th := newParcelTestHarness(t)
	token := th.issueToken(t, identityapp.Principal{
		UserID:   testMerchantOwnerID,
		Role:     identitydomain.RoleMerchant,
		TenantID: strPtr(testMerchantID),
	})

	rec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", token, testIdemKey1, validBookingJSON(testMerchantQuoteID))
	data := assertParcelDataEnvelope(t, rec, http.StatusCreated)
	if data["sender_id"] != testMerchantOwnerID {
		t.Fatalf("expected sender_id %s, got %v", testMerchantOwnerID, data["sender_id"])
	}
	if data["merchant_id"] != testMerchantID {
		t.Fatalf("expected server-resolved merchant_id %s, got %v", testMerchantID, data["merchant_id"])
	}
	if th.merchantReader.lastID != testMerchantID {
		t.Fatalf("expected merchant lookup for %s, got %s", testMerchantID, th.merchantReader.lastID)
	}
}

func TestBookParcel_AdminSelfOwnedSuccess(t *testing.T) {
	th := newParcelTestHarness(t)
	token := th.issueToken(t, identityapp.Principal{
		UserID:   testAdminUserID,
		Role:     identitydomain.RoleAdmin,
		TenantID: strPtr(testMerchantID), // ADMIN bookings remain self-owned (MerchantID nil)
	})

	rec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", token, testIdemKey1, validBookingJSON(testAdminQuoteID))
	data := assertParcelDataEnvelope(t, rec, http.StatusCreated)
	if data["sender_id"] != testAdminUserID {
		t.Fatalf("expected self-owned sender_id %s, got %v", testAdminUserID, data["sender_id"])
	}
	if _, hasMerchant := data["merchant_id"]; hasMerchant {
		t.Fatalf("did not expect merchant_id for ADMIN self-owned booking, got %v", data["merchant_id"])
	}
	if th.merchantReader.called {
		t.Fatal("expected merchantReader not to be queried for ADMIN booking")
	}

	expectedScope := "USER:" + testAdminUserID
	if _, ok := th.idemStore.records[idemCompositeKey(testIdemKey1, expectedScope)]; !ok {
		t.Fatalf("expected idempotency record scoped under %s", expectedScope)
	}

	if len(th.historyRepo.entries) != 1 {
		t.Fatalf("expected 1 initial status history entry, got %d", len(th.historyRepo.entries))
	}
	entry := th.historyRepo.entries[0]
	if entry.ActorType != domain.ActorTypeUser || entry.ActorID == nil || *entry.ActorID != testAdminUserID {
		t.Fatalf("unexpected booking actor in history entry: %+v", entry)
	}
}

func TestParcelEndpoints_MerchantEligibilityAndForbiddenRoles(t *testing.T) {
	th := newParcelTestHarness(t)

	// Book a real parcel first so GET /parcels/{id} and /history target a valid existing parcel.
	custToken := th.issueToken(t, identityapp.Principal{UserID: testCustomerUserID, Role: identitydomain.RoleCustomer})
	bookRec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, validBookingJSON(testCustomerQuoteID))
	parcelData := assertParcelDataEnvelope(t, bookRec, http.StatusCreated)
	parcelID := parcelData["id"].(string)

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/parcels", validBookingJSON(testMerchantQuoteID)},
		{http.MethodGet, "/api/v1/parcels/" + parcelID, ""},
		{http.MethodGet, "/api/v1/parcels/" + parcelID + "/history", ""},
	}

	t.Run("courier_and_dispatcher_rejected_by_middleware", func(t *testing.T) {
		for _, role := range []identitydomain.Role{identitydomain.RoleCourier, identitydomain.RoleDispatcher} {
			tok := th.issueToken(t, identityapp.Principal{UserID: testCourierUserID, Role: role})
			for _, ep := range endpoints {
				rec := th.doJSON(t, ep.method, ep.path, tok, testIdemKey2, ep.body)
				assertParcelErrorEnvelope(t, rec, http.StatusForbidden, "AUTH_FORBIDDEN")
			}
		}
	})

	t.Run("courier_rejected_directly_by_handler_defense_in_depth", func(t *testing.T) {
		ctx := identitytransport.WithPrincipal(context.Background(), &identityapp.Principal{
			UserID: testCourierUserID,
			Role:   identitydomain.RoleCourier,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/parcels/"+parcelID, nil).WithContext(ctx)
		req.SetPathValue("id", parcelID)
		rec := httptest.NewRecorder()
		th.handler.GetParcel(rec, req)
		assertParcelErrorEnvelope(t, rec, http.StatusForbidden, "PARCEL_FORBIDDEN")
	})

	t.Run("merchant_missing_tenant_id", func(t *testing.T) {
		tok := th.issueToken(t, identityapp.Principal{UserID: testMerchantOwnerID, Role: identitydomain.RoleMerchant})
		for _, ep := range endpoints {
			rec := th.doJSON(t, ep.method, ep.path, tok, testIdemKey2, ep.body)
			assertParcelErrorEnvelope(t, rec, http.StatusForbidden, "MISSING_TENANT_ID")
		}
	})

	t.Run("merchant_non_uuid_tenant_id", func(t *testing.T) {
		tok := th.issueToken(t, identityapp.Principal{UserID: testMerchantOwnerID, Role: identitydomain.RoleMerchant, TenantID: strPtr("mrc_slug_01")})
		for _, ep := range endpoints {
			rec := th.doJSON(t, ep.method, ep.path, tok, testIdemKey2, ep.body)
			assertParcelErrorEnvelope(t, rec, http.StatusForbidden, "TENANT_NOT_MERCHANT_UUID")
		}
	})

	t.Run("merchant_owner_mismatch", func(t *testing.T) {
		tok := th.issueToken(t, identityapp.Principal{UserID: testOtherCustomerID, Role: identitydomain.RoleMerchant, TenantID: strPtr(testMerchantID)})
		for _, ep := range endpoints {
			rec := th.doJSON(t, ep.method, ep.path, tok, testIdemKey2, ep.body)
			assertParcelErrorEnvelope(t, rec, http.StatusForbidden, "MERCHANT_OWNER_MISMATCH")
		}
	})

	t.Run("merchant_suspended_or_pending", func(t *testing.T) {
		for _, status := range []string{"SUSPENDED", "PENDING", "REJECTED"} {
			th.merchantReader.profiles[testMerchantID] = application.MerchantProfile{
				ID:          testMerchantID,
				OwnerUserID: testMerchantOwnerID,
				Status:      status,
			}
			tok := th.issueToken(t, identityapp.Principal{UserID: testMerchantOwnerID, Role: identitydomain.RoleMerchant, TenantID: strPtr(testMerchantID)})
			for _, ep := range endpoints {
				rec := th.doJSON(t, ep.method, ep.path, tok, testIdemKey2, ep.body)
				assertParcelErrorEnvelope(t, rec, http.StatusForbidden, "MERCHANT_NOT_APPROVED")
			}
		}
		// Restore approved merchant state.
		th.merchantReader.profiles[testMerchantID] = application.MerchantProfile{
			ID:          testMerchantID,
			OwnerUserID: testMerchantOwnerID,
			Status:      application.MerchantStatusApproved,
		}
	})

	if th.historyRepo.findByParcelFn != 0 {
		t.Fatalf("expected statusHistoryRepo.FindByParcelID not called during rejected checks, got %d", th.historyRepo.findByParcelFn)
	}
}

func TestBookParcel_QuoteAndValidationErrors(t *testing.T) {
	th := newParcelTestHarness(t)
	custToken := th.issueToken(t, identityapp.Principal{UserID: testCustomerUserID, Role: identitydomain.RoleCustomer})
	otherToken := th.issueToken(t, identityapp.Principal{UserID: testOtherCustomerID, Role: identitydomain.RoleCustomer})

	t.Run("quote_ownership_mismatch", func(t *testing.T) {
		rec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", otherToken, testIdemKey1, validBookingJSON(testCustomerQuoteID))
		assertParcelErrorEnvelope(t, rec, http.StatusForbidden, "QUOTE_FORBIDDEN")
	})

	t.Run("quote_not_found", func(t *testing.T) {
		rec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, validBookingJSON("0192e2b3-7c8a-7890-a123-000000000999"))
		assertParcelErrorEnvelope(t, rec, http.StatusNotFound, "QUOTE_NOT_FOUND")
	})

	t.Run("quote_expired", func(t *testing.T) {
		rec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, validBookingJSON(testExpiredQuoteID))
		assertParcelErrorEnvelope(t, rec, http.StatusUnprocessableEntity, "QUOTE_EXPIRED")
	})

	t.Run("invalid_quote_uuid", func(t *testing.T) {
		rec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, validBookingJSON("not-a-uuid"))
		assertParcelErrorEnvelope(t, rec, http.StatusUnprocessableEntity, "INVALID_QUOTE_ID")
	})

	t.Run("quote_zone_mismatch", func(t *testing.T) {
		body := strings.Replace(validBookingJSON(testCustomerQuoteID), `"ZONE_CENTRAL_01"`, `"ZONE_NORTHERN_01"`, 1)
		rec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, body)
		assertParcelErrorEnvelope(t, rec, http.StatusUnprocessableEntity, "QUOTE_PARAM_MISMATCH")
	})

	t.Run("quote_weight_mismatch", func(t *testing.T) {
		body := strings.Replace(validBookingJSON(testCustomerQuoteID), `"weight_kg": 2.5`, `"weight_kg": 3.0`, 1)
		rec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, body)
		assertParcelErrorEnvelope(t, rec, http.StatusUnprocessableEntity, "QUOTE_PARAM_MISMATCH")
	})

	t.Run("chargeable_weight_exceeds_30kg", func(t *testing.T) {
		body := strings.Replace(validBookingJSON(testCustomerQuoteID), `"weight_kg": 2.5`, `"weight_kg": 31.0`, 1)
		rec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, body)
		assertParcelErrorEnvelope(t, rec, http.StatusUnprocessableEntity, "PARCEL_LIMITS_EXCEEDED")
	})

	t.Run("invalid_zone_format", func(t *testing.T) {
		body := strings.Replace(validBookingJSON(testCustomerQuoteID), `"ZONE_WESTERN_01"`, `"ZONE-WESTERN-01"`, 1)
		rec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, body)
		assertParcelErrorEnvelope(t, rec, http.StatusUnprocessableEntity, "INVALID_ZONE_ID")
	})

	t.Run("missing_dimensions_or_addresses", func(t *testing.T) {
		missingDims := fmt.Sprintf(`{"quote_id":%q,"recipient_name":"Nimal","recipient_phone":"+94719876543","weight_kg":2.5,"pickup":{"contact_name":"K","contact_phone":"+94771234567","address_line1":"45 Galle Rd","city":"Colombo","postal_code":"00300","zone_id":"ZONE_WESTERN_01"},"dropoff":{"contact_name":"N","contact_phone":"+94719876543","address_line1":"12 Kandy Rd","city":"Kandy","postal_code":"20400","zone_id":"ZONE_CENTRAL_01"}}`, testCustomerQuoteID)
		assertParcelErrorEnvelope(t, th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, missingDims), http.StatusUnprocessableEntity, "INVALID_DIMENSIONS")

		missingPickup := fmt.Sprintf(`{"quote_id":%q,"recipient_name":"Nimal","recipient_phone":"+94719876543","weight_kg":2.5,"dimensions":{"length_cm":30,"width_cm":25,"height_cm":15},"dropoff":{"contact_name":"N","contact_phone":"+94719876543","address_line1":"12 Kandy Rd","city":"Kandy","postal_code":"20400","zone_id":"ZONE_CENTRAL_01"}}`, testCustomerQuoteID)
		assertParcelErrorEnvelope(t, th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, missingPickup), http.StatusUnprocessableEntity, "INVALID_ADDRESS")
	})

	t.Run("invalid_recipient", func(t *testing.T) {
		blankName := strings.Replace(validBookingJSON(testCustomerQuoteID), `"recipient_name": "Nimal Fernando"`, `"recipient_name": "   "`, 1)
		assertParcelErrorEnvelope(t, th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, blankName), http.StatusUnprocessableEntity, "INVALID_RECIPIENT")

		blankPhone := strings.Replace(validBookingJSON(testCustomerQuoteID), `"recipient_phone": "+94719876543"`, `"recipient_phone": ""`, 1)
		assertParcelErrorEnvelope(t, th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, blankPhone), http.StatusUnprocessableEntity, "INVALID_RECIPIENT")
	})

	// All errors above rolled back atomically so valid quote stays unused!
	if th.quoteConsumer.quotes[testCustomerQuoteID].used {
		t.Fatal("expected quote to remain unused after failed booking validations")
	}

	// Book once cleanly so quote becomes used, then try another booking with a DIFFERENT idempotency key -> 409 QUOTE_ALREADY_USED.
	firstRec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, validBookingJSON(testCustomerQuoteID))
	assertParcelDataEnvelope(t, firstRec, http.StatusCreated)

	secondRec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey2, validBookingJSON(testCustomerQuoteID))
	assertParcelErrorEnvelope(t, secondRec, http.StatusConflict, "QUOTE_ALREADY_USED")
}

func TestBookParcel_IdempotentReplayConflictAndCallerIsolation(t *testing.T) {
	th := newParcelTestHarness(t)
	cust1Token := th.issueToken(t, identityapp.Principal{UserID: testCustomerUserID, Role: identitydomain.RoleCustomer})
	cust2Token := th.issueToken(t, identityapp.Principal{UserID: testOtherCustomerID, Role: identitydomain.RoleCustomer})

	payload1 := validBookingJSON(testCustomerQuoteID)
	rec1 := th.doJSON(t, http.MethodPost, "/api/v1/parcels", cust1Token, testIdemKey1, payload1)
	data1 := assertParcelDataEnvelope(t, rec1, http.StatusCreated)
	if rec1.Header().Get("Idempotent-Replay") != "" {
		t.Fatal("first booking must not set Idempotent-Replay header")
	}

	// Replay with identical raw bytes and same key (even with uppercase hex) must return cached response without re-consuming quote.
	recReplay := th.doJSON(t, http.MethodPost, "/api/v1/parcels", cust1Token, strings.ToUpper(testIdemKey1), payload1)
	dataReplay := assertParcelDataEnvelope(t, recReplay, http.StatusCreated)
	if recReplay.Header().Get("Idempotent-Replay") != "true" {
		t.Fatalf("expected Idempotent-Replay: true on replay, got %q", recReplay.Header().Get("Idempotent-Replay"))
	}
	if dataReplay["id"] != data1["id"] || dataReplay["tracking_number"] != data1["tracking_number"] {
		t.Fatalf("replayed parcel differed from original: original=%v replay=%v", data1, dataReplay)
	}
	if th.quoteConsumer.consumeCount != 1 {
		t.Fatalf("expected quote consumer invoked once across initial + replay, got %d", th.quoteConsumer.consumeCount)
	}
	if th.parcelRepo.saveCalled != 1 {
		t.Fatalf("expected parcel repository Save called once, got %d", th.parcelRepo.saveCalled)
	}

	// Reusing the same Idempotency-Key with modified JSON bytes returns 409 IDEMPOTENCY_KEY_PAYLOAD_MISMATCH.
	differentPayload := strings.Replace(payload1, `"Handle with care"`, `"Urgent delivery"`, 1)
	recMismatch := th.doJSON(t, http.MethodPost, "/api/v1/parcels", cust1Token, testIdemKey1, differentPayload)
	assertParcelErrorEnvelope(t, recMismatch, http.StatusConflict, "IDEMPOTENCY_KEY_PAYLOAD_MISMATCH")

	// Different caller reusing the same Idempotency-Key UUID succeeds in its own caller scope!
	recCust2 := th.doJSON(t, http.MethodPost, "/api/v1/parcels", cust2Token, testIdemKey1, validBookingJSON(testCustomerQuote2ID))
	dataCust2 := assertParcelDataEnvelope(t, recCust2, http.StatusCreated)
	if recCust2.Header().Get("Idempotent-Replay") != "" {
		t.Fatal("different caller must not replay customer1 booking")
	}
	if dataCust2["id"] == data1["id"] || dataCust2["sender_id"] != testOtherCustomerID {
		t.Fatalf("expected independent parcel owned by customer2, got %v", dataCust2)
	}
}

func TestGetParcelAndHistory_AuthorizedIDORAndValidation(t *testing.T) {
	th := newParcelTestHarness(t)

	custToken := th.issueToken(t, identityapp.Principal{UserID: testCustomerUserID, Role: identitydomain.RoleCustomer})
	otherCustToken := th.issueToken(t, identityapp.Principal{UserID: testOtherCustomerID, Role: identitydomain.RoleCustomer})
	merchantToken := th.issueToken(t, identityapp.Principal{UserID: testMerchantOwnerID, Role: identitydomain.RoleMerchant, TenantID: strPtr(testMerchantID)})
	otherMerchantToken := th.issueToken(t, identityapp.Principal{UserID: testOtherMerchantOwner, Role: identitydomain.RoleMerchant, TenantID: strPtr(testOtherMerchantID)})
	adminToken := th.issueToken(t, identityapp.Principal{UserID: testAdminUserID, Role: identitydomain.RoleAdmin})

	custBook := assertParcelDataEnvelope(t, th.doJSON(t, http.MethodPost, "/api/v1/parcels", custToken, testIdemKey1, validBookingJSON(testCustomerQuoteID)), http.StatusCreated)
	custParcelID := custBook["id"].(string)

	merchBook := assertParcelDataEnvelope(t, th.doJSON(t, http.MethodPost, "/api/v1/parcels", merchantToken, testIdemKey1, validBookingJSON(testMerchantQuoteID)), http.StatusCreated)
	merchParcelID := merchBook["id"].(string)

	// Append a synthetic status transition so GET /history proves chronological multiple entries.
	pickedUpAt := th.clock.now.Add(15 * time.Minute)
	fromCreated := domain.StatusCreated
	entry2, err := domain.NewStatusHistoryEntry(
		"0192e2b3-7c8a-7890-a123-000000000599",
		custParcelID,
		&fromCreated,
		domain.StatusAssigned,
		strPtr(testAdminUserID),
		domain.ActorTypeUser,
		"ADMIN",
		"Dispatched to courier",
		pickedUpAt,
		0,
	)
	if err != nil {
		t.Fatalf("NewStatusHistoryEntry failed: %v", err)
	}
	if err := th.historyRepo.Save(context.Background(), nil, entry2); err != nil {
		t.Fatalf("historyRepo.Save failed: %v", err)
	}

	// Owner customer & admin can GET customer parcel and history.
	for _, tc := range []struct {
		name  string
		token string
	}{
		{"owning_customer", custToken},
		{"admin", adminToken},
	} {
		t.Run("customer_parcel_"+tc.name, func(t *testing.T) {
			rec := th.doJSON(t, http.MethodGet, "/api/v1/parcels/"+custParcelID, tc.token, "", "")
			got := assertParcelDataEnvelope(t, rec, http.StatusOK)
			if got["id"] != custParcelID || got["tracking_number"] != custBook["tracking_number"] {
				t.Fatalf("unexpected parcel payload: %v", got)
			}

			histRec := th.doJSON(t, http.MethodGet, "/api/v1/parcels/"+custParcelID+"/history", tc.token, "", "")
			histData := assertParcelDataEnvelope(t, histRec, http.StatusOK)
			if histData["parcel_id"] != custParcelID || histData["tracking_number"] != custBook["tracking_number"] {
				t.Fatalf("unexpected history header: %v", histData)
			}
			items, ok := histData["history"].([]any)
			if !ok || len(items) != 2 {
				t.Fatalf("expected 2 history items, got %v", histData["history"])
			}
			firstItem := items[0].(map[string]any)
			if firstItem["from_status"] != nil || firstItem["to_status"] != "CREATED" {
				t.Fatalf("expected initial from_status=null -> CREATED, got %v", firstItem)
			}
			secondItem := items[1].(map[string]any)
			if secondItem["from_status"] != "CREATED" || secondItem["to_status"] != "ASSIGNED" {
				t.Fatalf("expected second CREATED -> ASSIGNED transition, got %v", secondItem)
			}
		})
	}

	// Approved merchant & admin can GET merchant parcel and history.
	for _, tc := range []struct {
		name  string
		token string
	}{
		{"owning_merchant", merchantToken},
		{"admin", adminToken},
	} {
		t.Run("merchant_parcel_"+tc.name, func(t *testing.T) {
			rec := th.doJSON(t, http.MethodGet, "/api/v1/parcels/"+merchParcelID, tc.token, "", "")
			got := assertParcelDataEnvelope(t, rec, http.StatusOK)
			if got["merchant_id"] != testMerchantID {
				t.Fatalf("expected merchant_id %s, got %v", testMerchantID, got["merchant_id"])
			}

			histRec := th.doJSON(t, http.MethodGet, "/api/v1/parcels/"+merchParcelID+"/history", tc.token, "", "")
			histData := assertParcelDataEnvelope(t, histRec, http.StatusOK)
			items, ok := histData["history"].([]any)
			if !ok || len(items) != 1 {
				t.Fatalf("expected 1 history item for merchant parcel, got %v", histData["history"])
			}
		})
	}

	// Cross-customer / cross-merchant IDOR denied with 403 PARCEL_FORBIDDEN without reading history.
	historyCallsBeforeForbidden := th.historyRepo.findByParcelFn
	assertParcelErrorEnvelope(t, th.doJSON(t, http.MethodGet, "/api/v1/parcels/"+custParcelID, otherCustToken, "", ""), http.StatusForbidden, "PARCEL_FORBIDDEN")
	assertParcelErrorEnvelope(t, th.doJSON(t, http.MethodGet, "/api/v1/parcels/"+custParcelID+"/history", otherCustToken, "", ""), http.StatusForbidden, "PARCEL_FORBIDDEN")
	assertParcelErrorEnvelope(t, th.doJSON(t, http.MethodGet, "/api/v1/parcels/"+merchParcelID, otherMerchantToken, "", ""), http.StatusForbidden, "PARCEL_FORBIDDEN")
	assertParcelErrorEnvelope(t, th.doJSON(t, http.MethodGet, "/api/v1/parcels/"+merchParcelID+"/history", otherMerchantToken, "", ""), http.StatusForbidden, "PARCEL_FORBIDDEN")
	assertParcelErrorEnvelope(t, th.doJSON(t, http.MethodGet, "/api/v1/parcels/"+custParcelID, otherMerchantToken, "", ""), http.StatusForbidden, "PARCEL_FORBIDDEN")
	if th.historyRepo.findByParcelFn != historyCallsBeforeForbidden {
		t.Fatalf("expected FindByParcelID not called on forbidden access")
	}

	// Missing parcel returns 404 PARCEL_NOT_FOUND; non-UUID path ID returns 400 INVALID_PARCEL_ID.
	assertParcelErrorEnvelope(t, th.doJSON(t, http.MethodGet, "/api/v1/parcels/"+testMissingParcelID, custToken, "", ""), http.StatusNotFound, "PARCEL_NOT_FOUND")
	assertParcelErrorEnvelope(t, th.doJSON(t, http.MethodGet, "/api/v1/parcels/"+testMissingParcelID+"/history", custToken, "", ""), http.StatusNotFound, "PARCEL_NOT_FOUND")
	assertParcelErrorEnvelope(t, th.doJSON(t, http.MethodGet, "/api/v1/parcels/not-a-uuid", custToken, "", ""), http.StatusBadRequest, "INVALID_PARCEL_ID")
	assertParcelErrorEnvelope(t, th.doJSON(t, http.MethodGet, "/api/v1/parcels/not-a-uuid/history", custToken, "", ""), http.StatusBadRequest, "INVALID_PARCEL_ID")
}

func TestMapParcelError_AllFallbackBranches(t *testing.T) {
	if spec := mapParcelError(application.ErrTrackingNumberCollision); spec.status != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for tracking collision, got %d", spec.status)
	}
	if spec := mapParcelError(application.ErrConcurrencyConflict); spec.status != http.StatusConflict {
		t.Fatalf("expected 409 for concurrency conflict, got %d", spec.status)
	}
	if spec := mapParcelError(errors.New("unexpected db error")); spec.status != http.StatusInternalServerError {
		t.Fatalf("expected 500 for unknown error, got %d", spec.status)
	}

	// Ensure empty status history serializes to [] instead of null.
	tn, err := domain.NewTrackingNumber("SD-20261111-000001")
	if err != nil {
		t.Fatalf("NewTrackingNumber failed: %v", err)
	}
	resp := NewParcelHistoryResponse(domain.Parcel{
		ID:             testMissingParcelID,
		TrackingNumber: tn,
		Status:         domain.StatusCreated,
	}, nil)
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !strings.Contains(string(encoded), `"history":[]`) {
		t.Fatalf("expected empty history array in JSON, got %s", string(encoded))
	}
}

func TestParcelEndpoints_NonUUIDCallerAndMerchantLookupError(t *testing.T) {
	th := newParcelTestHarness(t)

	nonUUIDUserTok := th.issueToken(t, identityapp.Principal{UserID: "usr_customer_legacy", Role: identitydomain.RoleCustomer})
	rec := th.doJSON(t, http.MethodPost, "/api/v1/parcels", nonUUIDUserTok, testIdemKey1, validBookingJSON(testCustomerQuoteID))
	assertParcelErrorEnvelope(t, rec, http.StatusForbidden, "INVALID_CALLER_ID")

	recGet := th.doJSON(t, http.MethodGet, "/api/v1/parcels/"+testMissingParcelID, nonUUIDUserTok, "", "")
	assertParcelErrorEnvelope(t, recGet, http.StatusForbidden, "INVALID_CALLER_ID")

	th.merchantReader.err = errors.New("merchant db connection refused")
	merchTok := th.issueToken(t, identityapp.Principal{UserID: testMerchantOwnerID, Role: identitydomain.RoleMerchant, TenantID: strPtr(testMerchantID)})
	recErr := th.doJSON(t, http.MethodPost, "/api/v1/parcels", merchTok, testIdemKey1, validBookingJSON(testMerchantQuoteID))
	assertParcelErrorEnvelope(t, recErr, http.StatusInternalServerError, "INTERNAL_ERROR")
}
