package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/parcels/application"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

var _ application.ParcelRepository = (*PostgresParcelRepository)(nil)

// PostgresParcelRepository implements application.ParcelRepository backed by PostgreSQL.
type PostgresParcelRepository struct {
	defaultQuerier db.Querier
}

// NewPostgresParcelRepository constructs a PostgresParcelRepository.
func NewPostgresParcelRepository(defaultQuerier db.Querier) *PostgresParcelRepository {
	return &PostgresParcelRepository{defaultQuerier: defaultQuerier}
}

// Save inserts a new parcel record into PostgreSQL.
func (r *PostgresParcelRepository) Save(ctx context.Context, q db.Querier, parcel domain.Parcel) error {
	querier := q
	if querier == nil {
		querier = r.defaultQuerier
	}
	if querier == nil {
		return errors.New("parcel repo: nil db querier")
	}

	pickupJSON, err := json.Marshal(parcel.PickupAddress)
	if err != nil {
		return fmt.Errorf("marshal pickup address: %w", err)
	}
	dropoffJSON, err := json.Marshal(parcel.DropoffAddress)
	if err != nil {
		return fmt.Errorf("marshal dropoff address: %w", err)
	}

	const query = `
		INSERT INTO parcels (
			id,
			tracking_number,
			sender_id,
			merchant_id,
			recipient_name,
			recipient_phone,
			quote_id,
			pricing_rule_id,
			pricing_rule_version,
			delivery_fee_cents,
			currency,
			origin_zone_id,
			destination_zone_id,
			service_level,
			actual_weight_kg,
			volumetric_weight_kg,
			chargeable_weight_kg,
			length_cm,
			width_cm,
			height_cm,
			pickup_address,
			dropoff_address,
			status,
			delivery_attempts,
			version,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27
		);
	`

	_, err = querier.ExecContext(
		ctx,
		query,
		parcel.ID,
		parcel.TrackingNumber.String(),
		parcel.SenderID,
		parcel.MerchantID,
		parcel.RecipientName,
		parcel.RecipientPhone,
		parcel.QuoteID,
		parcel.PricingRuleID,
		parcel.PricingRuleVersion,
		parcel.DeliveryFee.Amount,
		parcel.DeliveryFee.Currency,
		parcel.OriginZoneID,
		parcel.DestinationZoneID,
		string(parcel.ServiceLevel),
		parcel.Weight.ActualKg,
		parcel.Weight.VolumetricKg,
		parcel.Weight.ChargeableKg,
		parcel.Dimensions.LengthCm,
		parcel.Dimensions.WidthCm,
		parcel.Dimensions.HeightCm,
		pickupJSON,
		dropoffJSON,
		string(parcel.Status),
		parcel.DeliveryAttempts,
		parcel.Version,
		parcel.CreatedAt,
		parcel.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save parcel to postgres: %w", err)
	}

	return nil
}

// ExistsTrackingNumber checks if a given tracking number already exists in parcels.
func (r *PostgresParcelRepository) ExistsTrackingNumber(ctx context.Context, q db.Querier, tn domain.TrackingNumber) (bool, error) {
	querier := q
	if querier == nil {
		querier = r.defaultQuerier
	}
	if querier == nil {
		return false, errors.New("parcel repo: nil db querier")
	}

	const query = `SELECT EXISTS(SELECT 1 FROM parcels WHERE tracking_number = $1);`

	var exists bool
	err := querier.QueryRowContext(ctx, query, tn.String()).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check tracking number exists: %w", err)
	}

	return exists, nil
}

// FindByID retrieves a parcel by its unique UUID identifier.
func (r *PostgresParcelRepository) FindByID(ctx context.Context, q db.Querier, id string) (domain.Parcel, error) {
	querier := q
	if querier == nil {
		querier = r.defaultQuerier
	}
	if querier == nil {
		return domain.Parcel{}, errors.New("parcel repo: nil db querier")
	}

	cleanID := strings.TrimSpace(id)
	if cleanID == "" {
		return domain.Parcel{}, domain.ErrInvalidParcelID
	}

	const query = `
		SELECT
			id,
			tracking_number,
			sender_id,
			merchant_id,
			recipient_name,
			recipient_phone,
			quote_id,
			pricing_rule_id,
			pricing_rule_version,
			delivery_fee_cents,
			currency,
			origin_zone_id,
			destination_zone_id,
			service_level,
			actual_weight_kg,
			volumetric_weight_kg,
			chargeable_weight_kg,
			length_cm,
			width_cm,
			height_cm,
			pickup_address,
			dropoff_address,
			status,
			delivery_attempts,
			version,
			created_at,
			updated_at
		FROM parcels
		WHERE id = $1
		LIMIT 1;
	`

	var (
		parcelID, tnStr, senderID string
		merchantID                sql.NullString
		recipName, recipPhone     string
		quoteID, ruleID           string
		ruleVersion               int
		feeCents                  int64
		currency                  string
		origZone, destZone        string
		svcLevel                  string
		actualKg, volKg, charKg   float64
		lengthCm, widthCm, htCm   float64
		pickupBytes, dropoffBytes []byte
		statusStr                 string
		attempts, version         int
		createdAt, updatedAt      time.Time
	)

	row := querier.QueryRowContext(ctx, query, cleanID)
	err := row.Scan(
		&parcelID,
		&tnStr,
		&senderID,
		&merchantID,
		&recipName,
		&recipPhone,
		&quoteID,
		&ruleID,
		&ruleVersion,
		&feeCents,
		&currency,
		&origZone,
		&destZone,
		&svcLevel,
		&actualKg,
		&volKg,
		&charKg,
		&lengthCm,
		&widthCm,
		&htCm,
		&pickupBytes,
		&dropoffBytes,
		&statusStr,
		&attempts,
		&version,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Parcel{}, domain.ErrInvalidParcelID
		}
		return domain.Parcel{}, fmt.Errorf("find parcel from postgres: %w", err)
	}

	tn, err := domain.NewTrackingNumber(tnStr)
	if err != nil {
		return domain.Parcel{}, fmt.Errorf("corrupt tracking number in db: %w", err)
	}

	var merchIDPtr *string
	if merchantID.Valid {
		merchIDPtr = &merchantID.String
	}

	fee, err := domain.NewMoney(feeCents, currency)
	if err != nil {
		return domain.Parcel{}, fmt.Errorf("corrupt delivery fee in db: %w", err)
	}

	weight, err := domain.NewWeight(actualKg, volKg, charKg)
	if err != nil {
		return domain.Parcel{}, fmt.Errorf("corrupt weight in db: %w", err)
	}

	dims, err := domain.NewDimensions(lengthCm, widthCm, htCm)
	if err != nil {
		return domain.Parcel{}, fmt.Errorf("corrupt dimensions in db: %w", err)
	}

	var pickupAddr, dropoffAddr domain.Address
	if err := json.Unmarshal(pickupBytes, &pickupAddr); err != nil {
		return domain.Parcel{}, fmt.Errorf("corrupt pickup address JSON: %w", err)
	}
	if err := json.Unmarshal(dropoffBytes, &dropoffAddr); err != nil {
		return domain.Parcel{}, fmt.Errorf("corrupt dropoff address JSON: %w", err)
	}

	return domain.ReconstituteParcel(domain.ReconstituteParcelParams{
		ID:                 parcelID,
		TrackingNumber:     tn,
		SenderID:           senderID,
		MerchantID:         merchIDPtr,
		RecipientName:      recipName,
		RecipientPhone:     recipPhone,
		QuoteID:            quoteID,
		PricingRuleID:      ruleID,
		PricingRuleVersion: ruleVersion,
		DeliveryFee:        fee,
		OriginZoneID:       origZone,
		DestinationZoneID:  destZone,
		ServiceLevel:       domain.ServiceLevel(svcLevel),
		Weight:             weight,
		Dimensions:         dims,
		PickupAddress:      pickupAddr,
		DropoffAddress:     dropoffAddr,
		Status:             domain.ParcelStatus(statusStr),
		DeliveryAttempts:   attempts,
		Version:            version,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	})
}

// UpdateStatus updates the parcel's lifecycle status, attempt count, version, and updated_at
// in PostgreSQL using optimistic concurrency control with a version condition.
func (r *PostgresParcelRepository) UpdateStatus(ctx context.Context, q db.Querier, parcel domain.Parcel, expectedVersion int) error {
	querier := q
	if querier == nil {
		querier = r.defaultQuerier
	}
	if querier == nil {
		return errors.New("parcel repo: nil db querier")
	}

	const query = `
		UPDATE parcels
		SET status = $1,
			delivery_attempts = $2,
			version = version + 1,
			updated_at = $3
		WHERE id = $4
		  AND version = $5;
	`

	res, err := querier.ExecContext(
		ctx,
		query,
		string(parcel.Status),
		parcel.DeliveryAttempts,
		parcel.UpdatedAt,
		parcel.ID,
		expectedVersion,
	)
	if err != nil {
		return fmt.Errorf("update parcel status in postgres: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected on parcel update: %w", err)
	}

	if rows == 0 {
		return application.ErrConcurrencyConflict
	}

	return nil
}
