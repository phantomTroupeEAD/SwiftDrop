package infrastructure

import (
	"fmt"
	"os"
	"strings"

	"github.com/swiftdrop/core-api/internal/delivery/application"
)

// NewStorageClientFromEnv initializes an application.StorageClient based on environment variables.
//
// Driver selection:
//   - OBJECT_STORAGE_DRIVER or STORAGE_DRIVER ("s3", "r2", "minio" -> S3StorageClient)
//   - Default is LocalStorageClient ("local" or unset).
func NewStorageClientFromEnv() (application.StorageClient, error) {
	driver := os.Getenv("OBJECT_STORAGE_DRIVER")
	if driver == "" {
		driver = os.Getenv("STORAGE_DRIVER")
	}
	driver = strings.ToLower(strings.TrimSpace(driver))

	switch driver {
	case "s3", "r2", "minio":
		endpoint := os.Getenv("S3_ENDPOINT")
		if endpoint == "" {
			endpoint = os.Getenv("OBJECT_STORAGE_ENDPOINT")
		}
		// If Cloudflare R2 driver is selected and Account ID is provided without explicit endpoint:
		if endpoint == "" {
			if accountID := os.Getenv("OBJECT_STORAGE_ACCOUNT_ID"); accountID != "" {
				endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
			} else if accountID := os.Getenv("R2_ACCOUNT_ID"); accountID != "" {
				endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
			}
		}

		bucket := os.Getenv("S3_BUCKET")
		if bucket == "" {
			bucket = os.Getenv("OBJECT_STORAGE_BUCKET")
		}
		if bucket == "" {
			bucket = "swiftdrop-proofs"
		}

		region := os.Getenv("S3_REGION")
		if region == "" {
			region = os.Getenv("OBJECT_STORAGE_REGION")
		}
		if region == "" {
			region = "auto"
		}

		accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
		if accessKey == "" {
			accessKey = os.Getenv("S3_ACCESS_KEY_ID")
		}
		if accessKey == "" {
			accessKey = os.Getenv("OBJECT_STORAGE_ACCESS_KEY")
		}
		if accessKey == "" {
			accessKey = os.Getenv("R2_ACCESS_KEY_ID")
		}

		secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
		if secretKey == "" {
			secretKey = os.Getenv("S3_SECRET_ACCESS_KEY")
		}
		if secretKey == "" {
			secretKey = os.Getenv("OBJECT_STORAGE_SECRET_KEY")
		}
		if secretKey == "" {
			secretKey = os.Getenv("R2_SECRET_ACCESS_KEY")
		}

		publicBaseURL := os.Getenv("S3_PUBLIC_BASE_URL")
		if publicBaseURL == "" {
			publicBaseURL = os.Getenv("OBJECT_STORAGE_PUBLIC_URL")
		}
		if publicBaseURL == "" {
			publicBaseURL = os.Getenv("OBJECT_STORAGE_PUBLIC_BASE_URL")
		}

		cfg := S3StorageConfig{
			Endpoint:      endpoint,
			Bucket:        bucket,
			Region:        region,
			AccessKey:     accessKey,
			SecretKey:     secretKey,
			PublicBaseURL: publicBaseURL,
		}
		return NewS3StorageClient(cfg), nil

	case "local", "disk", "":
		baseDir := os.Getenv("LOCAL_STORAGE_PATH")
		if baseDir == "" {
			baseDir = "./data/proofs"
		}
		baseURL := os.Getenv("LOCAL_STORAGE_BASE_URL")
		if baseURL == "" {
			baseURL = "/api/v1/deliveries/proofs"
		}
		return NewLocalStorageClient(baseDir, baseURL)

	default:
		return nil, fmt.Errorf("unsupported storage driver: %q (supported: 'local', 's3', 'r2', 'minio')", driver)
	}
}
