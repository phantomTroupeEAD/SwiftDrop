package tests

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/events"
	parcelsapp "github.com/swiftdrop/core-api/internal/parcels/application"
	parcelsdomain "github.com/swiftdrop/core-api/internal/parcels/domain"
	parcelsinfra "github.com/swiftdrop/core-api/internal/parcels/infrastructure"
	pricingapp "github.com/swiftdrop/core-api/internal/pricing/application"
	pricingdomain "github.com/swiftdrop/core-api/internal/pricing/domain"
	pricinginfra "github.com/swiftdrop/core-api/internal/pricing/infrastructure"
)

type fixedPricingClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fixedPricingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fixedPricingClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

type fixedParcelClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fixedParcelClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fixedParcelClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func TestBookParcelIdempotency_PostgreSQLIntegration(t *testing.T) {
	database, cleanup := SetupTestDB(t)
	defer cleanup()

	// Ensure all pooled operations execute on the connection bound to SetupTestDB's temporary itest_* schema.
	database.SetMaxOpenConns(1)

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	customer1ID := "018f4a7c-1005-7000-8000-000000000005" // seeded by 20261021000002_seed_mock_users.up.sql
	customer2ID := "018f4a7c-1006-7000-8000-000000000006"

	// Seed a second user for caller_scope isolation tests.
	const insertUserSQL = `
		INSERT INTO users (id, email, password_hash, display_name, phone, role, tenant_id, status)
		VALUES ($1, 'customer2@swiftdrop.local', 'hash', 'Customer Two', '+94770000006', 'CUSTOMER', NULL, 'ACTIVE')
		ON CONFLICT (id) DO NOTHING;
	`
	if _, err := database.ExecContext(ctx, insertUserSQL, customer2ID); err != nil {
		t.Fatalf("failed to seed second customer: %v", err)
	}

	// Seed active pricing tariff for ZONE_WESTERN_01 -> ZONE_CENTRAL_01.
	const insertTariffSQL = `
		INSERT INTO pricing_rules (
			id, version, origin_zone_id, destination_zone_id,
			base_fee_cents, per_kg_rate_cents, effective_from, effective_until, is_active
		) VALUES (
			'tariff-west-central-v1', 1, 'ZONE_WESTERN_01', 'ZONE_CENTRAL_01',
			35000, 10000, $1, NULL, TRUE
		);
	`
	if _, err := database.ExecContext(ctx, insertTariffSQL, now.Add(-1*time.Hour)); err != nil {
		t.Fatalf("failed to seed pricing rule: %v", err)
	}

	// Wire real PostgreSQL repositories and use cases.
	tariffRepo := pricinginfra.NewPostgresTariffRepository(database)
	quoteRepo := pricinginfra.NewPostgresQuoteRepository(database)
	pricingClock := &fixedPricingClock{now: now}
	quoteUC := pricingapp.NewGenerateQuoteUseCase(
		tariffRepo,
		quoteRepo,
		pricingClock,
		pricinginfra.NewUUIDv7QuoteIDGenerator(),
	)

	parcelClock := &fixedParcelClock{now: now}
	idemRepo := parcelsinfra.NewPostgresIdempotencyRepository(database)
	bookUC, err := parcelsapp.NewBookParcelUseCase(
		parcelsapp.NewDBTransactionManager(database),
		parcelsinfra.NewPostgresQuoteConsumer(database),
		parcelsinfra.NewPostgresParcelRepository(database),
		parcelsinfra.NewPostgresStatusHistoryRepository(database),
		events.NewPostgresOutboxWriter(),
		parcelsapp.NewCryptoTrackingNumberGenerator(),
		parcelsapp.NewUUIDv7Generator(),
		parcelClock,
	)
	if err != nil {
		t.Fatalf("NewBookParcelUseCase failed: %v", err)
	}
	bookUC.WithIdempotency(idemRepo, 24*time.Hour)

	generateQuoteForUser := func(userID string) string {
		pricingClock.Set(parcelClock.Now())
		q, qErr := quoteUC.Execute(ctx, pricingapp.GenerateQuoteCommand{
			OriginZoneID:      "ZONE_WESTERN_01",
			DestinationZoneID: "ZONE_CENTRAL_01",
			ActualWeightKg:    2.5,
			LengthCm:          30,
			WidthCm:           20,
			HeightCm:          15,
			ServiceLevel:      pricingdomain.ServiceLevelStandard,
			RequesterID:       &userID,
		})
		if qErr != nil {
			t.Fatalf("failed to generate quote for user %s: %v", userID, qErr)
		}
		return q.ID
	}

	buildBookingCmd := func(userID, quoteID, key, hash string) parcelsapp.BookParcelCommand {
		return parcelsapp.BookParcelCommand{
			QuoteID:        quoteID,
			SenderID:       &userID,
			ActorType:      parcelsdomain.ActorTypeUser,
			RecipientName:  "Sunil Perera",
			RecipientPhone: "+94771234567",
			WeightKg:       2.5,
			Dimensions: parcelsapp.DimensionsDTO{
				LengthCm: 30,
				WidthCm:  20,
				HeightCm: 15,
			},
			PickupAddress: parcelsapp.AddressDTO{
				ContactName:  "Kasun Perera",
				ContactPhone: "+94771234567",
				StreetLine1:  "45 Galle Road",
				City:         "Colombo",
				PostalCode:   "00300",
				ZoneID:       "ZONE_WESTERN_01",
				Instructions: "Ring front bell",
			},
			DropoffAddress: parcelsapp.AddressDTO{
				ContactName:  "Sunil Perera",
				ContactPhone: "+94719876543",
				StreetLine1:  "12 Kandy Road",
				City:         "Kandy",
				PostalCode:   "20000",
				ZoneID:       "ZONE_CENTRAL_01",
				Instructions: "Leave at reception",
			},
			CorrelationID: "0192e2b3-7c8a-7890-a123-000000000099",
			Idempotency: &parcelsapp.IdempotencyInput{
				Key:         key,
				CallerScope: "USER:" + userID,
				Operation:   parcelsapp.DefaultBookingOperation,
				RequestHash: hash,
			},
		}
	}

	assertCounts := func(wantParcels, wantHistory, wantOutbox, wantIdem int) {
		t.Helper()
		checks := []struct {
			table string
			want  int
		}{
			{"parcels", wantParcels},
			{"parcel_status_history", wantHistory},
			{"outbox_events", wantOutbox},
			{"idempotency_keys", wantIdem},
		}
		for _, c := range checks {
			var got int
			if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+c.table).Scan(&got); err != nil {
				t.Fatalf("count %s failed: %v", c.table, err)
			}
			if got != c.want {
				t.Errorf("table %s row count: got %d, want %d", c.table, got, c.want)
			}
		}
	}

	idemKey1 := "6d54d0c4-41d0-4d8e-a111-112233445566"
	hash1 := strings.Repeat("1", 64)
	hash2 := strings.Repeat("2", 64)

	quote1 := generateQuoteForUser(customer1ID)
	firstRes, err := bookUC.Execute(ctx, buildBookingCmd(customer1ID, quote1, idemKey1, hash1))
	if err != nil {
		t.Fatalf("first booking failed: %v", err)
	}
	if firstRes.Replayed {
		t.Fatal("expected Replayed=false on initial booking")
	}
	assertCounts(1, 1, 1, 1)

	// Verify persisted idempotency_keys fields including created_resource_id UUID and 24h TTL.
	var (
		storedResID   sql.NullString
		storedStatus  int
		storedCreated time.Time
		storedExpires time.Time
	)
	err = database.QueryRowContext(
		ctx,
		`SELECT created_resource_id::text, response_status, created_at, expires_at
		 FROM idempotency_keys WHERE key = $1 AND caller_scope = $2`,
		idemKey1,
		"USER:"+customer1ID,
	).Scan(&storedResID, &storedStatus, &storedCreated, &storedExpires)
	if err != nil {
		t.Fatalf("query idempotency_keys row failed: %v", err)
	}
	if !storedResID.Valid || storedResID.String != firstRes.Parcel.ID {
		t.Errorf("expected created_resource_id=%s, got %v", firstRes.Parcel.ID, storedResID)
	}
	if storedStatus != 201 {
		t.Errorf("expected response_status=201, got %d", storedStatus)
	}
	if !storedExpires.Equal(storedCreated.Add(24 * time.Hour)) {
		t.Errorf("expected 24h TTL: created=%v expires=%v", storedCreated, storedExpires)
	}

	t.Run("duplicate request within 24h returns replay without consuming quote or duplicating rows", func(t *testing.T) {
		parcelClock.Set(now.Add(2 * time.Hour))
		replayRes, err := bookUC.Execute(ctx, buildBookingCmd(customer1ID, quote1, idemKey1, hash1))
		if err != nil {
			t.Fatalf("replay booking failed: %v", err)
		}
		if !replayRes.Replayed {
			t.Fatal("expected Replayed=true on duplicate booking")
		}
		if replayRes.Parcel.ID != firstRes.Parcel.ID || replayRes.Parcel.TrackingNumber != firstRes.Parcel.TrackingNumber {
			t.Errorf("replay mismatch: got (%s,%s), want (%s,%s)",
				replayRes.Parcel.ID, replayRes.Parcel.TrackingNumber, firstRes.Parcel.ID, firstRes.Parcel.TrackingNumber)
		}
		assertCounts(1, 1, 1, 1)
	})

	t.Run("reusing key with different request_hash returns ErrIdempotencyPayloadMismatch", func(t *testing.T) {
		_, err := bookUC.Execute(ctx, buildBookingCmd(customer1ID, quote1, idemKey1, hash2))
		if !errors.Is(err, parcelsapp.ErrIdempotencyPayloadMismatch) {
			t.Fatalf("expected ErrIdempotencyPayloadMismatch, got: %v", err)
		}
		assertCounts(1, 1, 1, 1)
	})

	t.Run("caller_scope isolates identical key across two different callers", func(t *testing.T) {
		quote2 := generateQuoteForUser(customer2ID)
		otherRes, err := bookUC.Execute(ctx, buildBookingCmd(customer2ID, quote2, idemKey1, hash2))
		if err != nil {
			t.Fatalf("booking for second caller failed: %v", err)
		}
		if otherRes.Replayed {
			t.Fatal("expected Replayed=false for distinct caller_scope")
		}
		if otherRes.Parcel.ID == firstRes.Parcel.ID {
			t.Error("expected distinct parcel ID for second caller")
		}
		assertCounts(2, 2, 2, 2)
	})

	t.Run("expired key is treated as fresh request and replaces existing row", func(t *testing.T) {
		// Expire customer1's existing key in PostgreSQL.
		if _, err := database.ExecContext(
			ctx,
			`UPDATE idempotency_keys SET expires_at = created_at + INTERVAL '1 second' WHERE key = $1 AND caller_scope = $2`,
			idemKey1,
			"USER:"+customer1ID,
		); err != nil {
			t.Fatalf("failed to expire idempotency record: %v", err)
		}

		afterExpiry := now.Add(2 * time.Minute)
		parcelClock.Set(afterExpiry)

		freshQuote := generateQuoteForUser(customer1ID)
		freshRes, err := bookUC.Execute(ctx, buildBookingCmd(customer1ID, freshQuote, idemKey1, hash2))
		if err != nil {
			t.Fatalf("expected expired key to be replaced as fresh booking, got: %v", err)
		}
		if freshRes.Replayed {
			t.Fatal("expected Replayed=false after key expiration")
		}
		if freshRes.Parcel.ID == firstRes.Parcel.ID {
			t.Error("expected new parcel ID after expired key replacement")
		}

		var replacedID string
		if err := database.QueryRowContext(
			ctx,
			`SELECT created_resource_id::text FROM idempotency_keys WHERE key = $1 AND caller_scope = $2`,
			idemKey1,
			"USER:"+customer1ID,
		).Scan(&replacedID); err != nil {
			t.Fatalf("query replaced idempotency row failed: %v", err)
		}
		if replacedID != freshRes.Parcel.ID {
			t.Errorf("expected replaced created_resource_id=%s, got %s", freshRes.Parcel.ID, replacedID)
		}
		assertCounts(3, 3, 3, 2)
	})

	t.Run("concurrent duplicate requests produce single parcel", func(t *testing.T) {
		concurrentKey := "7f91e0b1-1234-4a56-9876-aabbccddeeff"
		concurrentQuote := generateQuoteForUser(customer1ID)
		parcelClock.Set(now)

		const workers = 5
		var wg sync.WaitGroup
		results := make([]parcelsapp.BookParcelResult, workers)
		errs := make([]error, workers)
		start := make(chan struct{})

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-start
				results[idx], errs[idx] = bookUC.Execute(
					ctx,
					buildBookingCmd(customer1ID, concurrentQuote, concurrentKey, hash1),
				)
			}(i)
		}
		close(start)
		wg.Wait()

		originals, replays := 0, 0
		var parcelID string
		for i := 0; i < workers; i++ {
			if errs[i] != nil {
				t.Fatalf("concurrent worker %d failed: %v", i, errs[i])
			}
			if parcelID == "" {
				parcelID = results[i].Parcel.ID
			} else if results[i].Parcel.ID != parcelID {
				t.Errorf("worker %d returned different parcel ID %s vs %s", i, results[i].Parcel.ID, parcelID)
			}
			if results[i].Replayed {
				replays++
			} else {
				originals++
			}
		}
		if originals != 1 || replays != workers-1 {
			t.Errorf("expected 1 original and %d replays, got originals=%d replays=%d", workers-1, originals, replays)
		}
		assertCounts(4, 4, 4, 3)
	})
}
