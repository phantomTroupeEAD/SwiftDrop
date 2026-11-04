package application

import (
	"context"
	"database/sql"
	"io"
	"time"

	"github.com/google/uuid"
)

// TransactionManager abstracts database transactions for application use cases.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// ParcelLifecycleUpdater is implemented by Member 2 (internal/parcels)
// to enforce status transitions on the parcel aggregate root.
type ParcelLifecycleUpdater interface {
	TransitionStatus(
		ctx context.Context,
		tx *sql.Tx,
		parcelID uuid.UUID,
		fromStatus, toStatus string,
		actorID uuid.UUID,
		actorRole, note string,
	) error

	GetStatusWithLock(
		ctx context.Context,
		tx *sql.Tx,
		parcelID uuid.UUID,
	) (currentStatus string, attemptCount int, err error)
}

// OutboxWriter is implemented by Member 8 (internal/events)
// to commit versioned integration events to PostgreSQL within the caller's transaction.
type OutboxWriter interface {
	Write(
		ctx context.Context,
		tx *sql.Tx,
		aggregateType string,
		aggregateID uuid.UUID,
		aggregateVersion int,
		eventType string,
		payload any,
		correlationID string,
	) error
}

// StorageClient abstracts object storage operations (Cloudflare R2 / AWS S3 / Local mock).
type StorageClient interface {
	UploadProofPhoto(ctx context.Context, key string, data io.Reader, contentType string) (string, error)
	GeneratePresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

