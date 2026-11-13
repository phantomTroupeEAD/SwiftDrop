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

func TestNewStorageClientFromEnv_R2_AccountIDFallback(t *testing.T) {
	os.Setenv("OBJECT_STORAGE_DRIVER", "r2")
	os.Setenv("OBJECT_STORAGE_ACCOUNT_ID", "a1b2c3d4e5f6")
	os.Setenv("OBJECT_STORAGE_BUCKET", "swiftdrop-pod-attachments")
	os.Setenv("OBJECT_STORAGE_ACCESS_KEY", "mock-r2-access-key")
	os.Setenv("OBJECT_STORAGE_SECRET_KEY", "mock-r2-secret-key")
	defer func() {
		os.Unsetenv("OBJECT_STORAGE_DRIVER")
		os.Unsetenv("OBJECT_STORAGE_ACCOUNT_ID")
		os.Unsetenv("OBJECT_STORAGE_BUCKET")
		os.Unsetenv("OBJECT_STORAGE_ACCESS_KEY")
		os.Unsetenv("OBJECT_STORAGE_SECRET_KEY")
	}()

	client, err := NewStorageClientFromEnv()
	require.NoError(t, err)
	require.IsType(t, &S3StorageClient{}, client)
	s3Client := client.(*S3StorageClient)
	assert.Equal(t, "https://a1b2c3d4e5f6.r2.cloudflarestorage.com", s3Client.config.Endpoint)
	assert.Equal(t, "swiftdrop-pod-attachments", s3Client.config.Bucket)
	assert.Equal(t, "auto", s3Client.config.Region)
	assert.Equal(t, "mock-r2-access-key", s3Client.config.AccessKey)
	assert.Equal(t, "mock-r2-secret-key", s3Client.config.SecretKey)
}

func TestNewStorageClientFromEnv_Unsupported(t *testing.T) {
	os.Setenv("OBJECT_STORAGE_DRIVER", "unsupported_storage_xyz")
	defer os.Unsetenv("OBJECT_STORAGE_DRIVER")

	client, err := NewStorageClientFromEnv()
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "unsupported storage driver")
}
