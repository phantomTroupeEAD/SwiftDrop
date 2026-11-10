package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/parcels/application"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

var _ application.StatusHistoryRepository = (*PostgresStatusHistoryRepository)(nil)

// PostgresStatusHistoryRepository implements append-only PostgreSQL persistence for parcel status history.
type PostgresStatusHistoryRepository struct {
	defaultQuerier db.Querier
}

// NewPostgresStatusHistoryRepository constructs a PostgresStatusHistoryRepository.
func NewPostgresStatusHistoryRepository(defaultQuerier db.Querier) *PostgresStatusHistoryRepository {
	return &PostgresStatusHistoryRepository{defaultQuerier: defaultQuerier}
}

// Save inserts an immutable status-history audit row into PostgreSQL.
// History records are strictly append-only; no update or delete operations are supported.
func (r *PostgresStatusHistoryRepository) Save(ctx context.Context, q db.Querier, entry domain.StatusHistoryEntry) error {
	querier, err := resolveQuerier(q, r.defaultQuerier, "status history repo: nil db querier")
	if err != nil {
		return err
	}

	if err := entry.Validate(0); err != nil {
		return err
	}

	var fromStatus sql.NullString
	if entry.FromStatus != nil {
		fromStatus = sql.NullString{String: string(*entry.FromStatus), Valid: true}
	}

	const query = `
		INSERT INTO parcel_status_history (
			id,
			parcel_id,
			from_status,
			to_status,
			actor_id,
			actor_type,
			actor_role,
			reason,
			occurred_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
	`

	_, err = querier.ExecContext(
		ctx,
		query,
		entry.ID,
		entry.ParcelID,
		fromStatus,
		string(entry.ToStatus),
		toNullString(entry.ActorID),
		string(entry.ActorType),
		entry.ActorRole,
		entry.Reason,
		entry.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("save parcel status history to postgres: %w", err)
	}

	return nil
}

// FindByParcelID retrieves chronological immutable status-history entries for a parcel.
func (r *PostgresStatusHistoryRepository) FindByParcelID(ctx context.Context, q db.Querier, parcelID string) ([]domain.StatusHistoryEntry, error) {
	querier, err := resolveQuerier(q, r.defaultQuerier, "status history repo: nil db querier")
	if err != nil {
		return nil, err
	}

	cleanParcelID := strings.TrimSpace(parcelID)
	if cleanParcelID == "" {
		return nil, domain.ErrInvalidParcelID
	}

	const query = `
		SELECT
			id,
			parcel_id,
			from_status,
			to_status,
			actor_id,
			actor_type,
			actor_role,
			reason,
			occurred_at
		FROM parcel_status_history
		WHERE parcel_id = $1
		ORDER BY occurred_at ASC, id ASC;
	`

	rows, err := querier.QueryContext(ctx, query, cleanParcelID)
	if err != nil {
		return nil, fmt.Errorf("find parcel status history from postgres: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.StatusHistoryEntry, 0)
	attemptCount := 0

	for rows.Next() {
		var (
			id, pID, toStatusStr, actorTypeStr     string
			fromStatusNS, actorIDNS, roleNS, rsnNS sql.NullString
			occurredAt                             time.Time
		)

		if err := rows.Scan(
			&id,
			&pID,
			&fromStatusNS,
			&toStatusStr,
			&actorIDNS,
			&actorTypeStr,
			&roleNS,
			&rsnNS,
			&occurredAt,
		); err != nil {
			return nil, fmt.Errorf("scan parcel status history row: %w", err)
		}

		var fromStatusPtr *domain.ParcelStatus
		if fromStatusNS.Valid {
			fs := domain.ParcelStatus(fromStatusNS.String)
			fromStatusPtr = &fs
		}

		toStatus := domain.ParcelStatus(toStatusStr)
		entry, err := domain.NewStatusHistoryEntry(
			id,
			pID,
			fromStatusPtr,
			toStatus,
			fromNullString(actorIDNS),
			domain.ActorType(actorTypeStr),
			roleNS.String,
			rsnNS.String,
			occurredAt,
			attemptCount,
		)
		if err != nil {
			return nil, fmt.Errorf("corrupt status history row in db: %w", err)
		}

		if toStatus == domain.StatusAttemptFailed {
			attemptCount++
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate parcel status history rows: %w", err)
	}

	return entries, nil
}
