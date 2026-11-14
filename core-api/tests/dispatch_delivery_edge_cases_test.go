package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	deliveryApp "github.com/swiftdrop/core-api/internal/delivery/application"
	deliveryDomain "github.com/swiftdrop/core-api/internal/delivery/domain"
	deliveryTransport "github.com/swiftdrop/core-api/internal/delivery/transport"
	dispatchApp "github.com/swiftdrop/core-api/internal/dispatch/application"
	dispatchDomain "github.com/swiftdrop/core-api/internal/dispatch/domain"
	dispatchTransport "github.com/swiftdrop/core-api/internal/dispatch/transport"
	identityApp "github.com/swiftdrop/core-api/internal/identity/application"
	identityDomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityTransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

type comprehensiveMockStorage struct {
	couriers    map[uuid.UUID]*dispatchDomain.Courier
	assignments map[uuid.UUID]*dispatchDomain.CourierAssignment
	statuses    map[uuid.UUID]string
	attempts    map[uuid.UUID]int
	attemptLogs map[uuid.UUID][]*deliveryDomain.DeliveryAttempt
	users       map[uuid.UUID]string // userID -> role
	photos      map[string][]byte
}

func newComprehensiveMockStorage() *comprehensiveMockStorage {
	return &comprehensiveMockStorage{
		couriers:    make(map[uuid.UUID]*dispatchDomain.Courier),
		assignments: make(map[uuid.UUID]*dispatchDomain.CourierAssignment),
		statuses:    make(map[uuid.UUID]string),
		attempts:    make(map[uuid.UUID]int),
		attemptLogs: make(map[uuid.UUID][]*deliveryDomain.DeliveryAttempt),
		users:       make(map[uuid.UUID]string),
		photos:      make(map[string][]byte),
	}
}

func (s *comprehensiveMockStorage) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return fn(nil)
}

func (s *comprehensiveMockStorage) GetUserRole(ctx context.Context, userID uuid.UUID) (string, bool, error) {
	role, ok := s.users[userID]
	return role, ok, nil
}

func (s *comprehensiveMockStorage) GetByID(ctx context.Context, userID uuid.UUID) (*dispatchDomain.Courier, error) {
	c, ok := s.couriers[userID]
	if !ok {
		return nil, nil
	}
	return c, nil
}

func (s *comprehensiveMockStorage) GetByIDWithLock(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (*dispatchDomain.Courier, error) {
	return s.GetByID(ctx, userID)
}

func (s *comprehensiveMockStorage) Save(ctx context.Context, tx *sql.Tx, courier *dispatchDomain.Courier) error {
	s.couriers[courier.UserID] = courier
	return nil
}

func (s *comprehensiveMockStorage) List(ctx context.Context, zoneID string, status *dispatchDomain.ShiftStatus, vehicleType *dispatchDomain.VehicleType) ([]*dispatchDomain.Courier, error) {
	var res []*dispatchDomain.Courier
	for _, c := range s.couriers {
		res = append(res, c)
	}
	return res, nil
}

func (s *comprehensiveMockStorage) Create(ctx context.Context, tx *sql.Tx, asgn *dispatchDomain.CourierAssignment) error {
	for _, existing := range s.assignments {
		if existing.ParcelID == asgn.ParcelID && existing.Status == dispatchDomain.AssignmentStatusActive {
			return dispatchApp.ErrDuplicateActiveAssignment
		}
	}
	s.assignments[asgn.ID] = asgn
	return nil
}

func (s *comprehensiveMockStorage) Update(ctx context.Context, tx *sql.Tx, asgn *dispatchDomain.CourierAssignment) error {
	s.assignments[asgn.ID] = asgn
	return nil
}

func (s *comprehensiveMockStorage) GetByIDAssignment(ctx context.Context, id uuid.UUID) (*dispatchDomain.CourierAssignment, error) {
	return s.assignments[id], nil
}

func (s *comprehensiveMockStorage) GetActiveByParcelID(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*dispatchDomain.CourierAssignment, error) {
	for _, a := range s.assignments {
		if a.ParcelID == parcelID && a.Status == dispatchDomain.AssignmentStatusActive {
			return a, nil
		}
	}
	return nil, nil
}

func (s *comprehensiveMockStorage) GetActiveByParcelIDReadOnly(ctx context.Context, parcelID uuid.UUID) (*dispatchDomain.CourierAssignment, error) {
	return s.GetActiveByParcelID(ctx, nil, parcelID)
}

func (s *comprehensiveMockStorage) CountActiveByCourierID(ctx context.Context, tx *sql.Tx, courierID uuid.UUID) (int, error) {
	cnt := 0
	for _, a := range s.assignments {
		if a.CourierID == courierID && a.Status == dispatchDomain.AssignmentStatusActive {
			cnt++
		}
	}
	return cnt, nil
}

func (s *comprehensiveMockStorage) GetUnassignedParcels(ctx context.Context, zoneID string) ([]dispatchApp.UnassignedParcelDTO, error) {
	var list []dispatchApp.UnassignedParcelDTO
	for pID, st := range s.statuses {
		if st == "CREATED" || st == "ATTEMPT_FAILED" {
			list = append(list, dispatchApp.UnassignedParcelDTO{
				ID:             pID.String(),
				TrackingNumber: "SD-" + pID.String()[:8],
				Status:         st,
			})
		}
	}
	return list, nil
}

func (s *comprehensiveMockStorage) GetCourierLoads(ctx context.Context, zoneID string) ([]dispatchApp.CourierLoadDTO, error) {
	var list []dispatchApp.CourierLoadDTO
	for _, c := range s.couriers {
		active, _ := s.CountActiveByCourierID(ctx, nil, c.UserID)
		list = append(list, dispatchApp.CourierLoadDTO{
			CourierID:         c.UserID.String(),
			Name:              "Courier Name",
			EmployeeCode:      c.EmployeeCode,
			CurrentZoneID:     c.CurrentZoneID,
			ActiveAssignments: active,
			MaxCapacity:       c.MaxCapacity,
			Status:            c.Status,
			VehicleType:       c.Vehicle,
		})
	}
	return list, nil
}

func (s *comprehensiveMockStorage) GetActiveAssignment(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*deliveryApp.ActiveAssignmentDTO, error) {
	for _, a := range s.assignments {
		if a.ParcelID == parcelID && a.Status == dispatchDomain.AssignmentStatusActive {
			return &deliveryApp.ActiveAssignmentDTO{
				AssignmentID: a.ID,
				ParcelID:     a.ParcelID,
				CourierID:    a.CourierID,
				Status:       string(a.Status),
			}, nil
		}
	}
	return nil, nil
}

func (s *comprehensiveMockStorage) CompleteAssignment(ctx context.Context, tx *sql.Tx, assignmentID uuid.UUID) error {
	for _, a := range s.assignments {
		if a.ID == assignmentID {
			_ = a.Complete()
			return nil
		}
	}
	return nil
}

func (s *comprehensiveMockStorage) InsertAttempt(ctx context.Context, tx *sql.Tx, attempt *deliveryDomain.DeliveryAttempt) error {
	s.attempts[attempt.ParcelID] = attempt.AttemptNumber
	s.attemptLogs[attempt.ParcelID] = append(s.attemptLogs[attempt.ParcelID], attempt)
	return nil
}

func (s *comprehensiveMockStorage) ListAttemptsByParcelID(ctx context.Context, parcelID uuid.UUID) ([]*deliveryDomain.DeliveryAttempt, error) {
	return s.attemptLogs[parcelID], nil
}

func (s *comprehensiveMockStorage) GetAttemptCount(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (int, error) {
	return s.attempts[parcelID], nil
}

func (s *comprehensiveMockStorage) GetStatusWithLock(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (string, int, error) {
	st, ok := s.statuses[parcelID]
	if !ok {
		return "", 0, fmt.Errorf("parcel not found: %w", sql.ErrNoRows)
	}
	return st, s.attempts[parcelID], nil
}

func (s *comprehensiveMockStorage) TransitionStatus(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID, fromStatus, toStatus string, actorID uuid.UUID, actorRole, note string) error {
	s.statuses[parcelID] = toStatus
	return nil
}

func (s *comprehensiveMockStorage) Write(ctx context.Context, tx *sql.Tx, aggregateType string, aggregateID uuid.UUID, aggregateVersion int, eventType string, payload any, correlationID string) error {
	return nil
}

func (s *comprehensiveMockStorage) UploadProofPhoto(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	b, _ := io.ReadAll(data)
	s.photos[key] = b
	return "https://cdn.swiftdrop.io/proofs/" + key, nil
}

func (s *comprehensiveMockStorage) GeneratePresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "https://cdn.swiftdrop.io/proofs/" + key, nil
}

func (s *comprehensiveMockStorage) GetActiveManifest(ctx context.Context, courierID uuid.UUID) ([]deliveryApp.CourierManifestItemDTO, error) {
	var items []deliveryApp.CourierManifestItemDTO
	for _, a := range s.assignments {
		if a.CourierID == courierID && a.Status == dispatchDomain.AssignmentStatusActive {
			items = append(items, deliveryApp.CourierManifestItemDTO{
				ParcelID:       a.ParcelID.String(),
				TrackingNumber: "SD-" + a.ParcelID.String()[:8],
				CurrentStatus:  s.statuses[a.ParcelID],
				RecipientName:  "Customer Name",
			})
		}
	}
	return items, nil
}

func TestDispatchAndDelivery_AllEndpointsAndEdgeCases(t *testing.T) {
	storage := newComprehensiveMockStorage()

	// Registered Users
	dispID := uuid.MustParse("018f4a7c-1002-7000-8000-000000000002")
	courierAID := uuid.MustParse("018f4a7c-1003-7000-8000-000000000003")
	courierBID := uuid.MustParse("018f4a7c-1004-7000-8000-000000000004")
	customerID := uuid.MustParse("018f4a7c-1005-7000-8000-000000000005")

	storage.users[dispID] = "DISPATCHER"
	storage.users[courierAID] = "COURIER"
	storage.users[courierBID] = "COURIER"
	storage.users[customerID] = "CUSTOMER"

	// Initial courier A profile (with capacity 25)
	courierA, _ := dispatchDomain.NewCourier(courierAID, "CR-7001", dispatchDomain.VehicleTypeMotorcycle, 25, "ZONE_COLOMBO_01")
	storage.couriers[courierAID] = courierA

	// Construct use cases
	regCourierUC := dispatchApp.NewRegisterCourierUseCase(storage, storage)
	listCouriersQ := dispatchApp.NewListCouriersQueryHandler(storage)
	boardQ := dispatchApp.NewGetDispatchBoardQueryHandler(storage)
	assignCourierUC := dispatchApp.NewAssignCourierUseCase(storage, storage, storage, storage, storage)
	cancelAsgnUC := dispatchApp.NewCancelAssignmentUseCase(storage, storage, storage, storage)
	reschedUC := dispatchApp.NewRescheduleParcelUseCase(storage, storage, storage, storage, storage)
	toggleShiftUC := dispatchApp.NewToggleShiftUseCase(storage)

	dispatchHandler := dispatchTransport.NewDispatchHandler(regCourierUC, listCouriersQ, boardQ, assignCourierUC, cancelAsgnUC, reschedUC)
	courierShiftHandler := dispatchTransport.NewCourierShiftHandler(toggleShiftUC)

	uploadUC := deliveryApp.NewUploadProofPhotoUseCase(storage, storage)
	completeUC := deliveryApp.NewCompleteDeliveryUseCase(storage, storage, storage, storage, storage)
	failedUC := deliveryApp.NewRecordFailedAttemptUseCase(storage, storage, storage, storage, storage)
	historyQ := deliveryApp.NewGetDeliveryHistoryQueryHandler(storage, storage, storage)
	manifestQ := deliveryApp.NewGetCourierManifestQueryHandler(storage)
	startRunUC := deliveryApp.NewStartDeliveryRunUseCase(storage, storage, storage, storage)

	deliveryHandler := deliveryTransport.NewDeliveryHandler(uploadUC, completeUC, failedUC, historyQ)
	manifestHandler := deliveryTransport.NewManifestHandler(manifestQ, startRunUC)

	mux := http.NewServeMux()

	authWrapper := func(handler http.HandlerFunc, requiredRole identityDomain.Role) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			role := identityTransport.GetUserRole(r.Context())
			if role == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "AUTH_UNAUTHORIZED"}})
				return
			}
			if role != requiredRole && requiredRole != "" && role != identityDomain.RoleAdmin {
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "AUTH_FORBIDDEN"}})
				return
			}
			handler(w, r)
		}
	}

	// Register Dispatch Routes
	mux.HandleFunc("POST /api/v1/dispatch/couriers", authWrapper(dispatchHandler.RegisterCourier, identityDomain.RoleDispatcher))
	mux.HandleFunc("GET /api/v1/dispatch/couriers", authWrapper(dispatchHandler.ListCouriers, identityDomain.RoleDispatcher))
	mux.HandleFunc("GET /api/v1/dispatch/board", authWrapper(dispatchHandler.GetDispatchBoard, identityDomain.RoleDispatcher))
	mux.HandleFunc("POST /api/v1/dispatch/assignments", authWrapper(dispatchHandler.AssignCourier, identityDomain.RoleDispatcher))
	mux.HandleFunc("POST /api/v1/dispatch/assignments/{id}/cancel", authWrapper(dispatchHandler.CancelAssignment, identityDomain.RoleDispatcher))
	mux.HandleFunc("POST /api/v1/dispatch/parcels/{id}/reschedule", authWrapper(dispatchHandler.RescheduleParcel, identityDomain.RoleDispatcher))
	mux.HandleFunc("POST /api/v1/courier/shift", authWrapper(courierShiftHandler.ToggleShift, identityDomain.RoleCourier))

	// Register Delivery Routes
	mux.HandleFunc("GET /api/v1/courier/parcels", authWrapper(manifestHandler.GetManifest, identityDomain.RoleCourier))
	mux.HandleFunc("POST /api/v1/courier/parcels/{id}/start-run", authWrapper(manifestHandler.StartRun, identityDomain.RoleCourier))
	mux.HandleFunc("POST /api/v1/courier/runs/start-batch", authWrapper(manifestHandler.StartBatchRun, identityDomain.RoleCourier))
	mux.HandleFunc("POST /api/v1/deliveries/{id}/proof-photo", authWrapper(deliveryHandler.UploadProofPhoto, identityDomain.RoleCourier))
	mux.HandleFunc("POST /api/v1/deliveries/{id}/complete", authWrapper(deliveryHandler.CompleteDelivery, identityDomain.RoleCourier))
	mux.HandleFunc("POST /api/v1/deliveries/{id}/failed", authWrapper(deliveryHandler.RecordFailedAttempt, identityDomain.RoleCourier))
	mux.HandleFunc("GET /api/v1/deliveries/parcels/{id}", authWrapper(deliveryHandler.GetDeliveryHistory, identityDomain.RoleDispatcher))

	callEndpoint := func(method, path string, actorID uuid.UUID, role identityDomain.Role, body any) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Content-Type", "application/json")
		if actorID != uuid.Nil {
			p := &identityApp.Principal{
				UserID: actorID.String(),
				Email:  string(role) + "@swiftdrop.io",
				Role:   role,
			}
			req = req.WithContext(identityTransport.WithPrincipal(req.Context(), p))
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w
	}

	// -------------------------------------------------------------------------
	// SECTION 1: DISPATCH FLEET MANAGEMENT ENDPOINTS & EDGE CASES
	// -------------------------------------------------------------------------

	t.Run("Endpoint 1: POST /api/v1/dispatch/couriers - Happy Path", func(t *testing.T) {
		payload := map[string]any{
			"user_id":         courierBID.String(),
			"employee_code":   "CR-7002",
			"vehicle_type":    "VAN",
			"max_capacity":    50,
			"current_zone_id": "ZONE_COLOMBO_01",
		}
		w := callEndpoint("POST", "/api/v1/dispatch/couriers", dispID, identityDomain.RoleDispatcher, payload)
		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), "CR-7002")
	})

	t.Run("Edge Case 1.1: Missing Authentication Token", func(t *testing.T) {
		w := callEndpoint("POST", "/api/v1/dispatch/couriers", uuid.Nil, "", map[string]any{})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "AUTH_UNAUTHORIZED")
	})

	t.Run("Edge Case 1.2: Insufficient Role (Customer attempting to register courier)", func(t *testing.T) {
		w := callEndpoint("POST", "/api/v1/dispatch/couriers", customerID, identityDomain.RoleCustomer, map[string]any{})
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "AUTH_FORBIDDEN")
	})

	t.Run("Edge Case 1.3: Malformed JSON Body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/dispatch/couriers", bytes.NewBufferString("{broken-json"))
		req.Header.Set("Content-Type", "application/json")
		p := &identityApp.Principal{UserID: dispID.String(), Role: identityDomain.RoleDispatcher}
		req = req.WithContext(identityTransport.WithPrincipal(req.Context(), p))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "INVALID_REQUEST_BODY")
	})

	t.Run("Edge Case 1.4: Invalid Vehicle Type", func(t *testing.T) {
		freshCourierID := uuid.New()
		storage.users[freshCourierID] = "COURIER"
		payload := map[string]any{
			"user_id":         freshCourierID.String(),
			"employee_code":   "CR-7005",
			"vehicle_type":    "BICYCLE", // Invalid
			"max_capacity":    10,
			"current_zone_id": "ZONE_COLOMBO_01",
		}
		w := callEndpoint("POST", "/api/v1/dispatch/couriers", dispID, identityDomain.RoleDispatcher, payload)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "INVALID_VEHICLE_TYPE")
	})

	t.Run("Edge Case 1.5: Capacity Exceeds 200", func(t *testing.T) {
		freshCourierID := uuid.New()
		storage.users[freshCourierID] = "COURIER"
		payload := map[string]any{
			"user_id":         freshCourierID.String(),
			"employee_code":   "CR-7006",
			"vehicle_type":    "MOTORCYCLE",
			"max_capacity":    250, // Limit is 200
			"current_zone_id": "ZONE_COLOMBO_01",
		}
		w := callEndpoint("POST", "/api/v1/dispatch/couriers", dispID, identityDomain.RoleDispatcher, payload)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "VALIDATION_FAILED")
	})

	t.Run("Edge Case 1.6: Target User is NOT a Courier Role", func(t *testing.T) {
		payload := map[string]any{
			"user_id":         customerID.String(), // Role is CUSTOMER
			"employee_code":   "CR-9999",
			"vehicle_type":    "MOTORCYCLE",
			"max_capacity":    25,
			"current_zone_id": "ZONE_COLOMBO_01",
		}
		w := callEndpoint("POST", "/api/v1/dispatch/couriers", dispID, identityDomain.RoleDispatcher, payload)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "USER_NOT_A_COURIER")
	})

	t.Run("Edge Case 1.7: Duplicate Courier Profile Registration", func(t *testing.T) {
		payload := map[string]any{
			"user_id":         courierAID.String(), // Already registered!
			"employee_code":   "CR-7001",
			"vehicle_type":    "MOTORCYCLE",
			"max_capacity":    25,
			"current_zone_id": "ZONE_COLOMBO_01",
		}
		w := callEndpoint("POST", "/api/v1/dispatch/couriers", dispID, identityDomain.RoleDispatcher, payload)
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "COURIER_PROFILE_EXISTS")
	})

	// -------------------------------------------------------------------------
	// SECTION 2: DISPATCH BOARD & FLEET QUERY ENDPOINTS
	// -------------------------------------------------------------------------

	t.Run("Endpoint 2: GET /api/v1/dispatch/couriers - Happy Path", func(t *testing.T) {
		w := callEndpoint("GET", "/api/v1/dispatch/couriers", dispID, identityDomain.RoleDispatcher, nil)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "CR-7001")
	})

	t.Run("Endpoint 3: GET /api/v1/dispatch/board - Happy Path", func(t *testing.T) {
		w := callEndpoint("GET", "/api/v1/dispatch/board?zone_id=ZONE_COLOMBO_01", dispID, identityDomain.RoleDispatcher, nil)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "courier_loads")
	})

	// -------------------------------------------------------------------------
	// SECTION 3: ASSIGNMENT LIFECYCLE & CAPACITY EDGE CASES
	// -------------------------------------------------------------------------

	parcelA := uuid.New()
	storage.statuses[parcelA] = "CREATED"

	t.Run("Edge Case 4.1: Assign Parcel - Courier is OFF_SHIFT", func(t *testing.T) {
		assert.Equal(t, dispatchDomain.ShiftStatusOffShift, courierA.Status)
		payload := map[string]any{
			"parcel_id":  parcelA.String(),
			"courier_id": courierAID.String(),
			"notes":      "Handle with care",
		}
		w := callEndpoint("POST", "/api/v1/dispatch/assignments", dispID, identityDomain.RoleDispatcher, payload)
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "COURIER_NOT_ON_SHIFT")
	})

	t.Run("Endpoint 7: POST /api/v1/courier/shift - Happy Path ON_SHIFT", func(t *testing.T) {
		payload := map[string]string{"target_status": "ON_SHIFT"}
		w := callEndpoint("POST", "/api/v1/courier/shift", courierAID, identityDomain.RoleCourier, payload)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, dispatchDomain.ShiftStatusOnShift, courierA.Status)
	})

	t.Run("Edge Case 7.1: Invalid Shift Status Transition", func(t *testing.T) {
		payload := map[string]string{"target_status": "SLEEPING"}
		w := callEndpoint("POST", "/api/v1/courier/shift", courierAID, identityDomain.RoleCourier, payload)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "INVALID_SHIFT_TRANSITION")
	})

	t.Run("Endpoint 4: POST /api/v1/dispatch/assignments - Happy Path", func(t *testing.T) {
		payload := map[string]any{
			"parcel_id":  parcelA.String(),
			"courier_id": courierAID.String(),
			"notes":      "Handle with care",
		}
		w := callEndpoint("POST", "/api/v1/dispatch/assignments", dispID, identityDomain.RoleDispatcher, payload)
		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Equal(t, "ASSIGNED", storage.statuses[parcelA])
	})

	t.Run("Edge Case 4.2: Assign Parcel - Duplicate Assignment Rejected (AT-013)", func(t *testing.T) {
		payload := map[string]any{
			"parcel_id":  parcelA.String(), // Already ASSIGNED!
			"courier_id": courierAID.String(),
		}
		w := callEndpoint("POST", "/api/v1/dispatch/assignments", dispID, identityDomain.RoleDispatcher, payload)
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "INVALID_PARCEL_STATE")
	})

	t.Run("Edge Case 4.3: Assign Parcel - Courier Max Capacity Exceeded", func(t *testing.T) {
		// Create a small-capacity courier (capacity = 1)
		smallCourierID := uuid.New()
		storage.users[smallCourierID] = "COURIER"
		smallCourier, _ := dispatchDomain.NewCourier(smallCourierID, "CR-SMALL", dispatchDomain.VehicleTypeMotorcycle, 1, "ZONE_01")
		_ = smallCourier.ToggleShift(dispatchDomain.ShiftStatusOnShift, false)
		storage.couriers[smallCourierID] = smallCourier

		p1, p2 := uuid.New(), uuid.New()
		storage.statuses[p1] = "CREATED"
		storage.statuses[p2] = "CREATED"

		// Assign p1 -> 1/1 capacity filled
		w1 := callEndpoint("POST", "/api/v1/dispatch/assignments", dispID, identityDomain.RoleDispatcher, map[string]any{
			"parcel_id": p1.String(), "courier_id": smallCourierID.String(),
		})
		assert.Equal(t, http.StatusCreated, w1.Code)

		// Assign p2 -> Exceeds capacity!
		w2 := callEndpoint("POST", "/api/v1/dispatch/assignments", dispID, identityDomain.RoleDispatcher, map[string]any{
			"parcel_id": p2.String(), "courier_id": smallCourierID.String(),
		})
		assert.Equal(t, http.StatusConflict, w2.Code)
		assert.Contains(t, w2.Body.String(), "COURIER_CAPACITY_EXCEEDED")
	})

	t.Run("Edge Case 5.1: Cancel Assignment - Missing Reason Rejected", func(t *testing.T) {
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/dispatch/assignments/%s/cancel", parcelA.String()), dispID, identityDomain.RoleDispatcher, map[string]string{
			"reason": "",
		})
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "VALIDATION_FAILED")
	})

	t.Run("Endpoint 5: POST /api/v1/dispatch/assignments/{id}/cancel - Happy Path", func(t *testing.T) {
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/dispatch/assignments/%s/cancel", parcelA.String()), dispID, identityDomain.RoleDispatcher, map[string]string{
			"reason": "Driver vehicle flat tire",
		})
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "CREATED", storage.statuses[parcelA]) // Reverted to CREATED!
	})

	t.Run("Edge Case 5.2: Cancel Assignment - Already Cancelled Returns 404", func(t *testing.T) {
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/dispatch/assignments/%s/cancel", parcelA.String()), dispID, identityDomain.RoleDispatcher, map[string]string{
			"reason": "Trying to cancel again",
		})
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "ASSIGNMENT_NOT_FOUND")
	})

	// -------------------------------------------------------------------------
	// SECTION 4: COURIER RUN & DELIVERY EXECUTION EDGE CASES (AT-006 & ADR-004)
	// -------------------------------------------------------------------------

	parcelRun := uuid.New()
	storage.statuses[parcelRun] = "CREATED"
	_ = callEndpoint("POST", "/api/v1/dispatch/assignments", dispID, identityDomain.RoleDispatcher, map[string]any{
		"parcel_id": parcelRun.String(), "courier_id": courierAID.String(),
	})

	t.Run("Endpoint 8: GET /api/v1/courier/parcels - Happy Path Manifest", func(t *testing.T) {
		w := callEndpoint("GET", "/api/v1/courier/parcels", courierAID, identityDomain.RoleCourier, nil)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), parcelRun.String())
	})

	t.Run("Edge Case 9.1: AT-006 IDOR Defense - Foreign Courier Starting Delivery Run", func(t *testing.T) {
		// Courier B attempts to start Courier A's parcel
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/courier/parcels/%s/start-run", parcelRun.String()), courierBID, identityDomain.RoleCourier, nil)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "COURIER_NOT_ASSIGNED")
	})

	t.Run("Endpoint 9: POST /api/v1/courier/parcels/{id}/start-run - Happy Path", func(t *testing.T) {
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/courier/parcels/%s/start-run", parcelRun.String()), courierAID, identityDomain.RoleCourier, nil)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "OUT_FOR_DELIVERY", storage.statuses[parcelRun])
	})

	t.Run("Edge Case 10.1: Batch Start Runs - Empty Array", func(t *testing.T) {
		w := callEndpoint("POST", "/api/v1/courier/runs/start-batch", courierAID, identityDomain.RoleCourier, map[string]any{
			"parcel_ids": []string{},
		})
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "VALIDATION_FAILED")
	})

	t.Run("Endpoint 10: POST /api/v1/courier/runs/start-batch - Happy Path", func(t *testing.T) {
		// Prepare parcelBatch in ASSIGNED state
		parcelBatch := uuid.New()
		storage.statuses[parcelBatch] = "CREATED"
		_ = callEndpoint("POST", "/api/v1/dispatch/assignments", dispID, identityDomain.RoleDispatcher, map[string]any{
			"parcel_id": parcelBatch.String(), "courier_id": courierAID.String(),
		})

		w := callEndpoint("POST", "/api/v1/courier/runs/start-batch", courierAID, identityDomain.RoleCourier, map[string]any{
			"parcel_ids": []string{parcelBatch.String()},
		})
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "OUT_FOR_DELIVERY", storage.statuses[parcelBatch])
	})

	t.Run("Endpoint 11: POST /api/v1/deliveries/{id}/proof-photo - Happy Path", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", "doorstep.jpg")
		_, _ = part.Write([]byte("fake-jpeg-image-bytes"))
		_ = writer.Close()

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/deliveries/%s/proof-photo", parcelRun.String()), &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		p := &identityApp.Principal{UserID: courierAID.String(), Role: identityDomain.RoleCourier}
		req = req.WithContext(identityTransport.WithPrincipal(req.Context(), p))

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), "photo_url")
	})

	t.Run("Edge Case 12.1: Complete Delivery - Missing Proof Payload", func(t *testing.T) {
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/deliveries/%s/complete", parcelRun.String()), courierAID, identityDomain.RoleCourier, map[string]any{
			"received_by_name": "Kasun Perera",
		})
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "VALIDATION_FAILED")
	})

	t.Run("Edge Case 12.2: Complete Delivery - Short Signature Stroke (< 50 chars)", func(t *testing.T) {
		payload := map[string]any{
			"proof_type":       "SIGNATURE",
			"signature_data":   "too-short", // 9 chars
			"received_by_name": "Kasun Perera",
		}
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/deliveries/%s/complete", parcelRun.String()), courierAID, identityDomain.RoleCourier, payload)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "INVALID_SIGNATURE_DATA")
	})

	t.Run("Edge Case 12.3: Complete Delivery - Missing Recipient Name (Empty)", func(t *testing.T) {
		payload := map[string]any{
			"proof_type":       "PHOTO",
			"proof_url":        "https://cdn.swiftdrop.io/p.jpg",
			"received_by_name": "", // empty fails transport validation
		}
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/deliveries/%s/complete", parcelRun.String()), courierAID, identityDomain.RoleCourier, payload)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "VALIDATION_FAILED")
	})

	t.Run("Endpoint 12: POST /api/v1/deliveries/{id}/complete - Happy Path", func(t *testing.T) {
		payload := map[string]any{
			"proof_type":       "PHOTO",
			"proof_url":        "https://cdn.swiftdrop.io/proofs/doorstep.jpg",
			"received_by_name": "Kasun Perera",
			"latitude":         6.927079,
			"longitude":        79.861244,
			"notes":            "Handed directly to recipient at front gate",
		}
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/deliveries/%s/complete", parcelRun.String()), courierAID, identityDomain.RoleCourier, payload)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "DELIVERED", storage.statuses[parcelRun])
		assert.Contains(t, w.Body.String(), `"status":"DELIVERED"`)
	})

	t.Run("Edge Case 12.4: Complete Delivery - Parcel Already DELIVERED", func(t *testing.T) {
		payload := map[string]any{
			"proof_type":       "PHOTO",
			"proof_url":        "https://cdn.swiftdrop.io/proofs/doorstep.jpg",
			"received_by_name": "Kasun Perera",
		}
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/deliveries/%s/complete", parcelRun.String()), courierAID, identityDomain.RoleCourier, payload)
		assert.Equal(t, http.StatusNotFound, w.Code) // Assignment completed/closed
		assert.Contains(t, w.Body.String(), "PARCEL_NOT_ASSIGNED")
	})

	// -------------------------------------------------------------------------
	// SECTION 5: FAILED ATTEMPTS & ADR-004 CEILING LIMITS
	// -------------------------------------------------------------------------

	parcelFail := uuid.New()
	storage.statuses[parcelFail] = "CREATED"
	_ = callEndpoint("POST", "/api/v1/dispatch/assignments", dispID, identityDomain.RoleDispatcher, map[string]any{
		"parcel_id": parcelFail.String(), "courier_id": courierAID.String(),
	})
	_ = callEndpoint("POST", fmt.Sprintf("/api/v1/courier/parcels/%s/start-run", parcelFail.String()), courierAID, identityDomain.RoleCourier, nil)

	t.Run("Edge Case 13.1: Record Failed Attempt - Invalid Failure Reason Code", func(t *testing.T) {
		payload := map[string]any{
			"failure_reason_code": "WEATHER_BAD", // Not in approved 7 ADR-004 codes
			"notes":               "Heavy rain",
		}
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/deliveries/%s/failed", parcelFail.String()), courierAID, identityDomain.RoleCourier, payload)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "INVALID_FAILURE_REASON")
	})

	t.Run("Edge Case 13.2: Record Failed Attempt - Reason OTHER without >= 15 chars note", func(t *testing.T) {
		payload := map[string]any{
			"failure_reason_code": "OTHER",
			"notes":               "Dog barked", // 10 chars (< 15)
		}
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/deliveries/%s/failed", parcelFail.String()), courierAID, identityDomain.RoleCourier, payload)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "DETAILED_NOTE_REQUIRED")
	})

	t.Run("Endpoint 13: POST /api/v1/deliveries/{id}/failed - Attempt 1 Non-terminal", func(t *testing.T) {
		payload := map[string]any{
			"failure_reason_code": "RECIPIENT_UNAVAILABLE",
			"notes":               "Security gate closed; called recipient twice",
		}
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/deliveries/%s/failed", parcelFail.String()), courierAID, identityDomain.RoleCourier, payload)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "ATTEMPT_FAILED", storage.statuses[parcelFail])
		assert.Contains(t, w.Body.String(), `"is_terminal":false`)
		assert.Contains(t, w.Body.String(), `"attempt_number":1`)
	})

	t.Run("Endpoint 6: POST /api/v1/dispatch/parcels/{id}/reschedule - Happy Path", func(t *testing.T) {
		payload := map[string]any{
			"courier_id": courierAID.String(),
			"notes":      "Recipient requested reattempt after 2 PM",
		}
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/dispatch/parcels/%s/reschedule", parcelFail.String()), dispID, identityDomain.RoleDispatcher, payload)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "ASSIGNED", storage.statuses[parcelFail])
	})

	// Advance to OUT_FOR_DELIVERY and execute Attempt 2
	_ = callEndpoint("POST", fmt.Sprintf("/api/v1/courier/parcels/%s/start-run", parcelFail.String()), courierAID, identityDomain.RoleCourier, nil)
	_ = callEndpoint("POST", fmt.Sprintf("/api/v1/deliveries/%s/failed", parcelFail.String()), courierAID, identityDomain.RoleCourier, map[string]any{
		"failure_reason_code": "ACCESS_RESTRICTED",
		"notes":               "Apartment intercom out of service",
	})
	assert.Equal(t, 2, storage.attempts[parcelFail])

	// Reschedule and advance to OUT_FOR_DELIVERY for Attempt 3 -> ADR-004 Ceiling Reached!
	_ = callEndpoint("POST", fmt.Sprintf("/api/v1/dispatch/parcels/%s/reschedule", parcelFail.String()), dispID, identityDomain.RoleDispatcher, map[string]any{
		"courier_id": courierAID.String(),
	})
	_ = callEndpoint("POST", fmt.Sprintf("/api/v1/courier/parcels/%s/start-run", parcelFail.String()), courierAID, identityDomain.RoleCourier, nil)

	t.Run("Edge Case 13.3: ADR-004 3-Attempt Ceiling Forces Terminal RETURNED State", func(t *testing.T) {
		payload := map[string]any{
			"failure_reason_code": "RECIPIENT_UNAVAILABLE",
			"notes":               "Final attempt: recipient unreachable",
		}
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/deliveries/%s/failed", parcelFail.String()), courierAID, identityDomain.RoleCourier, payload)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "RETURNED", storage.statuses[parcelFail])
		assert.Contains(t, w.Body.String(), `"is_terminal":true`)
		assert.Contains(t, w.Body.String(), `"parcel_status":"RETURNED"`)
		assert.Contains(t, w.Body.String(), `"attempt_number":3`)
	})

	t.Run("Edge Case 13.4: Attempt Count Exceeded Ceiling Rejection", func(t *testing.T) {
		// Attempting a 4th attempt on a returned parcel (assignment was closed)
		payload := map[string]any{
			"failure_reason_code": "RECIPIENT_UNAVAILABLE",
			"notes":               "Attempt 4 should never happen",
		}
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/deliveries/%s/failed", parcelFail.String()), courierAID, identityDomain.RoleCourier, payload)
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "PARCEL_NOT_ASSIGNED")
	})

	t.Run("Edge Case 13.5: ADR-004 Immediate Return upon REFUSED_BY_RECIPIENT", func(t *testing.T) {
		pRefuse := uuid.New()
		storage.statuses[pRefuse] = "CREATED"
		_ = callEndpoint("POST", "/api/v1/dispatch/assignments", dispID, identityDomain.RoleDispatcher, map[string]any{
			"parcel_id": pRefuse.String(), "courier_id": courierAID.String(),
		})
		_ = callEndpoint("POST", fmt.Sprintf("/api/v1/courier/parcels/%s/start-run", pRefuse.String()), courierAID, identityDomain.RoleCourier, nil)

		payload := map[string]any{
			"failure_reason_code": "REFUSED_BY_RECIPIENT",
			"notes":               "Recipient refused delivery at doorstep",
		}
		w := callEndpoint("POST", fmt.Sprintf("/api/v1/deliveries/%s/failed", pRefuse.String()), courierAID, identityDomain.RoleCourier, payload)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "RETURNED", storage.statuses[pRefuse])
		assert.Contains(t, w.Body.String(), `"is_terminal":true`)
	})

	t.Run("Endpoint 14: GET /api/v1/deliveries/parcels/{id} - Delivery History", func(t *testing.T) {
		w := callEndpoint("GET", fmt.Sprintf("/api/v1/deliveries/parcels/%s", parcelFail.String()), dispID, identityDomain.RoleDispatcher, nil)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "attempt_count")
	})
}

func (*comprehensiveMockStorage) GetAssignedParcels(context.Context, string) ([]dispatchApp.AssignedParcelDTO, error) {
	return nil, nil
}
