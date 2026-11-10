package tests

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	deliveryApp "github.com/swiftdrop/core-api/internal/delivery/application"
	deliveryDomain "github.com/swiftdrop/core-api/internal/delivery/domain"
)

type atomicTxManager struct {
	storage *atomicMockStorage
}

func (m *atomicTxManager) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	m.storage.beginTx()
	err := fn(nil)
	if err != nil {
		m.storage.rollbackTx()
		return err
	}
	m.storage.commitTx()
	return nil
}

type atomicMockStorage struct {
	assignmentStatus string
	parcelStatus     string
	attemptCount     int
	attempts         []*deliveryDomain.DeliveryAttempt
	failOutbox       bool

	// Transactional staging
	stagedAssignmentStatus string
	stagedParcelStatus     string
	stagedAttempts         []*deliveryDomain.DeliveryAttempt
	inTx                   bool
}

func (s *atomicMockStorage) beginTx() {
	s.inTx = true
	s.stagedAssignmentStatus = s.assignmentStatus
	s.stagedParcelStatus = s.parcelStatus
	s.stagedAttempts = append([]*deliveryDomain.DeliveryAttempt(nil), s.attempts...)
}

func (s *atomicMockStorage) commitTx() {
	s.assignmentStatus = s.stagedAssignmentStatus
	s.parcelStatus = s.stagedParcelStatus
	s.attempts = s.stagedAttempts
	s.inTx = false
}

func (s *atomicMockStorage) rollbackTx() {
	// Discard staged changes
	s.stagedAssignmentStatus = ""
	s.stagedParcelStatus = ""
	s.stagedAttempts = nil
	s.inTx = false
}

func (s *atomicMockStorage) GetActiveAssignment(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*deliveryApp.ActiveAssignmentDTO, error) {
	status := s.assignmentStatus
	if s.inTx {
		status = s.stagedAssignmentStatus
	}
	if status == "COMPLETED" {
		return nil, nil
	}
	return &deliveryApp.ActiveAssignmentDTO{
		AssignmentID: uuid.MustParse("0192e2b0-8c24-7df7-b087-0b1297e6e5a0"),
		ParcelID:     parcelID,
		CourierID:    uuid.MustParse("0192e2b0-8c24-7df7-b087-0b1297e6e5a1"),
		Status:       status,
	}, nil
}

func (s *atomicMockStorage) CompleteAssignment(ctx context.Context, tx *sql.Tx, assignmentID uuid.UUID) error {
	if s.inTx {
		s.stagedAssignmentStatus = "COMPLETED"
	} else {
		s.assignmentStatus = "COMPLETED"
	}
	return nil
}

func (s *atomicMockStorage) InsertAttempt(ctx context.Context, tx *sql.Tx, attempt *deliveryDomain.DeliveryAttempt) error {
	if s.inTx {
		s.stagedAttempts = append(s.stagedAttempts, attempt)
	} else {
		s.attempts = append(s.attempts, attempt)
	}
	return nil
}

func (s *atomicMockStorage) ListAttemptsByParcelID(ctx context.Context, parcelID uuid.UUID) ([]*deliveryDomain.DeliveryAttempt, error) {
	if s.inTx {
		return s.stagedAttempts, nil
	}
	return s.attempts, nil
}

func (s *atomicMockStorage) GetAttemptCount(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (int, error) {
	if s.inTx {
		return len(s.stagedAttempts), nil
	}
	return len(s.attempts), nil
}

func (s *atomicMockStorage) TransitionStatus(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID, fromStatus, toStatus string, actorID uuid.UUID, actorRole, note string) error {
	if s.inTx {
		s.stagedParcelStatus = toStatus
	} else {
		s.parcelStatus = toStatus
	}
	return nil
}

func (s *atomicMockStorage) GetStatusWithLock(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (string, int, error) {
	if s.inTx {
		return s.stagedParcelStatus, len(s.stagedAttempts), nil
	}
	return s.parcelStatus, s.attemptCount, nil
}

func (s *atomicMockStorage) Write(ctx context.Context, tx *sql.Tx, aggregateType string, aggregateID uuid.UUID, aggregateVersion int, eventType string, payload any, correlationID string) error {
	if s.failOutbox {
		return errors.New("simulated outbox disk full failure: transaction aborted")
	}
	return nil
}

// TestDeliveryAtomicity_RollbackOnFailure verifies that failure in any downstream
// transactional participant (e.g. outbox publishing) aborts the entire unit of work.
func TestDeliveryAtomicity_RollbackOnFailure(t *testing.T) {
	storage := &atomicMockStorage{
		assignmentStatus: "ACTIVE",
		parcelStatus:     "OUT_FOR_DELIVERY",
		attemptCount:     0,
		failOutbox:       true, // Simulate transactional error
	}
	txMgr := &atomicTxManager{storage: storage}

	courierID := uuid.MustParse("0192e2b0-8c24-7df7-b087-0b1297e6e5a1")
	parcelID := uuid.New()

	useCase := deliveryApp.NewCompleteDeliveryUseCase(
		txMgr,
		storage,
		storage,
		storage,
		storage,
	)

	cmd := deliveryApp.CompleteDeliveryCommand{
		ParcelID:       parcelID.String(),
		CourierID:      courierID.String(),
		ProofType:      "PHOTO",
		ProofURL:       "https://cdn.swiftdrop.io/proofs/sample.jpg",
		ReceivedByName: "Kasun Perera",
		Notes:          "Atomic test note",
	}

	result, err := useCase.Execute(context.Background(), cmd)

	// Invariant: The execution must fail due to outbox error
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "simulated outbox disk full failure")

	// Invariant: Atomicity guarantee ensures that on failure, no status changes remain committed
	assert.Equal(t, "OUT_FOR_DELIVERY", storage.parcelStatus, "Parcel status must NOT be DELIVERED after rollback")
	assert.Equal(t, "ACTIVE", storage.assignmentStatus, "Courier assignment must NOT be COMPLETED after rollback")
	assert.Empty(t, storage.attempts, "Delivery attempt record must NOT be persisted after rollback")
}
