package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dispatchApp "github.com/swiftdrop/core-api/internal/dispatch/application"
	dispatchDomain "github.com/swiftdrop/core-api/internal/dispatch/domain"
	dispatchTransport "github.com/swiftdrop/core-api/internal/dispatch/transport"
	identityApp "github.com/swiftdrop/core-api/internal/identity/application"
	identityDomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityTransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

// concurrentMockAssignmentStorage provides thread-safe mock storage simulating
// PostgreSQL row-level locking (SELECT ... FOR UPDATE) and unique index enforcement (AT-013).
type concurrentMockAssignmentStorage struct {
	mu          sync.Mutex
	couriers    map[uuid.UUID]*dispatchDomain.Courier
	assignments map[uuid.UUID]*dispatchDomain.CourierAssignment
	statuses    map[uuid.UUID]string
	attempts    map[uuid.UUID]int
}

func newConcurrentMockAssignmentStorage() *concurrentMockAssignmentStorage {
	return &concurrentMockAssignmentStorage{
		couriers:    make(map[uuid.UUID]*dispatchDomain.Courier),
		assignments: make(map[uuid.UUID]*dispatchDomain.CourierAssignment),
		statuses:    make(map[uuid.UUID]string),
		attempts:    make(map[uuid.UUID]int),
	}
}

func (s *concurrentMockAssignmentStorage) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn(nil)
}

func (s *concurrentMockAssignmentStorage) GetByIDWithLock(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (*dispatchDomain.Courier, error) {
	c, ok := s.couriers[userID]
	if !ok {
		return nil, nil
	}
	return c, nil
}

func (s *concurrentMockAssignmentStorage) GetByID(ctx context.Context, userID uuid.UUID) (*dispatchDomain.Courier, error) {
	return s.GetByIDWithLock(ctx, nil, userID)
}

func (s *concurrentMockAssignmentStorage) GetUserRole(ctx context.Context, userID uuid.UUID) (string, bool, error) {
	return "COURIER", true, nil
}

func (s *concurrentMockAssignmentStorage) Save(ctx context.Context, tx *sql.Tx, courier *dispatchDomain.Courier) error {
	s.couriers[courier.UserID] = courier
	return nil
}

func (s *concurrentMockAssignmentStorage) List(ctx context.Context, zoneID string, status *dispatchDomain.ShiftStatus, vehicleType *dispatchDomain.VehicleType) ([]*dispatchDomain.Courier, error) {
	var list []*dispatchDomain.Courier
	for _, c := range s.couriers {
		list = append(list, c)
	}
	return list, nil
}

func (s *concurrentMockAssignmentStorage) Create(ctx context.Context, tx *sql.Tx, asgn *dispatchDomain.CourierAssignment) error {
	// Simulates PostgreSQL unique index idx_assignments_parcel_single_active:
	// exactly ONE active assignment per parcel is permitted!
	for _, existing := range s.assignments {
		if existing.ParcelID == asgn.ParcelID && existing.Status == dispatchDomain.AssignmentStatusActive {
			return dispatchApp.ErrDuplicateActiveAssignment
		}
	}
	s.assignments[asgn.ID] = asgn
	return nil
}

func (s *concurrentMockAssignmentStorage) GetByIDAssignment(ctx context.Context, id uuid.UUID) (*dispatchDomain.CourierAssignment, error) {
	return s.assignments[id], nil
}

func (s *concurrentMockAssignmentStorage) GetActiveByParcelID(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*dispatchDomain.CourierAssignment, error) {
	for _, a := range s.assignments {
		if a.ParcelID == parcelID && a.Status == dispatchDomain.AssignmentStatusActive {
			return a, nil
		}
	}
	return nil, nil
}

func (s *concurrentMockAssignmentStorage) GetActiveByParcelIDReadOnly(ctx context.Context, parcelID uuid.UUID) (*dispatchDomain.CourierAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.GetActiveByParcelID(ctx, nil, parcelID)
}

func (s *concurrentMockAssignmentStorage) CountActiveByCourierID(ctx context.Context, tx *sql.Tx, courierID uuid.UUID) (int, error) {
	cnt := 0
	for _, a := range s.assignments {
		if a.CourierID == courierID && a.Status == dispatchDomain.AssignmentStatusActive {
			cnt++
		}
	}
	return cnt, nil
}

func (s *concurrentMockAssignmentStorage) Update(ctx context.Context, tx *sql.Tx, asgn *dispatchDomain.CourierAssignment) error {
	s.assignments[asgn.ID] = asgn
	return nil
}

func (s *concurrentMockAssignmentStorage) GetStatusWithLock(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (string, int, error) {
	st, ok := s.statuses[parcelID]
	if !ok {
		st = "CREATED"
	}
	return st, s.attempts[parcelID], nil
}

func (s *concurrentMockAssignmentStorage) TransitionStatus(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID, fromStatus, toStatus string, actorID uuid.UUID, actorRole, note string) error {
	s.statuses[parcelID] = toStatus
	return nil
}

func (s *concurrentMockAssignmentStorage) Write(ctx context.Context, tx *sql.Tx, aggregateType string, aggregateID uuid.UUID, aggregateVersion int, eventType string, payload any, correlationID string) error {
	return nil
}

func (s *concurrentMockAssignmentStorage) GetUnassignedParcels(ctx context.Context, zoneID string) ([]dispatchApp.UnassignedParcelDTO, error) {
	return nil, nil
}

func (s *concurrentMockAssignmentStorage) GetCourierLoads(ctx context.Context, zoneID string) ([]dispatchApp.CourierLoadDTO, error) {
	return nil, nil
}

// TestAT013_AssignmentConcurrencyRace tests that 10 concurrent goroutines racing
// to assign the same parcel result in exactly ONE 201 Created and 9 409 Conflict responses.
func TestAT013_AssignmentConcurrencyRace(t *testing.T) {
	storage := newConcurrentMockAssignmentStorage()

	// 1. Create target parcel in CREATED status
	targetParcelID := uuid.New()
	storage.statuses[targetParcelID] = "CREATED"

	// 2. Create 10 couriers on shift
	const concurrencyCount = 10
	courierIDs := make([]uuid.UUID, concurrencyCount)
	for i := 0; i < concurrencyCount; i++ {
		cid := uuid.New()
		courierIDs[i] = cid
		c, err := dispatchDomain.NewCourier(cid, fmt.Sprintf("CR-%04d", i+1), dispatchDomain.VehicleTypeMotorcycle, 25, "ZONE_COLOMBO_01")
		require.NoError(t, err)
		require.NoError(t, c.ToggleShift(dispatchDomain.ShiftStatusOnShift, true))
		_ = storage.Save(context.Background(), nil, c)
	}

	// 3. Construct use case & transport handler
	assignUC := dispatchApp.NewAssignCourierUseCase(storage, storage, storage, storage, storage)
	cancelUC := dispatchApp.NewCancelAssignmentUseCase(storage, storage, storage, storage)
	reschedUC := dispatchApp.NewRescheduleParcelUseCase(storage, storage, storage, storage, storage)
	regUC := dispatchApp.NewRegisterCourierUseCase(storage, storage)
	listQ := dispatchApp.NewListCouriersQueryHandler(storage)
	boardQ := dispatchApp.NewGetDispatchBoardQueryHandler(storage)

	dispatchHandler := dispatchTransport.NewDispatchHandler(regUC, listQ, boardQ, assignUC, cancelUC, reschedUC)

	// Create test HTTP router
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/dispatch/assignments", dispatchHandler.AssignCourier)

	// 4. Concurrently fire 10 simultaneous assignment requests
	var successCount int32
	var conflictCount int32
	var wg sync.WaitGroup
	startBarrier := make(chan struct{})

	dispatcherID := uuid.New().String()

	for i := 0; i < concurrencyCount; i++ {
		wg.Add(1)
		assignedCourierID := courierIDs[i]

		go func(cid uuid.UUID) {
			defer wg.Done()
			<-startBarrier // Synchronize all goroutines to fire at the exact same instant

			reqPayload, _ := json.Marshal(map[string]string{
				"parcel_id":  targetParcelID.String(),
				"courier_id": cid.String(),
				"notes":      "Concurrent test assignment",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch/assignments", bytes.NewReader(reqPayload))
			req.Header.Set("Content-Type", "application/json")

			// Inject Dispatcher Principal
			p := &identityApp.Principal{
				UserID: dispatcherID,
				Email:  "dispatcher@swiftdrop.io",
				Role:   identityDomain.RoleDispatcher,
			}
			req = req.WithContext(identityTransport.WithPrincipal(req.Context(), p))

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			switch rec.Code {
			case http.StatusCreated:
				atomic.AddInt32(&successCount, 1)
			case http.StatusConflict:
				atomic.AddInt32(&conflictCount, 1)
			default:
				t.Errorf("Unexpected status code %d: %s", rec.Code, rec.Body.String())
			}
		}(assignedCourierID)
	}

	// Trigger all goroutines simultaneously
	close(startBarrier)
	wg.Wait()

	// 5. Invariant Assertions
	assert.Equal(t, int32(1), atomic.LoadInt32(&successCount), "AT-013: Exactly ONE assignment request must succeed")
	assert.Equal(t, int32(concurrencyCount-1), atomic.LoadInt32(&conflictCount), "AT-013: All competing requests must fail with 409 Conflict")

	// 6. Persistence State Verification
	activeCount := 0
	for _, a := range storage.assignments {
		if a.ParcelID == targetParcelID && a.Status == dispatchDomain.AssignmentStatusActive {
			activeCount++
		}
	}
	assert.Equal(t, 1, activeCount, "AT-013: Exactly ONE active assignment must exist in persistence for target parcel")
	assert.Equal(t, "ASSIGNED", storage.statuses[targetParcelID], "AT-013: Parcel status must be ASSIGNED")
}

func (*concurrentMockAssignmentStorage) GetAssignedParcels(context.Context, string) ([]dispatchApp.AssignedParcelDTO, error) {
	return nil, nil
}
