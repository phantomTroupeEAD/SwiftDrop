package application

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/merchants/domain"
)

// In-memory test doubles for application tests

type mockTxRunner struct{}

func (m *mockTxRunner) WithinTx(ctx context.Context, fn func(context.Context, Querier) error) error {
	return fn(ctx, nil)
}
func (m *mockTxRunner) Reader() Querier { return nil }

type mockMerchantRepo struct {
	merchants map[string]MerchantView
	byOwner   map[string]string
}

func newMockMerchantRepo() *mockMerchantRepo {
	return &mockMerchantRepo{
		merchants: make(map[string]MerchantView),
		byOwner:   make(map[string]string),
	}
}

func (m *mockMerchantRepo) Insert(ctx context.Context, q Querier, v MerchantView) error {
	if _, ok := m.byOwner[v.OwnerUserID]; ok {
		return ErrMerchantExists
	}
	m.merchants[v.ID] = v
	m.byOwner[v.OwnerUserID] = v.ID
	return nil
}

func (m *mockMerchantRepo) Get(ctx context.Context, q Querier, id string) (MerchantView, error) {
	v, ok := m.merchants[id]
	if !ok {
		return MerchantView{}, ErrMerchantNotFound
	}
	return v, nil
}

func (m *mockMerchantRepo) FindByOwner(ctx context.Context, q Querier, ownerUserID string) (MerchantView, error) {
	id, ok := m.byOwner[ownerUserID]
	if !ok {
		return MerchantView{}, ErrNotFound
	}
	return m.merchants[id], nil
}

func (m *mockMerchantRepo) Status(ctx context.Context, q Querier, id string) (domain.MerchantStatus, error) {
	v, ok := m.merchants[id]
	if !ok {
		return "", ErrMerchantNotFound
	}
	return v.Status, nil
}

func (m *mockMerchantRepo) LockForUpdate(ctx context.Context, q Querier, id string) (MerchantView, error) {
	return m.Get(ctx, q, id)
}

func (m *mockMerchantRepo) SaveStatus(ctx context.Context, q Querier, merchant *domain.Merchant, expectedVersion int, reason string) error {
	v, ok := m.merchants[merchant.ID()]
	if !ok {
		return ErrMerchantNotFound
	}
	if v.Version != expectedVersion {
		return ErrVersionConflict
	}
	v.Status = merchant.Status()
	v.StatusReason = reason
	v.Version++
	m.merchants[merchant.ID()] = v
	return nil
}

func (m *mockMerchantRepo) List(ctx context.Context, q Querier, status string, limit, offset int) ([]MerchantView, int, error) {
	var res []MerchantView
	for _, v := range m.merchants {
		if status == "" || string(v.Status) == status {
			res = append(res, v)
		}
	}
	return res, len(res), nil
}

type mockCredRepo struct {
	creds []CredentialRecord
}

func (m *mockCredRepo) Insert(ctx context.Context, q Querier, r CredentialRecord) error {
	m.creds = append(m.creds, r)
	return nil
}

func (m *mockCredRepo) FindByPrefix(ctx context.Context, q Querier, prefix string) (AuthRecord, error) {
	for _, c := range m.creds {
		if c.Prefix == prefix {
			return AuthRecord{
				CredentialID:   c.ID,
				MerchantID:     c.MerchantID,
				Hash:           c.Hash,
				PepperVersion:  c.PepperVersion,
				Scopes:         c.Scopes,
				ExpiresAt:      c.ExpiresAt,
				MerchantStatus: domain.StatusApproved,
			}, nil
		}
	}
	return AuthRecord{}, ErrNotFound
}

func (m *mockCredRepo) List(ctx context.Context, q Querier, merchantID string) ([]CredentialView, error) {
	var out []CredentialView
	for _, c := range m.creds {
		if c.MerchantID == merchantID {
			out = append(out, CredentialView{
				ID:        c.ID,
				Label:     c.Label,
				Prefix:    c.Prefix,
				Scopes:    c.Scopes,
				ExpiresAt: c.ExpiresAt,
			})
		}
	}
	return out, nil
}

func (m *mockCredRepo) CountActive(ctx context.Context, q Querier, merchantID string, now time.Time) (int, error) {
	count := 0
	for _, c := range m.creds {
		if c.MerchantID == merchantID {
			if c.ExpiresAt == nil || c.ExpiresAt.After(now) {
				count++
			}
		}
	}
	return count, nil
}

func (m *mockCredRepo) Revoke(ctx context.Context, q Querier, merchantID, id string, now time.Time) (bool, bool, error) {
	return true, true, nil
}

func (m *mockCredRepo) RevokeAll(ctx context.Context, q Querier, merchantID string, now time.Time) (int, error) {
	return len(m.creds), nil
}

func (m *mockCredRepo) TouchLastUsed(ctx context.Context, q Querier, id string, now time.Time) error {
	return nil
}

type mockWebhookRepo struct {
	hooks []WebhookRecord
}

func (m *mockWebhookRepo) Insert(ctx context.Context, q Querier, r WebhookRecord) error {
	m.hooks = append(m.hooks, r)
	return nil
}

func (m *mockWebhookRepo) List(ctx context.Context, q Querier, merchantID string) ([]WebhookView, error) {
	var out []WebhookView
	for _, h := range m.hooks {
		if h.MerchantID == merchantID {
			out = append(out, WebhookView{
				ID:      h.ID,
				URL:     h.URL,
				Last4:   h.Last4,
				Events:  h.Events,
				Enabled: true,
			})
		}
	}
	return out, nil
}

func (m *mockWebhookRepo) CountLive(ctx context.Context, q Querier, merchantID string) (int, error) {
	return len(m.hooks), nil
}

func (m *mockWebhookRepo) SoftDelete(ctx context.Context, q Querier, merchantID, id string, now time.Time) (bool, error) {
	return true, nil
}

func (m *mockWebhookRepo) DisableAll(ctx context.Context, q Querier, merchantID string) (int, error) {
	return len(m.hooks), nil
}

type mockAuditWriter struct{}

func (mockAuditWriter) Write(ctx context.Context, q Querier, e AuditEntry) error { return nil }

type mockRoleAssigner struct{}

func (mockRoleAssigner) GrantMerchantRole(ctx context.Context, q Querier, userID, merchantID string) error {
	return nil
}

type mockKeyGen struct{}

func (mockKeyGen) NewAPIKey(env string) (domain.PlainAPIKey, error) {
	return domain.NewPlainAPIKey(env, "AbCdEf12", "12345678901234567890123456789012")
}

func (mockKeyGen) NewSigningSecret() (string, error) {
	return "whsec_test1234567890123456789012345678901234567890", nil
}

type mockHasher struct{}

func (mockHasher) CurrentVersion() int                    { return 1 }
func (mockHasher) Hash(v int, key string) (string, error) { return "mock_hash_" + key, nil }
func (mockHasher) Equal(a, b string) bool                 { return a == b }

type mockCipher struct{}

func (mockCipher) Encrypt(plain string) ([]byte, int, error) {
	return []byte("encrypted_" + plain), 1, nil
}

type mockResolver struct {
	ip netip.Addr
}

func (m mockResolver) LookupAddrs(ctx context.Context, host string) ([]netip.Addr, error) {
	if m.ip.IsValid() {
		return []netip.Addr{m.ip}, nil
	}
	return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil // example.com public IP
}

type mockBooking struct{}

func (mockBooking) CreateForMerchant(ctx context.Context, merchantID, idemKey, requestHash string, req OrderRequest) (OrderResult, bool, error) {
	res := OrderResult{
		ParcelID:       "parcel_123",
		TrackingNumber: "TRK-12345",
		Status:         "BOOKED",
	}
	res.Fee.Currency = "LKR"
	res.Fee.AmountMinor = 150000
	return res, false, nil
}

func (mockBooking) ListForMerchant(ctx context.Context, merchantID string) ([]OrderResult, error) {
	return nil, nil
}

type mockObserver struct{}

func (mockObserver) AuthResult(_ string)    {}
func (mockObserver) SSRFBlocked()           {}
func (mockObserver) StatusChanged(_ string) {}

func setupTestService() (*Service, *mockMerchantRepo, *mockCredRepo) {
	mRepo := newMockMerchantRepo()
	cRepo := &mockCredRepo{}
	hRepo := &mockWebhookRepo{}

	svc := NewService(Deps{
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
	}, Config{
		KeyEnvPrefix:  "sd_live_",
		MaxActiveKeys: 5,
		MaxWebhooks:   10,
		AllowHTTP:     false,
	})
	return svc, mRepo, cRepo
}

func TestSubmitApplication_Success(t *testing.T) {
	svc, _, _ := setupTestService()
	actor := Actor{UserID: "user_123", Role: "CUSTOMER"}

	in := ApplicationInput{
		BusinessName: "Acme Logistics",
		ContactEmail: "contact@acme.com",
		ContactPhone: "+94771234567",
		Address: domain.Address{
			Line1:      "123 Main St",
			City:       "Colombo",
			PostalCode: "00100",
			Country:    "LK",
		},
	}

	view, err := svc.SubmitApplication(context.Background(), actor, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.BusinessName != "Acme Logistics" {
		t.Fatalf("expected business name 'Acme Logistics', got %q", view.BusinessName)
	}
	if view.Status != domain.StatusPending {
		t.Fatalf("expected status PENDING, got %s", view.Status)
	}
}

func TestSubmitApplication_ValidationFailure(t *testing.T) {
	svc, _, _ := setupTestService()
	actor := Actor{UserID: "user_123", Role: "CUSTOMER"}

	// Invalid short business name
	in := ApplicationInput{
		BusinessName: "A",
		ContactEmail: "contact@acme.com",
		ContactPhone: "+94771234567",
		Address: domain.Address{
			Line1:      "123 Main St",
			City:       "Colombo",
			PostalCode: "00100",
			Country:    "LK",
		},
	}

	_, err := svc.SubmitApplication(context.Background(), actor, in)
	if err == nil {
		t.Fatal("expected validation error for 1-char business name, got nil")
	}
	var ve ValidationError
	if !errors.As(err, &ve) || ve.Field != "business_name" {
		t.Fatalf("expected ValidationError on business_name, got %v", err)
	}
}

func TestEventCatalog(t *testing.T) {
	if len(EventCatalog) < 5 {
		t.Fatalf("expected at least 5 events in catalog, got %d", len(EventCatalog))
	}
	if !inCatalog("parcel.delivered.v1") {
		t.Fatal("expected parcel.delivered.v1 in event catalog")
	}
	if inCatalog("unsupported.event.type") {
		t.Fatal("did not expect unsupported event in catalog")
	}
}

func TestChangeStatus_TransitionInvariants(t *testing.T) {
	svc, mRepo, _ := setupTestService()

	// Pre-populate a pending merchant
	mRepo.merchants["m_test"] = MerchantView{
		ID:           "m_test",
		OwnerUserID:  "user_123",
		BusinessName: "Test Merchant",
		Status:       domain.StatusPending,
		Version:      1,
	}

	admin := Actor{UserID: "admin_1", Role: "ADMIN"}

	// Admin approves merchant
	v, err := svc.ChangeStatus(context.Background(), admin, "m_test", domain.ActionApprove, 1, "Approved following verification")
	if err != nil {
		t.Fatalf("unexpected error on approve: %v", err)
	}
	if v.Status != domain.StatusApproved {
		t.Fatalf("expected status APPROVED, got %s", v.Status)
	}

	// Try approving already approved merchant -> invalid transition
	_, err = svc.ChangeStatus(context.Background(), admin, "m_test", domain.ActionApprove, 2, "Second approve")
	if err == nil {
		t.Fatal("expected invalid transition error when approving an approved merchant")
	}
}

func TestCreateAndVerifyAPIKey_FullPrefixAligned(t *testing.T) {
	svc, mRepo, _ := setupTestService()

	mRepo.merchants["m_approved"] = MerchantView{
		ID:           "m_approved",
		OwnerUserID:  "user_123",
		BusinessName: "Approved Merchant",
		Status:       domain.StatusApproved,
		Version:      1,
	}

	actor := Actor{UserID: "user_123", Role: "MERCHANT", TenantID: "m_approved"}
	credView, plainKey, err := svc.CreateAPIKey(context.Background(), actor, "Test Key", []string{"parcels:create", "parcels:read"}, nil)
	if err != nil {
		t.Fatalf("unexpected error creating key: %v", err)
	}

	// Assert full prefix has format sd_live_<8prefix> (16 chars)
	if !strings.HasPrefix(credView.Prefix, "sd_live_") || len(credView.Prefix) != 16 {
		t.Fatalf("expected 16-character full prefix starting with sd_live_, got %q (len %d)", credView.Prefix, len(credView.Prefix))
	}

	// Verify the API key through verification flow
	principal, err := svc.VerifyAPIKey(context.Background(), plainKey.Reveal())
	if err != nil {
		t.Fatalf("expected key verification to pass, got %v", err)
	}
	if principal.MerchantID != "m_approved" {
		t.Fatalf("expected merchant ID m_approved, got %s", principal.MerchantID)
	}
	if principal.KeyPrefix != credView.Prefix {
		t.Fatalf("expected principal KeyPrefix %s, got %s", credView.Prefix, principal.KeyPrefix)
	}
}
