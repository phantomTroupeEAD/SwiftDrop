package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/tracking/internal/domain"
)

func TestPostgresTrackingRepository_FindParcelByTrackingNumber_Success(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	var capturedQuery string
	var capturedArgs []driver.NamedValue

	database := registerMockDB("find_parcel_success", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		capturedQuery = query
		capturedArgs = args
		cols := []string{"id", "tracking_number", "status", "sender_id", "merchant_id", "destination_city", "destination_postal_code", "destination_zone_id", "created_at", "updated_at"}
		data := [][]driver.Value{
			{"parcel-uuid-1", "SD-20261106-000001", "IN_TRANSIT", "usr_sender_1", nil, "Kandy", "20000", "ZONE_KANDY_01", now, now.Add(time.Hour)},
		}
		return &testRows{columns: cols, data: data}, nil, nil
	})
	defer database.Close()

	repo := NewPostgresTrackingRepository(database, DefaultPoolConfig())
	parcel, err := repo.FindParcelByTrackingNumber(context.Background(), "SD-20261106-000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parcel.ID != "parcel-uuid-1" {
		t.Errorf("expected id parcel-uuid-1, got %s", parcel.ID)
	}
	if parcel.TrackingNumber.String() != "SD-20261106-000001" {
		t.Errorf("expected tracking number SD-20261106-000001, got %s", parcel.TrackingNumber)
	}
	if parcel.Status != domain.StatusInTransit {
		t.Errorf("expected status IN_TRANSIT, got %s", parcel.Status)
	}
	if parcel.DestinationCity != "Kandy" {
		t.Errorf("expected city Kandy, got %s", parcel.DestinationCity)
	}
	if parcel.SenderID == nil || *parcel.SenderID != "usr_sender_1" {
		t.Errorf("expected sender_id usr_sender_1, got %v", parcel.SenderID)
	}
	if parcel.MerchantID != nil {
		t.Errorf("expected nil merchant_id, got %v", *parcel.MerchantID)
	}
	if parcel.DestinationPostalCode != "20000" {
		t.Errorf("expected postal code 20000, got %s", parcel.DestinationPostalCode)
	}
	if parcel.DestinationZoneID != "ZONE_KANDY_01" {
		t.Errorf("expected zone id ZONE_KANDY_01, got %s", parcel.DestinationZoneID)
	}

	// Verify SQL query contains required dropoff_address->>'city' extraction
	if !strings.Contains(capturedQuery, "dropoff_address->>'city'") {
		t.Errorf("expected query to contain JSONB destination city extraction, got: %s", capturedQuery)
	}
	if strings.Contains(capturedQuery, "City") {
		t.Errorf("expected query to strictly use agreed snake_case dropoff_address->>'city' without PascalCase fallback, got: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "WHERE tracking_number = $1") {
		t.Errorf("expected parameterized tracking_number lookup, got: %s", capturedQuery)
	}
	if len(capturedArgs) != 1 || capturedArgs[0].Value != "SD-20261106-000001" {
		t.Errorf("expected arg 1 to be SD-20261106-000001, got %v", capturedArgs)
	}
}

func TestPostgresTrackingRepository_FindParcelByTrackingNumber_NotFound(t *testing.T) {
	database := registerMockDB("find_parcel_not_found", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		cols := []string{"id", "tracking_number", "status", "destination_city", "created_at", "updated_at"}
		return &testRows{columns: cols, data: nil}, nil, nil
	})
	defer database.Close()

	repo := NewPostgresTrackingRepository(database, DefaultPoolConfig())
	_, err := repo.FindParcelByTrackingNumber(context.Background(), "SD-20261106-000002")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrTrackingNotFound) {
		t.Errorf("expected ErrTrackingNotFound, got %v", err)
	}
}

func TestPostgresTrackingRepository_FindParcelByTrackingNumber_DBError(t *testing.T) {
	database := registerMockDB("find_parcel_dberr", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return nil, nil, errors.New("connection refused")
	})
	defer database.Close()

	repo := NewPostgresTrackingRepository(database, DefaultPoolConfig())
	_, err := repo.FindParcelByTrackingNumber(context.Background(), "SD-20261106-000003")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrDatabaseUnavailable) {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestPostgresTrackingRepository_FindStatusHistoryByParcelID_Success(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	var capturedQuery string

	database := registerMockDB("find_history_success", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		capturedQuery = query
		cols := []string{"id", "parcel_id", "from_status", "to_status", "actor_id", "actor_type", "actor_role", "reason", "occurred_at"}
		data := [][]driver.Value{
			{"hist-1", "parcel-uuid-1", nil, "CREATED", nil, "SYSTEM", "SYSTEM", "Booking created", now},
			{"hist-2", "parcel-uuid-1", "CREATED", "ASSIGNED", "usr-2", "USER", "DISPATCHER", "Courier assigned", now.Add(10 * time.Minute)},
			{"hist-3", "parcel-uuid-1", "ASSIGNED", "PICKED_UP", "usr-3", "USER", "COURIER", "Collected", now.Add(20 * time.Minute)},
		}
		return &testRows{columns: cols, data: data}, nil, nil
	})
	defer database.Close()

	repo := NewPostgresTrackingRepository(database, DefaultPoolConfig())
	history, err := repo.FindStatusHistoryByParcelID(context.Background(), "parcel-uuid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(history) != 3 {
		t.Fatalf("expected 3 history records, got %d", len(history))
	}
	if history[0].FromStatus != nil {
		t.Errorf("expected first milestone from_status to be nil, got %v", *history[0].FromStatus)
	}
	if history[0].ToStatus != "CREATED" {
		t.Errorf("expected to_status CREATED, got %s", history[0].ToStatus)
	}
	if history[0].ActorType != "SYSTEM" || history[0].ActorID != "" {
		t.Errorf("expected system actor with empty actor_id, got type=%s id=%s", history[0].ActorType, history[0].ActorID)
	}
	if history[1].FromStatus == nil || *history[1].FromStatus != "CREATED" {
		t.Errorf("expected second milestone from_status CREATED")
	}
	if history[1].ActorType != "USER" || history[1].ActorID != "usr-2" {
		t.Errorf("expected user actor, got type=%s id=%s", history[1].ActorType, history[1].ActorID)
	}
	if history[2].ToStatus != "PICKED_UP" {
		t.Errorf("expected third milestone to_status PICKED_UP, got %s", history[2].ToStatus)
	}

	// Verify query has deterministic ordering and uses occurred_at and actor_type
	if !strings.Contains(capturedQuery, "ORDER BY occurred_at ASC, id ASC") {
		t.Errorf("expected query to contain ORDER BY occurred_at ASC, id ASC, got: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "actor_type") {
		t.Errorf("expected query to contain actor_type, got: %s", capturedQuery)
	}
}

func TestPostgresTrackingRepository_FindTrackingSummary(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	var capturedHistoryQuery string

	database := registerMockDB("find_summary_success", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		if strings.Contains(query, "FROM parcels") {
			cols := []string{"id", "tracking_number", "status", "sender_id", "merchant_id", "destination_city", "destination_postal_code", "destination_zone_id", "created_at", "updated_at"}
			data := [][]driver.Value{
				{"parcel-uuid-99", "SD-20261106-999999", "DELIVERED", nil, "mrc_merchant_1", "Colombo", "00300", "ZONE_COLOMBO_03", now, now.Add(3 * time.Hour)},
			}
			return &testRows{columns: cols, data: data}, nil, nil
		}

		if strings.Contains(query, "FROM parcel_status_history") {
			capturedHistoryQuery = query
			cols := []string{"id", "parcel_id", "from_status", "to_status", "actor_id", "actor_type", "actor_role", "reason", "occurred_at"}
			data := [][]driver.Value{
				{"h-1", "parcel-uuid-99", nil, "CREATED", nil, "SYSTEM", "SYSTEM", "Booked", now},
				{"h-2", "parcel-uuid-99", "CREATED", "DELIVERED", "actor-2", "USER", "COURIER", "Delivered", now.Add(3 * time.Hour)},
			}
			return &testRows{columns: cols, data: data}, nil, nil
		}

		return nil, nil, errors.New("unrecognized query: " + query)
	})
	defer database.Close()

	repo := NewPostgresTrackingRepository(database, DefaultPoolConfig())
	parcel, history, err := repo.FindTrackingSummary(context.Background(), "SD-20261106-999999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parcel.TrackingNumber.String() != "SD-20261106-999999" {
		t.Errorf("expected tracking number SD-20261106-999999, got %s", parcel.TrackingNumber)
	}
	if parcel.DestinationCity != "Colombo" {
		t.Errorf("expected city Colombo, got %s", parcel.DestinationCity)
	}
	if parcel.MerchantID == nil || *parcel.MerchantID != "mrc_merchant_1" {
		t.Errorf("expected merchant_id mrc_merchant_1, got %v", parcel.MerchantID)
	}
	if parcel.DestinationPostalCode != "00300" {
		t.Errorf("expected postal code 00300, got %s", parcel.DestinationPostalCode)
	}
	if parcel.DestinationZoneID != "ZONE_COLOMBO_03" {
		t.Errorf("expected zone id ZONE_COLOMBO_03, got %s", parcel.DestinationZoneID)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	if history[0].ActorType != "SYSTEM" || history[0].ActorID != "" {
		t.Errorf("expected history[0] to be SYSTEM actor, got type=%s id=%s", history[0].ActorType, history[0].ActorID)
	}

	// Verify history query in transaction uses occurred_at and NOT timestamp
	if !strings.Contains(capturedHistoryQuery, "occurred_at") {
		t.Errorf("expected history query to use occurred_at, got: %s", capturedHistoryQuery)
	}
	if !strings.Contains(capturedHistoryQuery, "ORDER BY occurred_at ASC, id ASC") {
		t.Errorf("expected history query to have ORDER BY occurred_at ASC, id ASC, got: %s", capturedHistoryQuery)
	}
	if !strings.Contains(capturedHistoryQuery, "actor_type") {
		t.Errorf("expected history query to select actor_type, got: %s", capturedHistoryQuery)
	}
	if strings.Contains(capturedHistoryQuery, "timestamp") {
		t.Errorf("history query should not contain obsolete column timestamp, got: %s", capturedHistoryQuery)
	}

	// Verify Task 2: Transaction uses RepeatableRead isolation and ReadOnly=true for consistent snapshot
	txOpts := getLastTxOpts()
	if txOpts.Isolation != driver.IsolationLevel(sql.LevelRepeatableRead) {
		t.Errorf("expected transaction isolation RepeatableRead (%v), got %v", driver.IsolationLevel(sql.LevelRepeatableRead), txOpts.Isolation)
	}
	if !txOpts.ReadOnly {
		t.Error("expected transaction to be explicitly configured ReadOnly=true")
	}
}

func TestPostgresTrackingRepository_CheckReadiness(t *testing.T) {
	t.Run("nil db returns error", func(t *testing.T) {
		repo := NewPostgresTrackingRepository(nil, DefaultPoolConfig())
		err := repo.CheckReadiness(context.Background())
		if !errors.Is(err, domain.ErrDatabaseUnavailable) {
			t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
		}
	})

	t.Run("all probes succeed", func(t *testing.T) {
		database := registerMockDB("readiness_all_ok", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			return nil, testResult{rowsAffected: 0}, nil
		})
		defer database.Close()

		repo := NewPostgresTrackingRepository(database, DefaultPoolConfig())
		if err := repo.CheckReadiness(context.Background()); err != nil {
			t.Fatalf("unexpected readiness check error: %v", err)
		}
	})

	t.Run("parcels table missing triggers error", func(t *testing.T) {
		database := registerMockDB("readiness_parcels_missing", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			if strings.Contains(query, "FROM parcels") {
				return nil, nil, errors.New(`relation "parcels" does not exist`)
			}
			return nil, testResult{rowsAffected: 0}, nil
		})
		defer database.Close()

		repo := NewPostgresTrackingRepository(database, DefaultPoolConfig())
		err := repo.CheckReadiness(context.Background())
		if err == nil || !strings.Contains(err.Error(), "parcels table") {
			t.Fatalf("expected error indicating parcels table readiness probe failed, got: %v", err)
		}
	})

	t.Run("parcel_status_history missing triggers error", func(t *testing.T) {
		database := registerMockDB("readiness_history_missing", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			if strings.Contains(query, "FROM parcel_status_history") {
				return nil, nil, errors.New(`relation "parcel_status_history" does not exist`)
			}
			return nil, testResult{rowsAffected: 0}, nil
		})
		defer database.Close()

		repo := NewPostgresTrackingRepository(database, DefaultPoolConfig())
		err := repo.CheckReadiness(context.Background())
		if err == nil || !strings.Contains(err.Error(), "parcel_status_history table") {
			t.Fatalf("expected error indicating history table readiness probe failed, got: %v", err)
		}
	})
}

func TestPostgresTrackingRepository_Ping(t *testing.T) {
	t.Run("nil db error", func(t *testing.T) {
		repo := NewPostgresTrackingRepository(nil, DefaultPoolConfig())
		err := repo.Ping(context.Background())
		if !errors.Is(err, domain.ErrDatabaseUnavailable) {
			t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
		}
	})

	t.Run("successful ping", func(t *testing.T) {
		database := registerMockDB("ping_success", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			return nil, nil, nil
		})
		defer database.Close()

		repo := NewPostgresTrackingRepository(database, DefaultPoolConfig())
		if err := repo.Ping(context.Background()); err != nil {
			t.Fatalf("unexpected ping error: %v", err)
		}
	})
}
