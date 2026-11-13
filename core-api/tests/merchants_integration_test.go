package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	identityapp "github.com/swiftdrop/core-api/internal/identity/application"
	identityinfra "github.com/swiftdrop/core-api/internal/identity/infrastructure"
	merchantsapp "github.com/swiftdrop/core-api/internal/merchants/application"
	merchantsdomain "github.com/swiftdrop/core-api/internal/merchants/domain"
	merchantsinfra "github.com/swiftdrop/core-api/internal/merchants/infrastructure"
	merchantstransport "github.com/swiftdrop/core-api/internal/merchants/transport"
)

// testTxRunner wraps *sql.DB to satisfy merchantsapp.TxRunner.
type testTxRunner struct {
	db *sql.DB
}

func (r *testTxRunner) WithinTx(ctx context.Context, fn func(context.Context, merchantsapp.Querier) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *testTxRunner) Reader() merchantsapp.Querier {
	return r.db
}

// testJWTVerifier adapts Member 1's TokenProvider for merchantstransport.TokenVerifier.
type testJWTVerifier struct {
	prov identityapp.TokenProvider
}

func (v *testJWTVerifier) Verify(token string) (merchantstransport.Claims, error) {
	principal, err := v.prov.ValidateAccessToken(token)
	if err != nil {
		return merchantstransport.Claims{}, err
	}
	tenantID := ""
	if principal.TenantID != nil {
		tenantID = *principal.TenantID
	}
	return merchantstransport.Claims{
		Sub:      principal.UserID,
		Role:     string(principal.Role),
		TenantID: tenantID,
	}, nil
}

type testNoopObserver struct{}

func (testNoopObserver) AuthResult(_ string)    {}
func (testNoopObserver) SSRFBlocked()           {}
func (testNoopObserver) StatusChanged(_ string) {}

// TestMerchantsPostgresRepository_Integration runs against a real PostgreSQL schema
// verifying table constraints, optimistic locking, the merchant_webhook_targets view, and audit logging.
func TestMerchantsPostgresRepository_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	merchantRepo := merchantsinfra.MerchantRepo{}
	credRepo := merchantsinfra.CredentialRepo{}
	webhookRepo := merchantsinfra.WebhookRepo{}
	auditWriter := merchantsinfra.AuditWriter{}

	// Uses seeded customer from 20261021000002_seed_mock_users.up.sql
	ownerUserID := "018f4a7c-1005-7000-8000-000000000005"
	merchantID := "018f4a7c-4001-7000-8000-000000000041"

	t.Run("1. Merchant Insertion and Retrieval", func(t *testing.T) {
		view := merchantsapp.MerchantView{
			ID:           merchantID,
			OwnerUserID:  ownerUserID,
			BusinessName: "Integration Test Merchant Pvt Ltd",
			Status:       merchantsdomain.StatusPending,
			ContactEmail: "merchant.itest@swiftdrop.local",
			ContactPhone: "+94770000041",
			Address: merchantsdomain.Address{
				Line1:   "100 Galle Road",
				City:    "Colombo",
				Country: "LK",
			},
			Version:   1,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := merchantRepo.Insert(ctx, db, view); err != nil {
			t.Fatalf("failed to insert merchant: %v", err)
		}

		fetched, err := merchantRepo.Get(ctx, db, merchantID)
		if err != nil {
			t.Fatalf("failed to get merchant: %v", err)
		}
		if fetched.BusinessName != view.BusinessName {
			t.Errorf("expected business name %q, got %q", view.BusinessName, fetched.BusinessName)
		}
		if fetched.Status != merchantsdomain.StatusPending {
			t.Errorf("expected status %s, got %s", merchantsdomain.StatusPending, fetched.Status)
		}
		if fetched.Version != 1 {
			t.Errorf("expected version 1, got %d", fetched.Version)
		}

		byOwner, err := merchantRepo.FindByOwner(ctx, db, ownerUserID)
		if err != nil {
			t.Fatalf("failed to find merchant by owner: %v", err)
		}
		if byOwner.ID != merchantID {
			t.Errorf("expected merchant ID %s, got %s", merchantID, byOwner.ID)
		}
	})

	t.Run("2. Optimistic Concurrency and Status Transition", func(t *testing.T) {
		merchantDomain := merchantsdomain.Rehydrate(
			merchantID, ownerUserID, "Integration Test Merchant Pvt Ltd",
			merchantsdomain.StatusPending, 1,
		)

		adminID := "018f4a7c-1001-7000-8000-000000000001"
		if err := merchantDomain.Do(merchantsdomain.ActionApprove, adminID, "", now); err != nil {
			t.Fatalf("domain Do(ActionApprove) failed: %v", err)
		}

		// Save status with expected version 1 -> should succeed and bump to 2
		err := merchantRepo.SaveStatus(ctx, db, merchantDomain, 1, "Documents verified successfully")
		if err != nil {
			t.Fatalf("SaveStatus failed: %v", err)
		}

		updated, err := merchantRepo.Get(ctx, db, merchantID)
		if err != nil {
			t.Fatalf("failed to get updated merchant: %v", err)
		}
		if updated.Status != merchantsdomain.StatusApproved {
			t.Errorf("expected status APPROVED, got %s", updated.Status)
		}
		if updated.Version != 2 {
			t.Errorf("expected version 2, got %d", updated.Version)
		}

		// Conflict test: saving with stale version 1 must return ErrVersionConflict
		staleErr := merchantRepo.SaveStatus(ctx, db, merchantDomain, 1, "Attempting stale update")
		if staleErr != merchantsapp.ErrVersionConflict {
			t.Errorf("expected ErrVersionConflict, got %v", staleErr)
		}
	})

	t.Run("3. API Credentials CRUD and Prefix Lookup", func(t *testing.T) {
		credID := "018f4a7c-4002-7000-8000-000000000042"
		prefix := "sd_live_a1B2c3D4"
		hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

		record := merchantsapp.CredentialRecord{
			ID:            credID,
			MerchantID:    merchantID,
			Prefix:        prefix,
			Hash:          hash,
			PepperVersion: 1,
			Label:         "ERP Integration Key",
			Scopes:        []string{"parcels:create", "parcels:read"},
			CreatedBy:     ownerUserID,
			ExpiresAt:     nil,
		}

		if err := credRepo.Insert(ctx, db, record); err != nil {
			t.Fatalf("failed to insert API credential: %v", err)
		}

		authRec, err := credRepo.FindByPrefix(ctx, db, prefix)
		if err != nil {
			t.Fatalf("failed to find credential by prefix: %v", err)
		}
		if authRec.CredentialID != credID || authRec.MerchantID != merchantID {
			t.Errorf("expected credID %s merchantID %s, got credID %s merchantID %s",
				credID, merchantID, authRec.CredentialID, authRec.MerchantID)
		}
		if authRec.Hash != hash {
			t.Errorf("expected hash %s, got %s", hash, authRec.Hash)
		}

		activeCount, err := credRepo.CountActive(ctx, db, merchantID, now)
		if err != nil {
			t.Fatalf("failed to count active credentials: %v", err)
		}
		if activeCount != 1 {
			t.Errorf("expected active count 1, got %d", activeCount)
		}

		// Revoke credential: use a future timestamp to satisfy the DB constraint
		// ck_credentials_revoked (revoked_at >= created_at), because created_at is
		// set server-side by PostgreSQL DEFAULT now() which may be ahead of Go's now.
		revokeAt := now.Add(time.Second)
		revoked, exists, err := credRepo.Revoke(ctx, db, merchantID, credID, revokeAt)
		if err != nil || !revoked || !exists {
			t.Fatalf("failed to revoke credential: revoked=%v exists=%v err=%v", revoked, exists, err)
		}

		activeCountAfter, _ := credRepo.CountActive(ctx, db, merchantID, revokeAt)
		if activeCountAfter != 0 {
			t.Errorf("expected active count 0 after revocation, got %d", activeCountAfter)
		}
	})

	t.Run("4. Webhook Registration and View merchant_webhook_targets Integration", func(t *testing.T) {
		webhookID := "018f4a7c-4003-7000-8000-000000000043"
		url := "https://example.com/webhooks/swiftdrop"
		ciphertext := []byte("aes256_gcm_ciphertext_with_nonce_tag_placeholder_32b!")
		events := []string{"parcel.booked.v1", "parcel.delivered.v1"}

		hookRecord := merchantsapp.WebhookRecord{
			ID:         webhookID,
			MerchantID: merchantID,
			URL:        url,
			Ciphertext: ciphertext,
			KeyVersion: 1,
			Last4:      "1a2b",
			Events:     events,
			CreatedBy:  ownerUserID,
		}

		if err := webhookRepo.Insert(ctx, db, hookRecord); err != nil {
			t.Fatalf("failed to insert webhook: %v", err)
		}

		hooks, err := webhookRepo.List(ctx, db, merchantID)
		if err != nil {
			t.Fatalf("failed to list webhooks: %v", err)
		}
		if len(hooks) != 1 {
			t.Fatalf("expected 1 webhook, got %d", len(hooks))
		}
		if hooks[0].URL != url {
			t.Errorf("expected url %s, got %s", url, hooks[0].URL)
		}

		// Query the canonical SQL view merchant_webhook_targets (Member 8 integration).
		// View columns (from migration 20261101000400): endpoint_id, merchant_id, url,
		// secret_ciphertext, secret_key_version, subscribed_event_types, updated_at.
		// Note: the view only exposes rows where merchant.status = 'APPROVED', so the
		// merchant inserted in subtest 2 (now APPROVED) will appear here.
		queryView := `
			SELECT endpoint_id, merchant_id, url
			FROM merchant_webhook_targets
			WHERE endpoint_id = $1;
		`
		var epID, mID, targetURL string
		err = db.QueryRowContext(ctx, queryView, webhookID).Scan(&epID, &mID, &targetURL)
		if err != nil {
			t.Fatalf("failed to query merchant_webhook_targets view: %v", err)
		}
		if epID != webhookID || mID != merchantID {
			t.Errorf("view returned unexpected IDs: endpoint=%s merchant=%s", epID, mID)
		}
		if targetURL != url {
			t.Errorf("expected view URL %s, got %s", url, targetURL)
		}
	})

	t.Run("5. Centralized Audit Log Verification", func(t *testing.T) {
		auditEntry := merchantsapp.AuditEntry{
			ActorID:   "018f4a7c-1001-7000-8000-000000000001",
			ActorRole: "ADMIN",
			Action:    "MERCHANT_STATUS_APPROVED",
			TargetID:  merchantID,
			RequestID: "req-itest-1",
			Previous:  map[string]any{"status": "PENDING"},
			New:       map[string]any{"status": "APPROVED"},
		}

		if err := auditWriter.Write(ctx, db, auditEntry); err != nil {
			t.Fatalf("failed to write audit log: %v", err)
		}

		var count int
		checkAudit := `SELECT count(*) FROM audit_logs WHERE entity_type='MERCHANT' AND entity_id=$1 AND action='MERCHANT_STATUS_APPROVED';`
		if err := db.QueryRowContext(ctx, checkAudit, merchantID).Scan(&count); err != nil {
			t.Fatalf("failed to query audit_logs: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 audit log entry, got %d", count)
		}
	})
}

// TestMerchantsLifecycle_E2E verifies the complete HTTP flow:
// Customer Application -> Admin Approval -> Merchant Profile -> API Key Generation -> Webhook -> B2B Order with Idempotency.
func TestMerchantsLifecycle_E2E(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	defer cleanup()

	tokenProv := identityinfra.NewJWTTokenProvider("test-integration-secret-key-32chars")
	hasher := merchantsinfra.Hasher{
		Peppers: map[int][]byte{1: []byte("test-pepper-at-least-32-bytes-long!")},
		Current: 1,
	}
	cipher := merchantsinfra.Cipher{
		Key:     []byte("01234567890123456789012345678901"),
		Version: 1,
	}

	appService := merchantsapp.NewService(merchantsapp.Deps{
		Tx:        &testTxRunner{db: db},
		Merchants: merchantsinfra.MerchantRepo{},
		Creds:     merchantsinfra.CredentialRepo{},
		Hooks:     merchantsinfra.WebhookRepo{},
		Audit:     merchantsinfra.AuditWriter{},
		Roles:     merchantsinfra.RoleAssigner{},
		KeyGen:    merchantsinfra.KeyGenerator{},
		Hasher:    hasher,
		Cipher:    cipher,
		Resolver:  merchantsinfra.Resolver{},
		Booking:   merchantsinfra.NewBookingStub(),
		Observer:  testNoopObserver{},
		Now:       time.Now,
	}, merchantsapp.Config{
		KeyEnvPrefix:  merchantsdomain.EnvPrefixTest,
		MaxActiveKeys: 5,
		MaxWebhooks:   10,
		AllowHTTP:     true, // allow http in test harness
	})

	handler := &merchantstransport.Handler{
		Svc:      appService,
		Verifier: &testJWTVerifier{prov: tokenProv},
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	mux := http.NewServeMux()
	handler.Register(mux)

	customerID := "018f4a7c-1005-7000-8000-000000000005"
	adminID := "018f4a7c-1001-7000-8000-000000000001"

	customerToken, err := tokenProv.GenerateAccessToken(identityapp.Principal{
		UserID: customerID,
		Email:  "customer@swiftdrop.local",
		Role:   "CUSTOMER",
	})
	if err != nil {
		t.Fatalf("failed to mint customer token: %v", err)
	}

	adminToken, err := tokenProv.GenerateAccessToken(identityapp.Principal{
		UserID: adminID,
		Email:  "admin@swiftdrop.local",
		Role:   "ADMIN",
	})
	if err != nil {
		t.Fatalf("failed to mint admin token: %v", err)
	}

	var createdMerchantID string

	// 1. Customer Submits Application (POST /api/v1/merchants/applications)
	t.Run("E2E-1: Customer submits merchant application", func(t *testing.T) {
		body := map[string]interface{}{
			"business_name": "E2E Test Store",
			"contact_email": "owner@e2estore.lk",
			"contact_phone": "+94771234599",
			"address": map[string]string{
				"line1":   "45 Galle Road",
				"city":    "Colombo",
				"country": "LK",
			},
		}
		jsonBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/merchants/applications", bytes.NewReader(jsonBytes))
		req.Header.Set("Authorization", "Bearer "+customerToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			Data struct {
				ID           string `json:"merchant_id"`
				BusinessName string `json:"business_name"`
				Status       string `json:"status"`
			} `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Data.ID == "" || resp.Data.Status != "PENDING" {
			t.Fatalf("unexpected application response: %+v", resp.Data)
		}
		createdMerchantID = resp.Data.ID
	})

	// 2. Admin Approves Merchant Application (POST /api/v1/admin/merchants/{id}/approve)
	t.Run("E2E-2: Admin approves merchant application", func(t *testing.T) {
		approveBody := map[string]interface{}{
			"reason":           "Approved business registration documents",
			"expected_version": 1,
		}
		jsonBytes, _ := json.Marshal(approveBody)

		url := fmt.Sprintf("/api/v1/admin/merchants/%s/approve", createdMerchantID)
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBytes))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK from approve, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	// Mint Merchant Token now that merchant is approved
	merchantToken, err := tokenProv.GenerateAccessToken(identityapp.Principal{
		UserID:   customerID,
		Email:    "customer@swiftdrop.local",
		Role:     "MERCHANT",
		TenantID: &createdMerchantID,
	})
	if err != nil {
		t.Fatalf("failed to mint merchant token: %v", err)
	}

	// 3. Merchant Views Own Profile (GET /api/v1/merchants/me)
	t.Run("E2E-3: Merchant retrieves own profile", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/merchants/me", nil)
		req.Header.Set("Authorization", "Bearer "+merchantToken)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK from /merchants/me, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	var createdPlaintextKey string

	// 4. Merchant Creates B2B API Key (POST /api/v1/merchants/me/api-keys)
	t.Run("E2E-4: Merchant generates API key", func(t *testing.T) {
		keyBody := map[string]interface{}{
			"label":  "Production ERP Key",
			"scopes": []string{"parcels:create", "parcels:read"},
		}
		jsonBytes, _ := json.Marshal(keyBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/merchants/me/api-keys", bytes.NewReader(jsonBytes))
		req.Header.Set("Authorization", "Bearer "+merchantToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created from key creation, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			Data struct {
				KeyID        string `json:"key_id"`
				PlaintextKey string `json:"api_key"`
				KeyPrefix    string `json:"key_prefix"`
			} `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode key response: %v", err)
		}
		if resp.Data.PlaintextKey == "" {
			t.Fatalf("expected plaintext_key to be returned on creation")
		}
		createdPlaintextKey = resp.Data.PlaintextKey
	})

	// 5. Merchant Registers Webhook (POST /api/v1/merchants/me/webhooks)
	t.Run("E2E-5: Merchant registers webhook endpoint", func(t *testing.T) {
		hookBody := map[string]interface{}{
			"url":    "http://127.0.0.1:8080/webhooks/test",
			"events": []string{"parcel.delivered.v1"},
		}
		jsonBytes, _ := json.Marshal(hookBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/merchants/me/webhooks", bytes.NewReader(jsonBytes))
		req.Header.Set("Authorization", "Bearer "+merchantToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created from webhook creation, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	// 6. Merchant Books B2B Order with Idempotency Key (POST /api/v1/merchant/orders)
	t.Run("E2E-6: B2B Order creation with Idempotency-Key", func(t *testing.T) {
		orderBody := map[string]interface{}{
			"recipient_name":  "Alice Perera",
			"recipient_phone": "+94770001122",
			"delivery_address": map[string]string{
				"line1":   "250 Kandy Road",
				"city":    "Kadawatha",
				"country": "LK",
			},
			"weight_kg": 2.5,
			"notes":     "Fragile electronics",
		}
		jsonBytes, _ := json.Marshal(orderBody)
		idempotencyKey := "018f4a7c-5001-7000-8000-000000000055"

		// First call: Should create order
		req := httptest.NewRequest(http.MethodPost, "/api/v1/merchant/orders", bytes.NewReader(jsonBytes))
		req.Header.Set("X-API-Key", createdPlaintextKey)
		req.Header.Set("Idempotency-Key", idempotencyKey)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created from B2B order, got %d: %s", rec.Code, rec.Body.String())
		}

		// Second call: Replay with same Idempotency-Key -> returns 200 OK with Idempotent-Replay: true
		reqReplay := httptest.NewRequest(http.MethodPost, "/api/v1/merchant/orders", bytes.NewReader(jsonBytes))
		reqReplay.Header.Set("X-API-Key", createdPlaintextKey)
		reqReplay.Header.Set("Idempotency-Key", idempotencyKey)
		reqReplay.Header.Set("Content-Type", "application/json")
		recReplay := httptest.NewRecorder()

		mux.ServeHTTP(recReplay, reqReplay)

		if recReplay.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created for idempotent replay, got %d: %s", recReplay.Code, recReplay.Body.String())
		}
		if recReplay.Header().Get("Idempotent-Replay") != "true" {
			t.Errorf("expected Idempotent-Replay: true header, got %s", recReplay.Header().Get("Idempotent-Replay"))
		}
	})
}
