package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/platform/db"
	"github.com/swiftdrop/core-api/internal/pricing/application"
	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

// Ensure PostgresTariffRepository implements application.TariffRepository.
var _ application.TariffRepository = (*PostgresTariffRepository)(nil)

// PostgresTariffRepository implements application.TariffRepository backed by PostgreSQL.
type PostgresTariffRepository struct {
	querier db.Querier
}

// NewPostgresTariffRepository constructs a new PostgresTariffRepository.
func NewPostgresTariffRepository(querier db.Querier) *PostgresTariffRepository {
	return &PostgresTariffRepository{querier: querier}
}

// FindApplicable retrieves the active geographic pricing tariff for the given origin
// and destination zones applicable at the given evaluation timestamp.
func (r *PostgresTariffRepository) FindApplicable(
	ctx context.Context,
	originZoneID string,
	destinationZoneID string,
	at time.Time,
) (domain.Tariff, error) {
	if r.querier == nil {
		return domain.Tariff{}, errors.New("pricing: nil db querier")
	}

	cleanOrigin := strings.TrimSpace(originZoneID)
	cleanDest := strings.TrimSpace(destinationZoneID)
	if cleanOrigin == "" || cleanDest == "" {
		return domain.Tariff{}, domain.ErrInvalidTariff
	}

	if at.IsZero() {
		return domain.Tariff{}, fmt.Errorf("%w: evaluation timestamp cannot be zero", domain.ErrInvalidTariff)
	}

	const query = `
		SELECT
			id,
			origin_zone_id,
			destination_zone_id,
			base_fee_cents,
			per_kg_rate_cents,
			effective_from,
			effective_until,
			version,
			is_active,
			created_at,
			updated_at
		FROM pricing_rules
		WHERE origin_zone_id = $1
		  AND destination_zone_id = $2
		  AND is_active = TRUE
		  AND effective_from <= $3
		  AND (effective_until IS NULL OR effective_until > $3)
		ORDER BY version DESC, effective_from DESC
		LIMIT 1;
	`

	var (
		id                 string
		origZone, destZone string
		baseFeeCents       int64
		perKgRateCents     int64
		effectiveFrom      time.Time
		effectiveUntil     sql.NullTime
		version            int
		isActive           bool
		createdAt          time.Time
		updatedAt          sql.NullTime
	)

	row := r.querier.QueryRowContext(ctx, query, cleanOrigin, cleanDest, at)
	err := row.Scan(
		&id,
		&origZone,
		&destZone,
		&baseFeeCents,
		&perKgRateCents,
		&effectiveFrom,
		&effectiveUntil,
		&version,
		&isActive,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Tariff{}, domain.ErrTariffNotFound
		}
		return domain.Tariff{}, fmt.Errorf("find applicable tariff from postgres: %w", err)
	}

	var effectiveUntilPtr *time.Time
	if effectiveUntil.Valid {
		effectiveUntilPtr = &effectiveUntil.Time
	}

	var updatedTime time.Time
	if updatedAt.Valid {
		updatedTime = updatedAt.Time
	}

	tariff := domain.Tariff{
		ID:                id,
		OriginZoneID:      origZone,
		DestinationZoneID: destZone,
		BaseFee:           domain.Money{Amount: baseFeeCents, Currency: domain.CurrencyLKR},
		PerKgRate:         domain.Money{Amount: perKgRateCents, Currency: domain.CurrencyLKR},
		EffectiveFrom:     effectiveFrom,
		EffectiveUntil:    effectiveUntilPtr,
		Version:           version,
		IsActive:          isActive,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedTime,
	}

	if err := tariff.Validate(); err != nil {
		return domain.Tariff{}, fmt.Errorf("corrupt tariff record in database: %w", err)
	}

	return tariff, nil
}
