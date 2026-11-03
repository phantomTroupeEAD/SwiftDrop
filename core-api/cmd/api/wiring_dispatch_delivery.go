package main

import (
	"context"
	"database/sql"
	"io"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	deliveryApp "github.com/swiftdrop/core-api/internal/delivery/application"
	deliveryDomain "github.com/swiftdrop/core-api/internal/delivery/domain"
	deliveryInfra "github.com/swiftdrop/core-api/internal/delivery/infrastructure"
	deliveryTransport "github.com/swiftdrop/core-api/internal/delivery/transport"
	dispatchApp "github.com/swiftdrop/core-api/internal/dispatch/application"
	dispatchDomain "github.com/swiftdrop/core-api/internal/dispatch/domain"
	dispatchInfra "github.com/swiftdrop/core-api/internal/dispatch/infrastructure"
	dispatchTransport "github.com/swiftdrop/core-api/internal/dispatch/transport"
	identityTransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

// registerDispatchAndDeliveryModules wires Member 03 Dispatch and Delivery components
// into the Core API modular monolith router.
func registerDispatchAndDeliveryModules(
	mux *http.ServeMux,
	db *sql.DB,
	sec *identityTransport.SecurityMiddleware,
) {
	var (
		txMgr         dispatchApp.TransactionManager
		courierRepo   dispatchApp.CourierRepository
		asgnRepo      dispatchApp.AssignmentRepository
		boardReader   dispatchApp.DispatchBoardReader
		parcelUpdater dispatchApp.ParcelLifecycleUpdater
		outboxWriter  dispatchApp.OutboxWriter

		deliveryRepo   deliveryApp.DeliveryAttemptRepository
		asgnFinder     deliveryApp.ActiveAssignmentFinder
		manifestReader deliveryApp.CourierManifestReader
		storageClient  deliveryApp.StorageClient
	)

	if db != nil {
		log.Println("[INFO] Wiring Member 03 Dispatch & Delivery using PostgreSQL persistence.")

		txMgr = dispatchInfra.NewPostgresTransactionManager(db)
		courierRepo = dispatchInfra.NewPostgresCourierRepository(db)
		asgnRepo = dispatchInfra.NewPostgresAssignmentRepository(db)
		boardReader = dispatchInfra.NewPostgresDispatchBoardReader(db)
		parcelUpdater = dispatchInfra.NewPostgresParcelLifecycleUpdater(db)
		outboxWriter = dispatchInfra.NewPostgresOutboxWriter(db)

		deliveryRepo = deliveryInfra.NewPostgresDeliveryRepository(db)
		asgnFinder = deliveryInfra.NewPostgresAssignmentFinder(db)
		manifestReader = deliveryInfra.NewPostgresManifestReader(db)

		var err error
		storageClient, err = deliveryInfra.NewStorageClientFromEnv()
		if err != nil {
			log.Printf("[WARN] Failed to initialize storage client from environment: %v; falling back to local storage.", err)
			storageClient, _ = deliveryInfra.NewLocalStorageClient("./data/proofs", "/api/v1/deliveries/proofs")
		}
	} else {
		log.Println("[NOTICE] Wiring Member 03 Dispatch & Delivery using in-memory mock fallback.")

		mockMem := newMockDispatchDeliveryContainer()
		txMgr = mockMem
		courierRepo = mockMem
		asgnRepo = mockMem
		boardReader = mockMem
		parcelUpdater = mockMem
		outboxWriter = mockMem

		deliveryRepo = mockMem
		asgnFinder = mockMem
		manifestReader = mockMem
		storageClient = mockMem
	}

	// 1. Dispatch Application Use Cases
	regCourierUC := dispatchApp.NewRegisterCourierUseCase(courierRepo)
	listCouriersQ := dispatchApp.NewListCouriersQueryHandler(boardReader)
	boardQ := dispatchApp.NewGetDispatchBoardQueryHandler(boardReader)
	assignCourierUC := dispatchApp.NewAssignCourierUseCase(txMgr, courierRepo, asgnRepo, parcelUpdater, outboxWriter)
	cancelAsgnUC := dispatchApp.NewCancelAssignmentUseCase(txMgr, asgnRepo, parcelUpdater, outboxWriter)
	reschedUC := dispatchApp.NewRescheduleParcelUseCase(txMgr, courierRepo, asgnRepo, parcelUpdater, outboxWriter)
	toggleShiftUC := dispatchApp.NewToggleShiftUseCase(courierRepo)

	// 2. Delivery Application Use Cases
	deliveryTxMgr := &dispatchToDeliveryTxAdapter{txMgr: txMgr}
	deliveryParcelUpd := &dispatchToDeliveryParcelAdapter{updater: parcelUpdater}
	deliveryOutbox := &dispatchToDeliveryOutboxAdapter{writer: outboxWriter}

	uploadPhotoUC := deliveryApp.NewUploadProofPhotoUseCase(storageClient, asgnFinder)
	completeDeliveryUC := deliveryApp.NewCompleteDeliveryUseCase(deliveryTxMgr, asgnFinder, deliveryRepo, deliveryParcelUpd, deliveryOutbox)
	recordFailedUC := deliveryApp.NewRecordFailedAttemptUseCase(deliveryTxMgr, asgnFinder, deliveryRepo, deliveryParcelUpd, deliveryOutbox)
	getDeliveryHistoryQ := deliveryApp.NewGetDeliveryHistoryQueryHandler(deliveryRepo, deliveryParcelUpd, storageClient)
	manifestQ := deliveryApp.NewGetCourierManifestQueryHandler(manifestReader)
	startRunUC := deliveryApp.NewStartDeliveryRunUseCase(deliveryTxMgr, asgnFinder, deliveryParcelUpd, deliveryOutbox)

	// 3. Transport Handlers
	dispatchHandler := dispatchTransport.NewDispatchHandler(
		regCourierUC,
		listCouriersQ,
		boardQ,
		assignCourierUC,
		cancelAsgnUC,
		reschedUC,
	)
	courierShiftHandler := dispatchTransport.NewCourierShiftHandler(toggleShiftUC)

	deliveryHandler := deliveryTransport.NewDeliveryHandler(
		uploadPhotoUC,
		completeDeliveryUC,
		recordFailedUC,
		getDeliveryHistoryQ,
	)
	manifestHandler := deliveryTransport.NewManifestHandler(
		manifestQ,
		startRunUC,
	)

	// 4. Register HTTP Routes with RBAC Middleware
	dispatchTransport.RegisterRoutes(mux, dispatchHandler, courierShiftHandler, sec)
	deliveryTransport.RegisterRoutes(mux, deliveryHandler, manifestHandler, sec)
}

// Adapters bridging application interface types across dispatch and delivery packages
type dispatchToDeliveryTxAdapter struct {
	txMgr dispatchApp.TransactionManager
}

func (a *dispatchToDeliveryTxAdapter) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return a.txMgr.WithTransaction(ctx, fn)
}

type dispatchToDeliveryParcelAdapter struct {
	updater dispatchApp.ParcelLifecycleUpdater
}

func (a *dispatchToDeliveryParcelAdapter) TransitionStatus(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID, fromStatus, toStatus string, actorID uuid.UUID, actorRole, note string) error {
	return a.updater.TransitionStatus(ctx, tx, parcelID, fromStatus, toStatus, actorID, actorRole, note)
}

func (a *dispatchToDeliveryParcelAdapter) GetStatusWithLock(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (string, int, error) {
	return a.updater.GetStatusWithLock(ctx, tx, parcelID)
}

type dispatchToDeliveryOutboxAdapter struct {
	writer dispatchApp.OutboxWriter
}

func (a *dispatchToDeliveryOutboxAdapter) Write(ctx context.Context, tx *sql.Tx, aggregateType string, aggregateID uuid.UUID, aggregateVersion int, eventType string, payload any, correlationID string) error {
	return a.writer.Write(ctx, tx, aggregateType, aggregateID, aggregateVersion, eventType, payload, correlationID)
}

// In-Memory Fallback Container for offline local testing
type mockDispatchDeliveryContainer struct {
	mu          sync.RWMutex
	couriers    map[uuid.UUID]*dispatchDomain.Courier
	assignments map[uuid.UUID]*dispatchDomain.CourierAssignment
	attempts    map[uuid.UUID][]*deliveryDomain.DeliveryAttempt
	statuses    map[uuid.UUID]string
	attemptNums map[uuid.UUID]int
}

func newMockDispatchDeliveryContainer() *mockDispatchDeliveryContainer {
	m := &mockDispatchDeliveryContainer{
		couriers:    make(map[uuid.UUID]*dispatchDomain.Courier),
		assignments: make(map[uuid.UUID]*dispatchDomain.CourierAssignment),
		attempts:    make(map[uuid.UUID][]*deliveryDomain.DeliveryAttempt),
		statuses:    make(map[uuid.UUID]string),
		attemptNums: make(map[uuid.UUID]int),
	}

	// Pre-seed mock courier for local interactive testing
	// Matches seed courier ID from deploy/seeds/000006_couriers_seed.sql
	cid := uuid.MustParse("0192e2b0-8c24-7df7-b087-0b1297e6e5a1")
	c, _ := dispatchDomain.NewCourier(cid, "CR-7001", dispatchDomain.VehicleTypeMotorcycle, 25, "ZONE_COLOMBO_01")
	_ = c.ToggleShift(dispatchDomain.ShiftStatusOnShift, true)
	m.couriers[cid] = c

	return m
}

func (m *mockDispatchDeliveryContainer) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return fn(nil)
}

func (m *mockDispatchDeliveryContainer) GetByID(ctx context.Context, userID uuid.UUID) (*dispatchDomain.Courier, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.couriers[userID], nil
}

func (m *mockDispatchDeliveryContainer) GetByIDWithLock(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (*dispatchDomain.Courier, error) {
	return m.GetByID(ctx, userID)
}

func (m *mockDispatchDeliveryContainer) Save(ctx context.Context, tx *sql.Tx, courier *dispatchDomain.Courier) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.couriers[courier.UserID] = courier
	return nil
}

func (m *mockDispatchDeliveryContainer) List(ctx context.Context, zoneID string, status *dispatchDomain.ShiftStatus, vehicleType *dispatchDomain.VehicleType) ([]*dispatchDomain.Courier, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*dispatchDomain.Courier
	for _, c := range m.couriers {
		if zoneID != "" && c.CurrentZoneID != zoneID {
			continue
		}
		list = append(list, c)
	}
	return list, nil
}

func (m *mockDispatchDeliveryContainer) Create(ctx context.Context, tx *sql.Tx, asgn *dispatchDomain.CourierAssignment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assignments[asgn.ID] = asgn
	return nil
}

func (m *mockDispatchDeliveryContainer) GetByIDAssignment(ctx context.Context, id uuid.UUID) (*dispatchDomain.CourierAssignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.assignments[id], nil
}

func (m *mockDispatchDeliveryContainer) GetActiveByParcelID(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*dispatchDomain.CourierAssignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, a := range m.assignments {
		if a.ParcelID == parcelID && a.Status == dispatchDomain.AssignmentStatusActive {
			return a, nil
		}
	}
	return nil, nil
}

func (m *mockDispatchDeliveryContainer) CountActiveByCourierID(ctx context.Context, tx *sql.Tx, courierID uuid.UUID) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cnt := 0
	for _, a := range m.assignments {
		if a.CourierID == courierID && a.Status == dispatchDomain.AssignmentStatusActive {
			cnt++
		}
	}
	return cnt, nil
}

func (m *mockDispatchDeliveryContainer) Update(ctx context.Context, tx *sql.Tx, asgn *dispatchDomain.CourierAssignment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assignments[asgn.ID] = asgn
	return nil
}

func (m *mockDispatchDeliveryContainer) GetUnassignedParcels(ctx context.Context, zoneID string) ([]dispatchApp.UnassignedParcelDTO, error) {
	return []dispatchApp.UnassignedParcelDTO{
		{ID: uuid.New().String(), TrackingNumber: "SD-20261102-1001", WeightKg: 1.5, Status: "CREATED", CreatedAt: time.Now()},
	}, nil
}

func (m *mockDispatchDeliveryContainer) GetCourierLoads(ctx context.Context, zoneID string) ([]dispatchApp.CourierLoadDTO, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []dispatchApp.CourierLoadDTO
	for _, c := range m.couriers {
		if zoneID != "" && c.CurrentZoneID != zoneID {
			continue
		}
		list = append(list, dispatchApp.CourierLoadDTO{
			UserID:      c.UserID.String(),
			CourierName: c.EmployeeCode,
			ActiveCount: 0,
			MaxCapacity: c.MaxCapacity,
			Status:      c.Status.String(),
			VehicleType: c.Vehicle.String(),
		})
	}
	return list, nil
}

func (m *mockDispatchDeliveryContainer) TransitionStatus(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID, fromStatus, toStatus string, actorID uuid.UUID, actorRole, note string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statuses[parcelID] = toStatus
	return nil
}

func (m *mockDispatchDeliveryContainer) GetStatusWithLock(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (string, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.statuses[parcelID]
	if !ok {
		st = "OUT_FOR_DELIVERY"
	}
	return st, m.attemptNums[parcelID], nil
}

func (m *mockDispatchDeliveryContainer) Write(ctx context.Context, tx *sql.Tx, aggregateType string, aggregateID uuid.UUID, aggregateVersion int, eventType string, payload any, correlationID string) error {
	return nil
}

func (m *mockDispatchDeliveryContainer) InsertAttempt(ctx context.Context, tx *sql.Tx, attempt *deliveryDomain.DeliveryAttempt) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts[attempt.ParcelID] = append(m.attempts[attempt.ParcelID], attempt)
	m.attemptNums[attempt.ParcelID] = attempt.AttemptNumber
	return nil
}

func (m *mockDispatchDeliveryContainer) ListAttemptsByParcelID(ctx context.Context, parcelID uuid.UUID) ([]*deliveryDomain.DeliveryAttempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.attempts[parcelID], nil
}

func (m *mockDispatchDeliveryContainer) GetAttemptCount(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.attempts[parcelID]), nil
}

func (m *mockDispatchDeliveryContainer) GetActiveAssignment(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*deliveryApp.ActiveAssignmentDTO, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, a := range m.assignments {
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

func (m *mockDispatchDeliveryContainer) CompleteAssignment(ctx context.Context, tx *sql.Tx, assignmentID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.assignments[assignmentID]; ok {
		_ = a.Complete()
	}
	return nil
}

func (m *mockDispatchDeliveryContainer) GetActiveManifest(ctx context.Context, courierID uuid.UUID) ([]deliveryApp.CourierManifestItemDTO, error) {
	return []deliveryApp.CourierManifestItemDTO{
		{
			ParcelID:       uuid.New().String(),
			TrackingNumber: "SD-20261102-1001",
			CurrentStatus:  "OUT_FOR_DELIVERY",
			RecipientName:  "Sunil Silva",
			DeliveryStreet: "123 Galle Road",
			DeliveryCity:   "Colombo",
			AttemptCount:   0,
			AssignedAt:     time.Now(),
		},
	}, nil
}

func (m *mockDispatchDeliveryContainer) UploadProofPhoto(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	return key, nil
}

func (m *mockDispatchDeliveryContainer) GeneratePresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "/api/v1/deliveries/proofs/" + key, nil
}

