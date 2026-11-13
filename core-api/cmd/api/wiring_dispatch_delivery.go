package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/google/uuid"
	deliveryApp "github.com/swiftdrop/core-api/internal/delivery/application"
	deliveryInfra "github.com/swiftdrop/core-api/internal/delivery/infrastructure"
	deliveryTransport "github.com/swiftdrop/core-api/internal/delivery/transport"
	dispatchApp "github.com/swiftdrop/core-api/internal/dispatch/application"
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
		roleChecker   dispatchApp.UserRoleChecker
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
	} else {
		log.Println("[WARN] PostgreSQL database connection is unavailable; Member 03 Dispatch & Delivery wired with fail-fast persistence (operations requiring DB will fail per ACID rules).")
	}

	txMgr = dispatchInfra.NewPostgresTransactionManager(db)
	pgCourierRepo := dispatchInfra.NewPostgresCourierRepository(db)
	courierRepo = pgCourierRepo
	roleChecker = pgCourierRepo
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

	// 1. Dispatch Application Use Cases
	regCourierUC := dispatchApp.NewRegisterCourierUseCase(courierRepo, roleChecker)
	listCouriersQ := dispatchApp.NewListCouriersQueryHandler(courierRepo)
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
