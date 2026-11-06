package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/swiftdrop/core-api/internal/platform/db"
	"github.com/swiftdrop/core-api/internal/pricing/application"
	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

// ErrDuplicateQuote is returned when attempting to save a quote whose ID already exists.
var ErrDuplicateQuote = errors.New("pricing: quote already exists")

// Ensure PostgresQuoteRepository implements application.QuoteRepository.
var _ application.QuoteRepository = (*PostgresQuoteRepository)(nil)

// PostgresQuoteRepository implements application.QuoteRepository backed by PostgreSQL.
type PostgresQuoteRepository struct {
	querier db.Querier
}

// NewPostgresQuoteRepository constructs a new PostgresQuoteRepository.
func NewPostgresQuoteRepository(querier db.Querier) *PostgresQuoteRepository {
	return &PostgresQuoteRepository{querier: querier}
}

// Save inserts a new pricing quote record in PostgreSQL.
// It is strictly insert-only; saving an existing quote will return ErrDuplicateQuote
// and cannot overwrite used_at or any other quote attributes.
func (r *PostgresQuoteRepository) Save(ctx context.Context, quote domain.Quote) error {
	if r.querier == nil {
		return errors.New("pricing: nil db querier")
	}

	if err := quote.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid quote: %w", err)
	}

	const query = `
		INSERT INTO pricing_quotes (
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
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);
	`

	_, err := r.querier.ExecContext(
		ctx,
		query,
		quote.ID,
		quote.TariffID,
		quote.TariffVersion,
		quote.ChargeableWeightKg,
		string(quote.ServiceLevel),
		quote.Fee.Amount,
		quote.Fee.Currency,
		quote.CreatedAt,
		quote.ExpiresAt,
		quote.UsedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return fmt.Errorf("%w: %w", ErrDuplicateQuote, err)
		}
		if errors.Is(err, ErrDuplicateQuote) ||
			strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("%w: %w", ErrDuplicateQuote, err)
		}
		return fmt.Errorf("save pricing quote to postgres: %w", err)
	}

	return nil
}

// FindByID locates a pricing quote by its unique UUID identifier.
func (r *PostgresQuoteRepository) FindByID(ctx context.Context, id string) (domain.Quote, error) {
	if r.querier == nil {
		return domain.Quote{}, errors.New("pricing: nil db querier")
	}

	cleanID := strings.TrimSpace(id)
	if cleanID == "" {
		return domain.Quote{}, domain.ErrInvalidQuote
	}

	const query = `
		SELECT
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
		FROM pricing_quotes
		WHERE id = $1
		LIMIT 1;
	`

	var (
		quoteID, tariffID string
		tariffVersion     int
		chargeableWeight  float64
		serviceLevel      string
		feeCents          int64
		currency          string
		createdAt         time.Time
		expiresAt         time.Time
		usedAt            sql.NullTime
	)

	row := r.querier.QueryRowContext(ctx, query, cleanID)
	err := row.Scan(
		&quoteID,
		&tariffID,
		&tariffVersion,
		&chargeableWeight,
		&serviceLevel,
		&feeCents,
		&currency,
		&createdAt,
		&expiresAt,
		&usedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Quote{}, domain.ErrInvalidQuote
		}
		return domain.Quote{}, fmt.Errorf("find pricing quote from postgres: %w", err)
	}

	var usedAtPtr *time.Time
	if usedAt.Valid {
		usedAtPtr = &usedAt.Time
	}

	quote := domain.Quote{
		ID:                 quoteID,
		TariffID:           tariffID,
		TariffVersion:      tariffVersion,
		ChargeableWeightKg: chargeableWeight,
		ServiceLevel:       domain.ServiceLevel(serviceLevel),
		Fee:                domain.Money{Amount: feeCents, Currency: currency},
		CreatedAt:          createdAt,
		ExpiresAt:          expiresAt,
		UsedAt:             usedAtPtr,
	}

	if err := quote.Validate(); err != nil {
		return domain.Quote{}, fmt.Errorf("corrupt quote record in database: %w", err)
	}

	return quote, nil
}
