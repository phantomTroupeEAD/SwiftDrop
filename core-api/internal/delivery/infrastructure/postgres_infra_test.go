package infrastructure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPostgresDeliveryRepository_Constructor(t *testing.T) {
	repo := NewPostgresDeliveryRepository(nil)
	assert.NotNil(t, repo)
}

func TestPostgresAssignmentFinder_Constructor(t *testing.T) {
	finder := NewPostgresAssignmentFinder(nil)
	assert.NotNil(t, finder)
}

func TestPostgresManifestReader_Constructor(t *testing.T) {
	reader := NewPostgresManifestReader(nil)
	assert.NotNil(t, reader)
}

