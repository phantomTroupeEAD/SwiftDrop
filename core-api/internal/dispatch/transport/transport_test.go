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
	couriers  map[uuid.UUID]*domain.Courier
	userRoles map[uuid.UUID]string
}

func (m *mockCourierRepo) GetUserRole(ctx context.Context, userID uuid.UUID) (string, bool, error) {
	if m.userRoles != nil {
		role, ok := m.userRoles[userID]
		return role, ok, nil
	}
	return "COURIER", true, nil
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

func (m *mockAssignmentRepo) GetActiveByParcelIDReadOnly(ctx context.Context, parcelID uuid.UUID) (*domain.CourierAssignment, error) {
	return m.GetActiveByParcelID(ctx, nil, parcelID)
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
		{CourierID: uuid.New().String(), Name: "Driver 1", ActiveAssignments: 3, MaxCapacity: 25, Status: domain.ShiftStatusOnShift, VehicleType: domain.VehicleTypeMotorcycle},
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

	regUC := application.NewRegisterCourierUseCase(courierRepo, courierRepo)
	listQ := application.NewListCouriersQueryHandler(courierRepo)
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

func TestDispatchHandler_CancelAssignment(t *testing.T) {
	t.Run("Valid cancellation returns 200 OK", func(t *testing.T) {
		dh, _, courierID, parcelID := setupTestDispatchHandler()
		// Assign first
		assignBody, _ := json.Marshal(map[string]any{
			"parcel_id":  parcelID.String(),
			"courier_id": courierID.String(),
		})
		reqAssign := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/assignments", bytes.NewReader(assignBody))
		reqAssign = withAuth(reqAssign, uuid.NewString(), identityDomain.RoleDispatcher)
		recAssign := httptest.NewRecorder()
		dh.AssignCourier(recAssign, reqAssign)
		assert.Equal(t, http.StatusCreated, recAssign.Code)

		// Now cancel
		cancelBody, _ := json.Marshal(map[string]any{
			"reason": "Customer requested delivery hold",
		})
		reqCancel := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/assignments/"+parcelID.String()+"/cancel", bytes.NewReader(cancelBody))
		reqCancel.SetPathValue("id", parcelID.String())
		reqCancel = withAuth(reqCancel, uuid.NewString(), identityDomain.RoleDispatcher)
		recCancel := httptest.NewRecorder()

		dh.CancelAssignment(recCancel, reqCancel)
		assert.Equal(t, http.StatusOK, recCancel.Code)
		assert.Contains(t, recCancel.Body.String(), "CANCELLED")
	})

	t.Run("Missing reason returns 422 Unprocessable Entity", func(t *testing.T) {
		dh, _, _, parcelID := setupTestDispatchHandler()
		cancelBody, _ := json.Marshal(map[string]any{
			"reason": "",
		})
		reqCancel := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/assignments/"+parcelID.String()+"/cancel", bytes.NewReader(cancelBody))
		reqCancel.SetPathValue("id", parcelID.String())
		reqCancel = withAuth(reqCancel, uuid.NewString(), identityDomain.RoleDispatcher)
		recCancel := httptest.NewRecorder()

		dh.CancelAssignment(recCancel, reqCancel)
		assert.Equal(t, http.StatusUnprocessableEntity, recCancel.Code)
		assert.Contains(t, recCancel.Body.String(), "VALIDATION_FAILED")
	})
}

func TestDispatchHandler_RescheduleParcel(t *testing.T) {
	t.Run("Missing courier_id returns 422 Unprocessable Entity", func(t *testing.T) {
		dh, _, _, parcelID := setupTestDispatchHandler()
		reschedBody, _ := json.Marshal(map[string]any{
			"courier_id": "",
			"notes":      "Rescheduling attempt 2",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/parcels/"+parcelID.String()+"/reschedule", bytes.NewReader(reschedBody))
		req.SetPathValue("id", parcelID.String())
		req = withAuth(req, uuid.NewString(), identityDomain.RoleDispatcher)
		rec := httptest.NewRecorder()

		dh.RescheduleParcel(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "VALIDATION_FAILED")
	})
}

func TestDispatchHandler_RegisterCourier_Validation(t *testing.T) {
	dh, _, _, _ := setupTestDispatchHandler()

	t.Run("Invalid vehicle type returns 400 Bad Request", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"user_id":         uuid.NewString(),
			"employee_code":   "CR-1010",
			"vehicle_type":    "PLANE",
			"current_zone_id": "ZONE_COLOMBO_01",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/couriers", bytes.NewReader(body))
		req = withAuth(req, uuid.NewString(), identityDomain.RoleDispatcher)
		rec := httptest.NewRecorder()

		dh.RegisterCourier(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "INVALID_VEHICLE_TYPE")
	})

	t.Run("Capacity exceeds 200 returns 422 Unprocessable Entity", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"user_id":         uuid.NewString(),
			"employee_code":   "CR-1010",
			"vehicle_type":    "VAN",
			"max_capacity":    500,
			"current_zone_id": "ZONE_COLOMBO_01",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/couriers", bytes.NewReader(body))
		req = withAuth(req, uuid.NewString(), identityDomain.RoleDispatcher)
		rec := httptest.NewRecorder()

		dh.RegisterCourier(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "VALIDATION_FAILED")
	})

	t.Run("User with non-courier role returns 422 Unprocessable Entity", func(t *testing.T) {
		mockRepo := &mockCourierRepo{
			couriers:  make(map[uuid.UUID]*domain.Courier),
			userRoles: make(map[uuid.UUID]string),
		}
		adminUID := uuid.New()
		mockRepo.userRoles[adminUID] = "ADMIN"

		regUC := application.NewRegisterCourierUseCase(mockRepo, mockRepo)
		customDH := transport.NewDispatchHandler(regUC, nil, nil, nil, nil, nil)

		body, _ := json.Marshal(map[string]any{
			"user_id":         adminUID.String(),
			"employee_code":   "CR-1011",
			"vehicle_type":    "MOTORCYCLE",
			"max_capacity":    25,
			"current_zone_id": "ZONE_COLOMBO_01",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/couriers", bytes.NewReader(body))
		req = withAuth(req, uuid.NewString(), identityDomain.RoleDispatcher)
		rec := httptest.NewRecorder()

		customDH.RegisterCourier(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "USER_NOT_A_COURIER")
	})

	t.Run("User not found returns 404 Not Found", func(t *testing.T) {
		mockRepo := &mockCourierRepo{
			couriers:  make(map[uuid.UUID]*domain.Courier),
			userRoles: make(map[uuid.UUID]string), // empty, user does not exist
		}
		nonExistentUID := uuid.New()

		regUC := application.NewRegisterCourierUseCase(mockRepo, mockRepo)
		customDH := transport.NewDispatchHandler(regUC, nil, nil, nil, nil, nil)

		body, _ := json.Marshal(map[string]any{
			"user_id":         nonExistentUID.String(),
			"employee_code":   "CR-1012",
			"vehicle_type":    "MOTORCYCLE",
			"max_capacity":    25,
			"current_zone_id": "ZONE_COLOMBO_01",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/couriers", bytes.NewReader(body))
		req = withAuth(req, uuid.NewString(), identityDomain.RoleDispatcher)
		rec := httptest.NewRecorder()

		customDH.RegisterCourier(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Body.String(), "USER_NOT_FOUND")
	})

	t.Run("Duplicate courier profile returns 409 Conflict", func(t *testing.T) {
		existingUID := uuid.New()
		existingCourier, _ := domain.NewCourier(existingUID, "CR-1013", domain.VehicleTypeMotorcycle, 25, "ZONE_COLOMBO_01")
		mockRepo := &mockCourierRepo{
			couriers: map[uuid.UUID]*domain.Courier{
				existingUID: existingCourier,
			},
			userRoles: map[uuid.UUID]string{
				existingUID: "COURIER",
			},
		}

		regUC := application.NewRegisterCourierUseCase(mockRepo, mockRepo)
		customDH := transport.NewDispatchHandler(regUC, nil, nil, nil, nil, nil)

		body, _ := json.Marshal(map[string]any{
			"user_id":         existingUID.String(),
			"employee_code":   "CR-1014",
			"vehicle_type":    "MOTORCYCLE",
			"max_capacity":    25,
			"current_zone_id": "ZONE_COLOMBO_01",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/couriers", bytes.NewReader(body))
		req = withAuth(req, uuid.NewString(), identityDomain.RoleDispatcher)
		rec := httptest.NewRecorder()

		customDH.RegisterCourier(rec, req)
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), "COURIER_PROFILE_EXISTS")
	})

	t.Run("Valid courier registration returns 201 Created", func(t *testing.T) {
		validUID := uuid.New()
		mockRepo := &mockCourierRepo{
			couriers: make(map[uuid.UUID]*domain.Courier),
			userRoles: map[uuid.UUID]string{
				validUID: "COURIER",
			},
		}

		regUC := application.NewRegisterCourierUseCase(mockRepo, mockRepo)
		customDH := transport.NewDispatchHandler(regUC, nil, nil, nil, nil, nil)

		body, _ := json.Marshal(map[string]any{
			"user_id":         validUID.String(),
			"employee_code":   "CR-1015",
			"vehicle_type":    "MOTORCYCLE",
			"max_capacity":    25,
			"current_zone_id": "ZONE_COLOMBO_01",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/couriers", bytes.NewReader(body))
		req = withAuth(req, uuid.NewString(), identityDomain.RoleDispatcher)
		rec := httptest.NewRecorder()

		customDH.RegisterCourier(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), "CR-1015")
	})
}

func (*mockDispatchBoardReader) GetAssignedParcels(context.Context, string) ([]application.AssignedParcelDTO, error) {
	return nil, nil
}
