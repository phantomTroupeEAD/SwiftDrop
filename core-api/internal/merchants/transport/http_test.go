package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/merchants/application"
	"github.com/swiftdrop/core-api/internal/merchants/domain"
)

type mockVerifier struct {
	claims Claims
	err    error
}

func (m mockVerifier) Verify(_ string) (Claims, error) {
	return m.claims, m.err
}

func setupTestMux(claims Claims) (*http.ServeMux, *application.Service) {
	mux, svc, _ := setupTestMuxWithBooking(claims, mockBooking{})
	return mux, svc
}

func setupTestMuxWithBooking(claims Claims, booking application.ParcelBooking) (*http.ServeMux, *application.Service, *mockMerchantRepo) {
	mRepo := newMockMerchantRepo()
	cRepo := &mockCredRepo{}
	hRepo := &mockWebhookRepo{}

	svc := application.NewService(application.Deps{
		Tx:        &mockTxRunner{},
		Merchants: mRepo,
		Creds:     cRepo,
		Hooks:     hRepo,
		Audit:     mockAuditWriter{},
		Roles:     mockRoleAssigner{},
		KeyGen:    mockKeyGen{},
		Hasher:    mockHasher{},
		Cipher:    mockCipher{},
		Resolver:  mockResolver{},
		Booking:   booking,
		Observer:  mockObserver{},
		Now:       time.Now,
	}, application.Config{
		KeyEnvPrefix:  "sd_live_",
		MaxActiveKeys: 5,
		MaxWebhooks:   10,
		AllowHTTP:     false,
	})

	h := &Handler{
		Svc:      svc,
		Verifier: mockVerifier{claims: claims},
		Log:      slog.Default(),
	}

	mux := http.NewServeMux()
	h.Register(mux)
	return mux, svc, mRepo
}

// In-memory doubles for transport tests
type mockTxRunner struct{}

func (m *mockTxRunner) WithinTx(ctx context.Context, fn func(context.Context, application.Querier) error) error {
	return fn(ctx, nil)
}
func (m *mockTxRunner) Reader() application.Querier { return nil }

type mockMerchantRepo struct {
	merchants map[string]application.MerchantView
}

func newMockMerchantRepo() *mockMerchantRepo {
	return &mockMerchantRepo{merchants: make(map[string]application.MerchantView)}
}

func (m *mockMerchantRepo) Insert(_ context.Context, _ application.Querier, v application.MerchantView) error {
	m.merchants[v.ID] = v
	return nil
}
func (m *mockMerchantRepo) Get(_ context.Context, _ application.Querier, id string) (application.MerchantView, error) {
	v, ok := m.merchants[id]
	if !ok {
		return application.MerchantView{}, application.ErrMerchantNotFound
	}
	return v, nil
}
func (m *mockMerchantRepo) FindByOwner(_ context.Context, _ application.Querier, _ string) (application.MerchantView, error) {
	return application.MerchantView{}, application.ErrNotFound
}
func (m *mockMerchantRepo) Status(_ context.Context, _ application.Querier, id string) (domain.MerchantStatus, error) {
	v, ok := m.merchants[id]
	if !ok {
		return "", application.ErrMerchantNotFound
	}
	return v.Status, nil
}
func (m *mockMerchantRepo) LockForUpdate(ctx context.Context, q application.Querier, id string) (application.MerchantView, error) {
	return m.Get(ctx, q, id)
}
func (m *mockMerchantRepo) SaveStatus(_ context.Context, _ application.Querier, _ *domain.Merchant, _ int, _ string) error {
	return nil
}
func (m *mockMerchantRepo) List(_ context.Context, _ application.Querier, _ string, _, _ int) ([]application.MerchantView, int, error) {
	return nil, 0, nil
}

type mockCredRepo struct{}

func (mockCredRepo) Insert(_ context.Context, _ application.Querier, _ application.CredentialRecord) error {
	return nil
}
func (mockCredRepo) FindByPrefix(_ context.Context, _ application.Querier, _ string) (application.AuthRecord, error) {
	return application.AuthRecord{}, application.ErrNotFound
}
func (mockCredRepo) List(_ context.Context, _ application.Querier, _ string) ([]application.CredentialView, error) {
	return nil, nil
}
func (mockCredRepo) CountActive(_ context.Context, _ application.Querier, _ string, _ time.Time) (int, error) {
	return 0, nil
}
func (mockCredRepo) Revoke(_ context.Context, _ application.Querier, _, _ string, _ time.Time) (bool, bool, error) {
	return true, true, nil
}
func (mockCredRepo) RevokeAll(_ context.Context, _ application.Querier, _ string, _ time.Time) (int, error) {
	return 0, nil
}
func (mockCredRepo) TouchLastUsed(_ context.Context, _ application.Querier, _ string, _ time.Time) error {
	return nil
}

type mockWebhookRepo struct{}

func (mockWebhookRepo) Insert(_ context.Context, _ application.Querier, _ application.WebhookRecord) error {
	return nil
}
func (mockWebhookRepo) List(_ context.Context, _ application.Querier, _ string) ([]application.WebhookView, error) {
	return nil, nil
}
func (mockWebhookRepo) CountLive(_ context.Context, _ application.Querier, _ string) (int, error) {
	return 0, nil
}
func (mockWebhookRepo) SoftDelete(_ context.Context, _ application.Querier, _, _ string, _ time.Time) (bool, error) {
	return true, nil
}
func (mockWebhookRepo) DisableAll(_ context.Context, _ application.Querier, _ string) (int, error) {
	return 0, nil
}

type mockAuditWriter struct{}

func (mockAuditWriter) Write(_ context.Context, _ application.Querier, _ application.AuditEntry) error {
	return nil
}

type mockRoleAssigner struct{}

func (mockRoleAssigner) GrantMerchantRole(_ context.Context, _ application.Querier, _, _ string) error {
	return nil
}

func (mockRoleAssigner) RevokeMerchantRole(_ context.Context, _ application.Querier, _ string) error {
	return nil
}

type mockKeyGen struct{}

func (mockKeyGen) NewAPIKey(env string) (domain.PlainAPIKey, error) {
	return domain.NewPlainAPIKey(env, "AbCdEf12", "12345678901234567890123456789012")
}
func (mockKeyGen) NewSigningSecret() (string, error) {
	return "whsec_mocksecret1234567890123456789012345678901234567890", nil
}

type mockHasher struct{}

func (mockHasher) CurrentVersion() int                    { return 1 }
func (mockHasher) Hash(_ int, key string) (string, error) { return "hash_" + key, nil }
func (mockHasher) Equal(a, b string) bool                 { return a == b }

type mockCipher struct{}

func (mockCipher) Encrypt(plain string) ([]byte, int, error) {
	return []byte("enc_" + plain), 1, nil
}

type mockResolver struct{}

func (mockResolver) LookupAddrs(_ context.Context, _ string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
}

type mockBooking struct{}

func (mockBooking) CreateForMerchant(_ context.Context, _, _, _ string, _ application.OrderRequest) (application.OrderResult, bool, error) {
	return application.OrderResult{}, false, nil
}
func (mockBooking) ListForMerchant(_ context.Context, _ string) ([]application.OrderResult, error) {
	return nil, nil
}

type configurableMockBooking struct {
	createFn func(ctx context.Context, merchantID, idemKey, hash string, req application.OrderRequest) (application.OrderResult, bool, error)
	listFn   func(ctx context.Context, merchantID string) ([]application.OrderResult, error)
}

func (m configurableMockBooking) CreateForMerchant(ctx context.Context, merchantID, idemKey, hash string, req application.OrderRequest) (application.OrderResult, bool, error) {
	if m.createFn != nil {
		return m.createFn(ctx, merchantID, idemKey, hash, req)
	}
	return application.OrderResult{}, false, nil
}

func (m configurableMockBooking) ListForMerchant(ctx context.Context, merchantID string) ([]application.OrderResult, error) {
	if m.listFn != nil {
		return m.listFn(ctx, merchantID)
	}
	return nil, nil
}

type mockObserver struct{}

func (mockObserver) AuthResult(_ string)    {}
func (mockObserver) SSRFBlocked()           {}
func (mockObserver) StatusChanged(_ string) {}

func TestSubmitApplication_Unauthenticated(t *testing.T) {
	mux, _ := setupTestMux(Claims{})

	req := httptest.NewRequest("POST", "/api/v1/merchants/applications", bytes.NewBufferString("{}"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", w.Code)
	}
}

func TestSubmitApplication_HappyPath(t *testing.T) {
	claims := Claims{Sub: "user_customer_1", Role: "CUSTOMER"}
	mux, _ := setupTestMux(claims)

	payload := map[string]any{
		"business_name": "Acme Goods Ltd",
		"contact_email": "hello@acme.com",
		"contact_phone": "+94771234567",
		"address": map[string]string{
			"line1":       "10 Galle Road",
			"city":        "Colombo",
			"postal_code": "00300",
			"country":     "LK",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/merchants/applications", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegisterWebhook_RejectsSecretInBody(t *testing.T) {
	claims := Claims{Sub: "user_m1", Role: "MERCHANT", TenantID: "m_123"}
	mux, _ := setupTestMux(claims)

	payload := map[string]any{
		"url":    "https://api.merchant.com/webhook",
		"events": []string{"parcel.delivered.v1"},
		"secret": "whsec_client_provided",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/merchants/me/webhooks", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request when secret is provided, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateOrder_Success(t *testing.T) {
	tenantID := "0192e2b3-7c8a-7890-a123-000000000001"
	claims := Claims{Sub: "user_m1", Role: "MERCHANT", TenantID: tenantID}

	mockBook := configurableMockBooking{
		createFn: func(ctx context.Context, merchantID, idemKey, hash string, req application.OrderRequest) (application.OrderResult, bool, error) {
			if merchantID != tenantID {
				t.Fatalf("expected merchantID %s, got %s", tenantID, merchantID)
			}
			res := application.OrderResult{
				ParcelID:          "0192e2b3-7c8a-7890-a123-000000000101",
				TrackingNumber:    "SD-20261119-00001",
				MerchantReference: req.MerchantReference,
				Status:            "CREATED",
				CreatedAt:         "2026-11-19T12:00:00Z",
			}
			res.Fee.Currency = "LKR"
			res.Fee.AmountMinor = 65000
			return res, false, nil
		},
	}

	mux, _, mRepo := setupTestMuxWithBooking(claims, mockBook)
	_ = mRepo.Insert(context.Background(), nil, application.MerchantView{
		ID:     tenantID,
		Status: domain.StatusApproved,
	})

	orderPayload := map[string]any{
		"merchant_reference": "ORD-TEST-001",
		"service_level":      "STANDARD",
		"pickup": map[string]any{
			"contact_name":  "Warehouse",
			"contact_phone": "+94771234567",
			"street_line_1": "10 Galle Rd",
			"city":          "Colombo",
			"postal_code":   "00300",
			"zone_id":       "ZONE_WESTERN_01",
		},
		"dropoff": map[string]any{
			"contact_name":  "Recipient",
			"contact_phone": "+94779876543",
			"street_line_1": "12 Temple Rd",
			"city":          "Kandy",
			"postal_code":   "20000",
			"zone_id":       "ZONE_CENTRAL_01",
		},
		"parcel": map[string]any{
			"weight_kg": 2.5,
			"length_cm": 30.0,
			"width_cm":  20.0,
			"height_cm": 15.0,
		},
	}
	body, _ := json.Marshal(orderPayload)

	req := httptest.NewRequest("POST", "/api/v1/merchant/orders", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Idempotency-Key", "0192e2b3-7c8a-7890-a123-000000000099")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Idempotent-Replay") != "" {
		t.Errorf("expected no Idempotent-Replay header, got %s", w.Header().Get("Idempotent-Replay"))
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data envelope, got %v", resp)
	}
	if data["parcel_id"] != "0192e2b3-7c8a-7890-a123-000000000101" {
		t.Errorf("expected parcel_id 0192e2b3-7c8a-7890-a123-000000000101, got %v", data["parcel_id"])
	}
}

func TestCreateOrder_IdempotentReplay(t *testing.T) {
	tenantID := "0192e2b3-7c8a-7890-a123-000000000001"
	claims := Claims{Sub: "user_m1", Role: "MERCHANT", TenantID: tenantID}

	mockBook := configurableMockBooking{
		createFn: func(ctx context.Context, merchantID, idemKey, hash string, req application.OrderRequest) (application.OrderResult, bool, error) {
			res := application.OrderResult{
				ParcelID:       "0192e2b3-7c8a-7890-a123-000000000101",
				TrackingNumber: "SD-20261119-00001",
				Status:         "CREATED",
				CreatedAt:      "2026-11-19T12:00:00Z",
			}
			return res, true, nil // Replayed = true
		},
	}

	mux, _, mRepo := setupTestMuxWithBooking(claims, mockBook)
	_ = mRepo.Insert(context.Background(), nil, application.MerchantView{
		ID:     tenantID,
		Status: domain.StatusApproved,
	})

	orderPayload := map[string]any{
		"merchant_reference": "ORD-REPLAY",
		"pickup":             map[string]any{"city": "Colombo"},
		"dropoff":            map[string]any{"city": "Kandy"},
		"parcel":             map[string]any{"weight_kg": 2.0},
	}
	body, _ := json.Marshal(orderPayload)

	req := httptest.NewRequest("POST", "/api/v1/merchant/orders", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Idempotency-Key", "0192e2b3-7c8a-7890-a123-000000000099")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on replay, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Idempotent-Replay") != "true" {
		t.Errorf("expected Idempotent-Replay: true header, got: %q", w.Header().Get("Idempotent-Replay"))
	}
}

func TestCreateOrder_MissingIdempotencyKey(t *testing.T) {
	tenantID := "0192e2b3-7c8a-7890-a123-000000000001"
	claims := Claims{Sub: "user_m1", Role: "MERCHANT", TenantID: tenantID}

	mux, _, mRepo := setupTestMuxWithBooking(claims, mockBooking{})
	_ = mRepo.Insert(context.Background(), nil, application.MerchantView{
		ID:     tenantID,
		Status: domain.StatusApproved,
	})

	req := httptest.NewRequest("POST", "/api/v1/merchant/orders", bytes.NewBufferString("{}"))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "IDEMPOTENCY_KEY_REQUIRED") {
		t.Errorf("expected IDEMPOTENCY_KEY_REQUIRED code, got %s", w.Body.String())
	}
}

func TestCreateOrder_InvalidIdempotencyKey(t *testing.T) {
	tenantID := "0192e2b3-7c8a-7890-a123-000000000001"
	claims := Claims{Sub: "user_m1", Role: "MERCHANT", TenantID: tenantID}

	mux, _, mRepo := setupTestMuxWithBooking(claims, mockBooking{})
	_ = mRepo.Insert(context.Background(), nil, application.MerchantView{
		ID:     tenantID,
		Status: domain.StatusApproved,
	})

	req := httptest.NewRequest("POST", "/api/v1/merchant/orders", bytes.NewBufferString("{}"))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Idempotency-Key", "not-a-uuid")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "IDEMPOTENCY_KEY_INVALID") {
		t.Errorf("expected IDEMPOTENCY_KEY_INVALID code, got %s", w.Body.String())
	}
}

func TestCreateOrder_IdempotencyPayloadMismatch(t *testing.T) {
	tenantID := "0192e2b3-7c8a-7890-a123-000000000001"
	claims := Claims{Sub: "user_m1", Role: "MERCHANT", TenantID: tenantID}

	mockBook := configurableMockBooking{
		createFn: func(ctx context.Context, merchantID, idemKey, hash string, req application.OrderRequest) (application.OrderResult, bool, error) {
			return application.OrderResult{}, false, application.ErrIdempotencyReused
		},
	}

	mux, _, mRepo := setupTestMuxWithBooking(claims, mockBook)
	_ = mRepo.Insert(context.Background(), nil, application.MerchantView{
		ID:     tenantID,
		Status: domain.StatusApproved,
	})

	req := httptest.NewRequest("POST", "/api/v1/merchant/orders", bytes.NewBufferString(`{"parcel":{"weight_kg":1.0}}`))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Idempotency-Key", "0192e2b3-7c8a-7890-a123-000000000099")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "IDEMPOTENCY_KEY_PAYLOAD_MISMATCH") {
		t.Errorf("expected IDEMPOTENCY_KEY_PAYLOAD_MISMATCH code, got %s", w.Body.String())
	}
}

func TestCreateOrder_ValidationError(t *testing.T) {
	tenantID := "0192e2b3-7c8a-7890-a123-000000000001"
	claims := Claims{Sub: "user_m1", Role: "MERCHANT", TenantID: tenantID}

	mockBook := configurableMockBooking{
		createFn: func(ctx context.Context, merchantID, idemKey, hash string, req application.OrderRequest) (application.OrderResult, bool, error) {
			return application.OrderResult{}, false, application.ValidationError{
				Field: "parcel.dimensions",
				Issue: "length, width, and height in cm must be greater than 0",
			}
		},
	}

	mux, _, mRepo := setupTestMuxWithBooking(claims, mockBook)
	_ = mRepo.Insert(context.Background(), nil, application.MerchantView{
		ID:     tenantID,
		Status: domain.StatusApproved,
	})

	req := httptest.NewRequest("POST", "/api/v1/merchant/orders", bytes.NewBufferString(`{"parcel":{"weight_kg":1.0}}`))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Idempotency-Key", "0192e2b3-7c8a-7890-a123-000000000099")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "VALIDATION_FAILED") || !strings.Contains(w.Body.String(), "parcel.dimensions") {
		t.Errorf("expected VALIDATION_FAILED with parcel.dimensions, got %s", w.Body.String())
	}
}

func TestCreateOrder_Unauthenticated(t *testing.T) {
	mux, _ := setupTestMux(Claims{})

	req := httptest.NewRequest("POST", "/api/v1/merchant/orders", bytes.NewBufferString("{}"))
	req.Header.Set("Idempotency-Key", "0192e2b3-7c8a-7890-a123-000000000099")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", w.Code)
	}
}

func TestListOrders_Success(t *testing.T) {
	tenantID := "0192e2b3-7c8a-7890-a123-000000000001"
	claims := Claims{Sub: "user_m1", Role: "MERCHANT", TenantID: tenantID}

	mockBook := configurableMockBooking{
		listFn: func(ctx context.Context, merchantID string) ([]application.OrderResult, error) {
			if merchantID != tenantID {
				t.Fatalf("expected merchantID %s, got %s", tenantID, merchantID)
			}
			res := []application.OrderResult{
				{
					ParcelID:       "0192e2b3-7c8a-7890-a123-000000000102",
					TrackingNumber: "SD-20261119-00002",
					Status:         "IN_TRANSIT",
					CreatedAt:      "2026-11-19T11:00:00Z",
				},
				{
					ParcelID:       "0192e2b3-7c8a-7890-a123-000000000101",
					TrackingNumber: "SD-20261119-00001",
					Status:         "CREATED",
					CreatedAt:      "2026-11-19T10:00:00Z",
				},
			}
			return res, nil
		},
	}

	mux, _, mRepo := setupTestMuxWithBooking(claims, mockBook)
	_ = mRepo.Insert(context.Background(), nil, application.MerchantView{
		ID:     tenantID,
		Status: domain.StatusApproved,
	})

	req := httptest.NewRequest("GET", "/api/v1/merchant/orders", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data, ok := resp["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("expected 2 orders in data array, got %v", resp)
	}
	meta, ok := resp["meta"].(map[string]any)
	if !ok || meta["total_items"] != float64(2) {
		t.Errorf("expected total_items 2 in meta, got %v", meta)
	}
}
