package infrastructure

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

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

