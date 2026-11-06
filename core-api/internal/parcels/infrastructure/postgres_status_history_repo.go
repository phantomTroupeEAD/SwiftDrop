package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/swiftdrop/core-api/internal/parcels/application"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

var _ application.StatusHistoryRepository = (*PostgresStatusHistoryRepository)(nil)

// PostgresStatusHistoryRepository implements application.StatusHistoryRepository backed by PostgreSQL.
type PostgresStatusHistoryRepository struct {
	defaultQuerier db.Querier
}

// NewPostgresStatusHistoryRepository constructs a PostgresStatusHistoryRepository.
func NewPostgresStatusHistoryRepository(defaultQuerier db.Querier) *PostgresStatusHistoryRepository {
	return &PostgresStatusHistoryRepository{defaultQuerier: defaultQuerier}
}

// Save inserts an immutable status-history audit row into PostgreSQL.
func (r *PostgresStatusHistoryRepository) Save(ctx context.Context, q db.Querier, entry domain.StatusHistoryEntry) error {
	querier := q
	if querier == nil {
		querier = r.defaultQuerier
	}
	if querier == nil {
		return errors.New("status history repo: nil db querier")
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
			actor_role,
			reason,
			timestamp
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
	`

	_, err := querier.ExecContext(
		ctx,
		query,
		entry.ID,
		entry.ParcelID,
		fromStatus,
		string(entry.ToStatus),
		entry.ActorID,
		entry.ActorRole,
		entry.Reason,
		entry.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("save parcel status history to postgres: %w", err)
	}

	return nil
}

