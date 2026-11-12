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
	"github.com/swiftdrop/core-api/internal/platform/db"
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
			if !events.IsValidUUID(result.StatusHistory.ID) {
				t.Errorf("expected valid UUID status history ID, got %q", result.StatusHistory.ID)
			}
			if len(result.StatusHistory.ID) == 36 && result.StatusHistory.ID[14] != '7' {
				t.Errorf("expected UUIDv7 status history ID, got %q", result.StatusHistory.ID)
			}
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
			if historyRepo.entries[0].ID != result.StatusHistory.ID {
				t.Errorf("persisted history ID mismatch: expected %s, got %s", result.StatusHistory.ID, historyRepo.entries[0].ID)
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
			if !events.IsValidUUID(env.GetEventID()) {
				t.Errorf("expected valid UUID envelope event_id, got %q", env.GetEventID())
			}
			if env.GetEventID() == result.StatusHistory.ID {
				t.Errorf("expected distinct envelope event_id and status_history_id, both were %s", env.GetEventID())
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
			if rawMap["status_history_id"] != result.StatusHistory.ID {
				t.Errorf("expected JSON key status_history_id=%q, got %v", result.StatusHistory.ID, rawMap["status_history_id"])
			}
			var parsedPayload ParcelStatusChangedPayload
			if err := json.Unmarshal(payloadBytes, &parsedPayload); err != nil {
				t.Fatalf("failed to parse payload JSON: %v", err)
			}
			if parsedPayload.ParcelID != parcel.ID {
				t.Errorf("payload parcel_id mismatch: expected %s, got %s", parcel.ID, parsedPayload.ParcelID)
			}
			if parsedPayload.StatusHistoryID != result.StatusHistory.ID {
				t.Errorf("payload status_history_id mismatch: expected %s, got %s", result.StatusHistory.ID, parsedPayload.StatusHistoryID)
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

	var txResult TransitionParcelStatusResult
	var usedTx *sql.Tx

	// Caller (e.g. Member 3 dispatch) opens transaction and passes tx to ExecuteTx
	err := txManager.WithTx(context.Background(), func(tx *sql.Tx) error {
		usedTx = tx
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
		txResult = res
		if res.Parcel.Status != domain.StatusAssigned {
			t.Errorf("expected ASSIGNED, got %s", res.Parcel.Status)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected ExecuteTx error: %v", err)
	}

	// Verify all changes persisted together
	if usedTx == nil {
		t.Fatal("expected non-nil transaction")
	}
	if parcelRepo.parcels[parcel.ID].Status != domain.StatusAssigned {
		t.Errorf("expected parcel repo to reflect ASSIGNED")
	}
	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(historyRepo.entries))
	}
	if !events.IsValidUUID(txResult.StatusHistory.ID) || historyRepo.entries[0].ID != txResult.StatusHistory.ID {
		t.Errorf("expected matching valid UUID status history ID, result=%q persisted=%q", txResult.StatusHistory.ID, historyRepo.entries[0].ID)
	}
	envelopes := outbox.Events()
	if len(envelopes) != 1 {
		t.Fatalf("expected 1 outbox envelope, got %d", len(envelopes))
	}
	payloadBytes, err := envelopes[0].SerializePayload()
	if err != nil {
		t.Fatalf("failed to serialize payload: %v", err)
	}
	var parsed ParcelStatusChangedPayload
	if err := json.Unmarshal(payloadBytes, &parsed); err != nil {
		t.Fatalf("failed to parse payload: %v", err)
	}
	if parsed.StatusHistoryID != txResult.StatusHistory.ID {
		t.Errorf("expected payload status_history_id %q, got %q", txResult.StatusHistory.ID, parsed.StatusHistoryID)
	}
	if envelopes[0].GetEventID() == parsed.StatusHistoryID {
		t.Errorf("expected distinct event_id and status_history_id, both were %s", envelopes[0].GetEventID())
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

type failingIDGen struct {
	id  string
	err error
}

func (f *failingIDGen) GenerateID() (string, error) {
	return f.id, f.err
}

type countingIDGen struct {
	calls int
	inner IDGenerator
}

func (c *countingIDGen) GenerateID() (string, error) {
	c.calls++
	return c.inner.GenerateID()
}

type capturingOutboxWriter struct {
	txs       []*sql.Tx
	envelopes []events.AnyEnvelope
}

func (w *capturingOutboxWriter) Write(ctx context.Context, tx *sql.Tx, envelopes ...events.AnyEnvelope) error {
	if tx == nil {
		return events.ErrMissingTransaction
	}
	w.txs = append(w.txs, tx)
	w.envelopes = append(w.envelopes, envelopes...)
	return nil
}

type capturingParcelRepo struct {
	*mockParcelRepo
	updateQueriers []db.Querier
}

func (r *capturingParcelRepo) UpdateStatus(ctx context.Context, q db.Querier, parcel domain.Parcel, expectedVersion int) error {
	r.updateQueriers = append(r.updateQueriers, q)
	return r.mockParcelRepo.UpdateStatus(ctx, q, parcel, expectedVersion)
}

type capturingStatusHistoryRepo struct {
	*mockStatusHistoryRepo
	saveQueriers []db.Querier
}

func (r *capturingStatusHistoryRepo) Save(ctx context.Context, q db.Querier, entry domain.StatusHistoryEntry) error {
	r.saveQueriers = append(r.saveQueriers, q)
	return r.mockStatusHistoryRepo.Save(ctx, q, entry)
}

func TestTransitionParcelStatus_StatusHistoryIDCorrelation(t *testing.T) {
	now := time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)

	t.Run("generates one UUID persisted to history and payload status_history_id in same tx while preserving event_id", func(t *testing.T) {
		database := getTestDB(t)
		defer database.Close()

		const expectedHistoryID = "01936f2e-1b2c-7a3d-8e4f-112233445566"

		txManager := NewDBTransactionManager(database)
		parcelRepo := &capturingParcelRepo{mockParcelRepo: newMockParcelRepo()}
		historyRepo := &capturingStatusHistoryRepo{mockStatusHistoryRepo: newMockStatusHistoryRepo()}
		outbox := &capturingOutboxWriter{}
		idGen := &countingIDGen{inner: &mockIDGen{ids: []string{" " + expectedHistoryID + " "}}}

		parcel := createTestParcel(t, now, domain.StatusCreated, 0, 1)
		parcelRepo.parcels[parcel.ID] = parcel

		uc, err := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, idGen, fixedClock{now: now})
		if err != nil {
			t.Fatalf("failed to construct use case: %v", err)
		}

		res, err := uc.Execute(context.Background(), TransitionParcelStatusCommand{
			ParcelID:      parcel.ID,
			TargetStatus:  domain.StatusAssigned,
			ActorID:       strPtr("usr-disp-001"),
			ActorType:     domain.ActorTypeUser,
			CorrelationID: "corr-correlation-001",
		})
		if err != nil {
			t.Fatalf("unexpected transition error: %v", err)
		}

		if idGen.calls != 1 {
			t.Errorf("expected exactly 1 status history ID generation call, got %d", idGen.calls)
		}
		if res.StatusHistory.ID != expectedHistoryID {
			t.Errorf("expected returned status history ID %q, got %q", expectedHistoryID, res.StatusHistory.ID)
		}
		if len(historyRepo.entries) != 1 || historyRepo.entries[0].ID != expectedHistoryID {
			t.Fatalf("expected 1 persisted history row with ID %q, got %+v", expectedHistoryID, historyRepo.entries)
		}
		if len(outbox.envelopes) != 1 {
			t.Fatalf("expected 1 outbox envelope, got %d", len(outbox.envelopes))
		}

		env := outbox.envelopes[0]
		if !events.IsValidUUID(env.GetEventID()) {
			t.Errorf("expected valid UUID envelope event_id, got %q", env.GetEventID())
		}
		if env.GetEventID() == expectedHistoryID {
			t.Errorf("expected envelope event_id to remain independent from status_history_id %q", expectedHistoryID)
		}
		if env.GetSchemaVersion() != "1.0.0" {
			t.Errorf("expected schema_version 1.0.0, got %q", env.GetSchemaVersion())
		}

		payloadBytes, err := env.SerializePayload()
		if err != nil {
			t.Fatalf("failed to serialize payload: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(payloadBytes, &raw); err != nil {
			t.Fatalf("failed to unmarshal payload JSON: %v", err)
		}
		if raw["status_history_id"] != expectedHistoryID {
			t.Errorf("expected serialized payload.status_history_id=%q, got %v", expectedHistoryID, raw["status_history_id"])
		}

		var parsed ParcelStatusChangedPayload
		if err := json.Unmarshal(payloadBytes, &parsed); err != nil {
			t.Fatalf("failed to unmarshal ParcelStatusChangedPayload: %v", err)
		}
		if parsed.StatusHistoryID != expectedHistoryID {
			t.Errorf("expected ParcelStatusChangedPayload.StatusHistoryID=%q, got %q", expectedHistoryID, parsed.StatusHistoryID)
		}

		// Verify parcel update, history save, and outbox write all shared the exact same *sql.Tx
		if len(parcelRepo.updateQueriers) != 1 || len(historyRepo.saveQueriers) != 1 || len(outbox.txs) != 1 {
			t.Fatalf("expected 1 call each for update/history/outbox, got %d/%d/%d",
				len(parcelRepo.updateQueriers), len(historyRepo.saveQueriers), len(outbox.txs))
		}
		tx := outbox.txs[0]
		if tx == nil {
			t.Fatal("expected non-nil transaction in outbox write")
		}
		if parcelRepo.updateQueriers[0] != tx {
			t.Error("expected parcel UpdateStatus and OutboxWriter.Write to share the same *sql.Tx")
		}
		if historyRepo.saveQueriers[0] != tx {
			t.Error("expected statusHistory Save and OutboxWriter.Write to share the same *sql.Tx")
		}
	})

	t.Run("default UUIDv7 generator creates valid correlated UUIDv7 when idGen is nil", func(t *testing.T) {
		database := getTestDB(t)
		defer database.Close()

		txManager := NewDBTransactionManager(database)
		parcelRepo := newMockParcelRepo()
		historyRepo := newMockStatusHistoryRepo()
		outbox := eventstest.NewInMemoryWriter()

		parcel := createTestParcel(t, now, domain.StatusAssigned, 0, 2)
		parcelRepo.parcels[parcel.ID] = parcel

		uc, err := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, nil, fixedClock{now: now})
		if err != nil {
			t.Fatalf("failed to construct use case with default idGen: %v", err)
		}

		res, err := uc.Execute(context.Background(), TransitionParcelStatusCommand{
			ParcelID:     parcel.ID,
			TargetStatus: domain.StatusPickedUp,
			ActorID:      strPtr("usr-courier-001"),
			ActorType:    domain.ActorTypeUser,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !events.IsValidUUID(res.StatusHistory.ID) || res.StatusHistory.ID[14] != '7' {
			t.Errorf("expected RFC 9562 UUIDv7 status history ID, got %q", res.StatusHistory.ID)
		}
		if len(historyRepo.entries) != 1 || historyRepo.entries[0].ID != res.StatusHistory.ID {
			t.Fatalf("persisted status history ID mismatch: %+v", historyRepo.entries)
		}
		env := outbox.Events()[0]
		payloadBytes, err := env.SerializePayload()
		if err != nil {
			t.Fatalf("failed to serialize payload: %v", err)
		}
		var parsed ParcelStatusChangedPayload
		if err := json.Unmarshal(payloadBytes, &parsed); err != nil {
			t.Fatalf("failed to unmarshal payload: %v", err)
		}
		if parsed.StatusHistoryID != res.StatusHistory.ID {
			t.Errorf("expected correlated status_history_id %q, got %q", res.StatusHistory.ID, parsed.StatusHistoryID)
		}
		if !events.IsValidUUID(env.GetEventID()) || env.GetEventID() == parsed.StatusHistoryID {
			t.Errorf("expected distinct valid event_id (%q) != status_history_id (%q)", env.GetEventID(), parsed.StatusHistoryID)
		}
	})
}

func TestTransitionParcelStatus_InvalidGeneratedHistoryID(t *testing.T) {
	now := time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)
	genErr := errors.New("entropy source unavailable")

	tests := []struct {
		name      string
		idGen     IDGenerator
		wantErrIs error
	}{
		{
			name:      "generator error aborts transition before persistence",
			idGen:     &failingIDGen{err: genErr},
			wantErrIs: genErr,
		},
		{
			name:      "blank generated id rejected",
			idGen:     &failingIDGen{id: "   "},
			wantErrIs: domain.ErrInvalidStatusHistory,
		},
		{
			name:      "non-UUID generated id rejected",
			idGen:     &failingIDGen{id: "hist-not-a-uuid"},
			wantErrIs: domain.ErrInvalidStatusHistory,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			database := getTestDB(t)
			defer database.Close()

			txManager := NewDBTransactionManager(database)
			parcelRepo := newMockParcelRepo()
			historyRepo := newMockStatusHistoryRepo()
			outbox := eventstest.NewInMemoryWriter()

			parcel := createTestParcel(t, now, domain.StatusCreated, 0, 1)
			parcelRepo.parcels[parcel.ID] = parcel

			uc, err := NewTransitionParcelStatusUseCase(txManager, parcelRepo, historyRepo, outbox, tc.idGen, fixedClock{now: now})
			if err != nil {
				t.Fatalf("failed to construct use case: %v", err)
			}

			_, err = uc.Execute(context.Background(), TransitionParcelStatusCommand{
				ParcelID:     parcel.ID,
				TargetStatus: domain.StatusAssigned,
				ActorID:      strPtr("usr-disp-001"),
				ActorType:    domain.ActorTypeUser,
			})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tc.wantErrIs) {
				t.Fatalf("expected error wrapping %v, got %v", tc.wantErrIs, err)
			}
			if parcelRepo.updateCalled || historyRepo.saveCalled || outbox.Count() > 0 {
				t.Error("expected zero parcel/history/outbox writes when history ID generation is invalid")
			}
			testTxMu.Lock()
			rolledBack := lastTxRolledBack
			testTxMu.Unlock()
			if !rolledBack {
				t.Error("expected transaction rollback on invalid generated history ID")
			}
		})
	}
}
