package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	parcelsApp "github.com/swiftdrop/core-api/internal/parcels/application"
	parcelsDomain "github.com/swiftdrop/core-api/internal/parcels/domain"
)

func TestPostgresTransactionManager_NilDB(t *testing.T) {
	tm := NewPostgresTransactionManager(nil)
	assert.NotNil(t, tm)
}

func TestPostgresCourierRepository_Constructor(t *testing.T) {
	repo := NewPostgresCourierRepository(nil)
	assert.NotNil(t, repo)
}

func TestPostgresAssignmentRepository_Constructor(t *testing.T) {
	repo := NewPostgresAssignmentRepository(nil)
	assert.NotNil(t, repo)
}

func TestPostgresDispatchBoardReader_Constructor(t *testing.T) {
	reader := NewPostgresDispatchBoardReader(nil)
	assert.NotNil(t, reader)
}

func TestPostgresParcelLifecycleUpdater_Constructor(t *testing.T) {
	updater := NewPostgresParcelLifecycleUpdater(nil)
	assert.NotNil(t, updater)
}

type mockParcelLifecycleService struct {
	calledTx      bool
	calledNonTx   bool
	lastCmd       parcelsApp.TransitionParcelStatusCommand
	executeTxErr  error
	executeNonErr error
}

func (m *mockParcelLifecycleService) Execute(ctx context.Context, cmd parcelsApp.TransitionParcelStatusCommand) (parcelsApp.TransitionParcelStatusResult, error) {
	m.calledNonTx = true
	m.lastCmd = cmd
	return parcelsApp.TransitionParcelStatusResult{}, m.executeNonErr
}

func (m *mockParcelLifecycleService) ExecuteTx(ctx context.Context, tx *sql.Tx, cmd parcelsApp.TransitionParcelStatusCommand) (parcelsApp.TransitionParcelStatusResult, error) {
	m.calledTx = true
	m.lastCmd = cmd
	return parcelsApp.TransitionParcelStatusResult{}, m.executeTxErr
}

func TestPostgresParcelLifecycleUpdater_Delegation(t *testing.T) {
	mockService := &mockParcelLifecycleService{}
	updater := NewPostgresParcelLifecycleUpdater(nil, mockService)
	assert.NotNil(t, updater)

	parcelID := uuid.New()
	actorID := uuid.New()

	// 1. Without transaction - delegates to Execute
	err := updater.TransitionStatus(
		context.Background(),
		nil,
		parcelID,
		"CREATED",
		"ASSIGNED",
		actorID,
		"DISPATCHER",
		"Assigned to courier CR-001",
	)
	assert.NoError(t, err)
	assert.True(t, mockService.calledNonTx)
	assert.Equal(t, parcelID.String(), mockService.lastCmd.ParcelID)
	assert.Equal(t, parcelsDomain.ParcelStatus("ASSIGNED"), mockService.lastCmd.TargetStatus)
	assert.Equal(t, actorID.String(), *mockService.lastCmd.ActorID)
	assert.Equal(t, "DISPATCHER", mockService.lastCmd.ActorRole)
	assert.Equal(t, "Assigned to courier CR-001", mockService.lastCmd.Reason)

	// 2. Error propagation
	mockService.executeNonErr = errors.New("transition rejected")
	err = updater.TransitionStatus(
		context.Background(),
		nil,
		parcelID,
		"CREATED",
		"ASSIGNED",
		actorID,
		"DISPATCHER",
		"Note",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transition rejected")
}

func TestPostgresParcelLifecycleUpdater_NilDBErrors(t *testing.T) {
	updater := NewPostgresParcelLifecycleUpdater(nil)

	parcelID := uuid.New()
	_, _, err := updater.GetStatusWithLock(context.Background(), nil, parcelID)
	assert.Error(t, err)

	err = updater.TransitionStatus(context.Background(), nil, parcelID, "CREATED", "ASSIGNED", uuid.New(), "DISPATCHER", "note")
	assert.Error(t, err)
}

func TestPostgresOutboxWriter_Constructor(t *testing.T) {
	writer := NewPostgresOutboxWriter(nil)
	assert.NotNil(t, writer)
}
