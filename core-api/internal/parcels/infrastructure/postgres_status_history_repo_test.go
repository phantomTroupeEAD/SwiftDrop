package infrastructure

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/parcels/domain"
)

func TestPostgresStatusHistoryRepository_Save_Success(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	entry, _ := domain.NewStatusHistoryEntry(
		"history-uuid-001",
		"parcel-uuid-001",
		nil,
		domain.StatusCreated,
		"sender-001",
		"CUSTOMER",
		"Initial parcel booking",
		now,
		0,
	)

	var executedQuery string
	var capturedArgs []driver.NamedValue

	database := registerMockDB("save_history_success", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		executedQuery = query
		capturedArgs = args
		return nil, testResult{rowsAffected: 1}, nil
	})
	defer database.Close()

	repo := NewPostgresStatusHistoryRepository(database)
	err := repo.Save(context.Background(), nil, entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(executedQuery, "INSERT INTO parcel_status_history") {
		t.Errorf("expected INSERT INTO parcel_status_history, got: %s", executedQuery)
	}

	if len(capturedArgs) != 8 {
		t.Fatalf("expected 8 arguments, got %d", len(capturedArgs))
	}
	if capturedArgs[0].Value != entry.ID {
		t.Errorf("expected arg 1 to be %s, got %v", entry.ID, capturedArgs[0].Value)
	}
	if capturedArgs[1].Value != entry.ParcelID {
		t.Errorf("expected arg 2 to be %s, got %v", entry.ParcelID, capturedArgs[1].Value)
	}
	if capturedArgs[3].Value != string(domain.StatusCreated) {
		t.Errorf("expected arg 4 to be CREATED, got %v", capturedArgs[3].Value)
	}
}

