package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityapp "github.com/swiftdrop/core-api/internal/identity/application"
	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityinfra "github.com/swiftdrop/core-api/internal/identity/infrastructure"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/platform/middleware"
	"github.com/swiftdrop/core-api/internal/pricing/application"
	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

const (
	testCustomerUserID  = "018f4a7c-1005-7000-8000-000000000005"
	testAdminUserID     = "018f4a7c-1001-7000-8000-000000000001"
	testMerchantOwnerID = "018f4a7c-1004-7000-8000-000000000004"
	testOtherUserID     = "018f4a7c-9999-7000-8000-000000000099"
	testMerchantID      = "0192e2b3-7c8a-7890-a123-000000000010"
	testQuoteID         = "0192e2b3-7c8a-7890-a123-000000000777"
)

type fakeTariffRepository struct {
	tariff domain.Tariff
	err    error
	called bool
}

func (f *fakeTariffRepository) FindApplicable(_ context.Context, _, _ string, _ time.Time) (domain.Tariff, error) {
	f.called = true
	return f.tariff, f.err
}

type fakeQuoteRepository struct {
	savedQuote domain.Quote
	err        error
	called     bool
}

func (f *fakeQuoteRepository) Save(_ context.Context, quote domain.Quote) error {
	f.called = true
	f.savedQuote = quote
	return f.err
}

type fakeClock struct {
	now time.Time
}

func (f fakeClock) Now() time.Time {
	return f.now
}

type fakeQuoteIDGenerator struct {
	id  string
	err error
}

func (f fakeQuoteIDGenerator) NewID() (string, error) {
	return f.id, f.err
}

type fakeMerchantReader struct {
	profile application.MerchantProfile
	err     error
	called  bool
	lastID  string
}

func (f *fakeMerchantReader) FindByID(_ context.Context, merchantID string) (application.MerchantProfile, error) {
	f.called = true
	f.lastID = merchantID
	return f.profile, f.err
}

func testTariff(now time.Time) domain.Tariff {
	return domain.Tariff{
		ID:                "rule_west_central_v2",
		OriginZoneID:      "ZONE_WESTERN_01",
		DestinationZoneID: "ZONE_CENTRAL_01",
		BaseFee:           domain.Money{Amount: 35000, Currency: domain.CurrencyLKR},
		PerKgRate:         domain.Money{Amount: 8000, Currency: domain.CurrencyLKR},
		EffectiveFrom:     now.Add(-24 * time.Hour),
		IsActive:          true,
		Version:           2,
		CreatedAt:         now.Add(-24 * time.Hour),
		UpdatedAt:         now.Add(-24 * time.Hour),
	}
}

func validQuoteJSON() string {
	return `{
		"origin_zone_id": "ZONE_WESTERN_01",
		"destination_zone_id": "ZONE_CENTRAL_01",
		"actual_weight_kg": 2.5,
		"dimensions": {
			"length_cm": 30.0,
			"width_cm": 25.0,
			"height_cm": 15.0
		},
		"service_level": "STANDARD"
	}`
}

type testHarness struct {
	handler        *Handler
	mux            http.Handler
	tokenProvider  *identityinfra.JWTTokenProvider
	tariffRepo     *fakeTariffRepository
	quoteRepo      *fakeQuoteRepository
	idGen          *fakeQuoteIDGenerator
	merchantReader *fakeMerchantReader
	now            time.Time
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()

	now := time.Date(2026, time.November, 11, 10, 0, 0, 0, time.UTC)
	tariffRepo := &fakeTariffRepository{tariff: testTariff(now)}
	quoteRepo := &fakeQuoteRepository{}
	idGen := &fakeQuoteIDGenerator{id: testQuoteID}
	merchantReader := &fakeMerchantReader{
		profile: application.MerchantProfile{
			ID:          testMerchantID,
			OwnerUserID: testMerchantOwnerID,
			Status:      application.MerchantStatusApproved,
		},
	}

	uc := application.NewGenerateQuoteUseCase(tariffRepo, quoteRepo, fakeClock{now: now}, idGen)
	h, err := NewHandler(uc, merchantReader)
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	tokenProv := identityinfra.NewJWTTokenProvider("test-jwt-secret-key-at-least-32-chars")
	sec := identitytransport.NewSecurityMiddleware(tokenProv)

	mux := http.NewServeMux()
	RegisterRoutes(mux, h, sec)

	return &testHarness{
		handler:        h,
		mux:            middleware.RequestID(mux),
		tokenProvider:  tokenProv,
		tariffRepo:     tariffRepo,
		quoteRepo:      quoteRepo,
		idGen:          idGen,
		merchantReader: merchantReader,
		now:            now,
	}
}

func (th *testHarness) issueToken(t *testing.T, p identityapp.Principal) string {
	t.Helper()
	tok, err := th.tokenProvider.GenerateAccessToken(p)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}
	return tok
}

func strPtr(s string) *string {
	return &s
}

func assertErrorEnvelope(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int, expectedCode string) map[string]any {
	t.Helper()
	if rec.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d; body=%s", expectedStatus, rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON error body: %v", err)
	}

	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected canonical {\"error\": ...} envelope, got %v", body)
	}

	if errObj["code"] != expectedCode {
		t.Fatalf("expected error code %q, got %v", expectedCode, errObj["code"])
	}
	if _, hasMsg := errObj["message"].(string); !hasMsg {
		t.Fatalf("expected error message string, got %v", errObj)
	}
	if _, hasTS := errObj["timestamp"].(string); !hasTS {
		t.Fatalf("expected error timestamp string, got %v", errObj)
	}

	return errObj
}

func TestNewHandler_Validation(t *testing.T) {
	uc := application.NewGenerateQuoteUseCase(&fakeTariffRepository{}, &fakeQuoteRepository{}, fakeClock{}, fakeQuoteIDGenerator{})
	if _, err := NewHandler(nil, &fakeMerchantReader{}); err == nil {
		t.Fatal("expected error when quote use case is nil")
	}
	if _, err := NewHandler(uc, nil); err == nil {
		t.Fatal("expected error when merchant reader is nil")
	}
}

func TestCreateQuote_SuccessPaths(t *testing.T) {
	t.Run("valid CUSTOMER quote request", func(t *testing.T) {
		th := newTestHarness(t)
		token := th.issueToken(t, identityapp.Principal{
			UserID: testCustomerUserID,
			Email:  "customer@swiftdrop.local",
			Role:   identitydomain.RoleCustomer,
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d; body=%s", rec.Code, rec.Body.String())
		}
		if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("expected JSON Content-Type, got %q", rec.Header().Get("Content-Type"))
		}

		var env struct {
			Data QuoteResponse `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
			t.Fatalf("failed to decode data envelope: %v", err)
		}

		if env.Data.QuoteID != testQuoteID {
			t.Errorf("expected quote_id %s, got %s", testQuoteID, env.Data.QuoteID)
		}
		if env.Data.FeeCents != 47000 || env.Data.Currency != domain.CurrencyLKR {
			t.Errorf("expected fee 47000 LKR, got %d %s", env.Data.FeeCents, env.Data.Currency)
		}
		if env.Data.ChargeableWeightKg != 2.5 || env.Data.ServiceLevel != "STANDARD" {
			t.Errorf("unexpected weight/service level: %+v", env.Data)
		}
		if env.Data.PricingRuleID != "rule_west_central_v2" || env.Data.PricingRuleVersion != 2 {
			t.Errorf("unexpected tariff metadata: %+v", env.Data)
		}
		if env.Data.CreatedAt != "2026-11-11T10:00:00Z" || env.Data.ExpiresAt != "2026-11-11T10:15:00Z" {
			t.Errorf("unexpected timestamps: created=%s expires=%s", env.Data.CreatedAt, env.Data.ExpiresAt)
		}
		if env.Data.RequesterID == nil || *env.Data.RequesterID != testCustomerUserID {
			t.Errorf("expected requester_id %s, got %v", testCustomerUserID, env.Data.RequesterID)
		}
		if env.Data.MerchantID != nil {
			t.Errorf("expected nil merchant_id for customer quote, got %v", *env.Data.MerchantID)
		}
		if th.merchantReader.called {
			t.Error("merchantReader should not be called for CUSTOMER quote")
		}
		if !th.quoteRepo.called || th.quoteRepo.savedQuote.RequesterID == nil || *th.quoteRepo.savedQuote.RequesterID != testCustomerUserID {
			t.Error("expected persisted quote bound to customer user ID")
		}
	})

	t.Run("valid approved MERCHANT quote request", func(t *testing.T) {
		th := newTestHarness(t)
		token := th.issueToken(t, identityapp.Principal{
			UserID:   testMerchantOwnerID,
			Email:    "merchant@vendor.com",
			Role:     identitydomain.RoleMerchant,
			TenantID: strPtr(testMerchantID),
		})

		expressBody := strings.Replace(validQuoteJSON(), `"STANDARD"`, `"EXPRESS"`, 1)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(expressBody))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d; body=%s", rec.Code, rec.Body.String())
		}

		var env struct {
			Data QuoteResponse `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !th.merchantReader.called || th.merchantReader.lastID != testMerchantID {
			t.Fatalf("expected MerchantReader.FindByID(%s) to be called", testMerchantID)
		}
		if env.Data.FeeCents != 70500 || env.Data.ServiceLevel != "EXPRESS" {
			t.Errorf("expected EXPRESS fee 70500, got %d (%s)", env.Data.FeeCents, env.Data.ServiceLevel)
		}
		if env.Data.MerchantID == nil || *env.Data.MerchantID != testMerchantID {
			t.Errorf("expected merchant_id %s, got %v", testMerchantID, env.Data.MerchantID)
		}
		if env.Data.RequesterID == nil || *env.Data.RequesterID != testMerchantOwnerID {
			t.Errorf("expected requester_id %s, got %v", testMerchantOwnerID, env.Data.RequesterID)
		}
	})

	t.Run("valid ADMIN self-owned quote request", func(t *testing.T) {
		th := newTestHarness(t)
		token := th.issueToken(t, identityapp.Principal{
			UserID:   testAdminUserID,
			Email:    "admin@swiftdrop.local",
			Role:     identitydomain.RoleAdmin,
			TenantID: strPtr(testMerchantID), // even if tenant_id were set on JWT, ADMIN resolves self-owned RequesterID only
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d; body=%s", rec.Code, rec.Body.String())
		}

		var env struct {
			Data QuoteResponse `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if env.Data.RequesterID == nil || *env.Data.RequesterID != testAdminUserID {
			t.Errorf("expected admin requester_id %s, got %v", testAdminUserID, env.Data.RequesterID)
		}
		if env.Data.MerchantID != nil {
			t.Errorf("expected nil merchant_id for ADMIN self-owned quote, got %v", *env.Data.MerchantID)
		}
		if th.merchantReader.called {
			t.Error("merchantReader should not be called for ADMIN self-owned quote")
		}
	})

	t.Run("verifies canonical single-data envelope has no top-level total_fee_cents", func(t *testing.T) {
		th := newTestHarness(t)
		token := th.issueToken(t, identityapp.Principal{
			UserID: testCustomerUserID,
			Email:  "customer@swiftdrop.local",
			Role:   identitydomain.RoleCustomer,
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d", rec.Code)
		}

		var raw map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
			t.Fatalf("failed to decode raw response: %v", err)
		}
		if len(raw) != 1 || raw["data"] == nil {
			t.Fatalf("expected only top-level \"data\" key, got %v", raw)
		}
	})
}

func TestCreateQuote_AuthAndOwnershipErrors(t *testing.T) {
	t.Run("missing authentication", func(t *testing.T) {
		th := newTestHarness(t)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
		if th.tariffRepo.called || th.quoteRepo.called {
			t.Error("pricing repositories must not be called when unauthenticated")
		}
	})

	t.Run("invalid role through router middleware", func(t *testing.T) {
		th := newTestHarness(t)
		for _, role := range []identitydomain.Role{identitydomain.RoleCourier, identitydomain.RoleDispatcher} {
			token := th.issueToken(t, identityapp.Principal{
				UserID: testCustomerUserID,
				Email:  "staff@swiftdrop.local",
				Role:   role,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()

			th.mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("role %s: expected 403 Forbidden, got %d", role, rec.Code)
			}
		}
	})

	t.Run("invalid role checked directly in handler", func(t *testing.T) {
		th := newTestHarness(t)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req = req.WithContext(identitytransport.WithPrincipal(req.Context(), &identityapp.Principal{
			UserID: testCustomerUserID,
			Role:   identitydomain.RoleCourier,
		}))
		rec := httptest.NewRecorder()

		th.handler.CreateQuote(rec, req)

		assertErrorEnvelope(t, rec, http.StatusForbidden, "AUTH_FORBIDDEN")
	})

	t.Run("non-UUID authenticated user ID rejected", func(t *testing.T) {
		th := newTestHarness(t)
		token := th.issueToken(t, identityapp.Principal{
			UserID: "usr_01HCUSTOMER00000000000005",
			Role:   identitydomain.RoleCustomer,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		assertErrorEnvelope(t, rec, http.StatusForbidden, "INVALID_CALLER_ID")
	})

	t.Run("missing merchant tenant ID returns 403 MISSING_TENANT_ID", func(t *testing.T) {
		th := newTestHarness(t)
		for _, tenant := range []*string{nil, strPtr(""), strPtr("   ")} {
			token := th.issueToken(t, identityapp.Principal{
				UserID:   testMerchantOwnerID,
				Role:     identitydomain.RoleMerchant,
				TenantID: tenant,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-Request-ID", "corr-missing-tenant")
			rec := httptest.NewRecorder()

			th.mux.ServeHTTP(rec, req)

			errObj := assertErrorEnvelope(t, rec, http.StatusForbidden, "MISSING_TENANT_ID")
			if errObj["correlation_id"] != "corr-missing-tenant" {
				t.Errorf("expected correlation_id propagated, got %v", errObj["correlation_id"])
			}
			if th.merchantReader.called || th.quoteRepo.called {
				t.Error("merchantReader and quoteRepo must not be called for missing tenant_id")
			}
		}
	})

	t.Run("non-UUID merchant tenant ID returns 403 TENANT_NOT_MERCHANT_UUID", func(t *testing.T) {
		th := newTestHarness(t)
		token := th.issueToken(t, identityapp.Principal{
			UserID:   testMerchantOwnerID,
			Role:     identitydomain.RoleMerchant,
			TenantID: strPtr("mrc_01HMERCHANTTENANT00001"),
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		assertErrorEnvelope(t, rec, http.StatusForbidden, "TENANT_NOT_MERCHANT_UUID")
		if th.merchantReader.called {
			t.Error("merchantReader must not be queried for non-UUID tenant_id")
		}
	})

	t.Run("merchant not found returns 403 MERCHANT_NOT_FOUND", func(t *testing.T) {
		th := newTestHarness(t)
		th.merchantReader.err = application.ErrMerchantNotFound

		token := th.issueToken(t, identityapp.Principal{
			UserID:   testMerchantOwnerID,
			Role:     identitydomain.RoleMerchant,
			TenantID: strPtr(testMerchantID),
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		assertErrorEnvelope(t, rec, http.StatusForbidden, "MERCHANT_NOT_FOUND")
		if th.quoteRepo.called {
			t.Error("quoteRepo must not be called when merchant does not exist")
		}
	})

	t.Run("merchant owner mismatch returns 403 MERCHANT_OWNER_MISMATCH", func(t *testing.T) {
		th := newTestHarness(t)
		th.merchantReader.profile.OwnerUserID = testOtherUserID

		token := th.issueToken(t, identityapp.Principal{
			UserID:   testMerchantOwnerID,
			Role:     identitydomain.RoleMerchant,
			TenantID: strPtr(testMerchantID),
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		assertErrorEnvelope(t, rec, http.StatusForbidden, "MERCHANT_OWNER_MISMATCH")
		if th.quoteRepo.called {
			t.Error("quoteRepo must not be called on merchant owner mismatch")
		}
	})

	t.Run("merchant not approved returns 403 MERCHANT_NOT_APPROVED", func(t *testing.T) {
		for _, status := range []string{"PENDING", "REJECTED", "SUSPENDED", "DISABLED"} {
			t.Run(status, func(t *testing.T) {
				th := newTestHarness(t)
				th.merchantReader.profile.Status = status

				token := th.issueToken(t, identityapp.Principal{
					UserID:   testMerchantOwnerID,
					Role:     identitydomain.RoleMerchant,
					TenantID: strPtr(testMerchantID),
				})
				req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
				req.Header.Set("Authorization", "Bearer "+token)
				rec := httptest.NewRecorder()

				th.mux.ServeHTTP(rec, req)

				assertErrorEnvelope(t, rec, http.StatusForbidden, "MERCHANT_NOT_APPROVED")
				if th.quoteRepo.called {
					t.Error("quoteRepo must not be called for non-approved merchant")
				}
			})
		}
	})

	t.Run("merchant reader failure fails closed with 500 MERCHANT_LOOKUP_FAILED", func(t *testing.T) {
		th := newTestHarness(t)
		th.merchantReader.err = errors.New("postgres timeout")

		token := th.issueToken(t, identityapp.Principal{
			UserID:   testMerchantOwnerID,
			Role:     identitydomain.RoleMerchant,
			TenantID: strPtr(testMerchantID),
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		assertErrorEnvelope(t, rec, http.StatusInternalServerError, "MERCHANT_LOOKUP_FAILED")
	})
}

func TestCreateQuote_InputValidationAndFailureErrors(t *testing.T) {
	t.Run("invalid JSON request body returns 400", func(t *testing.T) {
		cases := []struct {
			name string
			body string
		}{
			{name: "malformed JSON", body: `{"origin_zone_id": `},
			{name: "empty body", body: ``},
			{name: "trailing extra JSON document", body: validQuoteJSON() + `{"extra":1}`},
			{name: "forged requester_id rejected", body: `{"origin_zone_id":"ZONE_WESTERN_01","destination_zone_id":"ZONE_CENTRAL_01","actual_weight_kg":2.0,"dimensions":{"length_cm":10,"width_cm":10,"height_cm":10},"service_level":"STANDARD","requester_id":"018f4a7c-9999-7000-8000-000000000099"}`},
			{name: "forged merchant_id rejected", body: `{"origin_zone_id":"ZONE_WESTERN_01","destination_zone_id":"ZONE_CENTRAL_01","actual_weight_kg":2.0,"dimensions":{"length_cm":10,"width_cm":10,"height_cm":10},"service_level":"STANDARD","merchant_id":"0192e2b3-7c8a-7890-a123-000000000010"}`},
			{name: "wrong field type", body: `{"origin_zone_id":"ZONE_WESTERN_01","destination_zone_id":"ZONE_CENTRAL_01","actual_weight_kg":"heavy","dimensions":{"length_cm":10,"width_cm":10,"height_cm":10},"service_level":"STANDARD"}`},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				th := newTestHarness(t)
				token := th.issueToken(t, identityapp.Principal{
					UserID: testCustomerUserID,
					Role:   identitydomain.RoleCustomer,
				})
				req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(tc.body))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()

				th.mux.ServeHTTP(rec, req)

				assertErrorEnvelope(t, rec, http.StatusBadRequest, "INVALID_REQUEST_PAYLOAD")
				if th.tariffRepo.called || th.quoteRepo.called {
					t.Error("repositories must not be called on malformed/disallowed JSON")
				}
			})
		}
	})

	t.Run("missing dimensions returns 422 INVALID_DIMENSIONS", func(t *testing.T) {
		th := newTestHarness(t)
		token := th.issueToken(t, identityapp.Principal{
			UserID: testCustomerUserID,
			Role:   identitydomain.RoleCustomer,
		})
		body := `{"origin_zone_id":"ZONE_WESTERN_01","destination_zone_id":"ZONE_CENTRAL_01","actual_weight_kg":2.0,"service_level":"STANDARD"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		assertErrorEnvelope(t, rec, http.StatusUnprocessableEntity, "INVALID_DIMENSIONS")
	})

	t.Run("oversized body returns 413 PAYLOAD_TOO_LARGE", func(t *testing.T) {
		th := newTestHarness(t)
		token := th.issueToken(t, identityapp.Principal{
			UserID: testCustomerUserID,
			Role:   identitydomain.RoleCustomer,
		})
		oversized := `{"origin_zone_id":"` + strings.Repeat("A", 70*1024) + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", bytes.NewBufferString(oversized))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		assertErrorEnvelope(t, rec, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE")
	})

	t.Run("unsupported Content-Type returns 415 UNSUPPORTED_MEDIA_TYPE", func(t *testing.T) {
		th := newTestHarness(t)
		token := th.issueToken(t, identityapp.Principal{
			UserID: testCustomerUserID,
			Role:   identitydomain.RoleCustomer,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		assertErrorEnvelope(t, rec, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE")
	})

	t.Run("invalid zone format returns 422 INVALID_ZONE_ID", func(t *testing.T) {
		th := newTestHarness(t)
		token := th.issueToken(t, identityapp.Principal{
			UserID: testCustomerUserID,
			Role:   identitydomain.RoleCustomer,
		})
		badZoneBody := strings.Replace(validQuoteJSON(), "ZONE_WESTERN_01", "ZONE-WESTERN-01", 1)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(badZoneBody))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		assertErrorEnvelope(t, rec, http.StatusUnprocessableEntity, "INVALID_ZONE_ID")
	})

	t.Run("invalid service level returns 422 INVALID_SERVICE_LEVEL", func(t *testing.T) {
		th := newTestHarness(t)
		token := th.issueToken(t, identityapp.Principal{
			UserID: testCustomerUserID,
			Role:   identitydomain.RoleCustomer,
		})
		badSvcBody := strings.Replace(validQuoteJSON(), `"STANDARD"`, `"OVERNIGHT"`, 1)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(badSvcBody))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		assertErrorEnvelope(t, rec, http.StatusUnprocessableEntity, "INVALID_SERVICE_LEVEL")
	})

	t.Run("exceeded chargeable weight returns 422 PARCEL_LIMITS_EXCEEDED", func(t *testing.T) {
		th := newTestHarness(t)
		token := th.issueToken(t, identityapp.Principal{
			UserID: testCustomerUserID,
			Role:   identitydomain.RoleCustomer,
		})
		heavyBody := strings.Replace(validQuoteJSON(), `"actual_weight_kg": 2.5`, `"actual_weight_kg": 31.0`, 1)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(heavyBody))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		assertErrorEnvelope(t, rec, http.StatusUnprocessableEntity, "PARCEL_LIMITS_EXCEEDED")
	})

	t.Run("tariff not found returns 422 PRICING_UNAVAILABLE", func(t *testing.T) {
		th := newTestHarness(t)
		th.tariffRepo.err = domain.ErrTariffNotFound
		token := th.issueToken(t, identityapp.Principal{
			UserID: testCustomerUserID,
			Role:   identitydomain.RoleCustomer,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		assertErrorEnvelope(t, rec, http.StatusUnprocessableEntity, "PRICING_UNAVAILABLE")
	})

	t.Run("quote repository save failure returns 500 INTERNAL_ERROR", func(t *testing.T) {
		th := newTestHarness(t)
		th.quoteRepo.err = errors.New("pq: insert timeout")
		token := th.issueToken(t, identityapp.Principal{
			UserID: testCustomerUserID,
			Role:   identitydomain.RoleCustomer,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		errObj := assertErrorEnvelope(t, rec, http.StatusInternalServerError, "INTERNAL_ERROR")
		if strings.Contains(errObj["message"].(string), "pq:") {
			t.Error("internal error details must not be leaked to API clients")
		}
	})

	t.Run("quote ID generation failure returns 500 INTERNAL_ERROR", func(t *testing.T) {
		th := newTestHarness(t)
		th.idGen.err = errors.New("entropy failure")
		token := th.issueToken(t, identityapp.Principal{
			UserID: testCustomerUserID,
			Role:   identitydomain.RoleCustomer,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pricing/quotes", strings.NewReader(validQuoteJSON()))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		th.mux.ServeHTTP(rec, req)

		assertErrorEnvelope(t, rec, http.StatusInternalServerError, "INTERNAL_ERROR")
	})
}
