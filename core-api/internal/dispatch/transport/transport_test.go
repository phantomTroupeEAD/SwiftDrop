package transport_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/swiftdrop/core-api/internal/dispatch/application"
	"github.com/swiftdrop/core-api/internal/dispatch/domain"
	"github.com/swiftdrop/core-api/internal/dispatch/transport"
	identityApp "github.com/swiftdrop/core-api/internal/identity/application"
	identityDomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityTransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

type mockTxManager struct{}

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return fn(nil)
}

type mockCourierRepo struct {
	couriers map[uuid.UUID]*domain.Courier
}

func (m *mockCourierRepo) GetByID(ctx context.Context, userID uuid.UUID) (*domain.Courier, error) {
	c, ok := m.couriers[userID]
	if !ok {
		return nil, nil
	}
	return c, nil
}

func (m *mockCourierRepo) GetByIDWithLock(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (*domain.Courier, error) {
	return m.GetByID(ctx, userID)
}

func (m *mockCourierRepo) Save(ctx context.Context, tx *sql.Tx, courier *domain.Courier) error {
	m.couriers[courier.UserID] = courier
	return nil
}

func (m *mockCourierRepo) List(ctx context.Context, zoneID string, status *domain.ShiftStatus, vehicleType *domain.VehicleType) ([]*domain.Courier, error) {
	var res []*domain.Courier
	for _, c := range m.couriers {
		res = append(res, c)
	}
	return res, nil
}

type mockAssignmentRepo struct {
	assignments map[uuid.UUID]*domain.CourierAssignment
}

func (m *mockAssignmentRepo) Create(ctx context.Context, tx *sql.Tx, asgn *domain.CourierAssignment) error {
	m.assignments[asgn.ID] = asgn
	return nil
}

func (m *mockAssignmentRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.CourierAssignment, error) {
	return m.assignments[id], nil
}

func (m *mockAssignmentRepo) GetActiveByParcelID(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*domain.CourierAssignment, error) {
	for _, a := range m.assignments {
		if a.ParcelID == parcelID && a.Status == domain.AssignmentStatusActive {
			return a, nil
		}
	}
	return nil, nil
}

func (m *mockAssignmentRepo) CountActiveByCourierID(ctx context.Context, tx *sql.Tx, courierID uuid.UUID) (int, error) {
	cnt := 0
	for _, a := range m.assignments {
		if a.CourierID == courierID && a.Status == domain.AssignmentStatusActive {
			cnt++
		}
	}
	return cnt, nil
}

func (m *mockAssignmentRepo) Update(ctx context.Context, tx *sql.Tx, asgn *domain.CourierAssignment) error {
	m.assignments[asgn.ID] = asgn
	return nil
}

type mockParcelUpdater struct {
	statuses map[uuid.UUID]string
	attempts map[uuid.UUID]int
}

func (m *mockParcelUpdater) TransitionStatus(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID, fromStatus, toStatus string, actorID uuid.UUID, actorRole, note string) error {
	m.statuses[parcelID] = toStatus
	return nil
}

func (m *mockParcelUpdater) GetStatusWithLock(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (string, int, error) {
	s, ok := m.statuses[parcelID]
	if !ok {
		s = "CREATED"
	}
	return s, m.attempts[parcelID], nil
}

type mockOutboxWriter struct{}

func (m *mockOutboxWriter) Write(ctx context.Context, tx *sql.Tx, aggregateType string, aggregateID uuid.UUID, aggregateVersion int, eventType string, payload any, correlationID string) error {
	return nil
}

type mockDispatchBoardReader struct{}

func (m *mockDispatchBoardReader) GetUnassignedParcels(ctx context.Context, zoneID string) ([]application.UnassignedParcelDTO, error) {
	return []application.UnassignedParcelDTO{
		{ID: uuid.New().String(), TrackingNumber: "SD-20261102-0001", Status: "CREATED", CreatedAt: time.Now()},
	}, nil
}

func (m *mockDispatchBoardReader) GetCourierLoads(ctx context.Context, zoneID string) ([]application.CourierLoadDTO, error) {
	return []application.CourierLoadDTO{
		{UserID: uuid.New().String(), CourierName: "Driver 1", ActiveCount: 3, MaxCapacity: 25, Status: "ON_SHIFT"},
	}, nil
}

func setupTestDispatchHandler() (*transport.DispatchHandler, *transport.CourierShiftHandler, uuid.UUID, uuid.UUID) {
	txMgr := &mockTxManager{}
	courierRepo := &mockCourierRepo{couriers: make(map[uuid.UUID]*domain.Courier)}
	asgnRepo := &mockAssignmentRepo{assignments: make(map[uuid.UUID]*domain.CourierAssignment)}
	parcelUpdater := &mockParcelUpdater{statuses: make(map[uuid.UUID]string), attempts: make(map[uuid.UUID]int)}
	outbox := &mockOutboxWriter{}
	boardReader := &mockDispatchBoardReader{}

	courierID := uuid.New()
	c, _ := domain.NewCourier(courierID, "CR-101", domain.VehicleTypeMotorcycle, 25, "ZONE_01")
	_ = c.ToggleShift(domain.ShiftStatusOnShift, true)
	_ = courierRepo.Save(context.Background(), nil, c)

	parcelID := uuid.New()
	parcelUpdater.statuses[parcelID] = "CREATED"

	regUC := application.NewRegisterCourierUseCase(courierRepo)
	listQ := application.NewListCouriersQueryHandler(boardReader)
	boardQ := application.NewGetDispatchBoardQueryHandler(boardReader)
	assignUC := application.NewAssignCourierUseCase(txMgr, courierRepo, asgnRepo, parcelUpdater, outbox)
	cancelUC := application.NewCancelAssignmentUseCase(txMgr, asgnRepo, parcelUpdater, outbox)
	reschedUC := application.NewRescheduleParcelUseCase(txMgr, courierRepo, asgnRepo, parcelUpdater, outbox)
	shiftUC := application.NewToggleShiftUseCase(courierRepo)

	dh := transport.NewDispatchHandler(regUC, listQ, boardQ, assignUC, cancelUC, reschedUC)
	ch := transport.NewCourierShiftHandler(shiftUC)

	return dh, ch, courierID, parcelID
}

func withAuth(r *http.Request, userID string, role identityDomain.Role) *http.Request {
	p := &identityApp.Principal{
		UserID: userID,
		Email:  "test@swiftdrop.io",
		Role:   role,
	}
	return r.WithContext(identityTransport.WithPrincipal(r.Context(), p))
}

func TestDispatchHandler_RegisterCourier(t *testing.T) {
	dh, _, _, _ := setupTestDispatchHandler()

	t.Run("Validation failure returns 422", func(t *testing.T) {
		reqBody := `{"user_id": "invalid-uuid"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/couriers", bytes.NewBufferString(reqBody))
		req = withAuth(req, uuid.NewString(), identityDomain.RoleDispatcher)
		rec := httptest.NewRecorder()

		dh.RegisterCourier(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("Valid payload creates courier profile", func(t *testing.T) {
		newID := uuid.New().String()
		reqBody := map[string]any{
			"user_id":         newID,
			"employee_code":   "CR-9999",
			"vehicle_type":    "VAN",
			"max_capacity":    60,
			"current_zone_id": "ZONE_01",
		}
		b, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/couriers", bytes.NewReader(b))
		req = withAuth(req, uuid.NewString(), identityDomain.RoleDispatcher)
		rec := httptest.NewRecorder()

		dh.RegisterCourier(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), "CR-9999")
	})
}

func TestDispatchHandler_AssignCourier(t *testing.T) {
	dh, _, courierID, parcelID := setupTestDispatchHandler()

	t.Run("Success returns 201 Created", func(t *testing.T) {
		reqBody := map[string]any{
			"parcel_id":  parcelID.String(),
			"courier_id": courierID.String(),
			"notes":      "Handle with care",
		}
		b, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/assignments", bytes.NewReader(b))
		req = withAuth(req, uuid.NewString(), identityDomain.RoleDispatcher)
		rec := httptest.NewRecorder()

		dh.AssignCourier(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), "ACTIVE")
	})
}

func TestDispatchHandler_GetDispatchBoard(t *testing.T) {
	dh, _, _, _ := setupTestDispatchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dispatch/board?zone_id=ZONE_01", nil)
	req = withAuth(req, uuid.NewString(), identityDomain.RoleDispatcher)
	rec := httptest.NewRecorder()

	dh.GetDispatchBoard(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "unassigned_parcels")
	assert.Contains(t, rec.Body.String(), "courier_loads")
}

func TestCourierShiftHandler_ToggleShift(t *testing.T) {
	_, ch, courierID, _ := setupTestDispatchHandler()

	reqBody := map[string]any{
		"target_status": "ON_BREAK",
	}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/courier/shift", bytes.NewReader(b))
	req = withAuth(req, courierID.String(), identityDomain.RoleCourier)
	rec := httptest.NewRecorder()

	ch.ToggleShift(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "ON_BREAK")
}

