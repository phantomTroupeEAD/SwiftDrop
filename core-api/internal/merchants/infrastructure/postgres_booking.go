package infrastructure

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/merchants/application"
)

var (
	zoneIDPattern = regexp.MustCompile(`^ZONE_[A-Z0-9_]{2,27}$`)
)

const (
	crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	defaultZoneOrigin = "ZONE_WESTERN_01"
	defaultZoneDest   = "ZONE_CENTRAL_01"
)

// PostgresParcelBooking implements application.ParcelBooking by persisting
// merchant B2B orders directly to the authoritative PostgreSQL database (parcels,
// parcel_status_history, outbox_events, and idempotency_keys).
type PostgresParcelBooking struct {
	db *sql.DB
}

// NewPostgresParcelBooking constructs a new PostgresParcelBooking adapter.
func NewPostgresParcelBooking(db *sql.DB) *PostgresParcelBooking {
	return &PostgresParcelBooking{db: db}
}

// parcelBookedPayload mirrors the authoritative parcel.booked.v1 outbox event schema.
type parcelBookedPayload struct {
	ParcelID           string    `json:"parcel_id"`
	TrackingNumber     string    `json:"tracking_number"`
	SenderID           *string   `json:"sender_id,omitempty"`
	MerchantID         *string   `json:"merchant_id,omitempty"`
	RecipientName      string    `json:"recipient_name"`
	RecipientPhone     string    `json:"recipient_phone"`
	QuoteID            string    `json:"quote_id"`
	PricingRuleID      string    `json:"pricing_rule_id"`
	PricingRuleVersion int       `json:"pricing_rule_version"`
	DeliveryFeeCents   int64     `json:"delivery_fee_cents"`
	Currency           string    `json:"currency"`
	OriginZoneID       string    `json:"origin_zone_id"`
	DestinationZoneID  string    `json:"destination_zone_id"`
	ServiceLevel       string    `json:"service_level"`
	ActualWeightKg     float64   `json:"actual_weight_kg"`
	ChargeableWeightKg float64   `json:"chargeable_weight_kg"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
}

// CreateForMerchant persists a new B2B merchant parcel order atomically in PostgreSQL.
func (b *PostgresParcelBooking) CreateForMerchant(
	ctx context.Context,
	merchantID, idemKey, requestHash string,
	req application.OrderRequest,
) (application.OrderResult, bool, error) {
	// 1. Validate basic input fields
	if req.Pickup == nil || req.Dropoff == nil || req.Parcel.WeightKG <= 0 {
		return application.OrderResult{}, false, application.ValidationError{
			Field: "parcel",
			Issue: "pickup, dropoff, and parcel.weight_kg > 0 are required",
		}
	}
	if req.Parcel.WeightKG > 30.0 {
		return application.OrderResult{}, false, application.ValidationError{
			Field: "parcel.weight_kg",
			Issue: "maximum package weight is 30.0 kg",
		}
	}

	if b.db == nil {
		return application.OrderResult{}, false, errors.New("postgres booking: nil db handle")
	}

	now := time.Now().UTC()
	callerScope := "MERCHANT:" + strings.TrimSpace(merchantID)

	// 2. Check Idempotency Replay before starting write transaction
	if idemKey != "" {
		const checkIdemSQL = `
			SELECT request_hash, response_body, expires_at
			FROM idempotency_keys
			WHERE key = $1 AND caller_scope = $2;
		`
		var (
			storedHash string
			storedBody []byte
			expiresAt  time.Time
		)
		err := b.db.QueryRowContext(ctx, checkIdemSQL, idemKey, callerScope).Scan(&storedHash, &storedBody, &expiresAt)
		if err == nil {
			if expiresAt.After(now) {
				if storedHash != requestHash {
					return application.OrderResult{}, false, application.ErrIdempotencyReused
				}
				var replayed application.OrderResult
				if unmarshalErr := json.Unmarshal(storedBody, &replayed); unmarshalErr == nil {
					return replayed, true, nil
				}
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return application.OrderResult{}, false, fmt.Errorf("idempotency check: %w", err)
		}
	}

	// 3. Normalize addresses and contact info
	pickupAddr := parseAddress(req.Pickup, defaultZoneOrigin)
	dropoffAddr := parseAddress(req.Dropoff, defaultZoneDest)
	if req.MerchantReference != "" {
		dropoffAddr["merchant_reference"] = req.MerchantReference
	}

	recipientName := getString(req.Dropoff, "contact_name", "name")
	if recipientName == "" {
		recipientName = "Valued Customer"
	}
	recipientPhone := getString(req.Dropoff, "contact_phone", "phone")
	if recipientPhone == "" {
		recipientPhone = "+94770000000"
	}

	serviceLevel := strings.ToUpper(strings.TrimSpace(req.ServiceLevel))
	if serviceLevel != "EXPRESS" && serviceLevel != "SAME_DAY" {
		serviceLevel = "STANDARD"
	}

	weightKg := req.Parcel.WeightKG
	// Volumetric weight: (L * W * H) / 5000 cm3/kg; default 10x10x10 package
	lengthCm, widthCm, heightCm := 10.0, 10.0, 10.0
	volumetricKg := (lengthCm * widthCm * heightCm) / 5000.0
	chargeableKg := math.Max(weightKg, volumetricKg)

	originZone := pickupAddr["zone_id"].(string)
	destinationZone := dropoffAddr["zone_id"].(string)

	pickupJSON, err := json.Marshal(pickupAddr)
	if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("marshal pickup address: %w", err)
	}
	dropoffJSON, err := json.Marshal(dropoffAddr)
	if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("marshal dropoff address: %w", err)
	}

	// 4. Atomic PostgreSQL Transaction
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// a. Resolve or ensure applicable commercial tariff in pricing_rules
	var (
		ruleID         string
		ruleVersion    int
		baseFeeCents   int64
		perKgRateCents int64
	)

	// Try route-specific pricing rule
	const findRouteRuleSQL = `
		SELECT id, version, base_fee_cents, per_kg_rate_cents
		FROM pricing_rules
		WHERE origin_zone_id = $1 AND destination_zone_id = $2 AND is_active = TRUE
		ORDER BY version DESC, effective_from DESC
		LIMIT 1;
	`
	err = tx.QueryRowContext(ctx, findRouteRuleSQL, originZone, destinationZone).Scan(
		&ruleID, &ruleVersion, &baseFeeCents, &perKgRateCents,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// Fallback: any active pricing rule
		const findAnyRuleSQL = `
			SELECT id, version, base_fee_cents, per_kg_rate_cents
			FROM pricing_rules
			WHERE is_active = TRUE
			ORDER BY version DESC, effective_from DESC
			LIMIT 1;
		`
		err = tx.QueryRowContext(ctx, findAnyRuleSQL).Scan(
			&ruleID, &ruleVersion, &baseFeeCents, &perKgRateCents,
		)
	}

	if errors.Is(err, sql.ErrNoRows) {
		// Fresh database fallback: seed default commercial tariff
		ruleID = "tariff-commercial-standard-v1"
		ruleVersion = 1
		baseFeeCents = 35000
		perKgRateCents = 10000

		const seedRuleSQL = `
			INSERT INTO pricing_rules (
				id, version, origin_zone_id, destination_zone_id,
				base_fee_cents, per_kg_rate_cents, effective_from, is_active
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, TRUE
			) ON CONFLICT (id, version) DO NOTHING;
		`
		_, err = tx.ExecContext(ctx, seedRuleSQL,
			ruleID, ruleVersion, originZone, destinationZone,
			baseFeeCents, perKgRateCents, now.Add(-1*time.Hour),
		)
		if err != nil {
			return application.OrderResult{}, false, fmt.Errorf("seed commercial tariff: %w", err)
		}
	} else if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("resolve pricing rule: %w", err)
	}

	// Calculate delivery fee
	deliveryFeeCents := baseFeeCents + int64(chargeableKg*float64(perKgRateCents))
	if deliveryFeeCents <= 0 {
		deliveryFeeCents = 65000 // default minimum 650 LKR
	}

	// b. Generate Crockford Base32 tracking number with collision retry
	trackingNumber, err := b.generateTrackingNumber(ctx, tx, now)
	if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("generate tracking number: %w", err)
	}

	// c. Generate UUIDv7 identifiers
	parcelID, err := events.GenerateUUIDv7()
	if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("generate parcel id: %w", err)
	}
	quoteID, err := events.GenerateUUIDv7()
	if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("generate quote id: %w", err)
	}
	historyID, err := events.GenerateUUIDv7()
	if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("generate history id: %w", err)
	}
	eventID, err := events.GenerateUUIDv7()
	if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("generate event id: %w", err)
	}

	// d. Insert consumed commercial quote
	const insertQuoteSQL = `
		INSERT INTO pricing_quotes (
			id, tariff_id, tariff_version, requester_id, merchant_id,
			chargeable_weight_kg, service_level, fee_cents, currency,
			created_at, expires_at, used_at
		) VALUES (
			$1, $2, $3, NULL, $4,
			$5, $6, $7, 'LKR',
			$8, $9, $8
		);
	`
	_, err = tx.ExecContext(ctx, insertQuoteSQL,
		quoteID, ruleID, ruleVersion, merchantID,
		chargeableKg, serviceLevel, deliveryFeeCents,
		now, now.Add(24*time.Hour),
	)
	if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("insert pricing quote: %w", err)
	}

	// e. Insert parcel into parcels table
	const insertParcelSQL = `
		INSERT INTO parcels (
			id, tracking_number, sender_id, merchant_id,
			recipient_name, recipient_phone, quote_id,
			pricing_rule_id, pricing_rule_version, delivery_fee_cents,
			currency, origin_zone_id, destination_zone_id,
			service_level, actual_weight_kg, volumetric_weight_kg,
			chargeable_weight_kg, length_cm, width_cm, height_cm,
			pickup_address, dropoff_address, status,
			attempt_count, version, created_at, updated_at
		) VALUES (
			$1, $2, NULL, $3,
			$4, $5, $6,
			$7, $8, $9,
			'LKR', $10, $11,
			$12, $13, $14,
			$15, $16, $17, $18,
			$19, $20, 'CREATED',
			0, 1, $21, $21
		);
	`
	_, err = tx.ExecContext(ctx, insertParcelSQL,
		parcelID, trackingNumber, merchantID,
		recipientName, recipientPhone, quoteID,
		ruleID, ruleVersion, deliveryFeeCents,
		originZone, destinationZone,
		serviceLevel, weightKg, volumetricKg,
		chargeableKg, lengthCm, widthCm, heightCm,
		pickupJSON, dropoffJSON, now,
	)
	if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("insert parcel: %w", err)
	}

	// f. Insert initial CREATED record into parcel_status_history
	const insertHistorySQL = `
		INSERT INTO parcel_status_history (
			id, parcel_id, from_status, to_status,
			actor_id, actor_type, actor_role, reason, occurred_at
		) VALUES (
			$1, $2, NULL, 'CREATED',
			$3, 'MERCHANT_API', 'MERCHANT', 'Initial merchant parcel booking', $4
		);
	`
	_, err = tx.ExecContext(ctx, insertHistorySQL, historyID, parcelID, merchantID, now)
	if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("insert status history: %w", err)
	}

	// g. Atomically insert parcel.booked.v1 event into outbox_events
	payload := parcelBookedPayload{
		ParcelID:           parcelID,
		TrackingNumber:     trackingNumber,
		SenderID:           nil,
		MerchantID:         &merchantID,
		RecipientName:      recipientName,
		RecipientPhone:     recipientPhone,
		QuoteID:            quoteID,
		PricingRuleID:      ruleID,
		PricingRuleVersion: ruleVersion,
		DeliveryFeeCents:   deliveryFeeCents,
		Currency:           "LKR",
		OriginZoneID:       originZone,
		DestinationZoneID:  destinationZone,
		ServiceLevel:       serviceLevel,
		ActualWeightKg:     weightKg,
		ChargeableWeightKg: chargeableKg,
		Status:             "CREATED",
		CreatedAt:          now,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("marshal outbox payload: %w", err)
	}

	correlationID := idemKey
	if correlationID == "" {
		correlationID = parcelID
	}

	const insertOutboxSQL = `
		INSERT INTO outbox_events (
			id, producer, occurred_at, correlation_id,
			aggregate_type, aggregate_id, aggregate_version,
			event_type, schema_version, payload, status
		) VALUES (
			$1, 'core-api', $2, $3,
			'parcel', $4, 1,
			'parcel.booked.v1', '1.0.0', $5, 'PENDING'
		);
	`
	_, err = tx.ExecContext(ctx, insertOutboxSQL,
		eventID, now, correlationID,
		parcelID, payloadBytes,
	)
	if err != nil {
		return application.OrderResult{}, false, fmt.Errorf("insert outbox event: %w", err)
	}

	// Construct result
	result := application.OrderResult{
		ParcelID:          parcelID,
		TrackingNumber:    trackingNumber,
		MerchantReference: req.MerchantReference,
		Status:            "CREATED",
		CreatedAt:         now.Format(time.RFC3339),
	}
	result.Fee.Currency = "LKR"
	result.Fee.AmountMinor = deliveryFeeCents

	// h. Persist idempotency record
	if idemKey != "" {
		respBody, encodeErr := json.Marshal(result)
		if encodeErr == nil {
			const insertIdemSQL = `
				INSERT INTO idempotency_keys (
					key, caller_scope, operation, request_hash,
					response_status, response_body, created_resource_id,
					created_at, expires_at
				) VALUES (
					$1, $2, 'POST /api/v1/merchant/orders', $3,
					201, $4, $5,
					$6, $7
				) ON CONFLICT (key, caller_scope) DO NOTHING;
			`
			_, _ = tx.ExecContext(ctx, insertIdemSQL,
				idemKey, callerScope, requestHash,
				respBody, parcelID,
				now, now.Add(24*time.Hour),
			)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return application.OrderResult{}, false, fmt.Errorf("commit booking transaction: %w", err)
	}

	return result, false, nil
}

// ListForMerchant retrieves all parcels booked by the given merchant from PostgreSQL.
func (b *PostgresParcelBooking) ListForMerchant(ctx context.Context, merchantID string) ([]application.OrderResult, error) {
	if b.db == nil {
		return nil, errors.New("postgres booking: nil db handle")
	}

	cleanID := strings.TrimSpace(merchantID)
	if cleanID == "" {
		return []application.OrderResult{}, nil
	}

	const query = `
		SELECT
			id,
			tracking_number,
			COALESCE(dropoff_address->>'merchant_reference', '') AS merchant_reference,
			status,
			delivery_fee_cents,
			currency,
			created_at
		FROM parcels
		WHERE merchant_id = $1
		ORDER BY created_at DESC;
	`

	rows, err := b.db.QueryContext(ctx, query, cleanID)
	if err != nil {
		return nil, fmt.Errorf("list merchant orders from postgres: %w", err)
	}
	defer rows.Close()

	results := make([]application.OrderResult, 0)
	for rows.Next() {
		var (
			res       application.OrderResult
			feeCents  int64
			currency  string
			createdAt time.Time
		)
		err := rows.Scan(
			&res.ParcelID,
			&res.TrackingNumber,
			&res.MerchantReference,
			&res.Status,
			&feeCents,
			&currency,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan merchant order: %w", err)
		}
		res.Fee.Currency = currency
		res.Fee.AmountMinor = feeCents
		res.CreatedAt = createdAt.Format(time.RFC3339)
		results = append(results, res)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate merchant orders: %w", err)
	}

	return results, nil
}

// generateTrackingNumber produces a canonical Crockford Base32 tracking number SD-YYYYMMDD-XXXXXX
// and verifies uniqueness against the parcels table.
func (b *PostgresParcelBooking) generateTrackingNumber(ctx context.Context, tx *sql.Tx, date time.Time) (string, error) {
	const maxRetries = 5
	const suffixLen = 6

	for attempt := 0; attempt < maxRetries; attempt++ {
		var randomBytes [suffixLen]byte
		if _, err := rand.Read(randomBytes[:]); err != nil {
			return "", fmt.Errorf("random bytes: %w", err)
		}

		suffix := make([]byte, suffixLen)
		for i := 0; i < suffixLen; i++ {
			suffix[i] = crockfordAlphabet[randomBytes[i]&0x1F]
		}

		trackingNumber := fmt.Sprintf("SD-%s-%s", date.Format("20060102"), string(suffix))

		var exists bool
		checkSQL := `SELECT EXISTS(SELECT 1 FROM parcels WHERE tracking_number = $1);`
		if err := tx.QueryRowContext(ctx, checkSQL, trackingNumber).Scan(&exists); err != nil {
			return "", fmt.Errorf("check tracking number uniqueness: %w", err)
		}

		if !exists {
			return trackingNumber, nil
		}
	}

	return "", errors.New("unable to allocate unique tracking number after retries")
}

// parseAddress extracts and normalizes address fields required by the parcels table JSONB check constraint.
func parseAddress(raw map[string]any, fallbackZone string) map[string]any {
	out := make(map[string]any)

	out["contact_name"] = getString(raw, "contact_name", "name")
	if out["contact_name"] == "" {
		out["contact_name"] = "Contact Person"
	}

	out["contact_phone"] = getString(raw, "contact_phone", "phone")
	if out["contact_phone"] == "" {
		out["contact_phone"] = "+94770000000"
	}

	// Address line: may be top-level or nested in "address" sub-map
	line1 := getString(raw, "address_line1", "line1")
	city := getString(raw, "city")
	postalCode := getString(raw, "postal_code", "postal")
	zoneID := getString(raw, "zone_id", "zone")

	if sub, ok := raw["address"].(map[string]any); ok {
		if line1 == "" {
			line1 = getString(sub, "line1", "address_line1")
		}
		if city == "" {
			city = getString(sub, "city")
		}
		if postalCode == "" {
			postalCode = getString(sub, "postal_code", "postal")
		}
		if zoneID == "" {
			zoneID = getString(sub, "zone_id", "zone")
		}
	} else if subStr, ok := raw["address"].(map[string]string); ok {
		if line1 == "" {
			if v, ok := subStr["line1"]; ok {
				line1 = v
			} else if v, ok := subStr["address_line1"]; ok {
				line1 = v
			}
		}
		if city == "" {
			if v, ok := subStr["city"]; ok {
				city = v
			}
		}
		if postalCode == "" {
			if v, ok := subStr["postal_code"]; ok {
				postalCode = v
			}
		}
		if zoneID == "" {
			if v, ok := subStr["zone_id"]; ok {
				zoneID = v
			}
		}
	}

	if line1 == "" {
		line1 = "100 Galle Road"
	}
	if city == "" {
		city = "Colombo"
	}
	if postalCode == "" {
		postalCode = "00100"
	}
	if zoneID == "" || !zoneIDPattern.MatchString(zoneID) {
		zoneID = fallbackZone
	}

	out["address_line1"] = line1
	out["city"] = city
	out["postal_code"] = postalCode
	out["zone_id"] = zoneID

	return out
}

func getString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}
