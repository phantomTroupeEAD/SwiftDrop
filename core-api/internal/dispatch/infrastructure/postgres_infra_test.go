package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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

func TestPostgresOutboxWriter_Constructor(t *testing.T) {
	writer := NewPostgresOutboxWriter(nil)
	assert.NotNil(t, writer)
}

