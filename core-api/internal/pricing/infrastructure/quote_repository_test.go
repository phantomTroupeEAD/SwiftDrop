package infrastructure

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

func validTestQuote() domain.Quote {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	fee, _ := domain.NewMoney(47000, domain.CurrencyLKR)

	return domain.Quote{
		ID:                 "quote-uuid-001",
		TariffID:           "rule-uuid-001",
		TariffVersion:      2,
		ChargeableWeightKg: 2.5,
		ServiceLevel:       domain.ServiceLevelStandard,
		Fee:                fee,
		CreatedAt:          now,
		ExpiresAt:          now.Add(domain.QuoteValidity),
		UsedAt:             nil,
	}
}

func TestPostgresQuoteRepository_Save_Success(t *testing.T) {
	quote := validTestQuote()
	var executedQuery string
	var capturedArgs []driver.NamedValue

	database := registerMockDB("save_quote_success", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		executedQuery = query
		capturedArgs = args
		return nil, testResult{rowsAffected: 1}, nil
	})
	defer database.Close()

	repo := NewPostgresQuoteRepository(database)

	err := repo.Save(context.Background(), quote)
	if err != nil {
		t.Fatalf("unexpected error saving quote: %v", err)
	}

	if executedQuery == "" {
		t.Fatal("expected query to be executed")
	}

	// Must be strictly insert-only: no ON CONFLICT DO UPDATE
	if strings.Contains(executedQuery, "ON CONFLICT") {
		t.Errorf("expected insert-only query without ON CONFLICT, got: %s", executedQuery)
	}

	if len(capturedArgs) != 10 {
		t.Fatalf("expected 10 arguments, got %d", len(capturedArgs))
	}

	if capturedArgs[0].Value != quote.ID {
		t.Errorf("expected arg 1 to be quote ID %s, got %v", quote.ID, capturedArgs[0].Value)
	}
	if capturedArgs[1].Value != quote.TariffID {
		t.Errorf("expected arg 2 to be tariff ID %s, got %v", quote.TariffID, capturedArgs[1].Value)
	}
	if capturedArgs[2].Value != int64(quote.TariffVersion) && capturedArgs[2].Value != quote.TariffVersion {
		t.Errorf("expected arg 3 to be version %d, got %v", quote.TariffVersion, capturedArgs[2].Value)
	}
	if capturedArgs[3].Value != quote.ChargeableWeightKg {
		t.Errorf("expected arg 4 to be weight %v, got %v", quote.ChargeableWeightKg, capturedArgs[3].Value)
	}
	if capturedArgs[4].Value != string(quote.ServiceLevel) {
		t.Errorf("expected arg 5 to be service level %s, got %v", quote.ServiceLevel, capturedArgs[4].Value)
	}
	if capturedArgs[5].Value != quote.Fee.Amount {
		t.Errorf("expected arg 6 to be fee amount %d, got %v", quote.Fee.Amount, capturedArgs[5].Value)
	}
}

func TestPostgresQuoteRepository_Save_ValidationErrors(t *testing.T) {
	repo := NewPostgresQuoteRepository(&sql.DB{})

	// Invalid quote (empty ID)
	quote := validTestQuote()
	quote.ID = ""
	err := repo.Save(context.Background(), quote)
	if err == nil {
		t.Error("expected error saving quote with empty ID, got nil")
	}

	// Nil querier
	nilRepo := NewPostgresQuoteRepository(nil)
	err = nilRepo.Save(context.Background(), validTestQuote())
	if err == nil {
		t.Error("expected error saving quote with nil querier, got nil")
	}
}

func TestPostgresQuoteRepository_Save_DatabaseError(t *testing.T) {
	dbErr := errors.New("unique violation: quote already exists")

	database := registerMockDB("save_quote_dberr", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return nil, nil, dbErr
	})
	defer database.Close()

	repo := NewPostgresQuoteRepository(database)

	err := repo.Save(context.Background(), validTestQuote())
	if err == nil || !errors.Is(err, dbErr) {
		t.Fatalf("expected error wrapping dbErr, got: %v", err)
	}
}

func TestPostgresQuoteRepository_FindByID_Success(t *testing.T) {
	expected := validTestQuote()

	columns := []string{
		"id",
		"tariff_id",
		"tariff_version",
		"chargeable_weight_kg",
		"service_level",
		"fee_cents",
		"currency",
		"created_at",
		"expires_at",
		"used_at",
	}

	data := [][]driver.Value{
		{
			expected.ID,
			expected.TariffID,
			int64(expected.TariffVersion),
			expected.ChargeableWeightKg,
			string(expected.ServiceLevel),
			expected.Fee.Amount,
			expected.Fee.Currency,
			expected.CreatedAt,
			expected.ExpiresAt,
			nil,
		},
	}

	database := registerMockDB("find_quote_success", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return &testRows{columns: columns, data: data}, nil, nil
	})
	defer database.Close()

	repo := NewPostgresQuoteRepository(database)

	quote, err := repo.FindByID(context.Background(), expected.ID)
	if err != nil {
		t.Fatalf("unexpected error finding quote: %v", err)
	}

	if quote.ID != expected.ID {
		t.Errorf("expected ID %s, got %s", expected.ID, quote.ID)
	}
	if quote.TariffID != expected.TariffID {
		t.Errorf("expected TariffID %s, got %s", expected.TariffID, quote.TariffID)
	}
	if quote.Fee.Amount != expected.Fee.Amount {
		t.Errorf("expected fee %d, got %d", expected.Fee.Amount, quote.Fee.Amount)
	}
	if quote.ServiceLevel != expected.ServiceLevel {
		t.Errorf("expected service level %s, got %s", expected.ServiceLevel, quote.ServiceLevel)
	}
	if quote.UsedAt != nil {
		t.Errorf("expected UsedAt nil, got %v", quote.UsedAt)
	}
}

func TestPostgresQuoteRepository_FindByID_NotFound(t *testing.T) {
	database := registerMockDB("find_quote_not_found", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return &testRows{columns: []string{"id"}, data: [][]driver.Value{}}, nil, nil
	})
	defer database.Close()

	repo := NewPostgresQuoteRepository(database)

	_, err := repo.FindByID(context.Background(), "non-existent-id")
	if !errors.Is(err, domain.ErrInvalidQuote) {
		t.Fatalf("expected ErrInvalidQuote on not found, got: %v", err)
	}
}

func TestPostgresQuoteRepository_FindByID_InputValidation(t *testing.T) {
	repo := NewPostgresQuoteRepository(&sql.DB{})

	// Empty ID
	_, err := repo.FindByID(context.Background(), "   ")
	if !errors.Is(err, domain.ErrInvalidQuote) {
		t.Errorf("expected ErrInvalidQuote for empty ID, got: %v", err)
	}

	// Nil querier
	nilRepo := NewPostgresQuoteRepository(nil)
	_, err = nilRepo.FindByID(context.Background(), "valid-id")
	if err == nil {
		t.Error("expected error for nil querier, got nil")
	}
}
