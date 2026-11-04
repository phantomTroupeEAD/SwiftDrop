package infrastructure

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStorageClient implements application.StorageClient by saving proof images
// to the local file system. Suitable for local development, CI pipelines, and offline testing.
type LocalStorageClient struct {
	baseDir string
	baseURL string
}

// NewLocalStorageClient constructs a LocalStorageClient.
// baseDir is the local filesystem path where photos are written.
// baseURL is the public URL prefix for accessing photos (e.g. "/api/v1/deliveries/proofs").
func NewLocalStorageClient(baseDir, baseURL string) (*LocalStorageClient, error) {
	if baseDir == "" {
		baseDir = "./data/proofs"
	}
	if baseURL == "" {
		baseURL = "/api/v1/deliveries/proofs"
	}

	cleanDir := filepath.Clean(baseDir)
	if err := os.MkdirAll(cleanDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to initialize local storage directory %q: %w", cleanDir, err)
	}

	return &LocalStorageClient{
		baseDir: cleanDir,
		baseURL: strings.TrimRight(baseURL, "/"),
	}, nil
}

// UploadProofPhoto streams binary image data to disk under the configured base directory.
func (c *LocalStorageClient) UploadProofPhoto(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Guard against directory traversal attacks
	cleanKey := filepath.Clean(key)
	if strings.Contains(cleanKey, "..") || filepath.IsAbs(cleanKey) {
		return "", fmt.Errorf("invalid storage key: %q", key)
	}

	destPath := filepath.Join(c.baseDir, cleanKey)
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory structure for %q: %w", destDir, err)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create proof file %q: %w", destPath, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, data); err != nil {
		return "", fmt.Errorf("failed to write proof photo data: %w", err)
	}

	return cleanKey, nil
}

// GeneratePresignedURL returns an HTTP URL endpoint where the proof photo can be viewed.
func (c *LocalStorageClient) GeneratePresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	cleanKey := strings.TrimLeft(filepath.ToSlash(filepath.Clean(key)), "/")
	return fmt.Sprintf("%s/%s", c.baseURL, cleanKey), nil
}

