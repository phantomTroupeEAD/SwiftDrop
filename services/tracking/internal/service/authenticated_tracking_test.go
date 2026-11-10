package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/swiftdrop/tracking/internal/auth"
	"github.com/swiftdrop/tracking/internal/domain"
	"github.com/swiftdrop/tracking/internal/repository"
)

var _ repository.TrackingRepository = (*mockAuthTrackingRepo)(nil)

type mockAuthTrackingRepo struct {
	findParcelFn  func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, error)
	findHistoryFn func(ctx context.Context, parcelID string) ([]domain.MilestoneRecord, error)
	findSummaryFn func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error)
}

func (m *mockAuthTrackingRepo) FindParcelByTrackingNumber(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, error) {
	if m.findParcelFn != nil {
		return m.findParcelFn(ctx, trackingNumber)
	}
	return nil, domain.ErrTrackingNotFound
}

func (m *mockAuthTrackingRepo) FindStatusHistoryByParcelID(ctx context.Context, parcelID string) ([]domain.MilestoneRecord, error) {
	if m.findHistoryFn != nil {
		return m.findHistoryFn(ctx, parcelID)
	}
	return nil, nil
}

func (m *mockAuthTrackingRepo) FindTrackingSummary(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
	if m.findSummaryFn != nil {
		return m.findSummaryFn(ctx, trackingNumber)
	}
	return nil, nil, domain.ErrTrackingNotFound
}

func (m *mockAuthTrackingRepo) CheckReadiness(_ context.Context) error { return nil }
func (m *mockAuthTrackingRepo) Ping(_ context.Context) error           { return nil }
func (m *mockAuthTrackingRepo) Close() error                           { return nil }

func strPtr(s string) *string {
	return &s
}

func TestAuthenticatedTrackingService(t *testing.T) {
	now := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	validTN, _ := domain.NewTrackingNumber("SD-20261110-A1B2C3")

	t.Run("Authorized customer can access their own parcel", func(t *testing.T) {
		repo := &mockAuthTrackingRepo{
			findSummaryFn: func(_ context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				parcel := &domain.ParcelTrackingInfo{
					ID:                    "parcel-1",
					TrackingNumber:        validTN,
					Status:                domain.StatusInTransit,
					SenderID:              strPtr("usr_customer_1"),
					DestinationCity:       "Colombo",
					DestinationPostalCode: "00300",
					DestinationZoneID:     "ZONE_COLOMBO_03",
					CreatedAt:             now.Add(-2 * time.Hour),
					UpdatedAt:             now,
				}
				history := []domain.MilestoneRecord{
					{
						ID:        "h-1",
						ParcelID:  "parcel-1",
						ToStatus:  "CREATED",
						ActorRole: "CUSTOMER",
						Timestamp: now.Add(-2 * time.Hour),
					},
					{
						ID:        "h-2",
						ParcelID:  "parcel-1",
						ToStatus:  "ATTEMPT_FAILED",
						ActorRole: "COURIER",
						Reason:    "RECIPIENT_UNAVAILABLE",
						Timestamp: now,
					},
				}
				return parcel, history, nil
			},
		}

		svc := NewAuthenticatedTrackingService(repo)
		principal := &auth.Principal{
			UserID: "usr_customer_1",
			Role:   auth.RoleCustomer,
		}

		resp, err := svc.GetAuthenticatedTracking(context.Background(), validTN.String(), principal)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.TrackingNumber != validTN.String() {
			t.Errorf("expected tracking number %s, got %s", validTN.String(), resp.TrackingNumber)
		}
		if resp.ParcelID != "parcel-1" {
			t.Errorf("expected parcel ID parcel-1, got %s", resp.ParcelID)
		}
		if resp.Destination.City != "Colombo" || resp.Destination.PostalCode != "00300" || resp.Destination.ZoneID != "ZONE_COLOMBO_03" {
			t.Errorf("unexpected destination details: %+v", resp.Destination)
		}
		if len(resp.Milestones) != 2 {
			t.Fatalf("expected 2 milestones, got %d", len(resp.Milestones))
		}
		if resp.Milestones[1].ReasonCode != "RECIPIENT_UNAVAILABLE" {
			t.Errorf("expected reason code RECIPIENT_UNAVAILABLE, got %s", resp.Milestones[1].ReasonCode)
		}
	})

	t.Run("Unauthorized customer cannot access another customer's parcel", func(t *testing.T) {
		repo := &mockAuthTrackingRepo{
			findSummaryFn: func(_ context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				parcel := &domain.ParcelTrackingInfo{
					ID:             "parcel-1",
					TrackingNumber: validTN,
					Status:         domain.StatusInTransit,
					SenderID:       strPtr("usr_customer_1"), // owner is customer 1
				}
				return parcel, nil, nil
			},
		}

		svc := NewAuthenticatedTrackingService(repo)
		principal := &auth.Principal{
			UserID: "usr_customer_2", // requester is customer 2
			Role:   auth.RoleCustomer,
		}

		_, err := svc.GetAuthenticatedTracking(context.Background(), validTN.String(), principal)
		if err == nil {
			t.Fatal("expected access denied error, got nil")
		}
		if !errors.Is(err, domain.ErrAccessDenied) {
			t.Errorf("expected ErrAccessDenied, got %v", err)
		}
	})

	t.Run("Authorized merchant can access their parcel", func(t *testing.T) {
		repo := &mockAuthTrackingRepo{
			findSummaryFn: func(_ context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				parcel := &domain.ParcelTrackingInfo{
					ID:                    "parcel-m1",
					TrackingNumber:        validTN,
					Status:                domain.StatusOutForDelivery,
					MerchantID:            strPtr("mrc_tenant_100"),
					DestinationCity:       "Kandy",
					DestinationPostalCode: "20000",
					DestinationZoneID:     "ZONE_KANDY_01",
					CreatedAt:             now,
					UpdatedAt:             now,
				}
				return parcel, nil, nil
			},
		}

		svc := NewAuthenticatedTrackingService(repo)
		principal := &auth.Principal{
			UserID:   "usr_merchant_rep",
			Role:     auth.RoleMerchant,
			TenantID: strPtr("mrc_tenant_100"),
		}

		resp, err := svc.GetAuthenticatedTracking(context.Background(), validTN.String(), principal)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.ParcelID != "parcel-m1" {
			t.Errorf("expected parcel-m1, got %s", resp.ParcelID)
		}
	})

	t.Run("Wrong merchant is denied access", func(t *testing.T) {
		repo := &mockAuthTrackingRepo{
			findSummaryFn: func(_ context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				parcel := &domain.ParcelTrackingInfo{
					ID:             "parcel-m1",
					TrackingNumber: validTN,
					MerchantID:     strPtr("mrc_tenant_100"),
				}
				return parcel, nil, nil
			},
		}

		svc := NewAuthenticatedTrackingService(repo)
		principal := &auth.Principal{
			UserID:   "usr_wrong_merchant",
			Role:     auth.RoleMerchant,
			TenantID: strPtr("mrc_other_tenant_999"), // wrong tenant
		}

		_, err := svc.GetAuthenticatedTracking(context.Background(), validTN.String(), principal)
		if err == nil {
			t.Fatal("expected access denied error, got nil")
		}
		if !errors.Is(err, domain.ErrAccessDenied) {
			t.Errorf("expected ErrAccessDenied, got %v", err)
		}
	})

	t.Run("Merchant with missing tenant_id is denied access", func(t *testing.T) {
		repo := &mockAuthTrackingRepo{
			findSummaryFn: func(_ context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				parcel := &domain.ParcelTrackingInfo{
					ID:             "parcel-m1",
					TrackingNumber: validTN,
					MerchantID:     strPtr("mrc_tenant_100"),
				}
				return parcel, nil, nil
			},
		}

		svc := NewAuthenticatedTrackingService(repo)
		principal := &auth.Principal{
			UserID:   "usr_merchant_no_tenant",
			Role:     auth.RoleMerchant,
			TenantID: nil, // no tenant ID
		}

		_, err := svc.GetAuthenticatedTracking(context.Background(), validTN.String(), principal)
		if err == nil {
			t.Fatal("expected access denied error, got nil")
		}
		if !errors.Is(err, domain.ErrAccessDenied) {
			t.Errorf("expected ErrAccessDenied, got %v", err)
		}
	})

	t.Run("Admin and Dispatcher have operational tracking access", func(t *testing.T) {
		repo := &mockAuthTrackingRepo{
			findSummaryFn: func(_ context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				parcel := &domain.ParcelTrackingInfo{
					ID:             "parcel-ops",
					TrackingNumber: validTN,
					Status:         domain.StatusDelivered,
					SenderID:       strPtr("usr_cust_x"),
					MerchantID:     nil,
					CreatedAt:      now,
				}
				return parcel, nil, nil
			},
		}

		svc := NewAuthenticatedTrackingService(repo)

		// Admin
		adminP := &auth.Principal{UserID: "usr_admin", Role: auth.RoleAdmin}
		resp, err := svc.GetAuthenticatedTracking(context.Background(), validTN.String(), adminP)
		if err != nil {
			t.Fatalf("unexpected admin error: %v", err)
		}
		if resp.ParcelID != "parcel-ops" {
			t.Errorf("expected parcel-ops, got %s", resp.ParcelID)
		}

		// Dispatcher
		dispP := &auth.Principal{UserID: "usr_dispatcher", Role: auth.RoleDispatcher}
		resp, err = svc.GetAuthenticatedTracking(context.Background(), validTN.String(), dispP)
		if err != nil {
			t.Fatalf("unexpected dispatcher error: %v", err)
		}
		if resp.ParcelID != "parcel-ops" {
			t.Errorf("expected parcel-ops, got %s", resp.ParcelID)
		}
	})

	t.Run("Courier is denied access on customer/merchant tracking endpoint", func(t *testing.T) {
		repo := &mockAuthTrackingRepo{
			findSummaryFn: func(_ context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				parcel := &domain.ParcelTrackingInfo{
					ID:             "parcel-courier-test",
					TrackingNumber: validTN,
					SenderID:       strPtr("usr_customer_1"),
				}
				return parcel, nil, nil
			},
		}

		svc := NewAuthenticatedTrackingService(repo)
		courierP := &auth.Principal{UserID: "usr_courier", Role: auth.RoleCourier}

		_, err := svc.GetAuthenticatedTracking(context.Background(), validTN.String(), courierP)
		if err == nil {
			t.Fatal("expected access denied for courier, got nil")
		}
		if !errors.Is(err, domain.ErrAccessDenied) {
			t.Errorf("expected ErrAccessDenied, got %v", err)
		}
	})

	t.Run("Unknown parcel returns ErrTrackingNotFound", func(t *testing.T) {
		repo := &mockAuthTrackingRepo{
			findSummaryFn: func(_ context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				return nil, nil, domain.ErrTrackingNotFound
			},
		}

		svc := NewAuthenticatedTrackingService(repo)
		principal := &auth.Principal{UserID: "usr_customer_1", Role: auth.RoleCustomer}

		_, err := svc.GetAuthenticatedTracking(context.Background(), validTN.String(), principal)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, domain.ErrTrackingNotFound) {
			t.Errorf("expected ErrTrackingNotFound, got %v", err)
		}
	})

	t.Run("Malformed tracking number returns ErrInvalidTrackingNumber", func(t *testing.T) {
		repo := &mockAuthTrackingRepo{}
		svc := NewAuthenticatedTrackingService(repo)
		principal := &auth.Principal{UserID: "usr_customer_1", Role: auth.RoleCustomer}

		_, err := svc.GetAuthenticatedTracking(context.Background(), "INVALID-TRACKING-NUM", principal)
		if err == nil {
			t.Fatal("expected error for malformed tracking number, got nil")
		}
		if !errors.Is(err, domain.ErrInvalidTrackingNumber) {
			t.Errorf("expected ErrInvalidTrackingNumber, got %v", err)
		}
	})

	t.Run("Database unavailable returns ErrDatabaseUnavailable", func(t *testing.T) {
		repo := &mockAuthTrackingRepo{
			findSummaryFn: func(_ context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
				return nil, nil, domain.ErrDatabaseUnavailable
			},
		}

		svc := NewAuthenticatedTrackingService(repo)
		principal := &auth.Principal{UserID: "usr_customer_1", Role: auth.RoleCustomer}

		_, err := svc.GetAuthenticatedTracking(context.Background(), validTN.String(), principal)
		if err == nil {
			t.Fatal("expected error for database unavailable, got nil")
		}
		if !errors.Is(err, domain.ErrDatabaseUnavailable) {
			t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
		}
	})

	t.Run("Unauthenticated request with nil principal returns ErrUnauthenticated", func(t *testing.T) {
		repo := &mockAuthTrackingRepo{}
		svc := NewAuthenticatedTrackingService(repo)

		_, err := svc.GetAuthenticatedTracking(context.Background(), validTN.String(), nil)
		if err == nil {
			t.Fatal("expected error for nil principal, got nil")
		}
		if !errors.Is(err, domain.ErrUnauthenticated) {
			t.Errorf("expected ErrUnauthenticated, got %v", err)
		}
	})
}

func TestSanitizeReasonCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", ""},
		{"Whitespace only", "   ", ""},
		{"Exact match RECIPIENT_UNAVAILABLE", "RECIPIENT_UNAVAILABLE", "RECIPIENT_UNAVAILABLE"},
		{"Lowercase match recipient_unavailable", "recipient_unavailable", "RECIPIENT_UNAVAILABLE"},
		{"Trimmed match with spaces", "  INVALID_ADDRESS  ", "INVALID_ADDRESS"},
		{"CUSTOMER_REQUESTED_RESCHEDULE", "CUSTOMER_REQUESTED_RESCHEDULE", "CUSTOMER_REQUESTED_RESCHEDULE"},
		{"ACCESS_RESTRICTED", "ACCESS_RESTRICTED", "ACCESS_RESTRICTED"},
		{"REFUSED_BY_RECIPIENT", "REFUSED_BY_RECIPIENT", "REFUSED_BY_RECIPIENT"},
		{"DAMAGED_IN_TRANSIT", "DAMAGED_IN_TRANSIT", "DAMAGED_IN_TRANSIT"},
		{"OTHER code", "OTHER", "OTHER"},
		{"Free-text courier remarks masked to OTHER", "Dog was barking, gate code 1234", "OTHER"},
		{"Internal warehouse note masked to OTHER", "Damaged box on shelf B4", "OTHER"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeReasonCode(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeReasonCode(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
