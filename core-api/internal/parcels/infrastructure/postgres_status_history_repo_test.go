package infrastructure

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/parcels/domain"
)

func TestPostgresStatusHistoryRepository_Save_ActorTypes(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	fromCreated := domain.StatusCreated
	fromPickedUp := domain.StatusPickedUp

	tests := []struct {
		name        string
		id          string
		fromStatus  *domain.ParcelStatus
		toStatus    domain.ParcelStatus
		actorID     *string
		actorType   domain.ActorType
		actorRole   string
		reason      string
		wantActorDB any
	}{
		{
			name:        "USER actor persists string actor_id and actor_type USER",
			id:          "history-uuid-001",
			fromStatus:  nil,
			toStatus:    domain.StatusCreated,
			actorID:     strPtr("sender-001"),
			actorType:   domain.ActorTypeUser,
			actorRole:   "CUSTOMER",
			reason:      "Initial parcel booking",
			wantActorDB: "sender-001",
		},
		{
			name:        "SYSTEM actor persists SQL NULL actor_id and actor_type SYSTEM",
			id:          "history-uuid-002",
			fromStatus:  &fromPickedUp,
			toStatus:    domain.StatusInTransit,
			actorID:     nil,
			actorType:   domain.ActorTypeSystem,
			actorRole:   "SYSTEM",
			reason:      "Automated hub scan",
			wantActorDB: nil,
		},
		{
			name:        "MERCHANT_API actor persists merchant actor_id and actor_type MERCHANT_API",
			id:          "history-uuid-003",
			fromStatus:  &fromCreated,
			toStatus:    domain.StatusCancelled,
			actorID:     strPtr("mrc-001"),
			actorType:   domain.ActorTypeMerchantAPI,
			actorRole:   "MERCHANT",
			reason:      "Cancelled by merchant integration",
			wantActorDB: "mrc-001",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := domain.NewStatusHistoryEntry(
				tc.id,
				"parcel-uuid-001",
				tc.fromStatus,
				tc.toStatus,
				tc.actorID,
				tc.actorType,
				tc.actorRole,
				tc.reason,
				now,
				0,
			)
			if err != nil {
				t.Fatalf("unexpected entry construction error: %v", err)
			}

			var executedQuery string
			var capturedArgs []driver.NamedValue

			database := registerMockDB("save_history_"+tc.id, func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
				executedQuery = query
				capturedArgs = args
				return nil, testResult{rowsAffected: 1}, nil
			})
			defer database.Close()

			repo := NewPostgresStatusHistoryRepository(database)
			if err := repo.Save(context.Background(), nil, entry); err != nil {
				t.Fatalf("unexpected Save error: %v", err)
			}

			if !strings.Contains(executedQuery, "INSERT INTO parcel_status_history") {
				t.Errorf("expected INSERT INTO parcel_status_history, got: %s", executedQuery)
			}
			for _, col := range []string{"actor_id", "actor_type", "actor_role", "reason", "occurred_at"} {
				if !strings.Contains(executedQuery, col) {
					t.Errorf("expected column %q in INSERT SQL: %s", col, executedQuery)
				}
			}

			if len(capturedArgs) != 9 {
				t.Fatalf("expected 9 arguments, got %d", len(capturedArgs))
			}
			if capturedArgs[0].Value != entry.ID {
				t.Errorf("expected arg 1 to be %s, got %v", entry.ID, capturedArgs[0].Value)
			}
			if capturedArgs[1].Value != entry.ParcelID {
				t.Errorf("expected arg 2 to be %s, got %v", entry.ParcelID, capturedArgs[1].Value)
			}
			if capturedArgs[3].Value != string(tc.toStatus) {
				t.Errorf("expected arg 4 to be %s, got %v", tc.toStatus, capturedArgs[3].Value)
			}
			if capturedArgs[4].Value != tc.wantActorDB {
				t.Errorf("expected arg 5 (actor_id) to be %v, got %v", tc.wantActorDB, capturedArgs[4].Value)
			}
			if capturedArgs[5].Value != string(tc.actorType) {
				t.Errorf("expected arg 6 (actor_type) to be %s, got %v", tc.actorType, capturedArgs[5].Value)
			}
			if !capturedArgs[8].Value.(time.Time).Equal(now) {
				t.Errorf("expected arg 9 (occurred_at) to equal %v, got %v", now, capturedArgs[8].Value)
			}
		})
	}
}

func TestPostgresStatusHistoryRepository_Save_RejectsInvalidActorAndPropagatesDBError(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	invalidEntry := domain.StatusHistoryEntry{
		ID:         "history-bad-001",
		ParcelID:   "parcel-uuid-001",
		ToStatus:   domain.StatusCreated,
		ActorID:    strPtr("SYSTEM"),
		ActorType:  domain.ActorTypeSystem,
		ActorRole:  "SYSTEM",
		Reason:     "Bad actor",
		OccurredAt: now,
	}

	databaseNoHit := registerMockDB("save_history_invalid", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		t.Fatal("DB should not be called for invalid status history entry")
		return nil, nil, nil
	})
	defer databaseNoHit.Close()

	repo := NewPostgresStatusHistoryRepository(databaseNoHit)
	if err := repo.Save(context.Background(), nil, invalidEntry); !errors.Is(err, domain.ErrInvalidActorID) {
		t.Fatalf("expected ErrInvalidActorID, got: %v", err)
	}

	validEntry, err := domain.NewStatusHistoryEntry(
		"history-db-err",
		"parcel-uuid-001",
		nil,
		domain.StatusCreated,
		nil,
		domain.ActorTypeSystem,
		"SYSTEM",
		"System test",
		now,
		0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dbErr := errors.New("history insert constraint violation")
	databaseErr := registerMockDB("save_history_dberr", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return nil, nil, dbErr
	})
	defer databaseErr.Close()

	repoErr := NewPostgresStatusHistoryRepository(databaseErr)
	if err := repoErr.Save(context.Background(), nil, validEntry); err == nil || !strings.Contains(err.Error(), "history insert constraint violation") {
		t.Fatalf("expected wrapped db error, got: %v", err)
	}
}

func TestPostgresStatusHistoryRepository_Save_RejectsEmptyID(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	database := registerMockDB("save_history_empty_id", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		t.Fatal("DB should not be called when status history ID is empty")
		return nil, nil, nil
	})
	defer database.Close()

	repo := NewPostgresStatusHistoryRepository(database)
	for _, blankID := range []string{"", "   "} {
		entry := domain.StatusHistoryEntry{
			ID:         blankID,
			ParcelID:   "parcel-uuid-001",
			ToStatus:   domain.StatusCreated,
			ActorID:    strPtr("usr-001"),
			ActorType:  domain.ActorTypeUser,
			ActorRole:  "CUSTOMER",
			Reason:     "Initial booking",
			OccurredAt: now,
		}
		if err := repo.Save(context.Background(), nil, entry); !errors.Is(err, domain.ErrInvalidStatusHistory) {
			t.Fatalf("expected ErrInvalidStatusHistory for blank id %q, got: %v", blankID, err)
		}
	}
}

func TestPostgresStatusHistoryRepository_FindByParcelID(t *testing.T) {
	t0 := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(15 * time.Minute)
	t2 := t0.Add(30 * time.Minute)

	columns := []string{
		"id", "parcel_id", "from_status", "to_status",
		"actor_id", "actor_type", "actor_role", "reason", "occurred_at",
	}
	data := [][]driver.Value{
		{"hist-1", "parcel-uuid-001", nil, "CREATED", "mrc-001", "MERCHANT_API", "MERCHANT", "Booked via API", t0},
		{"hist-2", "parcel-uuid-001", "CREATED", "ASSIGNED", "disp-001", "USER", "DISPATCHER", "Courier assigned", t1},
		{"hist-3", "parcel-uuid-001", "ASSIGNED", "PICKED_UP", nil, "SYSTEM", "SYSTEM", "Automated hub handoff", t2},
	}

	var executedQuery string
	database := registerMockDB("find_history_parcel", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		executedQuery = query
		return &testRows{columns: columns, data: data}, nil, nil
	})
	defer database.Close()

	repo := NewPostgresStatusHistoryRepository(database)
	entries, err := repo.FindByParcelID(context.Background(), nil, "parcel-uuid-001")
	if err != nil {
		t.Fatalf("unexpected FindByParcelID error: %v", err)
	}

	if !strings.Contains(executedQuery, "occurred_at") || !strings.Contains(executedQuery, "actor_type") {
		t.Errorf("expected SELECT query to use actor_type and occurred_at, got: %s", executedQuery)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(entries))
	}

	if entries[0].ActorType != domain.ActorTypeMerchantAPI || entries[0].ActorID == nil || *entries[0].ActorID != "mrc-001" {
		t.Errorf("unexpected entry 0 actor: %+v", entries[0])
	}
	if !entries[0].OccurredAt.Equal(t0) {
		t.Errorf("expected entry 0 occurred_at %v, got %v", t0, entries[0].OccurredAt)
	}
	if entries[1].ActorType != domain.ActorTypeUser || entries[1].ActorID == nil || *entries[1].ActorID != "disp-001" {
		t.Errorf("unexpected entry 1 actor: %+v", entries[1])
	}
	if entries[2].ActorType != domain.ActorTypeSystem || entries[2].ActorID != nil {
		t.Errorf("expected entry 2 SYSTEM nil ActorID, got %+v", entries[2])
	}
	if !entries[2].OccurredAt.Equal(t2) {
		t.Errorf("expected entry 2 occurred_at %v, got %v", t2, entries[2].OccurredAt)
	}
}
