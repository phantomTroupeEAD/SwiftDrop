package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/swiftdrop/core-api/internal/events/application"
	"github.com/swiftdrop/core-api/internal/events/swagger"
	identityapp "github.com/swiftdrop/core-api/internal/identity/application"
	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

type mockEventsRepo struct {
	outboxItems    []application.OutboxItem
	outboxStats    application.OutboxStats
	retryOutboxErr error

	deliveries    []application.UnifiedDeliveryItem
	retryDelivErr error

	merchantDelivs []application.MerchantDeliveryItem
	attempts       []application.WebhookAttemptItem
	attemptsErr    error
}

func (m *mockEventsRepo) ListOutbox(ctx context.Context, filter application.OutboxFilter, cursor httpx.Cursor, limit int) ([]application.OutboxItem, *httpx.Cursor, error) {
	return m.outboxItems, nil, nil
}

func (m *mockEventsRepo) GetOutboxStats(ctx context.Context) (*application.OutboxStats, error) {
	return &m.outboxStats, nil
}

func (m *mockEventsRepo) RetryOutbox(ctx context.Context, id string) error {
	return m.retryOutboxErr
}

func (m *mockEventsRepo) ListDeliveries(ctx context.Context, filter application.DeliveryFilter, cursor httpx.Cursor, limit int) ([]application.UnifiedDeliveryItem, *httpx.Cursor, error) {
	return m.deliveries, nil, nil
}

func (m *mockEventsRepo) RetryWebhookDelivery(ctx context.Context, id string) error {
	return m.retryDelivErr
}

func (m *mockEventsRepo) ListMerchantWebhookDeliveries(ctx context.Context, merchantID string, cursor httpx.Cursor, limit int) ([]application.MerchantDeliveryItem, *httpx.Cursor, error) {
	return m.merchantDelivs, nil, nil
}

func (m *mockEventsRepo) ListMerchantWebhookAttempts(ctx context.Context, merchantID, deliveryID string) ([]application.WebhookAttemptItem, error) {
	if m.attemptsErr != nil {
		return nil, m.attemptsErr
	}
	return m.attempts, nil
}

func TestOpenAPISpec_ValidityAndRouteParity(t *testing.T) {
	if len(OpenAPISpec) == 0 {
		t.Fatal("OpenAPISpec is empty")
	}

	var specMap map[string]interface{}
	if err := json.Unmarshal(OpenAPISpec, &specMap); err != nil {
		t.Fatalf("failed to parse OpenAPISpec as JSON: %v", err)
	}

	// Verify required OpenAPI 3.0 top-level fields
	for _, field := range []string{"openapi", "info", "paths", "components"} {
		if _, ok := specMap[field]; !ok {
			t.Errorf("missing required OpenAPI field: %s", field)
		}
	}

	pathsMap, ok := specMap["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("paths is not a JSON object")
	}

	handler := NewHandler(nil)
	routes := ExportedRoutes(handler)

	// Verify every exported route is documented in OpenAPI paths
	for _, route := range routes {
		pathItem, exists := pathsMap[route.Path]
		if !exists {
			t.Errorf("exported route %s %s is missing from openapi.json paths", route.Method, route.Path)
			continue
		}

		opMap, ok := pathItem.(map[string]interface{})
		if !ok {
			t.Errorf("path %s is not an object", route.Path)
			continue
		}

		methodKey := stringsToLower(route.Method)
		op, ok := opMap[methodKey]
		if !ok {
			t.Errorf("method %s for route %s is missing from openapi.json", route.Method, route.Path)
			continue
		}

		opObj, ok := op.(map[string]interface{})
		if !ok {
			t.Errorf("operation %s %s is not an object", route.Method, route.Path)
			continue
		}

		if _, hasResp := opObj["responses"]; !hasResp {
			t.Errorf("operation %s %s has no responses", route.Method, route.Path)
		}
	}
}

func stringsToLower(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b = append(b, c)
	}
	return string(b)
}

func TestOpenAPISpec_DocCopyMatchesEmbedded(t *testing.T) {
	// Assert docs/drafts/.../events-ops.openapi.json matches transport/openapi.json
	candidates := []string{
		filepath.Join("..", "..", "..", "docs", "drafts", "member-08-events-messaging", "swagger", "events-ops.openapi.json"),
		filepath.Join("..", "..", "..", "..", "docs", "drafts", "member-08-events-messaging", "swagger", "events-ops.openapi.json"),
	}

	var docBytes []byte
	var err error
	for _, cand := range candidates {
		docBytes, err = os.ReadFile(cand)
		if err == nil {
			break
		}
	}

	if len(docBytes) == 0 {
		t.Skip("docs copy not found relative to test runner; skipping parity check")
	}

	if !bytes.Equal(bytes.TrimSpace(OpenAPISpec), bytes.TrimSpace(docBytes)) {
		t.Errorf("events-ops.openapi.json in docs/drafts differs from core-api embedded openapi.json")
	}
}

func TestSwaggerRegistry(t *testing.T) {
	swagger.ResetForTesting()

	if err := swagger.Register("events", []byte(`{"openapi":"3.0.3"}`)); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	// Registering same name fails
	if err := swagger.Register("events", []byte(`{}`)); err == nil {
		t.Fatalf("expected duplicate registration error")
	}

	// Registering empty name fails
	if err := swagger.Register("", []byte(`{}`)); err == nil {
		t.Fatalf("expected empty name registration error")
	}

	// Second valid name succeeds
	if err := swagger.Register("parcels", []byte(`{"openapi":"3.0.3"}`)); err != nil {
		t.Fatalf("second valid register failed: %v", err)
	}

	specs := swagger.ListSpecs()
	if len(specs) != 2 {
		t.Errorf("expected 2 registered specs, got %d", len(specs))
	}
}

func TestTenantResolver_Validation(t *testing.T) {
	resolver := TenantResolver()

	t.Run("Missing principal returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler := resolver(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("Seeded non-UUID tenant returns 403 TENANT_NOT_MERCHANT_UUID", func(t *testing.T) {
		seededTenant := "mrc_01HMERCHANTTENANT00001"
		principal := &identityapp.Principal{
			UserID:   "usr-1",
			TenantID: &seededTenant,
			Role:     identitydomain.RoleMerchant,
		}

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := identitytransport.WithPrincipal(req.Context(), principal)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler := resolver(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 for non-UUID tenant, got %d", rec.Code)
		}

		var body map[string]interface{}
		_ = json.NewDecoder(rec.Body).Decode(&body)
		errObj, _ := body["error"].(map[string]interface{})
		if errObj["code"] != "TENANT_NOT_MERCHANT_UUID" {
			t.Errorf("expected TENANT_NOT_MERCHANT_UUID code, got %v", errObj["code"])
		}
	})

	t.Run("Valid UUID tenant passes through", func(t *testing.T) {
		uuidTenant := "0192e2b3-7c8a-7890-a123-000000000042"
		principal := &identityapp.Principal{
			UserID:   "usr-1",
			TenantID: &uuidTenant,
			Role:     identitydomain.RoleMerchant,
		}

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := identitytransport.WithPrincipal(req.Context(), principal)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		passedMerchantID := ""
		handler := resolver(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			passedMerchantID, _ = GetMerchantID(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK for valid UUID tenant, got %d", rec.Code)
		}
		if passedMerchantID != uuidTenant {
			t.Errorf("expected passed merchant id %s, got %s", uuidTenant, passedMerchantID)
		}
	})
}

func TestHandler_Endpoints(t *testing.T) {
	repo := &mockEventsRepo{}
	svc := application.NewEventsService(repo, nil)
	h := NewHandler(svc)

	t.Run("GET /api/v1/admin/events/outbox returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/events/outbox?limit=10", nil)
		rec := httptest.NewRecorder()
		h.HandleListOutbox(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("GET /api/v1/admin/events/outbox with limit > 100 returns 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/events/outbox?limit=500", nil)
		rec := httptest.NewRecorder()
		h.HandleListOutbox(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 for limit > 100, got %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/admin/events/outbox/{id}/retry conflict if not failed", func(t *testing.T) {
		repo.retryOutboxErr = application.ErrInvalidState
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/events/outbox/0192e2b3-7c8a-7890-a123-000000000001/retry", nil)
		req.SetPathValue("id", "0192e2b3-7c8a-7890-a123-000000000001")
		rec := httptest.NewRecorder()
		h.HandleRetryOutbox(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict, got %d", rec.Code)
		}
	})

	t.Run("GET /api/v1/merchant/webhook-deliveries/{id}/attempts returns 404 for foreign delivery", func(t *testing.T) {
		repo.attemptsErr = application.ErrNotFound
		req := httptest.NewRequest(http.MethodGet, "/api/v1/merchant/webhook-deliveries/0192e2b3-7c8a-7890-a123-000000000099/attempts", nil)
		req.SetPathValue("id", "0192e2b3-7c8a-7890-a123-000000000099")
		ctx := context.WithValue(req.Context(), merchantIDContextKey{}, "0192e2b3-7c8a-7890-a123-000000000042")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleListMerchantWebhookAttempts(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found for foreign ID, got %d", rec.Code)
		}
	})
}
