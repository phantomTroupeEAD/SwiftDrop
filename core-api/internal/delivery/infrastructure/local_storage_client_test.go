package infrastructure

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorageClient_UploadProofPhoto(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "proof_tests_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	client, err := NewLocalStorageClient(tempDir, "/api/v1/deliveries/proofs")
	require.NoError(t, err)

	ctx := context.Background()
	payload := []byte("fake-jpeg-image-binary-stream")
	key := "2026/11/02/test-proof-1.jpg"

	savedKey, err := client.UploadProofPhoto(ctx, key, bytes.NewReader(payload), "image/jpeg")
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(key), savedKey)

	// Verify file was written to disk
	savedPath := filepath.Join(tempDir, filepath.Clean(key))
	data, err := os.ReadFile(savedPath)
	require.NoError(t, err)
	assert.Equal(t, payload, data)
}

func TestLocalStorageClient_PathTraversalRejection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "proof_tests_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	client, err := NewLocalStorageClient(tempDir, "/api/v1/deliveries/proofs")
	require.NoError(t, err)

	ctx := context.Background()
	payload := []byte("malicious-payload")

	// Attempt path traversal
	_, err = client.UploadProofPhoto(ctx, "../../secret.txt", bytes.NewReader(payload), "text/plain")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid storage key")
}

func TestLocalStorageClient_GeneratePresignedURL(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "proof-presign-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	client, err := NewLocalStorageClient(tempDir, "https://api.swiftdrop.io/api/v1/deliveries/proofs")
	require.NoError(t, err)

	ctx := context.Background()
	url, err := client.GeneratePresignedURL(ctx, "parcels/123/proof.jpg", 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "https://api.swiftdrop.io/api/v1/deliveries/proofs/parcels/123/proof.jpg", url)
}
