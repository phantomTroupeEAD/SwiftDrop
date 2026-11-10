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
	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

func ptrStr(s string) *string {
	return &s
}

func validTestQuote() domain.Quote {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	fee, _ := domain.NewMoney(47000, domain.CurrencyLKR)

	return domain.Quote{
		ID:                 "quote-uuid-001",
		TariffID:           "rule-uuid-001",
		TariffVersion:      2,
		RequesterID:        ptrStr("018f4a7c-1005-7000-8000-000000000005"),
		MerchantID:         nil,
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
	if !strings.Contains(executedQuery, "requester_id") || !strings.Contains(executedQuery, "merchant_id") {
		t.Errorf("expected INSERT to include requester_id and merchant_id, got: %s", executedQuery)
	}

	if len(capturedArgs) != 12 {
		t.Fatalf("expected 12 arguments, got %d", len(capturedArgs))
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
	if capturedArgs[3].Value != *quote.RequesterID {
		t.Errorf("expected arg 4 to be requester ID %s, got %v", *quote.RequesterID, capturedArgs[3].Value)
	}
	if capturedArgs[4].Value != nil {
		t.Errorf("expected arg 5 to be SQL NULL for nil MerchantID, got %v", capturedArgs[4].Value)
	}
	if capturedArgs[5].Value != quote.ChargeableWeightKg {
		t.Errorf("expected arg 6 to be weight %v, got %v", quote.ChargeableWeightKg, capturedArgs[5].Value)
	}
	if capturedArgs[6].Value != string(quote.ServiceLevel) {
		t.Errorf("expected arg 7 to be service level %s, got %v", quote.ServiceLevel, capturedArgs[6].Value)
	}
	if capturedArgs[7].Value != quote.Fee.Amount {
		t.Errorf("expected arg 8 to be fee amount %d, got %v", quote.Fee.Amount, capturedArgs[7].Value)
	}
}

func TestPostgresQuoteRepository_Save_OwnershipModes(t *testing.T) {
	tests := []struct {
		name             string
		requesterID      *string
		merchantID       *string
		expectedReqArg   any
		expectedMerchArg any
	}{
		{
			name:             "customer-only quote persists requester_id and NULL merchant_id",
			requesterID:      ptrStr("018f4a7c-1005-7000-8000-000000000005"),
			merchantID:       nil,
			expectedReqArg:   "018f4a7c-1005-7000-8000-000000000005",
			expectedMerchArg: nil,
		},
		{
			name:             "merchant-only quote persists NULL requester_id and merchant_id",
			requesterID:      nil,
			merchantID:       ptrStr("018f4a7c-2001-7000-8000-000000000010"),
			expectedReqArg:   nil,
			expectedMerchArg: "018f4a7c-2001-7000-8000-000000000010",
		},
		{
			name:             "customer-plus-merchant quote persists both IDs",
			requesterID:      ptrStr("018f4a7c-1004-7000-8000-000000000004"),
			merchantID:       ptrStr("018f4a7c-2001-7000-8000-000000000010"),
			expectedReqArg:   "018f4a7c-1004-7000-8000-000000000004",
			expectedMerchArg: "018f4a7c-2001-7000-8000-000000000010",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			quote := validTestQuote()
			quote.RequesterID = tc.requesterID
			quote.MerchantID = tc.merchantID

			var capturedArgs []driver.NamedValue
			dbName := "save_quote_ownership_" + strings.ReplaceAll(tc.name, " ", "_")
			database := registerMockDB(dbName, func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
				capturedArgs = args
				return nil, testResult{rowsAffected: 1}, nil
			})
			defer database.Close()

			repo := NewPostgresQuoteRepository(database)
			if err := repo.Save(context.Background(), quote); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(capturedArgs) != 12 {
				t.Fatalf("expected 12 arguments, got %d", len(capturedArgs))
			}
			if capturedArgs[3].Value != tc.expectedReqArg {
				t.Errorf("requester_id arg mismatch: expected %v, got %v", tc.expectedReqArg, capturedArgs[3].Value)
			}
			if capturedArgs[4].Value != tc.expectedMerchArg {
				t.Errorf("merchant_id arg mismatch: expected %v, got %v", tc.expectedMerchArg, capturedArgs[4].Value)
			}
		})
	}
}

func TestPostgresQuoteRepository_Save_DuplicateQuote(t *testing.T) {
	database := registerMockDB("save_quote_duplicate_pq", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return nil, nil, &pq.Error{Code: "23505", Message: "duplicate key value violates unique constraint"}
	})
	defer database.Close()

	repo := NewPostgresQuoteRepository(database)
	err := repo.Save(context.Background(), validTestQuote())
	if !errors.Is(err, ErrDuplicateQuote) {
		t.Fatalf("expected ErrDuplicateQuote on 23505 violation, got: %v", err)
	}
}

func TestPostgresQuoteRepository_Save_RejectsMissingOwnership(t *testing.T) {
	dbCalled := false
	database := registerMockDB("save_quote_no_owner", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		dbCalled = true
		return nil, testResult{rowsAffected: 1}, nil
	})
	defer database.Close()

	quote := validTestQuote()
	quote.RequesterID = nil
	quote.MerchantID = nil

	repo := NewPostgresQuoteRepository(database)
	err := repo.Save(context.Background(), quote)
	if !errors.Is(err, domain.ErrInvalidQuote) {
		t.Fatalf("expected ErrInvalidQuote for ownerless quote, got: %v", err)
	}
	if dbCalled {
		t.Error("expected database not to be called when quote ownership is invalid")
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
	dbErr := errors.New("connection refused by database")

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
		"requester_id",
		"merchant_id",
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
			*expected.RequesterID,
			nil,
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
	if quote.RequesterID == nil || *quote.RequesterID != *expected.RequesterID {
		t.Errorf("expected RequesterID %s, got %v", *expected.RequesterID, quote.RequesterID)
	}
	if quote.MerchantID != nil {
		t.Errorf("expected nil MerchantID, got %v", quote.MerchantID)
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

func TestPostgresQuoteRepository_FindByID_OwnershipAndNullHandling(t *testing.T) {
	columns := []string{
		"id",
		"tariff_id",
		"tariff_version",
		"requester_id",
		"merchant_id",
		"chargeable_weight_kg",
		"service_level",
		"fee_cents",
		"currency",
		"created_at",
		"expires_at",
		"used_at",
	}

	base := validTestQuote()

	t.Run("customer-only with NULL merchant_id", func(t *testing.T) {
		reqID := "018f4a7c-1005-7000-8000-000000000005"
		database := registerMockDB("find_quote_customer_only", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			return &testRows{
				columns: columns,
				data: [][]driver.Value{{
					base.ID, base.TariffID, int64(base.TariffVersion),
					reqID, nil,
					base.ChargeableWeightKg, string(base.ServiceLevel),
					base.Fee.Amount, base.Fee.Currency, base.CreatedAt, base.ExpiresAt, nil,
				}},
			}, nil, nil
		})
		defer database.Close()

		repo := NewPostgresQuoteRepository(database)
		q, err := repo.FindByID(context.Background(), base.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q.RequesterID == nil || *q.RequesterID != reqID {
			t.Errorf("expected RequesterID %s, got %v", reqID, q.RequesterID)
		}
		if q.MerchantID != nil {
			t.Errorf("expected nil MerchantID, got %v", q.MerchantID)
		}
	})

	t.Run("merchant-only with NULL requester_id", func(t *testing.T) {
		merchID := "018f4a7c-2001-7000-8000-000000000010"
		database := registerMockDB("find_quote_merchant_only", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			return &testRows{
				columns: columns,
				data: [][]driver.Value{{
					base.ID, base.TariffID, int64(base.TariffVersion),
					nil, merchID,
					base.ChargeableWeightKg, string(base.ServiceLevel),
					base.Fee.Amount, base.Fee.Currency, base.CreatedAt, base.ExpiresAt, nil,
				}},
			}, nil, nil
		})
		defer database.Close()

		repo := NewPostgresQuoteRepository(database)
		q, err := repo.FindByID(context.Background(), base.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q.RequesterID != nil {
			t.Errorf("expected nil RequesterID, got %v", q.RequesterID)
		}
		if q.MerchantID == nil || *q.MerchantID != merchID {
			t.Errorf("expected MerchantID %s, got %v", merchID, q.MerchantID)
		}
	})

	t.Run("both requester_id and merchant_id populated", func(t *testing.T) {
		reqID := "018f4a7c-1004-7000-8000-000000000004"
		merchID := "018f4a7c-2001-7000-8000-000000000010"
		database := registerMockDB("find_quote_both_owners", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			return &testRows{
				columns: columns,
				data: [][]driver.Value{{
					base.ID, base.TariffID, int64(base.TariffVersion),
					reqID, merchID,
					base.ChargeableWeightKg, string(base.ServiceLevel),
					base.Fee.Amount, base.Fee.Currency, base.CreatedAt, base.ExpiresAt, nil,
				}},
			}, nil, nil
		})
		defer database.Close()

		repo := NewPostgresQuoteRepository(database)
		q, err := repo.FindByID(context.Background(), base.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q.RequesterID == nil || *q.RequesterID != reqID {
			t.Errorf("expected RequesterID %s, got %v", reqID, q.RequesterID)
		}
		if q.MerchantID == nil || *q.MerchantID != merchID {
			t.Errorf("expected MerchantID %s, got %v", merchID, q.MerchantID)
		}
	})

	t.Run("rejects corrupt row when both ownership columns are SQL NULL", func(t *testing.T) {
		database := registerMockDB("find_quote_both_null", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
			return &testRows{
				columns: columns,
				data: [][]driver.Value{{
					base.ID, base.TariffID, int64(base.TariffVersion),
					nil, nil,
					base.ChargeableWeightKg, string(base.ServiceLevel),
					base.Fee.Amount, base.Fee.Currency, base.CreatedAt, base.ExpiresAt, nil,
				}},
			}, nil, nil
		})
		defer database.Close()

		repo := NewPostgresQuoteRepository(database)
		_, err := repo.FindByID(context.Background(), base.ID)
		if !errors.Is(err, domain.ErrInvalidQuote) {
			t.Fatalf("expected ErrInvalidQuote for corrupt null-owner row, got: %v", err)
		}
	})
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
