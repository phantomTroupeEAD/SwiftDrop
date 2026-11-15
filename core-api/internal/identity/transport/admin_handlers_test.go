package transport_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
	"github.com/swiftdrop/core-api/internal/identity/infrastructure"
	"github.com/swiftdrop/core-api/internal/identity/transport"
)

func setupAdminTestRouter() (*http.ServeMux, application.TokenProvider, *application.UserService, application.UserRepository) {
	hasher := infrastructure.NewStandardPasswordHasher()
	tokenProvider := infrastructure.NewJWTTokenProvider("test-secret-at-least-32-chars-long")
	userRepo := infrastructure.NewMockUserRepository(hasher)
	auditWriter := infrastructure.NewMockAuditWriter()

	userService := application.NewUserService(userRepo, auditWriter)
	security := transport.NewSecurityMiddleware(tokenProvider)
	adminHandler := transport.NewAdminHandler(userService)

	mux := http.NewServeMux()
	transport.RegisterAdminRoutes(mux, adminHandler, security)

	return mux, tokenProvider, userService, userRepo
}

func TestAdminHandler_ListUsers_and_UpdateRole(t *testing.T) {
	mux, tokenProvider, _, userRepo := setupAdminTestRouter()
	ctx := context.Background()

	adminToken, _ := tokenProvider.GenerateAccessToken(application.Principal{
		UserID: "usr_01HADMIN000000000000000001",
		Email:  "admin@swiftdrop.local",
		Role:   domain.RoleAdmin,
	})

	customerToken, _ := tokenProvider.GenerateAccessToken(application.Principal{
		UserID: "usr_01HCUST000000000000000005",
		Email:  "customer@swiftdrop.local",
		Role:   domain.RoleCustomer,
	})

	t.Run("GET /admin/users is denied (403) for non-admin customer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("Expected 403 Forbidden, got %d", rec.Code)
		}
	})

	t.Run("GET /admin/users returns 200 and list of users for Admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?limit=10", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", rec.Code)
		}

		var res struct {
			Data struct {
				Items []transport.UserDTO `json:"items"`
				Total int                 `json:"total"`
			} `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(res.Data.Items) == 0 {
			t.Error("Expected user items in response")
		}
	})

	t.Run("PUT /admin/users/{id}/role promotes user to MERCHANT with tenant_id", func(t *testing.T) {
		customer, _ := userRepo.FindByEmail(ctx, "customer@swiftdrop.local")
		tenantID := "mrc_approved_store_999"

		payload, _ := json.Marshal(map[string]interface{}{
			"role":      "MERCHANT",
			"tenant_id": tenantID,
		})

		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/"+customer.ID+"/role", bytes.NewBuffer(payload))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			Data transport.UserDTO `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if res.Data.Role != domain.RoleMerchant {
			t.Errorf("Expected role MERCHANT, got %s", res.Data.Role)
		}
		if res.Data.TenantID == nil || *res.Data.TenantID != tenantID {
			t.Errorf("Expected tenant %s, got %v", tenantID, res.Data.TenantID)
		}
	})

	t.Run("PUT /admin/users/{id}/status suspends and reactivates a user", func(t *testing.T) {
		customer, _ := userRepo.FindByEmail(ctx, "customer@swiftdrop.local")

		// Suspend
		suspendPayload, _ := json.Marshal(map[string]interface{}{
			"status": "SUSPENDED",
		})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/"+customer.ID+"/status", bytes.NewBuffer(suspendPayload))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK on suspend, got %d: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			Data transport.UserDTO `json:"data"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&res)
		if res.Data.Status != domain.StatusSuspended {
			t.Errorf("Expected SUSPENDED status, got %s", res.Data.Status)
		}

		// Reactivate
		activePayload, _ := json.Marshal(map[string]interface{}{
			"status": "ACTIVE",
		})
		req2 := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/"+customer.ID+"/status", bytes.NewBuffer(activePayload))
		req2.Header.Set("Authorization", "Bearer "+adminToken)
		req2.Header.Set("Content-Type", "application/json")
		rec2 := httptest.NewRecorder()
		mux.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK on reactivate, got %d: %s", rec2.Code, rec2.Body.String())
		}
	})

	t.Run("GET /admin/users with search query and status filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?q=admin&status=ACTIVE", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			Data struct {
				Items []transport.UserDTO `json:"items"`
				Total int                 `json:"total"`
			} `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if res.Data.Total != 1 || len(res.Data.Items) != 1 {
			t.Errorf("Expected 1 matched user, got total=%d len=%d", res.Data.Total, len(res.Data.Items))
		}
	})

	t.Run("GET /admin/users/{id} retrieves single user", func(t *testing.T) {
		adminUser, _ := userRepo.FindByEmail(ctx, "admin@swiftdrop.local")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/"+adminUser.ID, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			Data transport.UserDTO `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if res.Data.ID != adminUser.ID || res.Data.Email != "admin@swiftdrop.local" {
			t.Errorf("Unexpected user returned: %+v", res.Data)
		}
	})

	t.Run("GET /admin/users/{id} returns 404 for nonexistent user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/nonexistent-id", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("Expected 404 Not Found, got %d", rec.Code)
		}
	})

	t.Run("PUT /admin/users/{id}/role blocks admin self-demotion (400)", func(t *testing.T) {
		adminUser, _ := userRepo.FindByEmail(ctx, "admin@swiftdrop.local")

		payload, _ := json.Marshal(map[string]interface{}{
			"role": "CUSTOMER",
		})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/"+adminUser.ID+"/role", bytes.NewBuffer(payload))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400 Bad Request on self-demotion, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("PUT /admin/users/{id}/status blocks admin self-suspension (400)", func(t *testing.T) {
		adminUser, _ := userRepo.FindByEmail(ctx, "admin@swiftdrop.local")

		payload, _ := json.Marshal(map[string]interface{}{
			"status": "SUSPENDED",
		})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/"+adminUser.ID+"/status", bytes.NewBuffer(payload))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400 Bad Request on self-suspension, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
