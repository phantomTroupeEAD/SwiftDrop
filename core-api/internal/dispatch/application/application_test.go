package application_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/dispatch/application"
	"github.com/swiftdrop/core-api/internal/dispatch/domain"
)

// MockTransactionManager executes callbacks directly without real database connections.
type MockTransactionManager struct{}

func (m *MockTransactionManager) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return fn(nil)
}

// MockCourierRepository provides in-memory courier storage for tests.
type MockCourierRepository struct {
	couriers map[uuid.UUID]*domain.Courier
}

func NewMockCourierRepository() *MockCourierRepository {
	return &MockCourierRepository{couriers: make(map[uuid.UUID]*domain.Courier)}
}

func (m *MockCourierRepository) GetByID(ctx context.Context, userID uuid.UUID) (*domain.Courier, error) {
	return m.couriers[userID], nil
}

func (m *MockCourierRepository) GetByIDWithLock(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (*domain.Courier, error) {
	return m.couriers[userID], nil
}

func (m *MockCourierRepository) Save(ctx context.Context, tx *sql.Tx, courier *domain.Courier) error {
	m.couriers[courier.UserID] = courier
	return nil
}

func (m *MockCourierRepository) List(ctx context.Context, zoneID string, status *domain.ShiftStatus, vehicleType *domain.VehicleType) ([]*domain.Courier, error) {
	var list []*domain.Courier
	for _, c := range m.couriers {
		list = append(list, c)
	}
	return list, nil
}

// MockAssignmentRepository provides in-memory assignment storage for tests.
type MockAssignmentRepository struct {
	assignments map[uuid.UUID]*domain.CourierAssignment
}

func NewMockAssignmentRepository() *MockAssignmentRepository {
	return &MockAssignmentRepository{assignments: make(map[uuid.UUID]*domain.CourierAssignment)}
}

func (m *MockAssignmentRepository) Create(ctx context.Context, tx *sql.Tx, asgn *domain.CourierAssignment) error {
	m.assignments[asgn.ID] = asgn
	return nil
}

func (m *MockAssignmentRepository) GetActiveByParcelID(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*domain.CourierAssignment, error) {
	for _, a := range m.assignments {
		if a.ParcelID == parcelID && a.Status == domain.AssignmentStatusActive {
			return a, nil
		}
	}
	return nil, nil
}

func (m *MockAssignmentRepository) GetActiveByParcelIDReadOnly(ctx context.Context, parcelID uuid.UUID) (*domain.CourierAssignment, error) {
	return m.GetActiveByParcelID(ctx, nil, parcelID)
}

func (m *MockAssignmentRepository) CountActiveByCourierID(ctx context.Context, tx *sql.Tx, courierID uuid.UUID) (int, error) {
	count := 0
	for _, a := range m.assignments {
		if a.CourierID == courierID && a.Status == domain.AssignmentStatusActive {
			count++
		}
	}
	return count, nil
}

func (m *MockAssignmentRepository) Update(ctx context.Context, tx *sql.Tx, asgn *domain.CourierAssignment) error {
	m.assignments[asgn.ID] = asgn
	return nil
}

// MockParcelLifecycleUpdater tracks parcel status transitions.
type MockParcelLifecycleUpdater struct {
	statusMap  map[uuid.UUID]string
	attempts   map[uuid.UUID]int
	lastStatus string
}

func NewMockParcelLifecycleUpdater() *MockParcelLifecycleUpdater {
	return &MockParcelLifecycleUpdater{
		statusMap: make(map[uuid.UUID]string),
		attempts:  make(map[uuid.UUID]int),
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
		return "CREATED", 0, nil
	}
	return st, m.attempts[parcelID], nil
}

// MockOutboxWriter captures emitted events.
type MockOutboxWriter struct {
	events []string
}

func (m *MockOutboxWriter) Write(ctx context.Context, tx *sql.Tx, aggregateType string, aggregateID uuid.UUID, aggregateVersion int, eventType string, payload any, correlationID string) error {
	m.events = append(m.events, eventType)
	return nil
}

// --- Test Suites ---

func TestAssignCourierUseCase_Success(t *testing.T) {
	ctx := context.Background()
	txMgr := &MockTransactionManager{}
	courierRepo := NewMockCourierRepository()
	asgnRepo := NewMockAssignmentRepository()
	parcelUpdater := NewMockParcelLifecycleUpdater()
	outboxWriter := &MockOutboxWriter{}

	courierID := uuid.New()
	parcelID := uuid.New()
	dispatcherID := uuid.New()

	// Setup courier ON_SHIFT with capacity 25
	courier, _ := domain.NewCourier(courierID, "CR-0012", domain.VehicleTypeMotorcycle, 25, "ZONE_COLOMBO_01")
	_ = courier.ToggleShift(domain.ShiftStatusOnShift, false)
	_ = courierRepo.Save(ctx, nil, courier)

	uc := application.NewAssignCourierUseCase(txMgr, courierRepo, asgnRepo, parcelUpdater, outboxWriter)

	res, err := uc.Execute(ctx, application.AssignCourierCommand{
		ParcelID:   parcelID.String(),
		CourierID:  courierID.String(),
		AssignedBy: dispatcherID.String(),
		Notes:      "Test assignment",
	})

	if err != nil {
		t.Fatalf("unexpected error on assign: %v", err)
	}
	if res.Status != "ACTIVE" {
		t.Errorf("expected ACTIVE status, got %s", res.Status)
	}
	if parcelUpdater.lastStatus != "ASSIGNED" {
		t.Errorf("expected parcel to be ASSIGNED, got %s", parcelUpdater.lastStatus)
	}
	if len(outboxWriter.events) != 1 || outboxWriter.events[0] != "parcel.assigned.v1" {
		t.Errorf("expected parcel.assigned.v1 outbox event, got %v", outboxWriter.events)
	}
}

func TestAssignCourierUseCase_Overload(t *testing.T) {
	ctx := context.Background()
	txMgr := &MockTransactionManager{}
	courierRepo := NewMockCourierRepository()
	asgnRepo := NewMockAssignmentRepository()
	parcelUpdater := NewMockParcelLifecycleUpdater()
	outboxWriter := &MockOutboxWriter{}

	courierID := uuid.New()
	courier, _ := domain.NewCourier(courierID, "CR-0012", domain.VehicleTypeMotorcycle, 1, "ZONE_COLOMBO_01")
	_ = courier.ToggleShift(domain.ShiftStatusOnShift, false)
	_ = courierRepo.Save(ctx, nil, courier)

	// Fill courier capacity to maximum (1 active assignment)
	existingAsgn := domain.NewCourierAssignment(uuid.New(), courierID, uuid.New(), "")
	_ = asgnRepo.Create(ctx, nil, existingAsgn)

	uc := application.NewAssignCourierUseCase(txMgr, courierRepo, asgnRepo, parcelUpdater, outboxWriter)

	_, err := uc.Execute(ctx, application.AssignCourierCommand{
		ParcelID:   uuid.New().String(),
		CourierID:  courierID.String(),
		AssignedBy: uuid.New().String(),
	})

	if !errors.Is(err, domain.ErrCourierOverloaded) {
		t.Errorf("expected ErrCourierOverloaded, got %v", err)
	}
}

func TestCancelAssignmentUseCase_Success(t *testing.T) {
	ctx := context.Background()
	txMgr := &MockTransactionManager{}
	asgnRepo := NewMockAssignmentRepository()
	parcelUpdater := NewMockParcelLifecycleUpdater()
	outboxWriter := &MockOutboxWriter{}

	parcelID := uuid.New()
	asgn := domain.NewCourierAssignment(parcelID, uuid.New(), uuid.New(), "")
	_ = asgnRepo.Create(ctx, nil, asgn)

	uc := application.NewCancelAssignmentUseCase(txMgr, asgnRepo, parcelUpdater, outboxWriter)

	err := uc.Execute(ctx, application.CancelAssignmentCommand{
		ParcelID:    parcelID.String(),
		CancelledBy: uuid.New().String(),
		Reason:      "Package damaged at depot prior to pickup",
	})

	if err != nil {
		t.Fatalf("unexpected error on cancel: %v", err)
	}
	if asgn.Status != domain.AssignmentStatusCancelled {
		t.Errorf("expected assignment status CANCELLED, got %s", asgn.Status)
	}
	if parcelUpdater.lastStatus != "CREATED" {
		t.Errorf("expected parcel status reverted to CREATED, got %s", parcelUpdater.lastStatus)
	}
	if len(outboxWriter.events) != 1 || outboxWriter.events[0] != "courier.unassigned.v1" {
		t.Errorf("expected courier.unassigned.v1 outbox event, got %v", outboxWriter.events)
	}
}

