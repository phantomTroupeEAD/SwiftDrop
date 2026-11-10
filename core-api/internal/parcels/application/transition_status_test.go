package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/events/eventstest"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
)

func createTestParcel(t *testing.T, baseTime time.Time, status domain.ParcelStatus, attempts int, version int) domain.Parcel {
	t.Helper()
	id, err := events.GenerateUUIDv7()
	if err != nil {
		t.Fatalf("failed to generate test parcel id: %v", err)
	}
	tn, _ := domain.FormatTrackingNumber(baseTime, "000001")
	fee, _ := domain.NewMoney(55000, domain.CurrencyLKR)
	weight, _ := domain.NewWeight(2.5, 1.8, 2.5)
	dims, _ := domain.NewDimensions(30, 20, 15)
	pickup, err := domain.NewAddress("Kasun", "+94771234567", "45 Galle Rd", "", "Colombo", "00300", "ZONE_WESTERN_01", "")
	if err != nil {
		t.Fatalf("failed to create test pickup address: %v", err)
	}
	dropoff, err := domain.NewAddress("Sunil", "+94719876543", "12 Kandy Rd", "", "Kandy", "20000", "ZONE_CENTRAL_01", "")
	if err != nil {
		t.Fatalf("failed to create test dropoff address: %v", err)
	}

	p, err := domain.ReconstituteParcel(domain.ReconstituteParcelParams{
		ID:                 id,
		TrackingNumber:     tn,
		SenderID:           strPtr("sender-001"),
		RecipientName:      "Sunil",
		RecipientPhone:     "+94719876543",
		QuoteID:            "quote-001",
		PricingRuleID:      "rule-001",
		PricingRuleVersion: 1,
		DeliveryFee:        fee,
		OriginZoneID:       "ZONE_WESTERN_01",
		DestinationZoneID:  "ZONE_CENTRAL_01",
		ServiceLevel:       domain.ServiceLevelStandard,
		Weight:             weight,
		Dimensions:         dims,
		PickupAddress:      pickup,
		DropoffAddress:     dropoff,
		Status:             status,
		AttemptCount:       attempts,
		Version:            version,
		CreatedAt:          baseTime.Add(-1 * time.Hour),
		UpdatedAt:          baseTime.Add(-1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("failed to reconstitute test parcel: %v", err)
	}
	return p
}

type errOutboxWriter struct {
	err error
}

func (w *errOutboxWriter) Write(ctx context.Context, tx *sql.Tx, envelopes ...events.AnyEnvelope) error {
	return w.err
}

func TestTransitionParcelStatus_SuccessfulTransitions(t *testing.T) {
	testCases := []struct {
		name              string
		initialStatus     domain.ParcelStatus
		initialAttempts   int
		targetStatus      domain.ParcelStatus
		expectedEvent     string
		expectedAttempts  int
		expectedActorRole string
	}{
		{
			name:              "CREATED to ASSIGNED",
			initialStatus:     domain.StatusCreated,
			initialAttempts:   0,
			targetStatus:      domain.StatusAssigned,
			expectedEvent:     events.ParcelAssignedV1,
			expectedAttempts:  0,
			expectedActorRole: "DISPATCHER",
		},
		{
			name:              "ASSIGNED to PICKED_UP",
			initialStatus:     domain.StatusAssigned,
			initialAttempts:   0,
			targetStatus:      domain.StatusPickedUp,
			expectedEvent:     events.ParcelPickedUpV1,
			expectedAttempts:  0,
			expectedActorRole: "COURIER",
		},
		{
			name:              "PICKED_UP to IN_TRANSIT",
			initialStatus:     domain.StatusPickedUp,
			initialAttempts:   0,
			targetStatus:      domain.StatusInTransit,
			expectedEvent:     events.ParcelInTransitV1,
			expectedAttempts:  0,
			expectedActorRole: "SYSTEM",
		},
		{
			name:              "IN_TRANSIT to OUT_FOR_DELIVERY",
			initialStatus:     domain.StatusInTransit,
			initialAttempts:   0,
			targetStatus:      domain.StatusOutForDelivery,
			expectedEvent:     events.ParcelOutForDeliveryV1,
			expectedAttempts:  0,
			expectedActorRole: "DISPATCHER",
		},
		{
			name:              "OUT_FOR_DELIVERY to DELIVERED",
			initialStatus:     domain.StatusOutForDelivery,
			initialAttempts:   0,
			targetStatus:      domain.StatusDelivered,
			expectedEvent:     events.ParcelDeliveredV1,
			expectedAttempts:  0,
			expectedActorRole: "COURIER",
		},
		{
			name:              "OUT_FOR_DELIVERY to ATTEMPT_FAILED increments attempts",
			initialStatus:     domain.StatusOutForDelivery,
			initialAttempts:   0,
			targetStatus:      domain.StatusAttemptFailed,
			expectedEvent:     events.ParcelDeliveryFailedV1,
			expectedAttempts:  1,
			expectedActorRole: "COURIER",
		},
		{
			name:              "ATTEMPT_FAILED to OUT_FOR_DELIVERY (reattempt 2)",
			initialStatus:     domain.StatusAttemptFailed,
			initialAttempts:   1,
			targetStatus:      domain.StatusOutForDelivery,
			expectedEvent:     events.ParcelOutForDeliveryV1,
			expectedAttempts:  1,
			expectedActorRole: "DISPATCHER",
		},
		{
			name:              "ATTEMPT_FAILED to RETURNED",
			initialStatus:     domain.StatusAttemptFailed,
			initialAttempts:   1,
			targetStatus:      domain.StatusReturned,
			expectedEvent:     events.ParcelReturnedV1,
			expectedAttempts:  1,
			expectedActorRole: "SYSTEM",
		},
		{
			name:              "CREATED to CANCELLED",
			initialStatus:     domain.StatusCreated,
			initialAttempts:   0,
			targetStatus:      domain.StatusCancelled,
			expectedEvent:     events.ParcelCancelledV1,
			expectedAttempts:  0,
			expectedActorRole: "CUSTOMER",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := getTestDB(t)
			txManager := NewDBTransactionManager(db)
			parcelRepo := newMockParcelRepo()
			historyRepo := newMockStatusHistoryRepo()
			outbox := eventstest.NewInMemoryWriter()
			clock := fixedClock{now: time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)}
			idGen := &mockIDGen{}

			parcel := createTestParcel(t, clock.now, tc.initialStatus, tc.initialAttempts, 1)
			parcelRepo.parcels[parcel.ID] = parcel

			uc, err := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, clock)
			if err != nil {
				t.Fatalf("failed to construct use case: %v", err)
			}

			cmd := TransitionParcelStatusCommand{
				ParcelID:      parcel.ID,
				TargetStatus:  tc.targetStatus,
				ActorID:       strPtr("usr-test-actor"),
				ActorType:     domain.ActorTypeUser,
				CorrelationID: "corr-12345",
			}

			result, err := uc.Execute(context.Background(), cmd)
			if err != nil {
				t.Fatalf("unexpected transition error: %v", err)
			}

			// Verify returned parcel aggregate
			if result.Parcel.Status != tc.targetStatus {
				t.Errorf("expected parcel status %s, got %s", tc.targetStatus, result.Parcel.Status)
			}
			if result.Parcel.Version != 2 {
				t.Errorf("expected parcel version 2, got %d", result.Parcel.Version)
			}
			if result.Parcel.AttemptCount != tc.expectedAttempts {
				t.Errorf("expected attempts %d, got %d", tc.expectedAttempts, result.Parcel.AttemptCount)
			}

			// Verify status history entry
			if result.StatusHistory.ToStatus != tc.targetStatus {
				t.Errorf("expected history ToStatus %s, got %s", tc.targetStatus, result.StatusHistory.ToStatus)
			}
			if result.StatusHistory.FromStatus == nil || *result.StatusHistory.FromStatus != tc.initialStatus {
				t.Errorf("expected history FromStatus %s, got %v", tc.initialStatus, result.StatusHistory.FromStatus)
			}
			if result.StatusHistory.ActorType != domain.ActorTypeUser {
				t.Errorf("expected ActorType USER, got %s", result.StatusHistory.ActorType)
			}
			if result.StatusHistory.ActorID == nil || *result.StatusHistory.ActorID != "usr-test-actor" {
				t.Errorf("expected ActorID usr-test-actor, got %v", result.StatusHistory.ActorID)
			}
			if result.StatusHistory.ActorRole != tc.expectedActorRole {
				t.Errorf("expected default actor role %s, got %s", tc.expectedActorRole, result.StatusHistory.ActorRole)
			}
			if !result.StatusHistory.OccurredAt.Equal(clock.now) {
				t.Errorf("expected OccurredAt %v, got %v", clock.now, result.StatusHistory.OccurredAt)
			}

			// Verify persistence in mock repositories
			persistedParcel := parcelRepo.parcels[parcel.ID]
			if persistedParcel.Status != tc.targetStatus {
				t.Errorf("persisted parcel status mismatch: expected %s, got %s", tc.targetStatus, persistedParcel.Status)
			}
			if len(historyRepo.entries) != 1 {
				t.Fatalf("expected 1 history entry, got %d", len(historyRepo.entries))
			}

			// Verify outbox integration event publication
			envelopes := outbox.Events()
			if len(envelopes) != 1 {
				t.Fatalf("expected 1 outbox event, got %d", len(envelopes))
			}
			env := envelopes[0]
			if env.GetEventType() != tc.expectedEvent {
				t.Errorf("expected event type %s, got %s", tc.expectedEvent, env.GetEventType())
			}
			if env.GetAggregateType() != "parcel" {
				t.Errorf("expected aggregate type 'parcel', got %s", env.GetAggregateType())
			}
			if env.GetAggregateID() != parcel.ID {
				t.Errorf("expected aggregate ID %s, got %s", parcel.ID, env.GetAggregateID())
			}
			if env.GetAggregateVersion() != 2 {
				t.Errorf("expected aggregate version 2, got %d", env.GetAggregateVersion())
			}

			// Verify serialized payload contains required lifecycle fields and preserves JSON delivery_attempts key
			payloadBytes, err := env.SerializePayload()
			if err != nil {
				t.Fatalf("failed to serialize outbox payload: %v", err)
			}
			var rawMap map[string]any
			if err := json.Unmarshal(payloadBytes, &rawMap); err != nil {
				t.Fatalf("failed to parse raw payload map: %v", err)
			}
			if rawMap["delivery_attempts"] == nil {
				t.Errorf("expected backward-compatible JSON key delivery_attempts in payload, got %s", string(payloadBytes))
			}
			var parsedPayload ParcelStatusChangedPayload
			if err := json.Unmarshal(payloadBytes, &parsedPayload); err != nil {
				t.Fatalf("failed to parse payload JSON: %v", err)
			}
			if parsedPayload.ParcelID != parcel.ID {
				t.Errorf("payload parcel_id mismatch: expected %s, got %s", parcel.ID, parsedPayload.ParcelID)
			}
			if parsedPayload.PreviousStatus != string(tc.initialStatus) {
				t.Errorf("payload previous_status mismatch: expected %s, got %s", tc.initialStatus, parsedPayload.PreviousStatus)
			}
			if parsedPayload.NewStatus != string(tc.targetStatus) {
				t.Errorf("payload new_status mismatch: expected %s, got %s", tc.targetStatus, parsedPayload.NewStatus)
			}
			if parsedPayload.AttemptCount != tc.expectedAttempts {
				t.Errorf("payload delivery_attempts mismatch: expected %d, got %d", tc.expectedAttempts, parsedPayload.AttemptCount)
			}
			if parsedPayload.ActorID == nil || *parsedPayload.ActorID != "usr-test-actor" {
				t.Errorf("payload actor_id mismatch: got %v", parsedPayload.ActorID)
			}

			// Verify transaction committed
			testTxMu.Lock()
			committed := lastTxCommitted
			rolledBack := lastTxRolledBack
			testTxMu.Unlock()
			if !committed || rolledBack {
				t.Errorf("expected transaction to commit cleanly, committed=%v, rolledBack=%v", committed, rolledBack)
			}
		})
	}
}

func TestTransitionParcelStatus_ParcelNotFound(t *testing.T) {
	db := getTestDB(t)
	txManager := NewDBTransactionManager(db)
	parcelRepo := newMockParcelRepo()
	historyRepo := newMockStatusHistoryRepo()
	outbox := eventstest.NewInMemoryWriter()
	clock := fixedClock{now: time.Now().UTC()}
	idGen := &mockIDGen{}

	uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, clock)

	cmd := TransitionParcelStatusCommand{
		ParcelID:     "018e3d60-6420-7f28-8d02-1a4e21a2c000",
		TargetStatus: domain.StatusAssigned,
	}

	_, err := uc.Execute(context.Background(), cmd)
	if !errors.Is(err, ErrParcelNotFound) {
		t.Fatalf("expected ErrParcelNotFound, got: %v", err)
	}

	testTxMu.Lock()
	rolledBack := lastTxRolledBack
	testTxMu.Unlock()
	if !rolledBack {
		t.Error("expected transaction to be rolled back on parcel not found")
	}
}

func TestTransitionParcelStatus_InvalidTransitions(t *testing.T) {
	invalidCases := []struct {
		name          string
		initialStatus domain.ParcelStatus
		targetStatus  domain.ParcelStatus
	}{
		{"CREATED to DELIVERED", domain.StatusCreated, domain.StatusDelivered},
		{"PICKED_UP to ASSIGNED", domain.StatusPickedUp, domain.StatusAssigned},
		{"IN_TRANSIT to DELIVERED (skipping OUT_FOR_DELIVERY)", domain.StatusInTransit, domain.StatusDelivered},
		{"Same status transition (CREATED to CREATED)", domain.StatusCreated, domain.StatusCreated},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			db := getTestDB(t)
			txManager := NewDBTransactionManager(db)
			parcelRepo := newMockParcelRepo()
			historyRepo := newMockStatusHistoryRepo()
			outbox := eventstest.NewInMemoryWriter()
			now := time.Now().UTC()
			clock := fixedClock{now: now}
			idGen := &mockIDGen{}

			parcel := createTestParcel(t, now, tc.initialStatus, 0, 1)
			parcelRepo.parcels[parcel.ID] = parcel

			uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, clock)

			cmd := TransitionParcelStatusCommand{
				ParcelID:     parcel.ID,
				TargetStatus: tc.targetStatus,
			}

			_, err := uc.Execute(context.Background(), cmd)
			if !errors.Is(err, domain.ErrIllegalStateTransition) {
				t.Fatalf("expected ErrIllegalStateTransition, got: %v", err)
			}

			testTxMu.Lock()
			rolledBack := lastTxRolledBack
			testTxMu.Unlock()
			if !rolledBack {
				t.Error("expected transaction to be rolled back on illegal transition")
			}
		})
	}
}

func TestTransitionParcelStatus_TerminalStateRejection(t *testing.T) {
	terminalStatuses := []domain.ParcelStatus{
		domain.StatusDelivered,
		domain.StatusReturned,
		domain.StatusCancelled,
	}

	for _, termStatus := range terminalStatuses {
		t.Run(string(termStatus), func(t *testing.T) {
			db := getTestDB(t)
			txManager := NewDBTransactionManager(db)
			parcelRepo := newMockParcelRepo()
			historyRepo := newMockStatusHistoryRepo()
			outbox := eventstest.NewInMemoryWriter()
			now := time.Now().UTC()
			clock := fixedClock{now: now}
			idGen := &mockIDGen{}

			parcel := createTestParcel(t, now, termStatus, 0, 1)
			parcelRepo.parcels[parcel.ID] = parcel

			uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, clock)

			cmd := TransitionParcelStatusCommand{
				ParcelID:     parcel.ID,
				TargetStatus: domain.StatusOutForDelivery,
			}

			_, err := uc.Execute(context.Background(), cmd)
			if !errors.Is(err, domain.ErrTerminalState) {
				t.Fatalf("expected ErrTerminalState for %s, got: %v", termStatus, err)
			}

			testTxMu.Lock()
			rolledBack := lastTxRolledBack
			testTxMu.Unlock()
			if !rolledBack {
				t.Error("expected transaction to be rolled back on terminal state transition attempt")
			}
		})
	}
}

func TestTransitionParcelStatus_CancellationAfterCreatedRejection(t *testing.T) {
	nonCreatedStatuses := []domain.ParcelStatus{
		domain.StatusAssigned,
		domain.StatusPickedUp,
		domain.StatusInTransit,
		domain.StatusOutForDelivery,
		domain.StatusAttemptFailed,
	}

	for _, s := range nonCreatedStatuses {
		t.Run("Cancel from "+string(s), func(t *testing.T) {
			db := getTestDB(t)
			txManager := NewDBTransactionManager(db)
			parcelRepo := newMockParcelRepo()
			historyRepo := newMockStatusHistoryRepo()
			outbox := eventstest.NewInMemoryWriter()
			now := time.Now().UTC()
			clock := fixedClock{now: now}
			idGen := &mockIDGen{}

			parcel := createTestParcel(t, now, s, 0, 1)
			parcelRepo.parcels[parcel.ID] = parcel

			uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, clock)

			cmd := TransitionParcelStatusCommand{
				ParcelID:     parcel.ID,
				TargetStatus: domain.StatusCancelled,
				Reason:       "Customer requested cancellation",
			}

			_, err := uc.Execute(context.Background(), cmd)
			if !errors.Is(err, domain.ErrCancellationNotAllowed) {
				t.Fatalf("expected ErrCancellationNotAllowed when cancelling from %s, got: %v", s, err)
			}

			testTxMu.Lock()
			rolledBack := lastTxRolledBack
			testTxMu.Unlock()
			if !rolledBack {
				t.Error("expected transaction rollback on illegal cancellation")
			}
		})
	}
}

func TestTransitionParcelStatus_ThirdDeliveryAttemptBehavior(t *testing.T) {
	db := getTestDB(t)
	txManager := NewDBTransactionManager(db)
	parcelRepo := newMockParcelRepo()
	historyRepo := newMockStatusHistoryRepo()
	outbox := eventstest.NewInMemoryWriter()
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.UTC)
	clock := fixedClock{now: now}
	idGen := &mockIDGen{}

	// Setup: parcel currently at ATTEMPT_FAILED with already 2 prior failed attempts.
	parcel := createTestParcel(t, now, domain.StatusAttemptFailed, 2, 5)
	parcelRepo.parcels[parcel.ID] = parcel

	uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, clock)

	// Step 1: 3rd delivery run begins (ATTEMPT_FAILED -> OUT_FOR_DELIVERY is permitted because attemptCount is 2 < 3)
	cmdRun3 := TransitionParcelStatusCommand{
		ParcelID:     parcel.ID,
		TargetStatus: domain.StatusOutForDelivery,
	}
	resRun3, err := uc.Execute(context.Background(), cmdRun3)
	if err != nil {
		t.Fatalf("expected 3rd run to OUT_FOR_DELIVERY to succeed, got: %v", err)
	}
	if resRun3.Parcel.Status != domain.StatusOutForDelivery {
		t.Fatalf("expected OUT_FOR_DELIVERY, got: %s", resRun3.Parcel.Status)
	}

	// Step 2: 3rd delivery attempt FAILS (OUT_FOR_DELIVERY -> ATTEMPT_FAILED increments attempts from 2 to 3)
	cmdFail3 := TransitionParcelStatusCommand{
		ParcelID:     parcel.ID,
		TargetStatus: domain.StatusAttemptFailed,
		Reason:       "Recipient unavailable after 3 attempts",
	}
	resFail3, err := uc.Execute(context.Background(), cmdFail3)
	if err != nil {
		t.Fatalf("expected recording 3rd failed attempt to succeed, got: %v", err)
	}
	if resFail3.Parcel.AttemptCount != 3 {
		t.Fatalf("expected attempt count to equal 3, got: %d", resFail3.Parcel.AttemptCount)
	}

	// Step 3: Attempting a 4th delivery run (ATTEMPT_FAILED -> OUT_FOR_DELIVERY) must FAIL!
	cmdRun4 := TransitionParcelStatusCommand{
		ParcelID:     parcel.ID,
		TargetStatus: domain.StatusOutForDelivery,
	}
	_, err = uc.Execute(context.Background(), cmdRun4)
	if !errors.Is(err, domain.ErrMaxDeliveryAttemptsExceeded) {
		t.Fatalf("expected ErrMaxDeliveryAttemptsExceeded for 4th reattempt, got: %v", err)
	}

	// Step 4: Transition to terminal RETURNED must SUCCEED!
	cmdReturn := TransitionParcelStatusCommand{
		ParcelID:     parcel.ID,
		TargetStatus: domain.StatusReturned,
		Reason:       "Max attempts reached; package returned to sender",
	}
	resReturn, err := uc.Execute(context.Background(), cmdReturn)
	if err != nil {
		t.Fatalf("expected transition to RETURNED to succeed, got: %v", err)
	}
	if resReturn.Parcel.Status != domain.StatusReturned {
		t.Fatalf("expected RETURNED status, got: %s", resReturn.Parcel.Status)
	}
	if !resReturn.Parcel.Status.IsTerminal() {
		t.Fatal("expected RETURNED to be recognized as terminal status")
	}
}

func TestTransitionParcelStatus_ConcurrencyConflict_ExpectedVersionMismatch(t *testing.T) {
	db := getTestDB(t)
	txManager := NewDBTransactionManager(db)
	parcelRepo := newMockParcelRepo()
	historyRepo := newMockStatusHistoryRepo()
	outbox := eventstest.NewInMemoryWriter()
	now := time.Now().UTC()
	clock := fixedClock{now: now}
	idGen := &mockIDGen{}

	parcel := createTestParcel(t, now, domain.StatusCreated, 0, 1)
	parcelRepo.parcels[parcel.ID] = parcel

	uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, clock)

	// Caller provides stale expected version 99
	staleVersion := 99
	cmd := TransitionParcelStatusCommand{
		ParcelID:        parcel.ID,
		TargetStatus:    domain.StatusAssigned,
		ExpectedVersion: &staleVersion,
	}

	_, err := uc.Execute(context.Background(), cmd)
	if !errors.Is(err, ErrConcurrencyConflict) {
		t.Fatalf("expected ErrConcurrencyConflict for stale version, got: %v", err)
	}

	testTxMu.Lock()
	rolledBack := lastTxRolledBack
	testTxMu.Unlock()
	if !rolledBack {
		t.Error("expected transaction rollback on version conflict")
	}
}

func TestTransitionParcelStatus_ConcurrencyConflict_OptimisticRepoZeroRows(t *testing.T) {
	db := getTestDB(t)
	txManager := NewDBTransactionManager(db)
	parcelRepo := newMockParcelRepo()
	historyRepo := newMockStatusHistoryRepo()
	outbox := eventstest.NewInMemoryWriter()
	now := time.Now().UTC()
	clock := fixedClock{now: now}
	idGen := &mockIDGen{}

	parcel := createTestParcel(t, now, domain.StatusCreated, 0, 1)
	parcelRepo.parcels[parcel.ID] = parcel

	// Simulate concurrent modification in DB between read and update
	parcelRepo.updateErr = ErrConcurrencyConflict

	uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, clock)

	cmd := TransitionParcelStatusCommand{
		ParcelID:     parcel.ID,
		TargetStatus: domain.StatusAssigned,
	}

	_, err := uc.Execute(context.Background(), cmd)
	if !errors.Is(err, ErrConcurrencyConflict) {
		t.Fatalf("expected ErrConcurrencyConflict from repo update, got: %v", err)
	}

	testTxMu.Lock()
	rolledBack := lastTxRolledBack
	testTxMu.Unlock()
	if !rolledBack {
		t.Error("expected transaction rollback on optimistic conflict")
	}
}

func TestTransitionParcelStatus_ParcelUpdateFailure(t *testing.T) {
	db := getTestDB(t)
	txManager := NewDBTransactionManager(db)
	parcelRepo := newMockParcelRepo()
	historyRepo := newMockStatusHistoryRepo()
	outbox := eventstest.NewInMemoryWriter()
	now := time.Now().UTC()
	clock := fixedClock{now: now}
	idGen := &mockIDGen{}

	parcel := createTestParcel(t, now, domain.StatusCreated, 0, 1)
	parcelRepo.parcels[parcel.ID] = parcel
	parcelRepo.updateErr = errors.New("disk full / db connection severed")

	uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, clock)

	cmd := TransitionParcelStatusCommand{
		ParcelID:     parcel.ID,
		TargetStatus: domain.StatusAssigned,
	}

	_, err := uc.Execute(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	testTxMu.Lock()
	rolledBack := lastTxRolledBack
	testTxMu.Unlock()
	if !rolledBack {
		t.Error("expected transaction rollback on parcel update failure")
	}
}

func TestTransitionParcelStatus_StatusHistoryInsertionFailure(t *testing.T) {
	db := getTestDB(t)
	txManager := NewDBTransactionManager(db)
	parcelRepo := newMockParcelRepo()
	historyRepo := newMockStatusHistoryRepo()
	historyRepo.saveErr = errors.New("status history table lock timeout")
	outbox := eventstest.NewInMemoryWriter()
	now := time.Now().UTC()
	clock := fixedClock{now: now}
	idGen := &mockIDGen{}

	parcel := createTestParcel(t, now, domain.StatusCreated, 0, 1)
	parcelRepo.parcels[parcel.ID] = parcel

	uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, clock)

	cmd := TransitionParcelStatusCommand{
		ParcelID:     parcel.ID,
		TargetStatus: domain.StatusAssigned,
	}

	_, err := uc.Execute(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	testTxMu.Lock()
	rolledBack := lastTxRolledBack
	testTxMu.Unlock()
	if !rolledBack {
		t.Error("expected transaction rollback on status history failure")
	}
}

func TestTransitionParcelStatus_OutboxWriteFailure(t *testing.T) {
	db := getTestDB(t)
	txManager := NewDBTransactionManager(db)
	parcelRepo := newMockParcelRepo()
	historyRepo := newMockStatusHistoryRepo()
	outbox := &errOutboxWriter{err: errors.New("outbox table deadlocked")}
	now := time.Now().UTC()
	clock := fixedClock{now: now}
	idGen := &mockIDGen{}

	parcel := createTestParcel(t, now, domain.StatusCreated, 0, 1)
	parcelRepo.parcels[parcel.ID] = parcel

	uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, clock)

	cmd := TransitionParcelStatusCommand{
		ParcelID:     parcel.ID,
		TargetStatus: domain.StatusAssigned,
	}

	_, err := uc.Execute(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	testTxMu.Lock()
	rolledBack := lastTxRolledBack
	testTxMu.Unlock()
	if !rolledBack {
		t.Error("expected transaction rollback on outbox failure")
	}
}

func TestTransitionParcelStatus_ExecuteTx_Integration(t *testing.T) {
	db := getTestDB(t)
	parcelRepo := newMockParcelRepo()
	historyRepo := newMockStatusHistoryRepo()
	outbox := eventstest.NewInMemoryWriter()
	now := time.Now().UTC()
	clock := fixedClock{now: now}
	idGen := &mockIDGen{}

	parcel := createTestParcel(t, now, domain.StatusCreated, 0, 1)
	parcelRepo.parcels[parcel.ID] = parcel

	txManager := NewDBTransactionManager(db)
	uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, clock)

	// Caller (e.g. Member 3 dispatch) opens transaction and passes tx to ExecuteTx
	err := txManager.WithTx(context.Background(), func(tx *sql.Tx) error {
		cmd := TransitionParcelStatusCommand{
			ParcelID:     parcel.ID,
			TargetStatus: domain.StatusAssigned,
			ActorID:      strPtr("courier-001"),
			ActorType:    domain.ActorTypeUser,
			ActorRole:    "DISPATCHER",
			Reason:       "Assigned via Dispatcher Board",
		}
		res, err := uc.ExecuteTx(context.Background(), tx, cmd)
		if err != nil {
			return err
		}
		if res.Parcel.Status != domain.StatusAssigned {
			t.Errorf("expected ASSIGNED, got %s", res.Parcel.Status)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected ExecuteTx error: %v", err)
	}

	// Verify all changes persisted together
	if parcelRepo.parcels[parcel.ID].Status != domain.StatusAssigned {
		t.Errorf("expected parcel repo to reflect ASSIGNED")
	}
	if len(historyRepo.entries) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(historyRepo.entries))
	}
	if len(outbox.Events()) != 1 {
		t.Errorf("expected 1 outbox envelope, got %d", len(outbox.Events()))
	}
}

func TestTransitionParcelStatus_ValidationErrors(t *testing.T) {
	db := getTestDB(t)
	txManager := NewDBTransactionManager(db)
	parcelRepo := newMockParcelRepo()
	historyRepo := newMockStatusHistoryRepo()
	outbox := eventstest.NewInMemoryWriter()
	clock := fixedClock{now: time.Now().UTC()}
	idGen := &mockIDGen{}

	uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, clock)

	// Blank parcel ID
	_, err := uc.Execute(context.Background(), TransitionParcelStatusCommand{
		ParcelID:     "",
		TargetStatus: domain.StatusAssigned,
	})
	if !errors.Is(err, domain.ErrInvalidParcelID) {
		t.Errorf("expected ErrInvalidParcelID, got: %v", err)
	}

	// Invalid target status
	_, err = uc.Execute(context.Background(), TransitionParcelStatusCommand{
		ParcelID:     "some-id",
		TargetStatus: domain.ParcelStatus("UNKNOWN_STATUS"),
	})
	if !errors.Is(err, domain.ErrInvalidParcelStatus) {
		t.Errorf("expected ErrInvalidParcelStatus, got: %v", err)
	}

	// Nil constructor checks
	if _, err := NewTransitionParcelStatusUseCase(nil, parcelRepo, historyRepo, outbox, idGen, clock); !errors.Is(err, ErrNilTransactionManager) {
		t.Errorf("expected ErrNilTransactionManager, got: %v", err)
	}
	if _, err := NewTransitionParcelStatusUseCase(txManager, nil, historyRepo, outbox, idGen, clock); !errors.Is(err, ErrNilParcelRepository) {
		t.Errorf("expected ErrNilParcelRepository, got: %v", err)
	}
	if _, err := NewTransitionParcelStatusUseCase(txManager, parcelRepo, nil, outbox, idGen, clock); !errors.Is(err, ErrNilStatusHistoryRepository) {
		t.Errorf("expected ErrNilStatusHistoryRepository, got: %v", err)
	}
	if _, err := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, nil, idGen, clock); !errors.Is(err, ErrNilOutboxWriter) {
		t.Errorf("expected ErrNilOutboxWriter, got: %v", err)
	}
}

func TestTransitionParcelStatus_ActorMetadataAndMerchantParcel(t *testing.T) {
	now := time.Date(2026, 11, 8, 15, 0, 0, 0, time.UTC)

	t.Run("SYSTEM transition uses nil ActorID and omits actor_id in JSON", func(t *testing.T) {
		db := getTestDB(t)
		txManager := NewDBTransactionManager(db)
		parcelRepo := newMockParcelRepo()
		historyRepo := newMockStatusHistoryRepo()
		outbox := eventstest.NewInMemoryWriter()
		uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, &mockIDGen{}, fixedClock{now: now})

		parcel := createTestParcel(t, now, domain.StatusAttemptFailed, 3, 2)
		parcelRepo.parcels[parcel.ID] = parcel

		res, err := uc.Execute(context.Background(), TransitionParcelStatusCommand{
			ParcelID:     parcel.ID,
			TargetStatus: domain.StatusReturned,
			ActorID:      nil,
			ActorType:    domain.ActorTypeSystem,
		})
		if err != nil {
			t.Fatalf("expected SYSTEM transition with nil ActorID to succeed, got: %v", err)
		}
		if res.StatusHistory.ActorID != nil {
			t.Errorf("expected nil ActorID, got %v", *res.StatusHistory.ActorID)
		}
		if res.StatusHistory.ActorType != domain.ActorTypeSystem {
			t.Errorf("expected ActorType SYSTEM, got %s", res.StatusHistory.ActorType)
		}
		if res.StatusHistory.ActorRole != "SYSTEM" {
			t.Errorf("expected ActorRole SYSTEM, got %s", res.StatusHistory.ActorRole)
		}

		payloadBytes, err := outbox.Events()[0].SerializePayload()
		if err != nil {
			t.Fatalf("serialize failed: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(payloadBytes, &raw); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if _, exists := raw["actor_id"]; exists {
			t.Errorf("expected actor_id to be omitted for nil SYSTEM actor, got %v", raw["actor_id"])
		}
	})

	t.Run("MERCHANT_API transition succeeds with merchant ActorID and default MERCHANT role", func(t *testing.T) {
		db := getTestDB(t)
		txManager := NewDBTransactionManager(db)
		parcelRepo := newMockParcelRepo()
		historyRepo := newMockStatusHistoryRepo()
		outbox := eventstest.NewInMemoryWriter()
		uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, &mockIDGen{}, fixedClock{now: now})

		parcel := createTestParcel(t, now, domain.StatusCreated, 0, 1)
		parcel.SenderID = nil
		parcel.MerchantID = strPtr("mrc-api-001")
		parcelRepo.parcels[parcel.ID] = parcel

		res, err := uc.Execute(context.Background(), TransitionParcelStatusCommand{
			ParcelID:     parcel.ID,
			TargetStatus: domain.StatusCancelled,
			ActorID:      strPtr(" mrc-api-001 "),
			ActorType:    domain.ActorTypeMerchantAPI,
			Reason:       "Cancelled by merchant integration",
		})
		if err != nil {
			t.Fatalf("expected MERCHANT_API cancellation to succeed, got: %v", err)
		}
		if res.Parcel.SenderID != nil {
			t.Errorf("expected nil SenderID on merchant-only parcel, got %v", res.Parcel.SenderID)
		}
		if res.StatusHistory.ActorType != domain.ActorTypeMerchantAPI {
			t.Errorf("expected ActorType MERCHANT_API, got %s", res.StatusHistory.ActorType)
		}
		if res.StatusHistory.ActorID == nil || *res.StatusHistory.ActorID != "mrc-api-001" {
			t.Errorf("expected trimmed ActorID mrc-api-001, got %v", res.StatusHistory.ActorID)
		}
		if res.StatusHistory.ActorRole != "MERCHANT" {
			t.Errorf("expected ActorRole MERCHANT, got %s", res.StatusHistory.ActorRole)
		}
	})

	t.Run("invalid actor metadata is rejected and rolls back transaction", func(t *testing.T) {
		tests := []struct {
			name      string
			actorID   *string
			actorType domain.ActorType
			wantErr   error
		}{
			{
				name:      "literal SYSTEM ActorID with SYSTEM ActorType rejected",
				actorID:   strPtr("SYSTEM"),
				actorType: domain.ActorTypeSystem,
				wantErr:   domain.ErrInvalidActorID,
			},
			{
				name:      "literal SYSTEM ActorID with USER ActorType rejected",
				actorID:   strPtr("SYSTEM"),
				actorType: domain.ActorTypeUser,
				wantErr:   domain.ErrInvalidActorID,
			},
			{
				name:      "USER with nil ActorID rejected",
				actorID:   nil,
				actorType: domain.ActorTypeUser,
				wantErr:   domain.ErrInvalidActorID,
			},
			{
				name:      "MERCHANT_API with whitespace ActorID rejected",
				actorID:   strPtr("   "),
				actorType: domain.ActorTypeMerchantAPI,
				wantErr:   domain.ErrInvalidActorID,
			},
			{
				name:      "unsupported ActorType rejected",
				actorID:   strPtr("bot-1"),
				actorType: domain.ActorType("BOT"),
				wantErr:   domain.ErrInvalidActorType,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				db := getTestDB(t)
				txManager := NewDBTransactionManager(db)
				parcelRepo := newMockParcelRepo()
				historyRepo := newMockStatusHistoryRepo()
				outbox := eventstest.NewInMemoryWriter()
				uc, _ := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, &mockIDGen{}, fixedClock{now: now})

				parcel := createTestParcel(t, now, domain.StatusCreated, 0, 1)
				parcelRepo.parcels[parcel.ID] = parcel

				_, err := uc.Execute(context.Background(), TransitionParcelStatusCommand{
					ParcelID:     parcel.ID,
					TargetStatus: domain.StatusCancelled,
					ActorID:      tc.actorID,
					ActorType:    tc.actorType,
				})
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error wrapping %v, got: %v", tc.wantErr, err)
				}
				if parcelRepo.updateCalled || historyRepo.saveCalled || outbox.Count() > 0 {
					t.Error("expected no persistence on invalid actor transition")
				}
				testTxMu.Lock()
				rolledBack := lastTxRolledBack
				testTxMu.Unlock()
				if !rolledBack {
					t.Error("expected transaction rollback on actor validation failure")
				}
			})
		}
	})
}
