package infrastructure

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/swiftdrop/core-api/internal/parcels/application"
)

func validIdempotencyRecord(now time.Time) application.IdempotencyRecord {
	resID := "0192e2b3-7c8a-7890-a123-000000000001"
	return application.IdempotencyRecord{
		Key:               "6d54d0c4-41d0-4d8e-a111-112233445566",
		CallerScope:       "USER:018f4a7c-1005-7000-8000-000000000005",
		Operation:         application.DefaultBookingOperation,
		RequestHash:       strings.Repeat("a", 64),
		ResponseStatus:    201,
		ResponseBody:      []byte(`{"parcel":{"id":"0192e2b3-7c8a-7890-a123-000000000001"}}`),
		CreatedResourceID: &resID,
		CreatedAt:         now,
		ExpiresAt:         now.Add(application.DefaultIdempotencyTTL),
	}
}

func TestPostgresIdempotencyRepository_AcquireLock(t *testing.T) {
	t.Run("executes pg_advisory_xact_lock with caller_scope and key", func(t *testing.T) {
		var executedQuery string
		var capturedArgs []driver.NamedValue

		database := registerMockDB("idem_lock_ok", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			executedQuery = query
			capturedArgs = args
			return nil, testResult{rowsAffected: 1}, nil
		})
		defer database.Close()

		repo := NewPostgresIdempotencyRepository(database)
		err := repo.AcquireLock(context.Background(), nil, "key-1", "USER:usr-1")
		if err != nil {
			t.Fatalf("unexpected AcquireLock error: %v", err)
		}

		if !strings.Contains(executedQuery, "pg_advisory_xact_lock") {
			t.Errorf("expected pg_advisory_xact_lock query, got: %s", executedQuery)
		}
		if len(capturedArgs) != 2 || capturedArgs[0].Value != "USER:usr-1" || capturedArgs[1].Value != "key-1" {
			t.Errorf("unexpected lock args: %+v", capturedArgs)
		}
	})

	t.Run("rejects blank key or caller_scope or nil querier", func(t *testing.T) {
		repo := NewPostgresIdempotencyRepository(nil)
		if err := repo.AcquireLock(context.Background(), nil, "k", "USER:u"); err == nil {
			t.Fatal("expected error for nil querier")
		}

		database := registerMockDB("idem_lock_invalid", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			t.Fatal("DB should not be queried on invalid input")
			return nil, nil, nil
		})
		defer database.Close()

		repo = NewPostgresIdempotencyRepository(database)
		if err := repo.AcquireLock(context.Background(), nil, "  ", "USER:u"); !errors.Is(err, application.ErrInvalidIdempotencyKey) {
			t.Fatalf("expected ErrInvalidIdempotencyKey, got: %v", err)
		}
		if err := repo.AcquireLock(context.Background(), nil, "k", "  "); !errors.Is(err, application.ErrInvalidCallerScope) {
			t.Fatalf("expected ErrInvalidCallerScope, got: %v", err)
		}
	})
}

func TestPostgresIdempotencyRepository_Find(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)
	columns := []string{
		"key", "caller_scope", "operation", "request_hash",
		"response_status", "response_body", "created_resource_id",
		"created_at", "expires_at",
	}

	t.Run("returns record and true when present", func(t *testing.T) {
		var executedQuery string
		database := registerMockDB("idem_find_hit", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			executedQuery = query
			data := [][]driver.Value{{
				"key-123",
				"MERCHANT:mrc-1",
				"POST /api/v1/parcels",
				strings.Repeat("b", 64),
				int64(201),
				[]byte(`{"parcel":{"id":"parcel-1"}}`),
				"parcel-1",
				now,
				expires,
			}}
			return &testRows{columns: columns, data: data}, nil, nil
		})
		defer database.Close()

		repo := NewPostgresIdempotencyRepository(database)
		rec, found, err := repo.Find(context.Background(), nil, "key-123", "MERCHANT:mrc-1")
		if err != nil {
			t.Fatalf("unexpected Find error: %v", err)
		}
		if !found {
			t.Fatal("expected found=true")
		}
		if !strings.Contains(executedQuery, "FROM idempotency_keys") || !strings.Contains(executedQuery, "FOR UPDATE") {
			t.Errorf("expected locked lookup on idempotency_keys, got: %s", executedQuery)
		}
		if rec.Key != "key-123" || rec.CallerScope != "MERCHANT:mrc-1" || rec.ResponseStatus != 201 {
			t.Errorf("unexpected record returned: %+v", rec)
		}
		if rec.CreatedResourceID == nil || *rec.CreatedResourceID != "parcel-1" {
			t.Errorf("expected CreatedResourceID parcel-1, got %v", rec.CreatedResourceID)
		}
		if !rec.CreatedAt.Equal(now) || !rec.ExpiresAt.Equal(expires) {
			t.Errorf("unexpected timestamps: created=%v expires=%v", rec.CreatedAt, rec.ExpiresAt)
		}
	})

	t.Run("returns false and nil error on sql.ErrNoRows", func(t *testing.T) {
		database := registerMockDB("idem_find_miss", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			return &testRows{columns: columns, data: [][]driver.Value{}}, nil, nil
		})
		defer database.Close()

		repo := NewPostgresIdempotencyRepository(database)
		_, found, err := repo.Find(context.Background(), nil, "missing-key", "USER:u1")
		if err != nil {
			t.Fatalf("expected nil error on miss, got: %v", err)
		}
		if found {
			t.Fatal("expected found=false on miss")
		}
	})

	t.Run("maps NULL created_resource_id cleanly", func(t *testing.T) {
		database := registerMockDB("idem_find_null_res", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			data := [][]driver.Value{{
				"key-null",
				"USER:u1",
				"POST /api/v1/parcels",
				strings.Repeat("c", 64),
				int64(201),
				[]byte(`{"ok":true}`),
				nil,
				now,
				expires,
			}}
			return &testRows{columns: columns, data: data}, nil, nil
		})
		defer database.Close()

		repo := NewPostgresIdempotencyRepository(database)
		rec, found, err := repo.Find(context.Background(), nil, "key-null", "USER:u1")
		if err != nil || !found {
			t.Fatalf("unexpected Find result: found=%v err=%v", found, err)
		}
		if rec.CreatedResourceID != nil {
			t.Errorf("expected nil CreatedResourceID, got %v", *rec.CreatedResourceID)
		}
	})

	t.Run("returns error on database query failure", func(t *testing.T) {
		database := registerMockDB("idem_find_err", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			return nil, nil, errors.New("db connection dropped")
		})
		defer database.Close()

		repo := NewPostgresIdempotencyRepository(database)
		_, found, err := repo.Find(context.Background(), nil, "key-1", "USER:u1")
		if err == nil || found {
			t.Fatalf("expected error and found=false, got found=%v err=%v", found, err)
		}
	})
}

func TestPostgresIdempotencyRepository_Save(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

	t.Run("persists all 9 columns with conditional upsert on expired records", func(t *testing.T) {
		rec := validIdempotencyRecord(now)
		rec.RequestHash = strings.Repeat("A", 64) // verifies lowercase normalization

		var executedQuery string
		var capturedArgs []driver.NamedValue
		database := registerMockDB("idem_save_ok", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			executedQuery = query
			capturedArgs = args
			return nil, testResult{rowsAffected: 1}, nil
		})
		defer database.Close()

		repo := NewPostgresIdempotencyRepository(database)
		if err := repo.Save(context.Background(), nil, rec); err != nil {
			t.Fatalf("unexpected Save error: %v", err)
		}

		if !strings.Contains(executedQuery, "INSERT INTO idempotency_keys") {
			t.Errorf("expected INSERT INTO idempotency_keys, got: %s", executedQuery)
		}
		if !strings.Contains(executedQuery, "ON CONFLICT (key, caller_scope) DO UPDATE") ||
			!strings.Contains(executedQuery, "WHERE idempotency_keys.expires_at <= $8") {
			t.Errorf("expected expired-only ON CONFLICT guard, got: %s", executedQuery)
		}
		if len(capturedArgs) != 9 {
			t.Fatalf("expected 9 args, got %d", len(capturedArgs))
		}
		if capturedArgs[0].Value != rec.Key {
			t.Errorf("expected key %s, got %v", rec.Key, capturedArgs[0].Value)
		}
		if capturedArgs[1].Value != rec.CallerScope {
			t.Errorf("expected caller_scope %s, got %v", rec.CallerScope, capturedArgs[1].Value)
		}
		if capturedArgs[3].Value != strings.Repeat("a", 64) {
			t.Errorf("expected lowercase request_hash, got %v", capturedArgs[3].Value)
		}
		if capturedArgs[6].Value != *rec.CreatedResourceID {
			t.Errorf("expected created_resource_id %s, got %v", *rec.CreatedResourceID, capturedArgs[6].Value)
		}
	})

	t.Run("persists SQL NULL when created_resource_id is nil", func(t *testing.T) {
		rec := validIdempotencyRecord(now)
		rec.CreatedResourceID = nil

		var capturedArgs []driver.NamedValue
		database := registerMockDB("idem_save_null_res", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			capturedArgs = args
			return nil, testResult{rowsAffected: 1}, nil
		})
		defer database.Close()

		repo := NewPostgresIdempotencyRepository(database)
		if err := repo.Save(context.Background(), nil, rec); err != nil {
			t.Fatalf("unexpected Save error: %v", err)
		}
		if capturedArgs[6].Value != nil {
			t.Errorf("expected nil SQL arg for created_resource_id, got %v", capturedArgs[6].Value)
		}
	})

	t.Run("returns ErrIdempotencyConflict when zero rows affected on unexpired conflict", func(t *testing.T) {
		rec := validIdempotencyRecord(now)
		database := registerMockDB("idem_save_zero_rows", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			return nil, testResult{rowsAffected: 0}, nil
		})
		defer database.Close()

		repo := NewPostgresIdempotencyRepository(database)
		err := repo.Save(context.Background(), nil, rec)
		if !errors.Is(err, application.ErrIdempotencyConflict) {
			t.Fatalf("expected ErrIdempotencyConflict, got: %v", err)
		}
	})

	t.Run("maps pq 23505 unique violation to ErrIdempotencyConflict", func(t *testing.T) {
		rec := validIdempotencyRecord(now)
		database := registerMockDB("idem_save_23505", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			return nil, nil, &pq.Error{Code: "23505", Constraint: "idempotency_keys_pkey"}
		})
		defer database.Close()

		repo := NewPostgresIdempotencyRepository(database)
		err := repo.Save(context.Background(), nil, rec)
		if !errors.Is(err, application.ErrIdempotencyConflict) {
			t.Fatalf("expected ErrIdempotencyConflict, got: %v", err)
		}
	})

	t.Run("rejects invalid record before hitting database", func(t *testing.T) {
		database := registerMockDB("idem_save_invalid", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			t.Fatal("DB should not be queried for invalid idempotency record")
			return nil, nil, nil
		})
		defer database.Close()

		repo := NewPostgresIdempotencyRepository(database)

		badHash := validIdempotencyRecord(now)
		badHash.RequestHash = "not-a-sha256"
		if err := repo.Save(context.Background(), nil, badHash); !errors.Is(err, application.ErrInvalidRequestHash) {
			t.Errorf("expected ErrInvalidRequestHash, got: %v", err)
		}

		badExpiry := validIdempotencyRecord(now)
		badExpiry.ExpiresAt = now
		if err := repo.Save(context.Background(), nil, badExpiry); !errors.Is(err, application.ErrInvalidIdempotencyRecord) {
			t.Errorf("expected ErrInvalidIdempotencyRecord for expires_at <= created_at, got: %v", err)
		}

		badJSON := validIdempotencyRecord(now)
		badJSON.ResponseBody = []byte("{invalid-json")
		if err := repo.Save(context.Background(), nil, badJSON); !errors.Is(err, application.ErrInvalidIdempotencyRecord) {
			t.Errorf("expected ErrInvalidIdempotencyRecord for invalid json body, got: %v", err)
		}
	})

	t.Run("returns sql error cleanly on exec failure", func(t *testing.T) {
		rec := validIdempotencyRecord(now)
		database := registerMockDB("idem_save_dberr", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			return nil, nil, sql.ErrConnDone
		})
		defer database.Close()

		repo := NewPostgresIdempotencyRepository(database)
		if err := repo.Save(context.Background(), nil, rec); !errors.Is(err, sql.ErrConnDone) {
			t.Fatalf("expected wrapped sql.ErrConnDone, got: %v", err)
		}
	})
}
