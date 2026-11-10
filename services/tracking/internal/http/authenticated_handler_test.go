package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/tracking/internal/auth"
	"github.com/swiftdrop/tracking/internal/domain"
	"github.com/swiftdrop/tracking/internal/dto"
	"github.com/swiftdrop/tracking/internal/repository"
	"github.com/swiftdrop/tracking/internal/service"
	"github.com/swiftdrop/tracking/internal/testutil"
)

const (
	testJWTSecret = "test-secret-32-bytes-long-for-testing!!"
	testJWTIssuer = "https://api.swiftdrop.io"
)

var _ repository.TrackingRepository = (*mockHTTPRepo)(nil)

type mockHTTPRepo struct {
	findSummaryFn func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error)
}

func (m *mockHTTPRepo) FindParcelByTrackingNumber(_ context.Context, _ string) (*domain.ParcelTrackingInfo, error) {
	return nil, nil
}
func (m *mockHTTPRepo) FindStatusHistoryByParcelID(_ context.Context, _ string) ([]domain.MilestoneRecord, error) {
	return nil, nil
}
func (m *mockHTTPRepo) FindTrackingSummary(ctx context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
	if m.findSummaryFn != nil {
		return m.findSummaryFn(ctx, tn)
	}
	return nil, nil, domain.ErrTrackingNotFound
}
func (m *mockHTTPRepo) CheckReadiness(_ context.Context) error { return nil }
func (m *mockHTTPRepo) Ping(_ context.Context) error           { return nil }
func (m *mockHTTPRepo) Close() error                           { return nil }

func strPtr(s string) *string {
	return &s
}

func setupAuthHandlerTest(repo repository.TrackingRepository, clock time.Time) (*AuthenticatedTrackingHandler, *auth.JWTVerifier) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	verifier := auth.NewJWTVerifier(testJWTSecret, testJWTIssuer)
	if !clock.IsZero() {
		verifier.SetNowFunc(func() time.Time { return clock })
	}
	svc := service.NewAuthenticatedTrackingService(repo)
	handler := NewAuthenticatedTrackingHandler(svc, verifier, logger)
	return handler, verifier
}

func TestAuthenticatedTrackingHandler_SecuritySuite(t *testing.T) {
	fixedNow := time.Date(2026, 11, 10, 16, 0, 0, 0, time.UTC)
	validTN := "SD-20261110-A1B2C3"
	parsedTN, _ := domain.NewTrackingNumber(validTN)

	defaultParcel := &domain.ParcelTrackingInfo{
		ID:                    "parcel-uuid-101",
		TrackingNumber:        parsedTN,
		Status:                domain.StatusInTransit,
		SenderID:              strPtr("usr_customer_valid"),
		MerchantID:            strPtr("mrc_merchant_valid"),
		DestinationCity:       "Colombo",
		DestinationPostalCode: "00300",
		DestinationZoneID:     "ZONE_COLOMBO_03",
		CreatedAt:             fixedNow.Add(-2 * time.Hour),
		UpdatedAt:             fixedNow,
	}

	defaultHistory := []domain.MilestoneRecord{
		{
			ID:        "h-1",
			ParcelID:  "parcel-uuid-101",
			ToStatus:  "CREATED",
			ActorRole: "CUSTOMER",
			Reason:    "",
			Timestamp: fixedNow.Add(-2 * time.Hour),
		},
		{
			ID:        "h-2",
			ParcelID:  "parcel-uuid-101",
			ToStatus:  "ATTEMPT_FAILED",
			ActorRole: "COURIER",
			Reason:    "RECIPIENT_UNAVAILABLE",
			Timestamp: fixedNow,
		},
	}

	mockRepo := &mockHTTPRepo{
		findSummaryFn: func(_ context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			if tn == validTN {
				return defaultParcel, defaultHistory, nil
			}
			return nil, nil, domain.ErrTrackingNotFound
		},
	}

	handler, _ := setupAuthHandlerTest(mockRepo, fixedNow)

	// 1. Missing Authorization header
	t.Run("Missing Authorization header returns 401 UNAUTHENTICATED", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		req.SetPathValue("tracking_number", validTN)
		rr := httptest.NewRecorder()

		handler.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
		}
		var errEnv ErrorEnvelope
		if err := json.NewDecoder(rr.Body).Decode(&errEnv); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if errEnv.Error.Code != "UNAUTHENTICATED" {
			t.Errorf("expected error code UNAUTHENTICATED, got %s", errEnv.Error.Code)
		}
	})

	// 2. Invalid Bearer syntax
	t.Run("Invalid Bearer syntax returns 401 UNAUTHENTICATED", func(t *testing.T) {
		syntaxCases := []string{
			"Basic dXNlcjpwYXNz",
			"Bearer",
			"Bearer   ",
			"Token abc.def.ghi",
			"bearer",
		}
		for _, syntax := range syntaxCases {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
			req.SetPathValue("tracking_number", validTN)
			req.Header.Set("Authorization", syntax)
			rr := httptest.NewRecorder()

			handler.GetAuthenticatedTracking(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 for header %q, got %d", syntax, rr.Code)
			}
			var errEnv ErrorEnvelope
			_ = json.NewDecoder(rr.Body).Decode(&errEnv)
			if errEnv.Error.Code != "UNAUTHENTICATED" {
				t.Errorf("expected UNAUTHENTICATED for header %q, got %s", syntax, errEnv.Error.Code)
			}
		}
	})

	// 3. Invalid Signature
	t.Run("Invalid signature returns 401 UNAUTHENTICATED", func(t *testing.T) {
		badToken, err := testutil.GenerateTestToken("wrong-signing-secret-key-32chars!", auth.JWTClaims{
			Issuer:    testJWTIssuer,
			Subject:   "usr_customer_valid",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		req.SetPathValue("tracking_number", validTN)
		req.Header.Set("Authorization", "Bearer "+badToken)
		rr := httptest.NewRecorder()

		handler.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
		}
		var errEnv ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errEnv)
		if errEnv.Error.Code != "UNAUTHENTICATED" {
			t.Errorf("expected UNAUTHENTICATED, got %s", errEnv.Error.Code)
		}
	})

	// 4. Expired Token
	t.Run("Expired token returns 401 UNAUTHENTICATED", func(t *testing.T) {
		expiredToken, err := testutil.GenerateTestToken(testJWTSecret, auth.JWTClaims{
			Issuer:    testJWTIssuer,
			Subject:   "usr_customer_valid",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(-10 * time.Minute).Unix(), // expired
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		req.SetPathValue("tracking_number", validTN)
		req.Header.Set("Authorization", "Bearer "+expiredToken)
		rr := httptest.NewRecorder()

		handler.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
		}
		var errEnv ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errEnv)
		if errEnv.Error.Code != "UNAUTHENTICATED" {
			t.Errorf("expected UNAUTHENTICATED, got %s", errEnv.Error.Code)
		}
		if !strings.Contains(errEnv.Error.Message, "expired") {
			t.Errorf("expected message to mention expired, got %s", errEnv.Error.Message)
		}
	})

	// 5. Incorrect Issuer
	t.Run("Incorrect issuer returns 401 UNAUTHENTICATED", func(t *testing.T) {
		wrongIssuerToken, err := testutil.GenerateTestToken(testJWTSecret, auth.JWTClaims{
			Issuer:    "https://unauthorized-issuer.org",
			Subject:   "usr_customer_valid",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		req.SetPathValue("tracking_number", validTN)
		req.Header.Set("Authorization", "Bearer "+wrongIssuerToken)
		rr := httptest.NewRecorder()

		handler.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
		}
		var errEnv ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errEnv)
		if errEnv.Error.Code != "UNAUTHENTICATED" {
			t.Errorf("expected UNAUTHENTICATED, got %s", errEnv.Error.Code)
		}
	})

	// 6. Unsupported Algorithm
	t.Run("Unsupported algorithm 'none' returns 401 UNAUTHENTICATED", func(t *testing.T) {
		noneToken, err := testutil.GenerateTestToken(testJWTSecret, auth.JWTClaims{
			Issuer:    testJWTIssuer,
			Subject:   "usr_customer_valid",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		}, "none")
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		req.SetPathValue("tracking_number", validTN)
		req.Header.Set("Authorization", "Bearer "+noneToken)
		rr := httptest.NewRecorder()

		handler.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
		}
		var errEnv ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errEnv)
		if errEnv.Error.Code != "UNAUTHENTICATED" {
			t.Errorf("expected UNAUTHENTICATED, got %s", errEnv.Error.Code)
		}
	})

	// 7. Revoked / Inactive Role
	t.Run("Inactive or unsupported role in token returns 401 UNAUTHENTICATED", func(t *testing.T) {
		invalidRoleToken, err := testutil.GenerateTestToken(testJWTSecret, auth.JWTClaims{
			Issuer:    testJWTIssuer,
			Subject:   "usr_suspended",
			Role:      auth.Role("SUSPENDED_USER"),
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		req.SetPathValue("tracking_number", validTN)
		req.Header.Set("Authorization", "Bearer "+invalidRoleToken)
		rr := httptest.NewRecorder()

		handler.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
		}
		var errEnv ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errEnv)
		if errEnv.Error.Code != "UNAUTHENTICATED" {
			t.Errorf("expected UNAUTHENTICATED, got %s", errEnv.Error.Code)
		}
	})

	// 8. Authorized Customer
	t.Run("Authorized customer gets 200 OK with full private details", func(t *testing.T) {
		customerToken, err := testutil.GenerateTestToken(testJWTSecret, auth.JWTClaims{
			Issuer:    testJWTIssuer,
			Subject:   "usr_customer_valid", // matches defaultParcel.SenderID
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		req.SetPathValue("tracking_number", validTN)
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rr := httptest.NewRecorder()

		handler.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var resp SingleDataEnvelope[dto.AuthenticatedTrackingResponse]
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response JSON: %v", err)
		}

		if resp.Data.TrackingNumber != validTN {
			t.Errorf("expected tracking number %s, got %s", validTN, resp.Data.TrackingNumber)
		}
		if resp.Data.ParcelID != "parcel-uuid-101" {
			t.Errorf("expected parcel_id parcel-uuid-101, got %s", resp.Data.ParcelID)
		}
		if resp.Data.Destination.City != "Colombo" || resp.Data.Destination.PostalCode != "00300" || resp.Data.Destination.ZoneID != "ZONE_COLOMBO_03" {
			t.Errorf("unexpected destination: %+v", resp.Data.Destination)
		}
		if len(resp.Data.Milestones) != 2 {
			t.Fatalf("expected 2 milestones, got %d", len(resp.Data.Milestones))
		}
		if resp.Data.Milestones[1].ActorRole != "COURIER" {
			t.Errorf("expected actor_role COURIER, got %s", resp.Data.Milestones[1].ActorRole)
		}
		if resp.Data.Milestones[1].ReasonCode != "RECIPIENT_UNAVAILABLE" {
			t.Errorf("expected reason code RECIPIENT_UNAVAILABLE, got %s", resp.Data.Milestones[1].ReasonCode)
		}
	})

	// 9. Unauthorized Customer
	t.Run("Unauthorized customer gets 403 ACCESS_DENIED", func(t *testing.T) {
		otherCustomerToken, err := testutil.GenerateTestToken(testJWTSecret, auth.JWTClaims{
			Issuer:    testJWTIssuer,
			Subject:   "usr_customer_OTHER", // does NOT match defaultParcel.SenderID
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		req.SetPathValue("tracking_number", validTN)
		req.Header.Set("Authorization", "Bearer "+otherCustomerToken)
		rr := httptest.NewRecorder()

		handler.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rr.Code)
		}
		var errEnv ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errEnv)
		if errEnv.Error.Code != "ACCESS_DENIED" {
			t.Errorf("expected ACCESS_DENIED, got %s", errEnv.Error.Code)
		}
	})

	// 10. Merchant Owner with verified credentials
	t.Run("Merchant owner with matching TenantID gets 200 OK", func(t *testing.T) {
		merchantToken, err := testutil.GenerateTestToken(testJWTSecret, auth.JWTClaims{
			Issuer:    testJWTIssuer,
			Subject:   "usr_merchant_operator",
			Role:      auth.RoleMerchant,
			TenantID:  strPtr("mrc_merchant_valid"), // matches defaultParcel.MerchantID
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		req.SetPathValue("tracking_number", validTN)
		req.Header.Set("Authorization", "Bearer "+merchantToken)
		rr := httptest.NewRecorder()

		handler.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})

	// 11. Wrong Merchant
	t.Run("Wrong merchant gets 403 ACCESS_DENIED", func(t *testing.T) {
		wrongMerchantToken, err := testutil.GenerateTestToken(testJWTSecret, auth.JWTClaims{
			Issuer:    testJWTIssuer,
			Subject:   "usr_wrong_merchant",
			Role:      auth.RoleMerchant,
			TenantID:  strPtr("mrc_other_tenant_id"), // different merchant
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		req.SetPathValue("tracking_number", validTN)
		req.Header.Set("Authorization", "Bearer "+wrongMerchantToken)
		rr := httptest.NewRecorder()

		handler.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rr.Code)
		}
		var errEnv ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errEnv)
		if errEnv.Error.Code != "ACCESS_DENIED" {
			t.Errorf("expected ACCESS_DENIED, got %s", errEnv.Error.Code)
		}
	})

	// 12. Cross-Tenant Access Attempts
	t.Run("Merchant attempting to access consumer parcel (merchant_id nil) gets 403 ACCESS_DENIED", func(t *testing.T) {
		consumerTN := "SD-20261110-B2C3D4"
		parsedConsumerTN, _ := domain.NewTrackingNumber(consumerTN)
		consumerParcel := &domain.ParcelTrackingInfo{
			ID:              "parcel-consumer-only",
			TrackingNumber:  parsedConsumerTN,
			Status:          domain.StatusCreated,
			SenderID:        strPtr("usr_customer_alone"),
			MerchantID:      nil, // B2C shipment, no merchant
			DestinationCity: "Galle",
			CreatedAt:       fixedNow,
		}

		crossRepo := &mockHTTPRepo{
			findSummaryFn: func(_ context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				if tn == consumerTN {
					return consumerParcel, nil, nil
				}
				return nil, nil, domain.ErrTrackingNotFound
			},
		}
		hCross, _ := setupAuthHandlerTest(crossRepo, fixedNow)

		merchantToken, _ := testutil.GenerateTestToken(testJWTSecret, auth.JWTClaims{
			Issuer:    testJWTIssuer,
			Subject:   "usr_merchant_attacker",
			Role:      auth.RoleMerchant,
			TenantID:  strPtr("mrc_merchant_valid"),
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+consumerTN, nil)
		req.SetPathValue("tracking_number", consumerTN)
		req.Header.Set("Authorization", "Bearer "+merchantToken)
		rr := httptest.NewRecorder()

		hCross.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden on cross-tenant access attempt, got %d", rr.Code)
		}
		var errEnv ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errEnv)
		if errEnv.Error.Code != "ACCESS_DENIED" {
			t.Errorf("expected ACCESS_DENIED, got %s", errEnv.Error.Code)
		}
	})

	// 13. Unknown Parcel
	t.Run("Unknown parcel returns 404 TRACKING_NOT_FOUND", func(t *testing.T) {
		unknownTN := "SD-20261110-999999"
		validToken, _ := testutil.GenerateTestToken(testJWTSecret, auth.JWTClaims{
			Issuer:    testJWTIssuer,
			Subject:   "usr_customer_valid",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+unknownTN, nil)
		req.SetPathValue("tracking_number", unknownTN)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rr := httptest.NewRecorder()

		handler.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rr.Code)
		}
		var errEnv ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errEnv)
		if errEnv.Error.Code != "TRACKING_NOT_FOUND" {
			t.Errorf("expected TRACKING_NOT_FOUND, got %s", errEnv.Error.Code)
		}
	})

	// 14. Malformed Tracking Number
	t.Run("Malformed tracking number returns 400 INVALID_TRACKING_NUMBER", func(t *testing.T) {
		malformedTN := "SD-INVALID-SYNTAX"
		validToken, _ := testutil.GenerateTestToken(testJWTSecret, auth.JWTClaims{
			Issuer:    testJWTIssuer,
			Subject:   "usr_customer_valid",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+malformedTN, nil)
		req.SetPathValue("tracking_number", malformedTN)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rr := httptest.NewRecorder()

		handler.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rr.Code)
		}
		var errEnv ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errEnv)
		if errEnv.Error.Code != "INVALID_TRACKING_NUMBER" {
			t.Errorf("expected INVALID_TRACKING_NUMBER, got %s", errEnv.Error.Code)
		}
	})

	// 15. Database Unavailable
	t.Run("Database unavailable returns 503 DEPENDENCY_UNAVAILABLE", func(t *testing.T) {
		dbErrRepo := &mockHTTPRepo{
			findSummaryFn: func(_ context.Context, _ string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				return nil, nil, domain.ErrDatabaseUnavailable
			},
		}
		hDBErr, _ := setupAuthHandlerTest(dbErrRepo, fixedNow)

		validToken, _ := testutil.GenerateTestToken(testJWTSecret, auth.JWTClaims{
			Issuer:    testJWTIssuer,
			Subject:   "usr_customer_valid",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		req.SetPathValue("tracking_number", validTN)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rr := httptest.NewRecorder()

		hDBErr.GetAuthenticatedTracking(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 Service Unavailable, got %d", rr.Code)
		}
		var errEnv ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errEnv)
		if errEnv.Error.Code != "DEPENDENCY_UNAVAILABLE" {
			t.Errorf("expected DEPENDENCY_UNAVAILABLE, got %s", errEnv.Error.Code)
		}
	})

	// 16. No Sensitive Fields in Errors
	t.Run("No sensitive fields, passwords, or PII exposed in error envelopes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		req.SetPathValue("tracking_number", validTN)
		// Send malformed token with secret string in it
		req.Header.Set("Authorization", "Bearer bad-token-with-passwords-or-secrets")
		rr := httptest.NewRecorder()

		handler.GetAuthenticatedTracking(rr, req)

		body := rr.Body.String()
		prohibitedStrings := []string{
			"password",
			"secret",
			"postgres://",
			"stack trace",
			"internal error at",
		}
		for _, s := range prohibitedStrings {
			if strings.Contains(strings.ToLower(body), s) {
				t.Errorf("detected sensitive information %q in error response: %s", s, body)
			}
		}
	})
}
