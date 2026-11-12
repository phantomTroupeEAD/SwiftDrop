package application

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/events/eventstest"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

type mockIdempotencyStore struct {
	mu         sync.Mutex
	keyLocks   map[string]*sync.Mutex
	records    map[string]IdempotencyRecord
	acquireErr error
	findErr    error
	saveErr    error
	saveCalled int
	findCalled int
	lockCalled int
	heldLocks  map[string]bool
}

func newMockIdempotencyStore() *mockIdempotencyStore {
	return &mockIdempotencyStore{
		keyLocks:  make(map[string]*sync.Mutex),
		records:   make(map[string]IdempotencyRecord),
		heldLocks: make(map[string]bool),
	}
}

func compositeIdemKey(key, scope string) string {
	return scope + "\x00" + key
}

func (m *mockIdempotencyStore) AcquireLock(_ context.Context, _ db.Querier, key, callerScope string) error {
	m.mu.Lock()
	m.lockCalled++
	err := m.acquireErr
	m.mu.Unlock()
	if err != nil {
		return err
	}

	k := compositeIdemKey(key, callerScope)
	m.mu.Lock()
	kl, ok := m.keyLocks[k]
	if !ok {
		kl = &sync.Mutex{}
		m.keyLocks[k] = kl
	}
	m.mu.Unlock()

	kl.Lock()
	m.mu.Lock()
	m.heldLocks[k] = true
	m.mu.Unlock()
	return nil
}

func (m *mockIdempotencyStore) releaseLock(key, callerScope string) {
	k := compositeIdemKey(key, callerScope)
	m.mu.Lock()
	kl, ok := m.keyLocks[k]
	wasHeld := m.heldLocks[k]
	if wasHeld {
		m.heldLocks[k] = false
	}
	m.mu.Unlock()
	if ok && wasHeld {
		kl.Unlock()
	}
}

func (m *mockIdempotencyStore) Find(_ context.Context, _ db.Querier, key, callerScope string) (IdempotencyRecord, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.findCalled++
	if m.findErr != nil {
		return IdempotencyRecord{}, false, m.findErr
	}
	rec, ok := m.records[compositeIdemKey(key, callerScope)]
	if !ok {
		return IdempotencyRecord{}, false, nil
	}
	return rec, true, nil
}

func (m *mockIdempotencyStore) Save(_ context.Context, _ db.Querier, record IdempotencyRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveCalled++
	if m.saveErr != nil {
		return m.saveErr
	}
	k := compositeIdemKey(record.Key, record.CallerScope)
	if existing, ok := m.records[k]; ok && !existing.IsExpired(record.CreatedAt) {
		return ErrIdempotencyConflict
	}
	m.records[k] = record
	return nil
}

// lockReleasingTxManager wraps DBTransactionManager and releases the mock per-key lock on tx exit,
// mimicking PostgreSQL's transaction-scoped pg_advisory_xact_lock lifecycle.
type lockReleasingTxManager struct {
	inner       TransactionManager
	store       *mockIdempotencyStore
	key         string
	callerScope string
}

func (t *lockReleasingTxManager) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	defer t.store.releaseLock(t.key, t.callerScope)
	return t.inner.WithTx(ctx, fn)
}

type mutableClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *mutableClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *mutableClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// trackingQuoteConsumer supports single-use enforcement and call counting across sequential/concurrent calls.
type singleUseQuoteConsumer struct {
	mu          sync.Mutex
	quotes      map[string]AcceptedQuoteSnapshot
	used        map[string]bool
	consumeHits int
}

func newSingleUseQuoteConsumer(quotes ...AcceptedQuoteSnapshot) *singleUseQuoteConsumer {
	m := &singleUseQuoteConsumer{
		quotes: make(map[string]AcceptedQuoteSnapshot),
		used:   make(map[string]bool),
	}
	for _, q := range quotes {
		m.quotes[q.QuoteID] = q
	}
	return m
}

func (c *singleUseQuoteConsumer) ConsumeQuote(_ context.Context, _ db.Querier, quoteID string, acceptedAt time.Time) (AcceptedQuoteSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.consumeHits++
	q, ok := c.quotes[quoteID]
	if !ok {
		return AcceptedQuoteSnapshot{}, ErrQuoteNotFound
	}
	if c.used[quoteID] {
		return AcceptedQuoteSnapshot{}, ErrQuoteAlreadyUsed
	}
	if !acceptedAt.Before(q.ExpiresAt) {
		return AcceptedQuoteSnapshot{}, ErrQuoteExpired
	}
	c.used[quoteID] = true
	q.UsedAt = acceptedAt
	return q, nil
}

func TestBookParcelUseCase_IdempotencyLifecycle(t *testing.T) {
	baseNow := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	hashA := strings.Repeat("a", 64)
	hashB := strings.Repeat("b", 64)
	idemKey := "d3b07384-d113-494b-9c8e-aa672580c822"

	t.Run("first request persists parcel, history, outbox, and 24h idempotency record", func(t *testing.T) {
		database := getTestDB(t)
		defer database.Close()

		store := newMockIdempotencyStore()
		quoteConsumer := newSingleUseQuoteConsumer(validTestQuoteSnapshot(baseNow))
		parcelRepo := newMockParcelRepo()
		historyRepo := newMockStatusHistoryRepo()
		outboxWriter := eventstest.NewInMemoryWriter()
		clock := &mutableClock{now: baseNow}

		txMgr := &lockReleasingTxManager{
			inner:       NewDBTransactionManager(database),
			store:       store,
			key:         idemKey,
			callerScope: "USER:usr-sender-001",
		}

		uc, err := NewBookParcelUseCase(txMgr, quoteConsumer, parcelRepo, historyRepo, outboxWriter, &mockTrackingGen{}, &mockIDGen{}, clock)
		if err != nil {
			t.Fatalf("NewBookParcelUseCase failed: %v", err)
		}
		uc.WithIdempotency(store, 0) // defaults to 24h

		cmd := validTestBookingCommand()
		cmd.Idempotency = &IdempotencyInput{
			Key:         idemKey,
			CallerScope: "USER:usr-sender-001",
			Operation:   DefaultBookingOperation,
			RequestHash: strings.ToUpper(hashA), // verify lowercase normalization
		}

		res, err := uc.Execute(context.Background(), cmd)
		if err != nil {
			t.Fatalf("first booking failed: %v", err)
		}
		if res.Replayed {
			t.Fatal("expected Replayed=false on first request")
		}
		if res.Idempotency == nil {
			t.Fatal("expected non-nil Idempotency record on result")
		}
		if res.Idempotency.CreatedResourceID == nil || *res.Idempotency.CreatedResourceID != res.Parcel.ID {
			t.Fatalf("expected CreatedResourceID %s, got %v", res.Parcel.ID, res.Idempotency.CreatedResourceID)
		}
		if res.Idempotency.ResponseStatus != 201 {
			t.Errorf("expected ResponseStatus 201, got %d", res.Idempotency.ResponseStatus)
		}
		if !res.Idempotency.ExpiresAt.Equal(baseNow.Add(24 * time.Hour)) {
			t.Errorf("expected 24h expiry %v, got %v", baseNow.Add(24*time.Hour), res.Idempotency.ExpiresAt)
		}
		if res.Idempotency.RequestHash != hashA {
			t.Errorf("expected lowercase hash %s, got %s", hashA, res.Idempotency.RequestHash)
		}

		// Duplicate request 23h59m later with identical key, caller_scope, and hash -> replay
		clock.Set(baseNow.Add(23*time.Hour + 59*time.Minute))
		quoteConsumer.consumeHits = 0
		parcelRepo.saveCalled = false
		historyRepo.saveCalled = false
		outboxCountBefore := outboxWriter.Count()

		replayRes, err := uc.Execute(context.Background(), cmd)
		if err != nil {
			t.Fatalf("duplicate booking replay failed: %v", err)
		}
		if !replayRes.Replayed {
			t.Fatal("expected Replayed=true on duplicate request within 24h")
		}
		if replayRes.Parcel.ID != res.Parcel.ID || replayRes.Parcel.TrackingNumber != res.Parcel.TrackingNumber {
			t.Errorf("replayed parcel mismatch: got (%s, %s), want (%s, %s)",
				replayRes.Parcel.ID, replayRes.Parcel.TrackingNumber, res.Parcel.ID, res.Parcel.TrackingNumber)
		}
		if replayRes.StatusHistory.ID != res.StatusHistory.ID {
			t.Errorf("replayed status history ID mismatch: got %s, want %s", replayRes.StatusHistory.ID, res.StatusHistory.ID)
		}
		if quoteConsumer.consumeHits != 0 {
			t.Errorf("expected quote consumer NOT to be called on replay, got %d calls", quoteConsumer.consumeHits)
		}
		if parcelRepo.saveCalled || historyRepo.saveCalled || outboxWriter.Count() != outboxCountBefore {
			t.Error("replay must not create additional parcel, status history, or outbox records")
		}

		// Reusing same key with different request_hash within 24h -> ErrIdempotencyPayloadMismatch
		mismatchCmd := cmd
		mismatchCmd.Idempotency = &IdempotencyInput{
			Key:         idemKey,
			CallerScope: "USER:usr-sender-001",
			Operation:   DefaultBookingOperation,
			RequestHash: hashB,
		}
		_, err = uc.Execute(context.Background(), mismatchCmd)
		if !errors.Is(err, ErrIdempotencyPayloadMismatch) {
			t.Fatalf("expected ErrIdempotencyPayloadMismatch, got: %v", err)
		}
		if quoteConsumer.consumeHits != 0 || parcelRepo.saveCalled || historyRepo.saveCalled {
			t.Error("payload mismatch must reject before consuming quote or mutating state")
		}
	})

	t.Run("expired key at 24h boundary is treated as fresh request and replaces expired record", func(t *testing.T) {
		database := getTestDB(t)
		defer database.Close()

		store := newMockIdempotencyStore()
		q1 := validTestQuoteSnapshot(baseNow)
		q2 := validTestQuoteSnapshot(baseNow.Add(24 * time.Hour))
		q2.QuoteID = "quote-uuid-002"
		q2.ExpiresAt = baseNow.Add(24*time.Hour + 15*time.Minute)

		quoteConsumer := newSingleUseQuoteConsumer(q1, q2)
		parcelRepo := newMockParcelRepo()
		historyRepo := newMockStatusHistoryRepo()
		outboxWriter := eventstest.NewInMemoryWriter()
		clock := &mutableClock{now: baseNow}

		txMgr := &lockReleasingTxManager{
			inner:       NewDBTransactionManager(database),
			store:       store,
			key:         idemKey,
			callerScope: "USER:usr-sender-001",
		}
		uc, _ := NewBookParcelUseCase(txMgr, quoteConsumer, parcelRepo, historyRepo, outboxWriter, &mockTrackingGen{
			candidates: []domain.TrackingNumber{"SD-20261110-AAAAAA", "SD-20261111-BBBBBB"},
		}, &mockIDGen{}, clock)
		uc.WithIdempotency(store, 24*time.Hour)

		cmd1 := validTestBookingCommand()
		cmd1.Idempotency = &IdempotencyInput{
			Key:         idemKey,
			CallerScope: "USER:usr-sender-001",
			Operation:   DefaultBookingOperation,
			RequestHash: hashA,
		}
		first, err := uc.Execute(context.Background(), cmd1)
		if err != nil {
			t.Fatalf("first booking failed: %v", err)
		}

		// Advance clock to exact 24h expiry boundary (now == expires_at -> expired)
		expiredNow := baseNow.Add(24 * time.Hour)
		clock.Set(expiredNow)

		cmd2 := validTestBookingCommand()
		cmd2.QuoteID = q2.QuoteID
		cmd2.Idempotency = &IdempotencyInput{
			Key:         idemKey,
			CallerScope: "USER:usr-sender-001",
			Operation:   DefaultBookingOperation,
			RequestHash: hashB, // different payload allowed once key expired
		}
		second, err := uc.Execute(context.Background(), cmd2)
		if err != nil {
			t.Fatalf("expected expired key to allow fresh booking, got: %v", err)
		}
		if second.Replayed {
			t.Fatal("expected Replayed=false after key expiration")
		}
		if second.Parcel.ID == first.Parcel.ID {
			t.Error("expected new parcel ID for fresh booking after key expiry")
		}
		if second.Idempotency == nil || !second.Idempotency.ExpiresAt.Equal(expiredNow.Add(24*time.Hour)) {
			t.Errorf("expected refreshed 24h expiry, got %+v", second.Idempotency)
		}
		if len(parcelRepo.parcels) != 2 || outboxWriter.Count() != 2 {
			t.Errorf("expected 2 parcels and 2 outbox events, got %d parcels and %d events", len(parcelRepo.parcels), outboxWriter.Count())
		}
	})

	t.Run("same key under different caller_scope values does not collide", func(t *testing.T) {
		database := getTestDB(t)
		defer database.Close()

		store := newMockIdempotencyStore()
		q1 := validTestQuoteSnapshot(baseNow)
		q2 := validTestQuoteSnapshot(baseNow)
		q2.QuoteID = "quote-uuid-merchant"
		q2.RequesterID = nil
		q2.MerchantID = strPtr("mrc-tenant-001")

		quoteConsumer := newSingleUseQuoteConsumer(q1, q2)
		parcelRepo := newMockParcelRepo()
		historyRepo := newMockStatusHistoryRepo()
		outboxWriter := eventstest.NewInMemoryWriter()
		clock := &mutableClock{now: baseNow}

		uc, _ := NewBookParcelUseCase(NewDBTransactionManager(database), quoteConsumer, parcelRepo, historyRepo, outboxWriter, &mockTrackingGen{
			candidates: []domain.TrackingNumber{"SD-20261110-CCCCCC", "SD-20261110-DDDDDD"},
		}, &mockIDGen{}, clock)
		uc.WithIdempotency(store, 24*time.Hour)

		userCmd := validTestBookingCommand()
		userCmd.Idempotency = &IdempotencyInput{
			Key:         idemKey,
			CallerScope: "USER:usr-sender-001",
			Operation:   DefaultBookingOperation,
			RequestHash: hashA,
		}
		userRes, err := uc.Execute(context.Background(), userCmd)
		if err != nil {
			t.Fatalf("user booking failed: %v", err)
		}
		store.releaseLock(idemKey, "USER:usr-sender-001")

		merchCmd := validTestBookingCommand()
		merchCmd.QuoteID = q2.QuoteID
		merchCmd.SenderID = nil
		merchCmd.MerchantID = strPtr("mrc-tenant-001")
		merchCmd.ActorType = domain.ActorTypeMerchantAPI
		merchCmd.Idempotency = &IdempotencyInput{
			Key:         idemKey,
			CallerScope: "MERCHANT:mrc-tenant-001",
			Operation:   DefaultBookingOperation,
			RequestHash: hashB,
		}
		merchRes, err := uc.Execute(context.Background(), merchCmd)
		if err != nil {
			t.Fatalf("merchant booking with same key failed: %v", err)
		}
		store.releaseLock(idemKey, "MERCHANT:mrc-tenant-001")

		if userRes.Replayed || merchRes.Replayed {
			t.Error("neither caller scope should see a replay from the other")
		}
		if userRes.Parcel.ID == merchRes.Parcel.ID {
			t.Error("expected distinct parcels across isolated caller scopes")
		}
	})

	t.Run("validation and transaction rollback guards", func(t *testing.T) {
		database := getTestDB(t)
		defer database.Close()

		store := newMockIdempotencyStore()
		ucNoStore, _ := NewBookParcelUseCase(
			NewDBTransactionManager(database),
			newSingleUseQuoteConsumer(validTestQuoteSnapshot(baseNow)),
			newMockParcelRepo(),
			newMockStatusHistoryRepo(),
			eventstest.NewInMemoryWriter(),
			&mockTrackingGen{},
			&mockIDGen{},
			fixedClock{now: baseNow},
		)

		cmd := validTestBookingCommand()
		cmd.Idempotency = &IdempotencyInput{
			Key:         idemKey,
			CallerScope: "USER:usr-sender-001",
			Operation:   DefaultBookingOperation,
			RequestHash: hashA,
		}
		if _, err := ucNoStore.Execute(context.Background(), cmd); !errors.Is(err, ErrNilIdempotencyStore) {
			t.Fatalf("expected ErrNilIdempotencyStore when store not configured, got: %v", err)
		}

		// Spoofed caller_scope must be rejected
		uc, _ := NewBookParcelUseCase(
			NewDBTransactionManager(database),
			newSingleUseQuoteConsumer(validTestQuoteSnapshot(baseNow)),
			newMockParcelRepo(),
			newMockStatusHistoryRepo(),
			eventstest.NewInMemoryWriter(),
			&mockTrackingGen{},
			&mockIDGen{},
			fixedClock{now: baseNow},
		)
		uc.WithIdempotency(store, 24*time.Hour)

		spoofCmd := cmd
		spoofCmd.Idempotency = &IdempotencyInput{
			Key:         idemKey,
			CallerScope: "USER:usr-attacker-999",
			Operation:   DefaultBookingOperation,
			RequestHash: hashA,
		}
		if _, err := uc.Execute(context.Background(), spoofCmd); !errors.Is(err, ErrInvalidCallerScope) {
			t.Fatalf("expected ErrInvalidCallerScope for mismatched caller scope, got: %v", err)
		}

		// Failed idempotency save triggers transaction rollback
		store.saveErr = errors.New("disk full")
		_, err := uc.Execute(context.Background(), cmd)
		store.releaseLock(idemKey, "USER:usr-sender-001")
		if err == nil || !strings.Contains(err.Error(), "disk full") {
			t.Fatalf("expected idempotency save error, got: %v", err)
		}
		testTxMu.Lock()
		rolledBack := lastTxRolledBack
		testTxMu.Unlock()
		if !rolledBack {
			t.Error("expected transaction rollback when idempotency record persistence fails")
		}
	})

	t.Run("concurrent duplicate requests serialize and create only one parcel", func(t *testing.T) {
		database := getTestDB(t)
		defer database.Close()

		store := newMockIdempotencyStore()
		quoteConsumer := newSingleUseQuoteConsumer(validTestQuoteSnapshot(baseNow))
		parcelRepo := newMockParcelRepo()
		historyRepo := newMockStatusHistoryRepo()
		outboxWriter := eventstest.NewInMemoryWriter()

		txMgr := &lockReleasingTxManager{
			inner:       NewDBTransactionManager(database),
			store:       store,
			key:         idemKey,
			callerScope: "USER:usr-sender-001",
		}
		uc, _ := NewBookParcelUseCase(
			txMgr,
			quoteConsumer,
			parcelRepo,
			historyRepo,
			outboxWriter,
			&mockTrackingGen{candidates: []domain.TrackingNumber{"SD-20261110-EEEEEE"}},
			&mockIDGen{},
			fixedClock{now: baseNow},
		)
		uc.WithIdempotency(store, 24*time.Hour)

		const workers = 6
		var wg sync.WaitGroup
		results := make([]BookParcelResult, workers)
		errs := make([]error, workers)

		start := make(chan struct{})
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-start
				cmd := validTestBookingCommand()
				cmd.Idempotency = &IdempotencyInput{
					Key:         idemKey,
					CallerScope: "USER:usr-sender-001",
					Operation:   DefaultBookingOperation,
					RequestHash: hashA,
				}
				results[idx], errs[idx] = uc.Execute(context.Background(), cmd)
			}(i)
		}
		close(start)
		wg.Wait()

		originals := 0
		replays := 0
		var parcelID string
		for i := 0; i < workers; i++ {
			if errs[i] != nil {
				t.Fatalf("worker %d failed unexpectedly: %v", i, errs[i])
			}
			if parcelID == "" {
				parcelID = results[i].Parcel.ID
			} else if results[i].Parcel.ID != parcelID {
				t.Errorf("worker %d got different parcel ID %s vs %s", i, results[i].Parcel.ID, parcelID)
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
		if quoteConsumer.consumeHits != 1 {
			t.Errorf("expected quote consumed exactly once, got %d", quoteConsumer.consumeHits)
		}
		if len(parcelRepo.parcels) != 1 || outboxWriter.Count() != 1 {
			t.Errorf("expected 1 parcel and 1 outbox event, got %d parcels and %d events", len(parcelRepo.parcels), outboxWriter.Count())
		}
	})
}

func TestCallerScopeForBooking(t *testing.T) {
	scope, err := CallerScopeForBooking(strPtr("usr-1"), nil)
	if err != nil || scope != "USER:usr-1" {
		t.Fatalf("expected USER:usr-1, got %q (err=%v)", scope, err)
	}

	scope, err = CallerScopeForBooking(strPtr("usr-1"), strPtr("mrc-9"))
	if err != nil || scope != "MERCHANT:mrc-9" {
		t.Fatalf("expected MERCHANT:mrc-9 when merchant_id present, got %q (err=%v)", scope, err)
	}

	if _, err := CallerScopeForBooking(nil, nil); !errors.Is(err, domain.ErrInvalidOwner) {
		t.Fatalf("expected ErrInvalidOwner, got: %v", err)
	}

	// Verify validate edge case for blank operation
	in := IdempotencyInput{
		Key:         "k1",
		CallerScope: "USER:u1",
		Operation:   " ",
		RequestHash: strings.Repeat("a", 64),
	}
	if _, err := in.Normalize(); !errors.Is(err, ErrInvalidIdempotencyOperation) {
		t.Fatalf("expected ErrInvalidIdempotencyOperation, got: %v", err)
	}

	// Verify max key length guard
	tooLongKey := strings.Repeat("k", 256)
	in.Operation = DefaultBookingOperation
	in.Key = tooLongKey
	if _, err := in.Normalize(); !errors.Is(err, ErrInvalidIdempotencyKey) {
		t.Fatalf("expected ErrInvalidIdempotencyKey for >255 chars, got: %v", err)
	}
}
