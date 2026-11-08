package infrastructure

import (
	"context"
	"database/sql/driver"
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
	pickup, _ := domain.NewAddress("Kasun", "+94771234567", "45 Galle Rd", "", "Colombo", "00300", "ZONE-A")
	dropoff, _ := domain.NewAddress("Sunil", "+94719876543", "12 Kandy Rd", "", "Kandy", "20000", "ZONE-B")

	p, _ := domain.NewParcel(domain.CreateParcelParams{
		ID:                 "parcel-uuid-001",
		TrackingNumber:     tn,
		SenderID:           "sender-001",
		RecipientName:      "Sunil",
		RecipientPhone:     "+94719876543",
		QuoteID:            "quote-001",
		PricingRuleID:      "rule-001",
		PricingRuleVersion: 1,
		DeliveryFee:        fee,
		OriginZoneID:       "ZONE-A",
		DestinationZoneID:  "ZONE-B",
		ServiceLevel:       domain.ServiceLevelStandard,
		Weight:             weight,
		Dimensions:         dims,
		PickupAddress:      pickup,
		DropoffAddress:     dropoff,
		CreatedAt:          now,
	})
	return p
}

func TestPostgresParcelRepository_Save_Success(t *testing.T) {
	parcel := validTestParcel()
	var executedQuery string
	var capturedArgs []driver.NamedValue

	database := registerMockDB("save_parcel_success", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		executedQuery = query
		capturedArgs = args
		return nil, testResult{rowsAffected: 1}, nil
	})
	defer database.Close()

	repo := NewPostgresParcelRepository(database)
	err := repo.Save(context.Background(), nil, parcel)
	if err != nil {
		t.Fatalf("unexpected error saving parcel: %v", err)
	}

	if !strings.Contains(executedQuery, "INSERT INTO parcels") {
		t.Errorf("expected INSERT query, got: %s", executedQuery)
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
	parcel.Status = domain.StatusAssigned
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
	if capturedArgs[0].Value != string(domain.StatusAssigned) {
		t.Errorf("expected arg 1 to be %s, got %v", domain.StatusAssigned, capturedArgs[0].Value)
	}
	if val, ok := capturedArgs[1].Value.(int64); ok {
		if val != int64(parcel.DeliveryAttempts) {
			t.Errorf("expected arg 2 to be %d, got %v", parcel.DeliveryAttempts, capturedArgs[1].Value)
		}
	} else if capturedArgs[1].Value != parcel.DeliveryAttempts {
		t.Errorf("expected arg 2 to be %d, got %v", parcel.DeliveryAttempts, capturedArgs[1].Value)
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
