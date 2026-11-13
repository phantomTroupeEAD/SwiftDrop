package application_test

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/delivery/application"
	"github.com/swiftdrop/core-api/internal/delivery/domain"
)

// MockTransactionManager executes callbacks directly without real database connections.
type MockTransactionManager struct{}

func (m *MockTransactionManager) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return fn(nil)
}

// MockActiveAssignmentFinder tracks active courier assignments in-memory.
type MockActiveAssignmentFinder struct {
	assignments map[uuid.UUID]*application.ActiveAssignmentDTO
	completed   map[uuid.UUID]bool
}

func NewMockActiveAssignmentFinder() *MockActiveAssignmentFinder {
	return &MockActiveAssignmentFinder{
		assignments: make(map[uuid.UUID]*application.ActiveAssignmentDTO),
		completed:   make(map[uuid.UUID]bool),
	}
}

func (m *MockActiveAssignmentFinder) GetActiveAssignment(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*application.ActiveAssignmentDTO, error) {
	return m.assignments[parcelID], nil
}

func (m *MockActiveAssignmentFinder) CompleteAssignment(ctx context.Context, tx *sql.Tx, assignmentID uuid.UUID) error {
	m.completed[assignmentID] = true
	return nil
}

// MockDeliveryAttemptRepository tracks attempt records in-memory.
type MockDeliveryAttemptRepository struct {
	attempts map[uuid.UUID][]*domain.DeliveryAttempt
}

func NewMockDeliveryAttemptRepository() *MockDeliveryAttemptRepository {
	return &MockDeliveryAttemptRepository{
		attempts: make(map[uuid.UUID][]*domain.DeliveryAttempt),
	}
}

func (m *MockDeliveryAttemptRepository) InsertAttempt(ctx context.Context, tx *sql.Tx, attempt *domain.DeliveryAttempt) error {
	m.attempts[attempt.ParcelID] = append(m.attempts[attempt.ParcelID], attempt)
	return nil
}

func (m *MockDeliveryAttemptRepository) ListAttemptsByParcelID(ctx context.Context, parcelID uuid.UUID) ([]*domain.DeliveryAttempt, error) {
	return m.attempts[parcelID], nil
}

func (m *MockDeliveryAttemptRepository) GetAttemptCount(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (int, error) {
	return len(m.attempts[parcelID]), nil
}

// MockParcelLifecycleUpdater tracks parcel status updates.
type MockParcelLifecycleUpdater struct {
	statusMap   map[uuid.UUID]string
	attemptsMap map[uuid.UUID]int
	lastStatus  string
}

func NewMockParcelLifecycleUpdater() *MockParcelLifecycleUpdater {
	return &MockParcelLifecycleUpdater{
		statusMap:   make(map[uuid.UUID]string),
		attemptsMap: make(map[uuid.UUID]int),
	}
}

func (m *MockParcelLifecycleUpdater) TransitionStatus(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID, fromStatus, toStatus string, actorID uuid.UUID, actorRole, note string) error {
	m.statusMap[parcelID] = toStatus
	m.lastStatus = toStatus
	return nil
}

func (m *MockParcelLifecycleUpdater) GetStatusWithLock(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (string, int, error) {
	st, exists := m.statusMap[parcelID]
	if !exists {
		return "OUT_FOR_DELIVERY", m.attemptsMap[parcelID], nil
	}
	return st, m.attemptsMap[parcelID], nil
}

// MockOutboxWriter tracks emitted domain events.
type MockOutboxWriter struct {
	events []string
}

func (m *MockOutboxWriter) Write(ctx context.Context, tx *sql.Tx, aggregateType string, aggregateID uuid.UUID, aggregateVersion int, eventType string, payload any, correlationID string) error {
	m.events = append(m.events, eventType)
	return nil
}

// MockStorageClient simulates S3/R2 storage for unit tests.
type MockStorageClient struct{}

func (m *MockStorageClient) UploadProofPhoto(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	return "https://storage.swiftdrop.io/" + key, nil
}

func (m *MockStorageClient) GeneratePresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "https://storage.swiftdrop.io/" + key + "?presigned=true", nil
}

// --- Test Suites ---

func TestCompleteDeliveryUseCase_Success_Photo(t *testing.T) {
	ctx := context.Background()
	txMgr := &MockTransactionManager{}
	asgnFinder := NewMockActiveAssignmentFinder()
	deliveryRepo := NewMockDeliveryAttemptRepository()
	parcelUpdater := NewMockParcelLifecycleUpdater()
	outboxWriter := &MockOutboxWriter{}

	parcelID := uuid.New()
	courierID := uuid.New()
	asgnID := uuid.New()

	// Assign parcel to courierID
	asgnFinder.assignments[parcelID] = &application.ActiveAssignmentDTO{
		AssignmentID: asgnID,
		ParcelID:     parcelID,
		CourierID:    courierID,
		Status:       "ACTIVE",
	}
	parcelUpdater.statusMap[parcelID] = "OUT_FOR_DELIVERY"

	uc := application.NewCompleteDeliveryUseCase(txMgr, asgnFinder, deliveryRepo, parcelUpdater, outboxWriter)

	res, err := uc.Execute(ctx, application.CompleteDeliveryCommand{
		ParcelID:       parcelID.String(),
		CourierID:      courierID.String(),
		ProofType:      "PHOTO",
		ProofURL:       "https://storage.swiftdrop.io/proofs/doorstep_1.jpg",
		ReceivedByName: "Jane Smith",
		Notes:          "Delivered to front porch",
	})

	if err != nil {
		t.Fatalf("unexpected error on complete delivery: %v", err)
	}
	if res.Status != "DELIVERED" {
		t.Errorf("expected status DELIVERED, got %s", res.Status)
	}
	if !res.ProofVerified {
		t.Errorf("expected ProofVerified to be true")
	}
	if parcelUpdater.lastStatus != "DELIVERED" {
		t.Errorf("expected parcel status DELIVERED, got %s", parcelUpdater.lastStatus)
	}
	if !asgnFinder.completed[asgnID] {
		t.Errorf("expected courier assignment to be marked completed")
	}
	if len(outboxWriter.events) != 1 || outboxWriter.events[0] != "parcel.delivered.v1" {
		t.Errorf("expected parcel.delivered.v1 outbox event, got %v", outboxWriter.events)
	}
}

func TestCompleteDeliveryUseCase_IDOR_Defense(t *testing.T) {
	// AT-006: Ensure courier A cannot complete delivery for courier B's parcel
	ctx := context.Background()
	txMgr := &MockTransactionManager{}
	asgnFinder := NewMockActiveAssignmentFinder()
	deliveryRepo := NewMockDeliveryAttemptRepository()
	parcelUpdater := NewMockParcelLifecycleUpdater()
	outboxWriter := &MockOutboxWriter{}

	parcelID := uuid.New()
	assignedCourier := uuid.New()
	attackingCourier := uuid.New()

	asgnFinder.assignments[parcelID] = &application.ActiveAssignmentDTO{
		AssignmentID: uuid.New(),
		ParcelID:     parcelID,
		CourierID:    assignedCourier,
		Status:       "ACTIVE",
	}
	parcelUpdater.statusMap[parcelID] = "OUT_FOR_DELIVERY"

	uc := application.NewCompleteDeliveryUseCase(txMgr, asgnFinder, deliveryRepo, parcelUpdater, outboxWriter)

	// Attacking courier attempts to complete
	_, err := uc.Execute(ctx, application.CompleteDeliveryCommand{
		ParcelID:       parcelID.String(),
		CourierID:      attackingCourier.String(),
		ProofType:      "PHOTO",
		ProofURL:       "https://storage.swiftdrop.io/proofs/doorstep_1.jpg",
		ReceivedByName: "Jane Smith",
	})

	if !errors.Is(err, domain.ErrNotAssignedCourier) {
		t.Fatalf("expected ErrNotAssignedCourier for IDOR attempt, got %v", err)
	}
}

func TestRecordFailedAttemptUseCase_ADR004_TerminalReturn(t *testing.T) {
	ctx := context.Background()
	txMgr := &MockTransactionManager{}
	asgnFinder := NewMockActiveAssignmentFinder()
	deliveryRepo := NewMockDeliveryAttemptRepository()
	parcelUpdater := NewMockParcelLifecycleUpdater()
	outboxWriter := &MockOutboxWriter{}

	parcelID := uuid.New()
	courierID := uuid.New()

	asgnFinder.assignments[parcelID] = &application.ActiveAssignmentDTO{
		AssignmentID: uuid.New(),
		ParcelID:     parcelID,
		CourierID:    courierID,
		Status:       "ACTIVE",
	}
	parcelUpdater.statusMap[parcelID] = "OUT_FOR_DELIVERY"

	uc := application.NewRecordFailedAttemptUseCase(txMgr, asgnFinder, deliveryRepo, parcelUpdater, outboxWriter)

	// Case 1: Attempt 1 failure -> moves to transient ATTEMPT_FAILED
	res1, err := uc.Execute(ctx, application.RecordFailedAttemptCommand{
		ParcelID:   parcelID.String(),
		CourierID:  courierID.String(),
		ReasonCode: "RECIPIENT_UNAVAILABLE",
		Notes:      "Customer did not answer intercom",
	})
	if err != nil {
		t.Fatalf("unexpected error on attempt 1 failure: %v", err)
	}
	if res1.Status != "ATTEMPT_FAILED" {
		t.Errorf("expected ATTEMPT_FAILED, got %s", res1.Status)
	}
	if outboxWriter.events[len(outboxWriter.events)-1] != "parcel.delivery_failed.v1" {
		t.Errorf("expected parcel.delivery_failed.v1 event")
	}

	// Case 2: Attempt 3 failure -> moves to terminal RETURNED
	parcelUpdater.attemptsMap[parcelID] = 2 // Previous attempts = 2, so this is attempt 3
	parcelUpdater.statusMap[parcelID] = "OUT_FOR_DELIVERY"

	res3, err := uc.Execute(ctx, application.RecordFailedAttemptCommand{
		ParcelID:   parcelID.String(),
		CourierID:  courierID.String(),
		ReasonCode: "RECIPIENT_UNAVAILABLE",
		Notes:      "Third attempt failed: no response",
	})
	if err != nil {
		t.Fatalf("unexpected error on attempt 3 failure: %v", err)
	}
	if res3.Status != "RETURNED" {
		t.Errorf("expected RETURNED status on 3rd attempt, got %s", res3.Status)
	}
	if outboxWriter.events[len(outboxWriter.events)-1] != "parcel.returned.v1" {
		t.Errorf("expected parcel.returned.v1 event")
	}

	// Case 3: Refusal by recipient -> immediate terminal RETURNED on attempt 1
	parcelIDRefused := uuid.New()
	asgnFinder.assignments[parcelIDRefused] = &application.ActiveAssignmentDTO{
		AssignmentID: uuid.New(),
		ParcelID:     parcelIDRefused,
		CourierID:    courierID,
		Status:       "ACTIVE",
	}
	parcelUpdater.statusMap[parcelIDRefused] = "OUT_FOR_DELIVERY"

	resRefused, err := uc.Execute(ctx, application.RecordFailedAttemptCommand{
		ParcelID:   parcelIDRefused.String(),
		CourierID:  courierID.String(),
		ReasonCode: "REFUSED_BY_RECIPIENT",
		Notes:      "Customer refused package: unsolicited goods",
	})
	if err != nil {
		t.Fatalf("unexpected error on refusal: %v", err)
	}
	if resRefused.Status != "RETURNED" {
		t.Errorf("expected immediate RETURNED on refusal, got %s", resRefused.Status)
	}
}

func TestStartDeliveryRunUseCase_SingleAndBatch(t *testing.T) {
	ctx := context.Background()
	txMgr := &MockTransactionManager{}
	asgnFinder := NewMockActiveAssignmentFinder()
	parcelUpdater := NewMockParcelLifecycleUpdater()
	outboxWriter := &MockOutboxWriter{}

	courierID := uuid.New()
	p1 := uuid.New()
	p2 := uuid.New()

	for _, p := range []uuid.UUID{p1, p2} {
		asgnFinder.assignments[p] = &application.ActiveAssignmentDTO{
			AssignmentID: uuid.New(),
			ParcelID:     p,
			CourierID:    courierID,
			Status:       "ACTIVE",
		}
		parcelUpdater.statusMap[p] = "ASSIGNED"
	}

	uc := application.NewStartDeliveryRunUseCase(txMgr, asgnFinder, parcelUpdater, outboxWriter)

	// Batch run start
	res, err := uc.ExecuteBatch(ctx, application.StartBatchRunCommand{
		ParcelIDs:    []string{p1.String(), p2.String()},
		CourierID:    courierID.String(),
		DepotHubCode: "HUB_COLOMBO_CENTRAL",
	})

	if err != nil {
		t.Fatalf("unexpected error on batch run start: %v", err)
	}
	if res.TotalStarted != 2 {
		t.Errorf("expected 2 parcels started, got %d", res.TotalStarted)
	}
	if parcelUpdater.statusMap[p1] != "OUT_FOR_DELIVERY" || parcelUpdater.statusMap[p2] != "OUT_FOR_DELIVERY" {
		t.Errorf("expected parcels to be in OUT_FOR_DELIVERY status")
	}
}
