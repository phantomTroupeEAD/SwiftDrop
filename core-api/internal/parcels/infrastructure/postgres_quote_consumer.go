package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/parcels/application"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

var _ application.QuoteConsumer = (*PostgresQuoteConsumer)(nil)

// PostgresQuoteConsumer implements application.QuoteConsumer using atomic PostgreSQL SQL statements.
type PostgresQuoteConsumer struct {
	defaultQuerier db.Querier
}

// NewPostgresQuoteConsumer constructs a PostgresQuoteConsumer.
func NewPostgresQuoteConsumer(defaultQuerier db.Querier) *PostgresQuoteConsumer {
	return &PostgresQuoteConsumer{defaultQuerier: defaultQuerier}
}

// ConsumeQuote atomically marks an unexpired, unused quote as consumed using a conditional UPDATE CTE joined with pricing_rules.
// If q is provided (e.g. an active transaction *sql.Tx), it executes within that transaction.
func (c *PostgresQuoteConsumer) ConsumeQuote(
	ctx context.Context,
	q db.Querier,
	quoteID string,
	acceptedAt time.Time,
) (application.AcceptedQuoteSnapshot, error) {
	querier := q
	if querier == nil {
		querier = c.defaultQuerier
	}
	if querier == nil {
		return application.AcceptedQuoteSnapshot{}, errors.New("quote consumer: nil db querier")
	}

	cleanID := strings.TrimSpace(quoteID)
	if cleanID == "" {
		return application.AcceptedQuoteSnapshot{}, application.ErrQuoteNotFound
	}
	if acceptedAt.IsZero() {
		acceptedAt = time.Now().UTC()
	}

	// 1. Atomic CTE: conditionally update only if used_at IS NULL AND expires_at > acceptedAt
	const atomicConsumeQuery = `
		WITH consumed AS (
			UPDATE pricing_quotes
			SET used_at = $2
			WHERE id = $1
			  AND used_at IS NULL
			  AND expires_at > $2
			RETURNING
				id,
				tariff_id,
				tariff_version,
				chargeable_weight_kg,
				service_level,
				fee_cents,
				currency,
				created_at,
				expires_at,
				used_at
		)
		SELECT
			c.id,
			c.tariff_id,
			c.tariff_version,
			r.origin_zone_id,
			r.destination_zone_id,
			c.chargeable_weight_kg,
			c.service_level,
			c.fee_cents,
			c.currency,
			c.created_at,
			c.expires_at,
			c.used_at
		FROM consumed c
		JOIN pricing_rules r ON r.id = c.tariff_id AND r.version = c.tariff_version
		LIMIT 1;
	`

	var (
		id, tariffID, originZoneID, destinationZoneID, serviceLevel, currency string
		tariffVersion                                                         int
		chargeableWeightKg                                                    float64
		feeCents                                                              int64
		createdAt, expiresAt, usedAt                                          time.Time
	)

	row := querier.QueryRowContext(ctx, atomicConsumeQuery, cleanID, acceptedAt)
	err := row.Scan(
		&id,
		&tariffID,
		&tariffVersion,
		&originZoneID,
		&destinationZoneID,
		&chargeableWeightKg,
		&serviceLevel,
		&feeCents,
		&currency,
		&createdAt,
		&expiresAt,
		&usedAt,
	)
	if err == nil {
		return application.AcceptedQuoteSnapshot{
			QuoteID:            id,
			TariffID:           tariffID,
			TariffVersion:      tariffVersion,
			OriginZoneID:       originZoneID,
			DestinationZoneID:  destinationZoneID,
			ChargeableWeightKg: chargeableWeightKg,
			ServiceLevel:       serviceLevel,
			FeeCents:           feeCents,
			Currency:           currency,
			CreatedAt:          createdAt,
			ExpiresAt:          expiresAt,
			UsedAt:             usedAt,
		}, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return application.AcceptedQuoteSnapshot{}, fmt.Errorf("consume quote query failed: %w", err)
	}

	// 2. Diagnostic inspection to provide exact semantic error distinction
	const diagQuery = `
		SELECT expires_at, used_at
		FROM pricing_quotes
		WHERE id = $1
		LIMIT 1;
	`

	var (
		diagExpiresAt time.Time
		diagUsedAt    sql.NullTime
	)

	diagRow := querier.QueryRowContext(ctx, diagQuery, cleanID)
	if diagErr := diagRow.Scan(&diagExpiresAt, &diagUsedAt); diagErr != nil {
		if errors.Is(diagErr, sql.ErrNoRows) {
			return application.AcceptedQuoteSnapshot{}, application.ErrQuoteNotFound
		}
		return application.AcceptedQuoteSnapshot{}, fmt.Errorf("quote diagnostic query failed: %w", diagErr)
	}

	if diagUsedAt.Valid {
		return application.AcceptedQuoteSnapshot{}, application.ErrQuoteAlreadyUsed
	}

	if !acceptedAt.Before(diagExpiresAt) {
		return application.AcceptedQuoteSnapshot{}, application.ErrQuoteExpired
	}

	return application.AcceptedQuoteSnapshot{}, fmt.Errorf("quote cannot be consumed: underlying tariff rule not found")
}
