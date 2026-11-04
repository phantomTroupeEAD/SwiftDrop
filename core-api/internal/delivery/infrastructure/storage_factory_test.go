package infrastructure

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStorageClientFromEnv_LocalDefault(t *testing.T) {
	os.Unsetenv("OBJECT_STORAGE_DRIVER")
	os.Unsetenv("STORAGE_DRIVER")

	client, err := NewStorageClientFromEnv()
	require.NoError(t, err)
	assert.IsType(t, &LocalStorageClient{}, client)
}

func TestNewStorageClientFromEnv_S3(t *testing.T) {
	os.Setenv("OBJECT_STORAGE_DRIVER", "s3")
	os.Setenv("S3_ENDPOINT", "https://s3.ap-southeast-1.amazonaws.com")
	os.Setenv("S3_BUCKET", "test-bucket")
	defer func() {
		os.Unsetenv("OBJECT_STORAGE_DRIVER")
		os.Unsetenv("S3_ENDPOINT")
		os.Unsetenv("S3_BUCKET")
	}()

	client, err := NewStorageClientFromEnv()
	require.NoError(t, err)
	assert.IsType(t, &S3StorageClient{}, client)
}

func TestNewStorageClientFromEnv_Unsupported(t *testing.T) {
	os.Setenv("OBJECT_STORAGE_DRIVER", "unsupported_storage_xyz")
	defer os.Unsetenv("OBJECT_STORAGE_DRIVER")

	client, err := NewStorageClientFromEnv()
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "unsupported storage driver")
}

