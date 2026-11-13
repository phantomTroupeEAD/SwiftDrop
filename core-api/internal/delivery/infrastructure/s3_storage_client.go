package infrastructure

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// S3StorageConfig encapsulates configuration for S3-compatible object storage
// (Cloudflare R2, AWS S3, or MinIO).
type S3StorageConfig struct {
	Endpoint      string
	Bucket        string
	Region        string
	AccessKey     string
	SecretKey     string
	PublicBaseURL string
}

// S3StorageClient implements application.StorageClient for Cloudflare R2 / AWS S3
// using standard HTTP and AWS Signature Version 4.
type S3StorageClient struct {
	config     S3StorageConfig
	httpClient *http.Client
}

// NewS3StorageClient constructs an S3StorageClient instance.
func NewS3StorageClient(cfg S3StorageConfig) *S3StorageClient {
	if cfg.Region == "" {
		cfg.Region = "auto"
	}
	return &S3StorageClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// UploadProofPhoto uploads a proof image to S3/R2 object storage.
func (c *S3StorageClient) UploadProofPhoto(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	if contentType == "" {
		contentType = "image/jpeg"
	}

	cleanKey := strings.TrimLeft(key, "/")
	reqURL := c.buildURL(cleanKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, data)
	if err != nil {
		return "", fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	// If credentials are provided, sign the request
	now := time.Now().UTC()
	c.signV4Header(req, now)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload proof to object storage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("object storage upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return cleanKey, nil
}

// GeneratePresignedURL generates a presigned URL or public CDN URL with the specified TTL.
func (c *S3StorageClient) GeneratePresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	cleanKey := strings.TrimLeft(key, "/")

	// If a custom public CDN or custom domain URL is configured, use it directly
	if c.config.PublicBaseURL != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(c.config.PublicBaseURL, "/"), cleanKey), nil
	}

	// Generate AWS SigV4 presigned query URL
	now := time.Now().UTC()
	expiresSeconds := int(ttl.Seconds())
	if expiresSeconds <= 0 {
		expiresSeconds = 900 // 15 minutes default
	}

	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, c.config.Region)

	rawURL := c.buildURL(cleanKey)
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid storage URL: %w", err)
	}

	q := parsedURL.Query()
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", fmt.Sprintf("%s/%s", c.config.AccessKey, credentialScope))
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", fmt.Sprintf("%d", expiresSeconds))
	q.Set("X-Amz-SignedHeaders", "host")

	canonicalQuery := q.Encode()

	// Canonical Request for presigned URL
	canonicalURI := parsedURL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalHeaders := fmt.Sprintf("host:%s\n", parsedURL.Host)
	signedHeaders := "host"
	payloadHash := "UNSIGNED-PAYLOAD"

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		http.MethodGet,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		hex.EncodeToString(sha256Sum([]byte(canonicalRequest))),
	)

	signingKey := getSignatureKey(c.config.SecretKey, dateStamp, c.config.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	q.Set("X-Amz-Signature", signature)
	parsedURL.RawQuery = q.Encode()

	return parsedURL.String(), nil
}

func (c *S3StorageClient) buildURL(key string) string {
	endpoint := strings.TrimRight(c.config.Endpoint, "/")
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", c.config.Bucket, c.config.Region)
		return fmt.Sprintf("%s/%s", endpoint, key)
	}

	// For path-style endpoints (e.g. MinIO or standard endpoint)
	return fmt.Sprintf("%s/%s/%s", endpoint, c.config.Bucket, key)
}

func (c *S3StorageClient) signV4Header(req *http.Request, t time.Time) {
	if c.config.AccessKey == "" || c.config.SecretKey == "" {
		return
	}

	amzDate := t.Format("20060102T150405Z")
	dateStamp := t.Format("20060102")

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", "UNSIGNED-PAYLOAD")

	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:UNSIGNED-PAYLOAD\nx-amz-date:%s\n",
		req.URL.Host, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\nUNSIGNED-PAYLOAD",
		req.Method,
		canonicalURI,
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
	)

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, c.config.Region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		hex.EncodeToString(sha256Sum([]byte(canonicalRequest))),
	)

	signingKey := getSignatureKey(c.config.SecretKey, dateStamp, c.config.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.config.AccessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

func sha256Sum(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func getSignatureKey(key, dateStamp, regionName, serviceName string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+key), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(regionName))
	kService := hmacSHA256(kRegion, []byte(serviceName))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}
