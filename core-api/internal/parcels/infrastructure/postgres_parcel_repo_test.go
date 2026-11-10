package infrastructure

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/parcels/application"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
)

func validTestParcel() domain.Parcel {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	tn, _ := domain.FormatTrackingNumber(now, "000001")
	fee, _ := domain.NewMoney(55000, domain.CurrencyLKR)
	weight, _ := domain.NewWeight(2.5, 1.8, 2.5)
	dims, _ := domain.NewDimensions(30, 20, 15)
	pickup, _ := domain.NewAddress("Kasun", "+94771234567", "45 Galle Rd", "Suite 2", "Colombo", "00300", "ZONE_WESTERN_01", "Ring front bell")
	dropoff, _ := domain.NewAddress("Sunil", "+94719876543", "12 Kandy Rd", "", "Kandy", "20000", "ZONE_CENTRAL_01", "Call upon arrival")

	p, _ := domain.NewParcel(domain.CreateParcelParams{
		ID:                 "parcel-uuid-001",
		TrackingNumber:     tn,
		SenderID:           strPtr("sender-001"),
		MerchantID:         nil,
		RecipientName:      "Sunil",
		RecipientPhone:     "+94719876543",
		QuoteID:            "quote-001",
		PricingRuleID:      "rule-001",
		PricingRuleVersion: 1,
		DeliveryFee:        fee,
		OriginZoneID:       "ZONE_WESTERN_01",
		DestinationZoneID:  "ZONE_CENTRAL_01",
		ServiceLevel:       domain.ServiceLevelStandard,
		Weight:             weight,
		Dimensions:         dims,
		PickupAddress:      pickup,
		DropoffAddress:     dropoff,
		CreatedAt:          now,
	})
	return p
}

func TestPostgresParcelRepository_Save_OwnershipAndJSONB(t *testing.T) {
	t.Run("sender-only parcel persists nullable merchant_id, attempt_count, and snake_case JSONB with instructions", func(t *testing.T) {
		parcel := validTestParcel()
		var executedQuery string
		var capturedArgs []driver.NamedValue

		database := registerMockDB("save_parcel_sender_only", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			executedQuery = query
			capturedArgs = args
			return nil, testResult{rowsAffected: 1}, nil
		})
		defer database.Close()

		repo := NewPostgresParcelRepository(database)
		if err := repo.Save(context.Background(), nil, parcel); err != nil {
			t.Fatalf("unexpected error saving parcel: %v", err)
		}

		if !strings.Contains(executedQuery, "INSERT INTO parcels") {
			t.Errorf("expected INSERT query, got: %s", executedQuery)
		}
		if !strings.Contains(executedQuery, "attempt_count") || strings.Contains(executedQuery, "delivery_attempts") {
			t.Errorf("expected SQL to use attempt_count instead of delivery_attempts, got: %s", executedQuery)
		}

		if len(capturedArgs) != 27 {
			t.Fatalf("expected 27 arguments, got %d", len(capturedArgs))
		}
		if capturedArgs[0].Value != parcel.ID {
			t.Errorf("expected arg 1 to be %s, got %v", parcel.ID, capturedArgs[0].Value)
		}
		if capturedArgs[1].Value != parcel.TrackingNumber.String() {
			t.Errorf("expected arg 2 to be %s, got %v", parcel.TrackingNumber.String(), capturedArgs[1].Value)
		}
		if capturedArgs[2].Value != "sender-001" {
			t.Errorf("expected arg 3 (sender_id) to be sender-001, got %v", capturedArgs[2].Value)
		}
		if capturedArgs[3].Value != nil {
			t.Errorf("expected arg 4 (merchant_id) to be nil, got %v", capturedArgs[3].Value)
		}

		pickupBytes, ok := capturedArgs[20].Value.([]byte)
		if !ok {
			t.Fatalf("expected pickup_address JSON bytes at arg 21, got %T", capturedArgs[20].Value)
		}
		var pickupMap map[string]any
		if err := json.Unmarshal(pickupBytes, &pickupMap); err != nil {
			t.Fatalf("unmarshal pickup JSON failed: %v", err)
		}
		if pickupMap["address_line1"] != "45 Galle Rd" || pickupMap["instructions"] != "Ring front bell" || pickupMap["zone_id"] != "ZONE_WESTERN_01" {
			t.Errorf("unexpected snake_case pickup JSONB: %v", pickupMap)
		}
	})

	t.Run("merchant-only parcel persists SQL NULL sender_id", func(t *testing.T) {
		parcel := validTestParcel()
		parcel.SenderID = nil
		parcel.MerchantID = strPtr("merchant-001")

		var capturedArgs []driver.NamedValue
		database := registerMockDB("save_parcel_merchant_only", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			capturedArgs = args
			return nil, testResult{rowsAffected: 1}, nil
		})
		defer database.Close()

		repo := NewPostgresParcelRepository(database)
		if err := repo.Save(context.Background(), nil, parcel); err != nil {
			t.Fatalf("unexpected error saving merchant-only parcel: %v", err)
		}

		if capturedArgs[2].Value != nil {
			t.Errorf("expected arg 3 (sender_id) to be nil for merchant-only booking, got %v", capturedArgs[2].Value)
		}
		if capturedArgs[3].Value != "merchant-001" {
			t.Errorf("expected arg 4 (merchant_id) to be merchant-001, got %v", capturedArgs[3].Value)
		}
	})

	t.Run("rejects parcel with no owner", func(t *testing.T) {
		parcel := validTestParcel()
		parcel.SenderID = nil
		parcel.MerchantID = nil

		database := registerMockDB("save_parcel_no_owner", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			t.Fatal("DB should not be queried when both owners are missing")
			return nil, nil, nil
		})
		defer database.Close()

		repo := NewPostgresParcelRepository(database)
		err := repo.Save(context.Background(), nil, parcel)
		if !errors.Is(err, domain.ErrInvalidOwner) {
			t.Fatalf("expected ErrInvalidOwner, got: %v", err)
		}
	})
}

func TestPostgresParcelRepository_FindByID(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	updatedAt := now.Add(2 * time.Hour)
	tn, _ := domain.FormatTrackingNumber(now, "000001")

	columns := []string{
		"id", "tracking_number", "sender_id", "merchant_id", "recipient_name", "recipient_phone",
		"quote_id", "pricing_rule_id", "pricing_rule_version", "delivery_fee_cents", "currency",
		"origin_zone_id", "destination_zone_id", "service_level",
		"actual_weight_kg", "volumetric_weight_kg", "chargeable_weight_kg",
		"length_cm", "width_cm", "height_cm", "pickup_address", "dropoff_address",
		"status", "attempt_count", "version", "created_at", "updated_at",
	}

	pickupJSON := []byte(`{"contact_name":"Kasun","contact_phone":"+94771234567","address_line1":"45 Galle Rd","address_line2":"Apt 3","city":"Colombo","postal_code":"00300","zone_id":"ZONE_WESTERN_01","instructions":"Gate code 44"}`)
	dropoffJSON := []byte(`{"contact_name":"Sunil","contact_phone":"+94719876543","address_line1":"12 Kandy Rd","address_line2":"","city":"Kandy","postal_code":"20000","zone_id":"ZONE_CENTRAL_01"}`)

	t.Run("reconstitutes merchant-only parcel with SQL NULL sender_id and instructions JSONB", func(t *testing.T) {
		var executedQuery string
		database := registerMockDB("find_parcel_merchant_only", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			executedQuery = query
			data := [][]driver.Value{{
				"parcel-uuid-001", tn.String(), nil, "mrc-001", "Sunil", "+94719876543",
				"quote-001", "rule-001", int64(2), int64(55000), "LKR",
				"ZONE_WESTERN_01", "ZONE_CENTRAL_01", "EXPRESS",
				2.5, 1.8, 2.5,
				30.0, 20.0, 15.0, pickupJSON, dropoffJSON,
				"IN_TRANSIT", int64(2), int64(4), now, updatedAt,
			}}
			return &testRows{columns: columns, data: data}, nil, nil
		})
		defer database.Close()

		repo := NewPostgresParcelRepository(database)
		parcel, err := repo.FindByID(context.Background(), nil, "parcel-uuid-001")
		if err != nil {
			t.Fatalf("unexpected FindByID error: %v", err)
		}

		if !strings.Contains(executedQuery, "attempt_count") || strings.Contains(executedQuery, "delivery_attempts") {
			t.Errorf("expected SELECT to query attempt_count instead of delivery_attempts, got: %s", executedQuery)
		}
		if parcel.SenderID != nil {
			t.Errorf("expected nil SenderID, got %v", *parcel.SenderID)
		}
		if parcel.MerchantID == nil || *parcel.MerchantID != "mrc-001" {
			t.Errorf("expected MerchantID mrc-001, got %v", parcel.MerchantID)
		}
		if parcel.AttemptCount != 2 || parcel.Version != 4 || parcel.Status != domain.StatusInTransit {
			t.Errorf("unexpected status/attempt/version: status=%s attempts=%d ver=%d", parcel.Status, parcel.AttemptCount, parcel.Version)
		}
		if parcel.PickupAddress.StreetLine1 != "45 Galle Rd" || parcel.PickupAddress.Instructions != "Gate code 44" {
			t.Errorf("unexpected pickup address round-trip: %+v", parcel.PickupAddress)
		}
		if parcel.DropoffAddress.Instructions != "" || parcel.DropoffAddress.ZoneID != "ZONE_CENTRAL_01" {
			t.Errorf("unexpected dropoff address round-trip: %+v", parcel.DropoffAddress)
		}
	})

	t.Run("reconstitutes sender-only parcel with SQL NULL merchant_id", func(t *testing.T) {
		database := registerMockDB("find_parcel_sender_only", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			data := [][]driver.Value{{
				"parcel-uuid-002", tn.String(), "usr-001", nil, "Sunil", "+94719876543",
				"quote-002", "rule-001", int64(1), int64(55000), "LKR",
				"ZONE_WESTERN_01", "ZONE_CENTRAL_01", "STANDARD",
				2.5, 1.8, 2.5,
				30.0, 20.0, 15.0, pickupJSON, dropoffJSON,
				"CREATED", int64(0), int64(1), now, now,
			}}
			return &testRows{columns: columns, data: data}, nil, nil
		})
		defer database.Close()

		repo := NewPostgresParcelRepository(database)
		parcel, err := repo.FindByID(context.Background(), nil, "parcel-uuid-002")
		if err != nil {
			t.Fatalf("unexpected FindByID error: %v", err)
		}
		if parcel.SenderID == nil || *parcel.SenderID != "usr-001" {
			t.Errorf("expected SenderID usr-001, got %v", parcel.SenderID)
		}
		if parcel.MerchantID != nil {
			t.Errorf("expected nil MerchantID, got %v", *parcel.MerchantID)
		}
	})

	t.Run("rejects corrupt row missing both owners", func(t *testing.T) {
		database := registerMockDB("find_parcel_no_owner", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			data := [][]driver.Value{{
				"parcel-uuid-003", tn.String(), nil, nil, "Sunil", "+94719876543",
				"quote-003", "rule-001", int64(1), int64(55000), "LKR",
				"ZONE_WESTERN_01", "ZONE_CENTRAL_01", "STANDARD",
				2.5, 1.8, 2.5,
				30.0, 20.0, 15.0, pickupJSON, dropoffJSON,
				"CREATED", int64(0), int64(1), now, now,
			}}
			return &testRows{columns: columns, data: data}, nil, nil
		})
		defer database.Close()

		repo := NewPostgresParcelRepository(database)
		if _, err := repo.FindByID(context.Background(), nil, "parcel-uuid-003"); !errors.Is(err, domain.ErrInvalidOwner) {
			t.Fatalf("expected ErrInvalidOwner for corrupt row missing both owners, got: %v", err)
		}
	})
}

func TestPostgresParcelRepository_ExistsTrackingNumber(t *testing.T) {
	now := time.Now().UTC()
	tn, _ := domain.FormatTrackingNumber(now, "000001")

	database := registerMockDB("exists_tn", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		columns := []string{"exists"}
		data := [][]driver.Value{{true}}
		return &testRows{columns: columns, data: data}, nil, nil
	})
	defer database.Close()

	repo := NewPostgresParcelRepository(database)
	exists, err := repo.ExistsTrackingNumber(context.Background(), nil, tn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected tracking number to exist")
	}
}

func TestPostgresParcelRepository_UpdateStatus_Success(t *testing.T) {
	parcel := validTestParcel()
	parcel.Status = domain.StatusAttemptFailed
	parcel.AttemptCount = 2
	parcel.Version = 2
	expectedVersion := 1

	var executedQuery string
	var capturedArgs []driver.NamedValue

	database := registerMockDB("update_status_success", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		executedQuery = query
		capturedArgs = args
		return nil, testResult{rowsAffected: 1}, nil
	})
	defer database.Close()

	repo := NewPostgresParcelRepository(database)
	err := repo.UpdateStatus(context.Background(), nil, parcel, expectedVersion)
	if err != nil {
		t.Fatalf("unexpected error updating status: %v", err)
	}

	if !strings.Contains(executedQuery, "UPDATE parcels") {
		t.Errorf("expected UPDATE query, got: %s", executedQuery)
	}
	if !strings.Contains(executedQuery, "SET status = $1") {
		t.Errorf("expected SET status = $1, got: %s", executedQuery)
	}
	if !strings.Contains(executedQuery, "attempt_count = $2") || strings.Contains(executedQuery, "delivery_attempts") {
		t.Errorf("expected UPDATE to set attempt_count = $2 without delivery_attempts, got: %s", executedQuery)
	}
	if !strings.Contains(executedQuery, "version = version + 1") {
		t.Errorf("expected version = version + 1, got: %s", executedQuery)
	}
	if !strings.Contains(executedQuery, "WHERE id = $4") {
		t.Errorf("expected WHERE id = $4, got: %s", executedQuery)
	}
	if !strings.Contains(executedQuery, "AND version = $5") {
		t.Errorf("expected AND version = $5, got: %s", executedQuery)
	}

	if len(capturedArgs) != 5 {
		t.Fatalf("expected 5 arguments, got %d", len(capturedArgs))
	}
	if capturedArgs[0].Value != string(domain.StatusAttemptFailed) {
		t.Errorf("expected arg 1 to be %s, got %v", domain.StatusAttemptFailed, capturedArgs[0].Value)
	}
	if val, ok := capturedArgs[1].Value.(int64); ok {
		if val != int64(parcel.AttemptCount) {
			t.Errorf("expected arg 2 to be %d, got %v", parcel.AttemptCount, capturedArgs[1].Value)
		}
	} else if capturedArgs[1].Value != parcel.AttemptCount {
		t.Errorf("expected arg 2 to be %d, got %v", parcel.AttemptCount, capturedArgs[1].Value)
	}
	if capturedArgs[3].Value != parcel.ID {
		t.Errorf("expected arg 4 to be %s, got %v", parcel.ID, capturedArgs[3].Value)
	}
	if val, ok := capturedArgs[4].Value.(int64); ok {
		if val != int64(expectedVersion) {
			t.Errorf("expected arg 5 to be %d, got %v", expectedVersion, capturedArgs[4].Value)
		}
	} else if capturedArgs[4].Value != expectedVersion {
		t.Errorf("expected arg 5 to be %d, got %v", expectedVersion, capturedArgs[4].Value)
	}
}

func TestPostgresParcelRepository_UpdateStatus_ConcurrencyConflict(t *testing.T) {
	parcel := validTestParcel()
	parcel.Status = domain.StatusAssigned

	database := registerMockDB("update_status_conflict", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return nil, testResult{rowsAffected: 0}, nil
	})
	defer database.Close()

	repo := NewPostgresParcelRepository(database)
	err := repo.UpdateStatus(context.Background(), nil, parcel, 1)
	if !errors.Is(err, application.ErrConcurrencyConflict) {
		t.Fatalf("expected ErrConcurrencyConflict, got: %v", err)
	}
}

func TestPostgresParcelRepository_UpdateStatus_DBError(t *testing.T) {
	parcel := validTestParcel()
	dbErr := errors.New("connection reset by peer")

	database := registerMockDB("update_status_dberr", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return nil, nil, dbErr
	})
	defer database.Close()

	repo := NewPostgresParcelRepository(database)
	err := repo.UpdateStatus(context.Background(), nil, parcel, 1)
	if err == nil || !strings.Contains(err.Error(), "connection reset by peer") {
		t.Fatalf("expected db error wrapped, got: %v", err)
	}
}
