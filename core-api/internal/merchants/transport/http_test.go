package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/merchants/application"
	"github.com/swiftdrop/core-api/internal/merchants/domain"
)

type mockVerifier struct {
	claims Claims
	err    error
}

func (m mockVerifier) Verify(token string) (Claims, error) {
	return m.claims, m.err
}

func setupTestMux(claims Claims) (*http.ServeMux, *application.Service) {
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
		Booking:   mockBooking{},
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
	return mux, svc
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

func (m *mockMerchantRepo) Insert(ctx context.Context, q application.Querier, v application.MerchantView) error {
	m.merchants[v.ID] = v
	return nil
}
func (m *mockMerchantRepo) Get(ctx context.Context, q application.Querier, id string) (application.MerchantView, error) {
	v, ok := m.merchants[id]
	if !ok {
		return application.MerchantView{}, application.ErrMerchantNotFound
	}
	return v, nil
}
func (m *mockMerchantRepo) FindByOwner(ctx context.Context, q application.Querier, ownerUserID string) (application.MerchantView, error) {
	return application.MerchantView{}, application.ErrNotFound
}
func (m *mockMerchantRepo) Status(ctx context.Context, q application.Querier, id string) (domain.MerchantStatus, error) {
	v, ok := m.merchants[id]
	if !ok {
		return "", application.ErrMerchantNotFound
	}
	return v.Status, nil
}
func (m *mockMerchantRepo) LockForUpdate(ctx context.Context, q application.Querier, id string) (application.MerchantView, error) {
	return m.Get(ctx, q, id)
}
func (m *mockMerchantRepo) SaveStatus(ctx context.Context, q application.Querier, merchant *domain.Merchant, expectedVersion int, reason string) error {
	return nil
}
func (m *mockMerchantRepo) List(ctx context.Context, q application.Querier, status string, limit, offset int) ([]application.MerchantView, int, error) {
	return nil, 0, nil
}

type mockCredRepo struct{}

func (mockCredRepo) Insert(ctx context.Context, q application.Querier, r application.CredentialRecord) error {
	return nil
}
func (mockCredRepo) FindByPrefix(ctx context.Context, q application.Querier, prefix string) (application.AuthRecord, error) {
	return application.AuthRecord{}, application.ErrNotFound
}
func (mockCredRepo) List(ctx context.Context, q application.Querier, merchantID string) ([]application.CredentialView, error) {
	return nil, nil
}
func (mockCredRepo) CountActive(ctx context.Context, q application.Querier, merchantID string, now time.Time) (int, error) {
	return 0, nil
}
func (mockCredRepo) Revoke(ctx context.Context, q application.Querier, merchantID, id string, now time.Time) (bool, bool, error) {
	return true, true, nil
}
func (mockCredRepo) RevokeAll(ctx context.Context, q application.Querier, merchantID string, now time.Time) (int, error) {
	return 0, nil
}
func (mockCredRepo) TouchLastUsed(ctx context.Context, q application.Querier, id string, now time.Time) error {
	return nil
}

type mockWebhookRepo struct{}

func (mockWebhookRepo) Insert(ctx context.Context, q application.Querier, r application.WebhookRecord) error {
	return nil
}
func (mockWebhookRepo) List(ctx context.Context, q application.Querier, merchantID string) ([]application.WebhookView, error) {
	return nil, nil
}
func (mockWebhookRepo) CountLive(ctx context.Context, q application.Querier, merchantID string) (int, error) {
	return 0, nil
}
func (mockWebhookRepo) SoftDelete(ctx context.Context, q application.Querier, merchantID, id string, now time.Time) (bool, error) {
	return true, nil
}
func (mockWebhookRepo) DisableAll(ctx context.Context, q application.Querier, merchantID string) (int, error) {
	return 0, nil
}

type mockAuditWriter struct{}

func (mockAuditWriter) Write(ctx context.Context, q application.Querier, e application.AuditEntry) error {
	return nil
}

type mockRoleAssigner struct{}

func (mockRoleAssigner) GrantMerchantRole(ctx context.Context, q application.Querier, userID, merchantID string) error {
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
func (mockHasher) Hash(v int, key string) (string, error) { return "hash_" + key, nil }
func (mockHasher) Equal(a, b string) bool                 { return a == b }

type mockCipher struct{}

func (mockCipher) Encrypt(plain string) ([]byte, int, error) {
	return []byte("enc_" + plain), 1, nil
}

type mockResolver struct{}

func (mockResolver) LookupAddrs(ctx context.Context, host string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
}

type mockBooking struct{}

func (mockBooking) CreateForMerchant(ctx context.Context, merchantID, idemKey, requestHash string, req application.OrderRequest) (application.OrderResult, bool, error) {
	return application.OrderResult{}, false, nil
}
func (mockBooking) ListForMerchant(ctx context.Context, merchantID string) ([]application.OrderResult, error) {
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
