package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/parcels/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

const (
	queryCustomerUserID     = "018f4a7c-1005-7000-8000-000000000005"
	queryOtherCustomerID    = "018f4a7c-9999-7000-8000-000000000099"
	queryAdminUserID        = "018f4a7c-1001-7000-8000-000000000001"
	queryMerchantOwnerID    = "018f4a7c-1004-7000-8000-000000000004"
	queryMerchantID         = "0192e2b3-7c8a-7890-a123-000000000010"
	queryOtherMerchantID    = "0192e2b3-7c8a-7890-a123-000000000020"
	queryCourierUserID      = "018f4a7c-1003-7000-8000-000000000003"
	queryDispatcherUserID   = "018f4a7c-1002-7000-8000-000000000002"
	queryCustomerParcelID   = "0192e2b3-7c8a-7890-a123-000000000101"
	queryMerchantParcelID   = "0192e2b3-7c8a-7890-a123-000000000102"
	queryMissingParcelID    = "0192e2b3-7c8a-7890-a123-000000000404"
	queryHistoryEntryID1    = "0192e2b3-7c8a-7890-a123-000000000501"
	queryHistoryEntryID2    = "0192e2b3-7c8a-7890-a123-000000000502"
	queryQuoteIDPlaceholder = "0192e2b3-7c8a-7890-a123-000000000777"
)

type spyParcelRepo struct {
	*mockParcelRepo
	findCalled bool
}

func (s *spyParcelRepo) FindByID(ctx context.Context, q db.Querier, id string) (domain.Parcel, error) {
	s.findCalled = true
	return s.mockParcelRepo.FindByID(ctx, q, id)
}

type fakeQueryMerchantReader struct {
	profile MerchantProfile
	err     error
	called  bool
	lastID  string
}

func (f *fakeQueryMerchantReader) FindByID(_ context.Context, merchantID string) (MerchantProfile, error) {
	f.called = true
	f.lastID = merchantID
	return f.profile, f.err
}

func sampleOwnedParcel(t *testing.T, id string, senderID, merchantID *string, tnSuffix string) domain.Parcel {
	t.Helper()
	now := time.Date(2026, time.November, 11, 9, 0, 0, 0, time.UTC)
	tn, err := domain.FormatTrackingNumber(now, tnSuffix)
	if err != nil {
		t.Fatalf("FormatTrackingNumber failed: %v", err)
	}
	fee, _ := domain.NewMoney(47000, domain.CurrencyLKR)
	weight, _ := domain.NewWeight(2.5, 2.25, 2.5)
	dims, _ := domain.NewDimensions(30, 25, 15)
	pickup, _ := domain.NewAddress("Kasun", "+94771234567", "45 Galle Rd", "", "Colombo", "00300", "ZONE_WESTERN_01", "")
	dropoff, _ := domain.NewAddress("Nimal", "+94719876543", "12 Kandy Rd", "", "Peradeniya", "20400", "ZONE_CENTRAL_01", "")

	parcel, err := domain.ReconstituteParcel(domain.ReconstituteParcelParams{
		ID:                 id,
		TrackingNumber:     tn,
		SenderID:           senderID,
		MerchantID:         merchantID,
		RecipientName:      "Nimal Fernando",
		RecipientPhone:     "+94719876543",
		QuoteID:            queryQuoteIDPlaceholder,
		PricingRuleID:      "rule_west_central_v2",
		PricingRuleVersion: 2,
		DeliveryFee:        fee,
		OriginZoneID:       "ZONE_WESTERN_01",
		DestinationZoneID:  "ZONE_CENTRAL_01",
		ServiceLevel:       domain.ServiceLevelStandard,
		Weight:             weight,
		Dimensions:         dims,
		PickupAddress:      pickup,
		DropoffAddress:     dropoff,
		Status:             domain.StatusAssigned,
		AttemptCount:       0,
		Version:            2,
		CreatedAt:          now,
		UpdatedAt:          now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("ReconstituteParcel failed: %v", err)
	}
	return parcel
}

func sampleHistoryEntries(t *testing.T, parcelID string) []domain.StatusHistoryEntry {
	t.Helper()
	now := time.Date(2026, time.November, 11, 9, 0, 0, 0, time.UTC)
	fromCreated := domain.StatusCreated

	e1, err := domain.NewStatusHistoryEntry(
		queryHistoryEntryID1,
		parcelID,
		nil,
		domain.StatusCreated,
		strPtr(queryCustomerUserID),
		domain.ActorTypeUser,
		RoleCustomer,
		"Initial parcel booking",
		now,
		0,
	)
	if err != nil {
		t.Fatalf("entry 1 failed: %v", err)
	}

	e2, err := domain.NewStatusHistoryEntry(
		queryHistoryEntryID2,
		parcelID,
		&fromCreated,
		domain.StatusAssigned,
		strPtr(queryDispatcherUserID),
		domain.ActorTypeUser,
		RoleDispatcher,
		"Assigned to courier",
		now.Add(10*time.Minute),
		0,
	)
	if err != nil {
		t.Fatalf("entry 2 failed: %v", err)
	}

	return []domain.StatusHistoryEntry{e1, e2}
}

type queryTestEnv struct {
	parcelRepo     *spyParcelRepo
	historyRepo    *mockStatusHistoryRepo
	merchantReader *fakeQueryMerchantReader
	getParcelUC    *GetParcelUseCase
	getHistoryUC   *GetParcelHistoryUseCase
}

func newQueryTestEnv(t *testing.T) *queryTestEnv {
	t.Helper()
	baseParcelRepo := newMockParcelRepo()
	parcelRepo := &spyParcelRepo{mockParcelRepo: baseParcelRepo}
	historyRepo := newMockStatusHistoryRepo()
	merchantReader := &fakeQueryMerchantReader{
		profile: MerchantProfile{
			ID:          queryMerchantID,
			OwnerUserID: queryMerchantOwnerID,
			Status:      MerchantStatusApproved,
		},
	}

	custParcel := sampleOwnedParcel(t, queryCustomerParcelID, strPtr(queryCustomerUserID), nil, "CST001")
	merchParcel := sampleOwnedParcel(t, queryMerchantParcelID, strPtr(queryMerchantOwnerID), strPtr(queryMerchantID), "MRC001")
	baseParcelRepo.parcels[custParcel.ID] = custParcel
	baseParcelRepo.parcels[merchParcel.ID] = merchParcel

	historyRepo.entries = append(historyRepo.entries, sampleHistoryEntries(t, queryCustomerParcelID)...)
	historyRepo.entries = append(historyRepo.entries, sampleHistoryEntries(t, queryMerchantParcelID)...)

	getParcelUC, err := NewGetParcelUseCase(parcelRepo, merchantReader)
	if err != nil {
		t.Fatalf("NewGetParcelUseCase failed: %v", err)
	}
	getHistoryUC, err := NewGetParcelHistoryUseCase(parcelRepo, historyRepo, merchantReader)
	if err != nil {
		t.Fatalf("NewGetParcelHistoryUseCase failed: %v", err)
	}

	return &queryTestEnv{
		parcelRepo:     parcelRepo,
		historyRepo:    historyRepo,
		merchantReader: merchantReader,
		getParcelUC:    getParcelUC,
		getHistoryUC:   getHistoryUC,
	}
}

func TestQueryConstructors_Validation(t *testing.T) {
	repo := newMockParcelRepo()
	hist := newMockStatusHistoryRepo()
	mr := &fakeQueryMerchantReader{}

	if _, err := NewGetParcelUseCase(nil, mr); !errors.Is(err, ErrNilParcelRepository) {
		t.Fatalf("expected ErrNilParcelRepository, got %v", err)
	}
	if _, err := NewGetParcelUseCase(repo, nil); !errors.Is(err, ErrNilMerchantReader) {
		t.Fatalf("expected ErrNilMerchantReader, got %v", err)
	}
	if _, err := NewGetParcelHistoryUseCase(nil, hist, mr); !errors.Is(err, ErrNilParcelRepository) {
		t.Fatalf("expected ErrNilParcelRepository, got %v", err)
	}
	if _, err := NewGetParcelHistoryUseCase(repo, nil, mr); !errors.Is(err, ErrNilStatusHistoryRepository) {
		t.Fatalf("expected ErrNilStatusHistoryRepository, got %v", err)
	}
	if _, err := NewGetParcelHistoryUseCase(repo, hist, nil); !errors.Is(err, ErrNilMerchantReader) {
		t.Fatalf("expected ErrNilMerchantReader, got %v", err)
	}
}

func TestGetParcelAndHistory_SuccessByRole(t *testing.T) {
	t.Run("CUSTOMER accesses own parcel and history", func(t *testing.T) {
		env := newQueryTestEnv(t)
		caller := CallerContext{UserID: queryCustomerUserID, Role: RoleCustomer}

		parcel, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{
			ParcelID: queryCustomerParcelID,
			Caller:   caller,
		})
		if err != nil {
			t.Fatalf("GetParcel unexpected error: %v", err)
		}
		if parcel.ID != queryCustomerParcelID {
			t.Fatalf("expected parcel %s, got %s", queryCustomerParcelID, parcel.ID)
		}

		res, err := env.getHistoryUC.Execute(context.Background(), GetParcelHistoryQuery{
			ParcelID: queryCustomerParcelID,
			Caller:   caller,
		})
		if err != nil {
			t.Fatalf("GetParcelHistory unexpected error: %v", err)
		}
		if res.Parcel.ID != queryCustomerParcelID || len(res.Entries) != 2 {
			t.Fatalf("expected parcel + 2 history entries, got parcel=%s count=%d", res.Parcel.ID, len(res.Entries))
		}
		if env.merchantReader.called {
			t.Error("merchantReader must not be called for CUSTOMER access")
		}
	})

	t.Run("approved MERCHANT accesses own tenant parcel and history", func(t *testing.T) {
		env := newQueryTestEnv(t)
		caller := CallerContext{
			UserID:   queryMerchantOwnerID,
			Role:     RoleMerchant,
			TenantID: strPtr(queryMerchantID),
		}

		parcel, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{
			ParcelID: queryMerchantParcelID,
			Caller:   caller,
		})
		if err != nil {
			t.Fatalf("GetParcel unexpected error: %v", err)
		}
		if parcel.MerchantID == nil || *parcel.MerchantID != queryMerchantID {
			t.Fatalf("expected merchant_id %s, got %v", queryMerchantID, parcel.MerchantID)
		}

		res, err := env.getHistoryUC.Execute(context.Background(), GetParcelHistoryQuery{
			ParcelID: queryMerchantParcelID,
			Caller:   caller,
		})
		if err != nil {
			t.Fatalf("GetParcelHistory unexpected error: %v", err)
		}
		if len(res.Entries) != 2 || !env.merchantReader.called || env.merchantReader.lastID != queryMerchantID {
			t.Fatalf("expected verified merchant lookup and 2 history entries, got len=%d called=%v", len(res.Entries), env.merchantReader.called)
		}
	})

	t.Run("ADMIN accesses both customer and merchant parcels and history", func(t *testing.T) {
		env := newQueryTestEnv(t)
		caller := CallerContext{UserID: queryAdminUserID, Role: RoleAdmin}

		for _, pid := range []string{queryCustomerParcelID, queryMerchantParcelID} {
			p, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{ParcelID: pid, Caller: caller})
			if err != nil {
				t.Fatalf("GetParcel(%s) failed: %v", pid, err)
			}
			if p.ID != pid {
				t.Errorf("expected parcel %s, got %s", pid, p.ID)
			}

			hist, err := env.getHistoryUC.Execute(context.Background(), GetParcelHistoryQuery{ParcelID: pid, Caller: caller})
			if err != nil {
				t.Fatalf("GetParcelHistory(%s) failed: %v", pid, err)
			}
			if len(hist.Entries) != 2 {
				t.Errorf("expected 2 entries for %s, got %d", pid, len(hist.Entries))
			}
		}
		if env.merchantReader.called {
			t.Error("merchantReader must not be called for ADMIN")
		}
	})
}

func TestGetParcelAndHistory_AuthorizationAndIDOR(t *testing.T) {
	t.Run("CUSTOMER cannot access another customer's or merchant-owned parcel", func(t *testing.T) {
		for _, targetID := range []string{queryCustomerParcelID, queryMerchantParcelID} {
			env := newQueryTestEnv(t)
			caller := CallerContext{UserID: queryOtherCustomerID, Role: RoleCustomer}

			_, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{ParcelID: targetID, Caller: caller})
			if !errors.Is(err, ErrForbiddenParcelAccess) {
				t.Fatalf("GetParcel(%s): expected ErrForbiddenParcelAccess, got %v", targetID, err)
			}

			_, err = env.getHistoryUC.Execute(context.Background(), GetParcelHistoryQuery{ParcelID: targetID, Caller: caller})
			if !errors.Is(err, ErrForbiddenParcelAccess) {
				t.Fatalf("GetParcelHistory(%s): expected ErrForbiddenParcelAccess, got %v", targetID, err)
			}
			if env.historyRepo.findByParcelSeen {
				t.Fatalf("history repository must NEVER be queried when caller fails authorization")
			}
		}
	})

	t.Run("approved MERCHANT cannot access another merchant's or customer-only parcel", func(t *testing.T) {
		otherMerchantParcel := sampleOwnedParcel(t, "0192e2b3-7c8a-7890-a123-000000000103", strPtr(queryOtherCustomerID), strPtr(queryOtherMerchantID), "0TH001")

		for _, targetID := range []string{queryCustomerParcelID, otherMerchantParcel.ID} {
			env := newQueryTestEnv(t)
			env.parcelRepo.parcels[otherMerchantParcel.ID] = otherMerchantParcel
			caller := CallerContext{
				UserID:   queryMerchantOwnerID,
				Role:     RoleMerchant,
				TenantID: strPtr(queryMerchantID),
			}

			_, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{ParcelID: targetID, Caller: caller})
			if !errors.Is(err, ErrForbiddenParcelAccess) {
				t.Fatalf("GetParcel(%s): expected ErrForbiddenParcelAccess, got %v", targetID, err)
			}

			_, err = env.getHistoryUC.Execute(context.Background(), GetParcelHistoryQuery{ParcelID: targetID, Caller: caller})
			if !errors.Is(err, ErrForbiddenParcelAccess) {
				t.Fatalf("GetParcelHistory(%s): expected ErrForbiddenParcelAccess, got %v", targetID, err)
			}
			if env.historyRepo.findByParcelSeen {
				t.Fatalf("history repository must NEVER be queried when merchant does not own parcel")
			}
		}
	})

	t.Run("MERCHANT missing tenant_id is rejected before DB lookup", func(t *testing.T) {
		for _, tenant := range []*string{nil, strPtr(""), strPtr("   ")} {
			env := newQueryTestEnv(t)
			caller := CallerContext{UserID: queryMerchantOwnerID, Role: RoleMerchant, TenantID: tenant}

			_, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{ParcelID: queryMerchantParcelID, Caller: caller})
			if !errors.Is(err, ErrForbiddenParcelAccess) || !errors.Is(err, ErrMissingTenantID) {
				t.Fatalf("expected ErrForbiddenParcelAccess + ErrMissingTenantID, got %v", err)
			}

			_, err = env.getHistoryUC.Execute(context.Background(), GetParcelHistoryQuery{ParcelID: queryMerchantParcelID, Caller: caller})
			if !errors.Is(err, ErrMissingTenantID) {
				t.Fatalf("expected ErrMissingTenantID on history, got %v", err)
			}
			if env.merchantReader.called || env.parcelRepo.findCalled || env.historyRepo.findByParcelSeen {
				t.Error("no repos should be queried when tenant_id is missing")
			}
		}
	})

	t.Run("MERCHANT non-UUID tenant_id is rejected before DB lookup", func(t *testing.T) {
		env := newQueryTestEnv(t)
		caller := CallerContext{
			UserID:   queryMerchantOwnerID,
			Role:     RoleMerchant,
			TenantID: strPtr("mrc_01HMERCHANTTENANT00001"),
		}

		_, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{ParcelID: queryMerchantParcelID, Caller: caller})
		if !errors.Is(err, ErrForbiddenParcelAccess) || !errors.Is(err, ErrInvalidTenantID) {
			t.Fatalf("expected ErrInvalidTenantID, got %v", err)
		}
		if env.merchantReader.called || env.parcelRepo.findCalled || env.historyRepo.findByParcelSeen {
			t.Error("no repos should be queried for non-UUID tenant_id")
		}
	})

	t.Run("MERCHANT not found is rejected before parcel lookup", func(t *testing.T) {
		env := newQueryTestEnv(t)
		env.merchantReader.err = ErrMerchantNotFound
		caller := CallerContext{
			UserID:   queryMerchantOwnerID,
			Role:     RoleMerchant,
			TenantID: strPtr(queryMerchantID),
		}

		_, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{ParcelID: queryMerchantParcelID, Caller: caller})
		if !errors.Is(err, ErrForbiddenParcelAccess) || !errors.Is(err, ErrMerchantNotFound) {
			t.Fatalf("expected ErrMerchantNotFound, got %v", err)
		}
		if env.parcelRepo.findCalled || env.historyRepo.findByParcelSeen {
			t.Error("parcelRepo must not be queried when merchant record does not exist")
		}
	})

	t.Run("MERCHANT owner mismatch is rejected before parcel lookup", func(t *testing.T) {
		env := newQueryTestEnv(t)
		env.merchantReader.profile.OwnerUserID = queryOtherCustomerID
		caller := CallerContext{
			UserID:   queryMerchantOwnerID,
			Role:     RoleMerchant,
			TenantID: strPtr(queryMerchantID),
		}

		_, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{ParcelID: queryMerchantParcelID, Caller: caller})
		if !errors.Is(err, ErrForbiddenParcelAccess) || !errors.Is(err, ErrMerchantOwnerMismatch) {
			t.Fatalf("expected ErrMerchantOwnerMismatch, got %v", err)
		}
		if env.parcelRepo.findCalled || env.historyRepo.findByParcelSeen {
			t.Error("parcelRepo must not be queried on merchant owner mismatch")
		}
	})

	t.Run("SUSPENDED and other non-approved MERCHANTS are rejected on both parcel and history endpoints", func(t *testing.T) {
		for _, status := range []string{"PENDING", "REJECTED", "SUSPENDED", "DISABLED"} {
			t.Run(status, func(t *testing.T) {
				env := newQueryTestEnv(t)
				env.merchantReader.profile.Status = status
				caller := CallerContext{
					UserID:   queryMerchantOwnerID,
					Role:     RoleMerchant,
					TenantID: strPtr(queryMerchantID),
				}

				_, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{ParcelID: queryMerchantParcelID, Caller: caller})
				if !errors.Is(err, ErrForbiddenParcelAccess) || !errors.Is(err, ErrMerchantNotApproved) {
					t.Fatalf("GetParcel(%s): expected ErrMerchantNotApproved, got %v", status, err)
				}

				_, err = env.getHistoryUC.Execute(context.Background(), GetParcelHistoryQuery{ParcelID: queryMerchantParcelID, Caller: caller})
				if !errors.Is(err, ErrForbiddenParcelAccess) || !errors.Is(err, ErrMerchantNotApproved) {
					t.Fatalf("GetParcelHistory(%s): expected ErrMerchantNotApproved, got %v", status, err)
				}

				if env.parcelRepo.findCalled || env.historyRepo.findByParcelSeen {
					t.Errorf("non-approved merchant (%s) must be rejected before parcel lookup", status)
				}
			})
		}
	})

	t.Run("COURIER and DISPATCHER are rejected with ErrForbiddenParcelAccess", func(t *testing.T) {
		for _, role := range []struct {
			name string
			role string
			id   string
		}{
			{name: "COURIER", role: RoleCourier, id: queryCourierUserID},
			{name: "DISPATCHER", role: RoleDispatcher, id: queryDispatcherUserID},
			{name: "UNKNOWN", role: "GUEST", id: queryCustomerUserID},
		} {
			t.Run(role.name, func(t *testing.T) {
				env := newQueryTestEnv(t)
				caller := CallerContext{UserID: role.id, Role: role.role}

				_, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{ParcelID: queryCustomerParcelID, Caller: caller})
				if !errors.Is(err, ErrForbiddenParcelAccess) {
					t.Fatalf("expected ErrForbiddenParcelAccess for %s, got %v", role.role, err)
				}

				_, err = env.getHistoryUC.Execute(context.Background(), GetParcelHistoryQuery{ParcelID: queryMissingParcelID, Caller: caller})
				if !errors.Is(err, ErrForbiddenParcelAccess) {
					t.Fatalf("expected ErrForbiddenParcelAccess even for missing parcel as %s, got %v", role.role, err)
				}

				if env.parcelRepo.findCalled || env.historyRepo.findByParcelSeen {
					t.Errorf("repos must not be queried for forbidden role %s", role.role)
				}
			})
		}
	})

	t.Run("non-UUID caller UserID is rejected", func(t *testing.T) {
		env := newQueryTestEnv(t)
		caller := CallerContext{UserID: "usr_01HCUSTOMER00000000000005", Role: RoleCustomer}

		_, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{ParcelID: queryCustomerParcelID, Caller: caller})
		if !errors.Is(err, ErrForbiddenParcelAccess) || !errors.Is(err, ErrInvalidCallerID) {
			t.Fatalf("expected ErrInvalidCallerID, got %v", err)
		}
	})
}

func TestGetParcelAndHistory_NotFoundValidationAndFailures(t *testing.T) {
	t.Run("invalid non-UUID parcel ID returns ErrInvalidParcelID without hitting repository", func(t *testing.T) {
		env := newQueryTestEnv(t)
		caller := CallerContext{UserID: queryCustomerUserID, Role: RoleCustomer}

		for _, badID := range []string{"", "not-a-uuid", "123"} {
			_, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{ParcelID: badID, Caller: caller})
			if !errors.Is(err, domain.ErrInvalidParcelID) {
				t.Fatalf("expected ErrInvalidParcelID for %q, got %v", badID, err)
			}
			_, err = env.getHistoryUC.Execute(context.Background(), GetParcelHistoryQuery{ParcelID: badID, Caller: caller})
			if !errors.Is(err, domain.ErrInvalidParcelID) {
				t.Fatalf("expected ErrInvalidParcelID on history for %q, got %v", badID, err)
			}
		}
		if env.parcelRepo.findCalled || env.historyRepo.findByParcelSeen {
			t.Error("repositories must not be queried for non-UUID parcel IDs")
		}
	})

	t.Run("nonexistent valid parcel UUID returns ErrParcelNotFound without reading history", func(t *testing.T) {
		env := newQueryTestEnv(t)
		caller := CallerContext{UserID: queryAdminUserID, Role: RoleAdmin}

		_, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{ParcelID: queryMissingParcelID, Caller: caller})
		if !errors.Is(err, ErrParcelNotFound) {
			t.Fatalf("expected ErrParcelNotFound, got %v", err)
		}

		// Also simulate sql.ErrNoRows directly from a repository implementation.
		env.parcelRepo.findErr = sql.ErrNoRows
		_, err = env.getHistoryUC.Execute(context.Background(), GetParcelHistoryQuery{ParcelID: queryMissingParcelID, Caller: caller})
		if !errors.Is(err, ErrParcelNotFound) {
			t.Fatalf("expected ErrParcelNotFound on sql.ErrNoRows, got %v", err)
		}
		if env.historyRepo.findByParcelSeen {
			t.Error("history repo must not be called for nonexistent parcel")
		}
	})

	t.Run("propagates repository and merchant reader failures", func(t *testing.T) {
		env := newQueryTestEnv(t)
		env.merchantReader.err = errors.New("merchant db down")
		_, err := env.getParcelUC.Execute(context.Background(), GetParcelQuery{
			ParcelID: queryMerchantParcelID,
			Caller: CallerContext{
				UserID:   queryMerchantOwnerID,
				Role:     RoleMerchant,
				TenantID: strPtr(queryMerchantID),
			},
		})
		if err == nil || errors.Is(err, ErrForbiddenParcelAccess) {
			t.Fatalf("expected infra error propagation, got %v", err)
		}

		env = newQueryTestEnv(t)
		env.parcelRepo.findErr = errors.New("parcel db down")
		_, err = env.getParcelUC.Execute(context.Background(), GetParcelQuery{
			ParcelID: queryCustomerParcelID,
			Caller:   CallerContext{UserID: queryCustomerUserID, Role: RoleCustomer},
		})
		if err == nil || errors.Is(err, ErrParcelNotFound) {
			t.Fatalf("expected parcel repo error propagation, got %v", err)
		}

		env = newQueryTestEnv(t)
		env.historyRepo.findByParcelErr = errors.New("history db down")
		_, err = env.getHistoryUC.Execute(context.Background(), GetParcelHistoryQuery{
			ParcelID: queryCustomerParcelID,
			Caller:   CallerContext{UserID: queryCustomerUserID, Role: RoleCustomer},
		})
		if err == nil {
			t.Fatal("expected history repo error propagation")
		}
	})
}

func TestResolveParcelCallerForBooking(t *testing.T) {
	t.Run("resolves CUSTOMER and ADMIN self-owned booking ownership", func(t *testing.T) {
		mr := &fakeQueryMerchantReader{}
		for _, tc := range []struct {
			role   string
			userID string
		}{
			{role: RoleCustomer, userID: queryCustomerUserID},
			{role: RoleAdmin, userID: queryAdminUserID},
		} {
			got, err := ResolveParcelCallerForBooking(context.Background(), CallerContext{
				UserID:   tc.userID,
				Role:     tc.role,
				TenantID: strPtr(queryMerchantID),
			}, mr)
			if err != nil {
				t.Fatalf("role %s failed: %v", tc.role, err)
			}
			if got.SenderID == nil || *got.SenderID != tc.userID || got.MerchantID != nil || got.ActorType != domain.ActorTypeUser {
				t.Fatalf("unexpected resolved booking caller for %s: %+v", tc.role, got)
			}
		}
		if mr.called {
			t.Error("merchantReader should not be called for CUSTOMER/ADMIN")
		}
	})

	t.Run("resolves approved MERCHANT booking ownership with both SenderID and MerchantID", func(t *testing.T) {
		mr := &fakeQueryMerchantReader{
			profile: MerchantProfile{
				ID:          queryMerchantID,
				OwnerUserID: queryMerchantOwnerID,
				Status:      MerchantStatusApproved,
			},
		}
		got, err := ResolveParcelCallerForBooking(context.Background(), CallerContext{
			UserID:   queryMerchantOwnerID,
			Role:     RoleMerchant,
			TenantID: strPtr(queryMerchantID),
		}, mr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.SenderID == nil || *got.SenderID != queryMerchantOwnerID ||
			got.MerchantID == nil || *got.MerchantID != queryMerchantID ||
			got.ActorType != domain.ActorTypeUser {
			t.Fatalf("unexpected merchant booking resolution: %+v", got)
		}
	})

	t.Run("rejects unapproved merchant and disallowed roles on booking", func(t *testing.T) {
		mr := &fakeQueryMerchantReader{
			profile: MerchantProfile{
				ID:          queryMerchantID,
				OwnerUserID: queryMerchantOwnerID,
				Status:      "SUSPENDED",
			},
		}
		if _, err := ResolveParcelCallerForBooking(context.Background(), CallerContext{
			UserID:   queryMerchantOwnerID,
			Role:     RoleMerchant,
			TenantID: strPtr(queryMerchantID),
		}, mr); !errors.Is(err, ErrMerchantNotApproved) {
			t.Fatalf("expected ErrMerchantNotApproved, got %v", err)
		}

		if _, err := ResolveParcelCallerForBooking(context.Background(), CallerContext{
			UserID: queryCourierUserID,
			Role:   RoleCourier,
		}, mr); !errors.Is(err, ErrForbiddenParcelAccess) {
			t.Fatalf("expected ErrForbiddenParcelAccess for courier, got %v", err)
		}
	})
}
