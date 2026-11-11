package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/swiftdrop/core-api/internal/parcels/application"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

var _ application.IdempotencyStore = (*PostgresIdempotencyRepository)(nil)

// PostgresIdempotencyRepository implements application.IdempotencyStore backed by PostgreSQL.
type PostgresIdempotencyRepository struct {
	defaultQuerier db.Querier
}

// NewPostgresIdempotencyRepository constructs a PostgresIdempotencyRepository.
func NewPostgresIdempotencyRepository(defaultQuerier db.Querier) *PostgresIdempotencyRepository {
	return &PostgresIdempotencyRepository{defaultQuerier: defaultQuerier}
}

// AcquireLock obtains a transaction-scoped advisory lock for (caller_scope, key)
// so concurrent duplicate requests serialize before quote consumption or parcel creation.
func (r *PostgresIdempotencyRepository) AcquireLock(ctx context.Context, q db.Querier, key, callerScope string) error {
	querier, err := resolveQuerier(q, r.defaultQuerier, "idempotency repo: nil db querier")
	if err != nil {
		return err
	}

	cleanKey := strings.TrimSpace(key)
	if cleanKey == "" {
		return application.ErrInvalidIdempotencyKey
	}
	cleanScope := strings.TrimSpace(callerScope)
	if cleanScope == "" {
		return application.ErrInvalidCallerScope
	}

	const lockQuery = `SELECT pg_advisory_xact_lock(hashtext($1::text), hashtext($2::text));`
	if _, err := querier.ExecContext(ctx, lockQuery, cleanScope, cleanKey); err != nil {
		return fmt.Errorf("acquire idempotency advisory lock: %w", err)
	}

	return nil
}

// Find retrieves an idempotency record by its composite primary key (key, caller_scope)
// with a row lock when executed inside an active transaction.
func (r *PostgresIdempotencyRepository) Find(ctx context.Context, q db.Querier, key, callerScope string) (application.IdempotencyRecord, bool, error) {
	querier, err := resolveQuerier(q, r.defaultQuerier, "idempotency repo: nil db querier")
	if err != nil {
		return application.IdempotencyRecord{}, false, err
	}

	cleanKey := strings.TrimSpace(key)
	if cleanKey == "" {
		return application.IdempotencyRecord{}, false, application.ErrInvalidIdempotencyKey
	}
	cleanScope := strings.TrimSpace(callerScope)
	if cleanScope == "" {
		return application.IdempotencyRecord{}, false, application.ErrInvalidCallerScope
	}

	const query = `
		SELECT
			key,
			caller_scope,
			operation,
			request_hash,
			response_status,
			response_body,
			created_resource_id,
			created_at,
			expires_at
		FROM idempotency_keys
		WHERE key = $1
		  AND caller_scope = $2
		FOR UPDATE;
	`

	var (
		rec               application.IdempotencyRecord
		createdResourceID sql.NullString
		createdAt         time.Time
		expiresAt         time.Time
	)

	err = querier.QueryRowContext(ctx, query, cleanKey, cleanScope).Scan(
		&rec.Key,
		&rec.CallerScope,
		&rec.Operation,
		&rec.RequestHash,
		&rec.ResponseStatus,
		&rec.ResponseBody,
		&createdResourceID,
		&createdAt,
		&expiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return application.IdempotencyRecord{}, false, nil
		}
		return application.IdempotencyRecord{}, false, fmt.Errorf("find idempotency key from postgres: %w", err)
	}

	rec.CreatedResourceID = fromNullString(createdResourceID)
	rec.CreatedAt = createdAt.UTC()
	rec.ExpiresAt = expiresAt.UTC()
	return rec, true, nil
}

// Save inserts a new idempotency record or atomically replaces an expired record
// (where existing expires_at <= record.CreatedAt). Unexpired active keys are never overwritten.
func (r *PostgresIdempotencyRepository) Save(ctx context.Context, q db.Querier, record application.IdempotencyRecord) error {
	querier, err := resolveQuerier(q, r.defaultQuerier, "idempotency repo: nil db querier")
	if err != nil {
		return err
	}

	if err := record.Validate(); err != nil {
		return err
	}

	norm, err := (application.IdempotencyInput{
		Key:         record.Key,
		CallerScope: record.CallerScope,
		Operation:   record.Operation,
		RequestHash: record.RequestHash,
	}).Normalize()
	if err != nil {
		return err
	}

	const query = `
		INSERT INTO idempotency_keys (
			key,
			caller_scope,
			operation,
			request_hash,
			response_status,
			response_body,
			created_resource_id,
			created_at,
			expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (key, caller_scope) DO UPDATE
		SET operation = EXCLUDED.operation,
			request_hash = EXCLUDED.request_hash,
			response_status = EXCLUDED.response_status,
			response_body = EXCLUDED.response_body,
			created_resource_id = EXCLUDED.created_resource_id,
			created_at = EXCLUDED.created_at,
			expires_at = EXCLUDED.expires_at
		WHERE idempotency_keys.expires_at <= $8;
	`

	res, err := querier.ExecContext(
		ctx,
		query,
		norm.Key,
		norm.CallerScope,
		norm.Operation,
		norm.RequestHash,
		record.ResponseStatus,
		record.ResponseBody,
		toNullString(record.CreatedResourceID),
		record.CreatedAt.UTC(),
		record.ExpiresAt.UTC(),
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return fmt.Errorf("%w: %w", application.ErrIdempotencyConflict, err)
		}
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("%w: %w", application.ErrIdempotencyConflict, err)
		}
		return fmt.Errorf("save idempotency key to postgres: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected on idempotency save: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: active unexpired record already exists", application.ErrIdempotencyConflict)
	}

	return nil
}
