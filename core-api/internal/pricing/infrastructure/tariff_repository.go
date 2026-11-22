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

// List retrieves a paginated list of pricing rules matching the optional filters.
func (r *PostgresTariffRepository) List(
	ctx context.Context,
	filter application.ListTariffsFilter,
	offset int,
	limit int,
) ([]domain.Tariff, int, error) {
	if r.querier == nil {
		return nil, 0, errors.New("pricing: nil db querier")
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	var conditions []string
	var args []any
	argIdx := 1

	cleanOrigin := strings.TrimSpace(filter.OriginZoneID)
	if cleanOrigin != "" {
		conditions = append(conditions, fmt.Sprintf("origin_zone_id = $%d", argIdx))
		args = append(args, cleanOrigin)
		argIdx++
	}

	cleanDest := strings.TrimSpace(filter.DestinationZoneID)
	if cleanDest != "" {
		conditions = append(conditions, fmt.Sprintf("destination_zone_id = $%d", argIdx))
		args = append(args, cleanDest)
		argIdx++
	}

	if filter.ActiveOnly != nil {
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", argIdx))
		args = append(args, *filter.ActiveOnly)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := "SELECT COUNT(*) FROM pricing_rules" + whereClause
	var total int
	err := r.querier.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count pricing rules from postgres: %w", err)
	}

	if total == 0 {
		return []domain.Tariff{}, 0, nil
	}

	selectQuery := fmt.Sprintf(`
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
		FROM pricing_rules%s
		ORDER BY origin_zone_id ASC, destination_zone_id ASC, version DESC, id ASC
		LIMIT $%d OFFSET $%d;
	`, whereClause, argIdx, argIdx+1)

	queryArgs := append(args, limit, offset)
	rows, err := r.querier.QueryContext(ctx, selectQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list pricing rules from postgres: %w", err)
	}
	defer rows.Close()

	tariffs := make([]domain.Tariff, 0)
	for rows.Next() {
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

		if err := rows.Scan(
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
		); err != nil {
			return nil, 0, fmt.Errorf("scan pricing rule from postgres: %w", err)
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

		tariffs = append(tariffs, tariff)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate pricing rules rows: %w", err)
	}

	return tariffs, total, nil
}
