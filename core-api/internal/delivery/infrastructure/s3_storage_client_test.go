package infrastructure

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3StorageClient_GeneratePresignedURL_PublicBaseURL(t *testing.T) {
	cfg := S3StorageConfig{
		PublicBaseURL: "https://cdn.swiftdrop.io/proofs",
		Bucket:        "swiftdrop-proofs",
	}
	client := NewS3StorageClient(cfg)

	ctx := context.Background()
	url, err := client.GeneratePresignedURL(ctx, "2026/11/02/proof.jpg", 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.swiftdrop.io/proofs/2026/11/02/proof.jpg", url)
}

func TestS3StorageClient_GeneratePresignedURL_SigV4(t *testing.T) {
	cfg := S3StorageConfig{
		Endpoint:  "https://s3.ap-southeast-1.amazonaws.com",
		Bucket:    "swiftdrop-proofs",
		Region:    "ap-southeast-1",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}
	client := NewS3StorageClient(cfg)

	ctx := context.Background()
	url, err := client.GeneratePresignedURL(ctx, "proofs/sample.png", 15*time.Minute)
	require.NoError(t, err)

	// Verify SigV4 query parameters
	assert.True(t, strings.HasPrefix(url, "https://s3.ap-southeast-1.amazonaws.com/swiftdrop-proofs/proofs/sample.png?"))
	assert.Contains(t, url, "X-Amz-Algorithm=AWS4-HMAC-SHA256")
	assert.Contains(t, url, "X-Amz-Credential=AKIAIOSFODNN7EXAMPLE")
	assert.Contains(t, url, "X-Amz-Expires=900")
	assert.Contains(t, url, "X-Amz-SignedHeaders=host")
	assert.Contains(t, url, "X-Amz-Signature=")
}
