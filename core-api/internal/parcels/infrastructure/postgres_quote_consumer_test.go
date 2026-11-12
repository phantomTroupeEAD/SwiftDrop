package infrastructure

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/parcels/application"
)

func TestPostgresQuoteConsumer_ConsumeQuote_OwnershipModes(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		quoteID       string
		dbRequesterID driver.Value
		dbMerchantID  driver.Value
		wantRequester *string
		wantMerchant  *string
	}{
		{
			name:          "customer-owned quote with SQL NULL merchant_id",
			quoteID:       "quote-uuid-cust-001",
			dbRequesterID: "usr-cust-001",
			dbMerchantID:  nil,
			wantRequester: strPtr("usr-cust-001"),
			wantMerchant:  nil,
		},
		{
			name:          "merchant-owned quote with SQL NULL requester_id",
			quoteID:       "quote-uuid-merch-001",
			dbRequesterID: nil,
			dbMerchantID:  "mrc-001",
			wantRequester: nil,
			wantMerchant:  strPtr("mrc-001"),
		},
		{
			name:          "combined customer and merchant quote ownership",
			quoteID:       "quote-uuid-both-001",
			dbRequesterID: "usr-merch-owner-001",
			dbMerchantID:  "mrc-001",
			wantRequester: strPtr("usr-merch-owner-001"),
			wantMerchant:  strPtr("mrc-001"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			columns := []string{
				"id", "tariff_id", "tariff_version", "requester_id", "merchant_id",
				"origin_zone_id", "destination_zone_id", "chargeable_weight_kg", "service_level",
				"fee_cents", "currency", "created_at", "expires_at", "used_at",
			}
			data := [][]driver.Value{
				{
					tc.quoteID, "rule-001", int64(2), tc.dbRequesterID, tc.dbMerchantID,
					"ZONE_WESTERN_01", "ZONE_CENTRAL_01", 2.5, "STANDARD",
					int64(55000), "LKR", now.Add(-5 * time.Minute), now.Add(10 * time.Minute), now,
				},
			}

			var executedQuery string
			database := registerMockDB("consume_quote_"+tc.quoteID, func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
				executedQuery = query
				return &testRows{columns: columns, data: data}, nil, nil
			})
			defer database.Close()

			consumer := NewPostgresQuoteConsumer(database)
			snapshot, err := consumer.ConsumeQuote(context.Background(), nil, tc.quoteID, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(executedQuery, "requester_id") || !strings.Contains(executedQuery, "merchant_id") {
				t.Errorf("expected query to select requester_id and merchant_id, got: %s", executedQuery)
			}
			if snapshot.QuoteID != tc.quoteID {
				t.Errorf("expected quote ID %s, got %s", tc.quoteID, snapshot.QuoteID)
			}
			if snapshot.TariffID != "rule-001" || snapshot.TariffVersion != 2 {
				t.Errorf("expected tariff rule-001 v2, got %s v%d", snapshot.TariffID, snapshot.TariffVersion)
			}
			if (snapshot.RequesterID == nil) != (tc.wantRequester == nil) ||
				(snapshot.RequesterID != nil && *snapshot.RequesterID != *tc.wantRequester) {
				t.Errorf("expected RequesterID %v, got %v", tc.wantRequester, snapshot.RequesterID)
			}
			if (snapshot.MerchantID == nil) != (tc.wantMerchant == nil) ||
				(snapshot.MerchantID != nil && *snapshot.MerchantID != *tc.wantMerchant) {
				t.Errorf("expected MerchantID %v, got %v", tc.wantMerchant, snapshot.MerchantID)
			}
			if snapshot.OriginZoneID != "ZONE_WESTERN_01" || snapshot.DestinationZoneID != "ZONE_CENTRAL_01" {
				t.Errorf("unexpected zones: %s -> %s", snapshot.OriginZoneID, snapshot.DestinationZoneID)
			}
			if snapshot.FeeCents != 55000 || snapshot.Currency != "LKR" {
				t.Errorf("unexpected fee: %d %s", snapshot.FeeCents, snapshot.Currency)
			}
			if snapshot.ServiceLevel != "STANDARD" {
				t.Errorf("expected service level STANDARD, got %s", snapshot.ServiceLevel)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func TestPostgresQuoteConsumer_ConsumeQuote_MissingQuote(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	quoteID := "non-existent-quote"

	database := registerMockDB("consume_quote_missing", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		// Both CTE and diagnostic query return 0 rows
		return &testRows{columns: []string{"id"}, data: [][]driver.Value{}}, nil, nil
	})
	defer database.Close()

	consumer := NewPostgresQuoteConsumer(database)
	_, err := consumer.ConsumeQuote(context.Background(), nil, quoteID, now)
	if err == nil || !errors.Is(err, application.ErrQuoteNotFound) {
		t.Fatalf("expected ErrQuoteNotFound, got: %v", err)
	}
}

func TestPostgresQuoteConsumer_ConsumeQuote_ExpiredQuote(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-10 * time.Minute)
	quoteID := "expired-quote-001"

	database := registerMockDB("consume_quote_expired", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		if strings.Contains(query, "WITH consumed AS") {
			// CTE returns 0 rows because expires_at > acceptedAt check failed
			return &testRows{columns: []string{"id"}, data: [][]driver.Value{}}, nil, nil
		}
		// Diagnostic query reveals quote expired in the past and was never used
		diagColumns := []string{"expires_at", "used_at"}
		diagData := [][]driver.Value{
			{expiredAt, nil},
		}
		return &testRows{columns: diagColumns, data: diagData}, nil, nil
	})
	defer database.Close()

	consumer := NewPostgresQuoteConsumer(database)
	_, err := consumer.ConsumeQuote(context.Background(), nil, quoteID, now)
	if err == nil || !errors.Is(err, application.ErrQuoteExpired) {
		t.Fatalf("expected ErrQuoteExpired, got: %v", err)
	}
}

func TestPostgresQuoteConsumer_ConsumeQuote_AlreadyUsedQuote(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	expiresAt := now.Add(10 * time.Minute)
	usedAt := now.Add(-2 * time.Minute)
	quoteID := "already-used-quote-001"

	database := registerMockDB("consume_quote_used", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		if strings.Contains(query, "WITH consumed AS") {
			// CTE returns 0 rows because used_at IS NULL check failed
			return &testRows{columns: []string{"id"}, data: [][]driver.Value{}}, nil, nil
		}
		// Diagnostic query reveals quote was already used
		diagColumns := []string{"expires_at", "used_at"}
		diagData := [][]driver.Value{
			{expiresAt, usedAt},
		}
		return &testRows{columns: diagColumns, data: diagData}, nil, nil
	})
	defer database.Close()

	consumer := NewPostgresQuoteConsumer(database)
	_, err := consumer.ConsumeQuote(context.Background(), nil, quoteID, now)
	if err == nil || !errors.Is(err, application.ErrQuoteAlreadyUsed) {
		t.Fatalf("expected ErrQuoteAlreadyUsed, got: %v", err)
	}
}

func TestPostgresQuoteConsumer_ConsumeQuote_ConcurrentConsumption(t *testing.T) {
	// Simulates two concurrent booking attempts racing to consume the exact same quote.
	// Only ONE attempt can atomically update used_at in PostgreSQL.
	// The second attempt MUST receive ErrQuoteAlreadyUsed.
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	quoteID := "race-condition-quote-001"

	var stateMu sync.Mutex
	consumed := false

	database := registerMockDB("consume_quote_concurrent", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		stateMu.Lock()
		defer stateMu.Unlock()

		if strings.Contains(query, "WITH consumed AS") {
			if !consumed {
				// Worker 1 wins the atomic update race!
				consumed = true
				columns := []string{
					"id", "tariff_id", "tariff_version", "requester_id", "merchant_id",
					"origin_zone_id", "destination_zone_id", "chargeable_weight_kg", "service_level",
					"fee_cents", "currency", "created_at", "expires_at", "used_at",
				}
				data := [][]driver.Value{
					{
						quoteID, "rule-001", int64(1), "usr-race-001", nil,
						"ZONE_WESTERN_01", "ZONE_CENTRAL_01", 2.5, "STANDARD",
						int64(55000), "LKR", now.Add(-time.Minute), now.Add(14 * time.Minute), now,
					},
				}
				return &testRows{columns: columns, data: data}, nil, nil
			}
			// Worker 2 loses: 0 rows updated
			return &testRows{columns: []string{"id"}, data: [][]driver.Value{}}, nil, nil
		}

		// Diagnostic query for the loser: returns that the quote is now marked used
		diagColumns := []string{"expires_at", "used_at"}
		diagData := [][]driver.Value{
			{now.Add(14 * time.Minute), now},
		}
		return &testRows{columns: diagColumns, data: diagData}, nil, nil
	})
	defer database.Close()

	consumer := NewPostgresQuoteConsumer(database)

	var wg sync.WaitGroup
	results := make([]error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			_, err := consumer.ConsumeQuote(context.Background(), nil, quoteID, now)
			results[idx] = err
		}()
	}

	wg.Wait()

	// Exactly one worker must succeed (nil error), and exactly one must fail with ErrQuoteAlreadyUsed
	successCount := 0
	alreadyUsedCount := 0
	for _, res := range results {
		if res == nil {
			successCount++
		} else if errors.Is(res, application.ErrQuoteAlreadyUsed) {
			alreadyUsedCount++
		}
	}

	if successCount != 1 {
		t.Errorf("expected exactly 1 success under concurrent booking, got %d", successCount)
	}
	if alreadyUsedCount != 1 {
		t.Errorf("expected exactly 1 ErrQuoteAlreadyUsed under concurrent booking, got %d", alreadyUsedCount)
	}
}
