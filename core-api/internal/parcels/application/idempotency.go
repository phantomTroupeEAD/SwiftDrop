package application

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/parcels/domain"
)

const (
	// DefaultIdempotencyTTL defines the standard 24-hour lifetime of idempotency records.
	DefaultIdempotencyTTL = 24 * time.Hour

	// DefaultBookingOperation identifies parcel booking idempotency records.
	DefaultBookingOperation = "POST /api/v1/parcels"

	// defaultBookingResponseStatus stores the canonical HTTP status code for a created parcel.
	defaultBookingResponseStatus = 201
)

var (
	callerScopePattern = regexp.MustCompile(`^(USER|MERCHANT):[^\s]+$`)
	requestHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// IdempotencyInput supplies caller-scoped idempotency metadata for mutating booking commands.
type IdempotencyInput struct {
	Key         string
	CallerScope string
	Operation   string
	RequestHash string
}

// Normalize validates and normalizes all idempotency input fields.
func (in IdempotencyInput) Normalize() (IdempotencyInput, error) {
	key := strings.TrimSpace(in.Key)
	if key == "" || len(key) > 255 {
		return IdempotencyInput{}, fmt.Errorf("%w: key must be between 1 and 255 characters", ErrInvalidIdempotencyKey)
	}

	scope := strings.TrimSpace(in.CallerScope)
	if len(scope) > 64 || !callerScopePattern.MatchString(scope) {
		return IdempotencyInput{}, fmt.Errorf("%w: caller_scope must match USER:<id> or MERCHANT:<id> within 64 chars", ErrInvalidCallerScope)
	}

	op := strings.TrimSpace(in.Operation)
	if op == "" || len(op) > 64 {
		return IdempotencyInput{}, fmt.Errorf("%w: operation must be between 1 and 64 characters", ErrInvalidIdempotencyOperation)
	}

	hash := strings.ToLower(strings.TrimSpace(in.RequestHash))
	if !requestHashPattern.MatchString(hash) {
		return IdempotencyInput{}, fmt.Errorf("%w: request_hash must be a 64-character SHA-256 hex digest", ErrInvalidRequestHash)
	}

	return IdempotencyInput{
		Key:         key,
		CallerScope: scope,
		Operation:   op,
		RequestHash: hash,
	}, nil
}

// CallerScopeForBooking builds the canonical caller_scope string from normalized booking owners.
func CallerScopeForBooking(senderID, merchantID *string) (string, error) {
	normSender, normMerchant, err := normalizeBookingOwners(senderID, merchantID)
	if err != nil {
		return "", err
	}
	if normMerchant != nil {
		scope := "MERCHANT:" + *normMerchant
		if len(scope) > 64 || !callerScopePattern.MatchString(scope) {
			return "", fmt.Errorf("%w: merchant caller_scope exceeds constraints", ErrInvalidCallerScope)
		}
		return scope, nil
	}
	scope := "USER:" + *normSender
	if len(scope) > 64 || !callerScopePattern.MatchString(scope) {
		return "", fmt.Errorf("%w: user caller_scope exceeds constraints", ErrInvalidCallerScope)
	}
	return scope, nil
}

// IdempotencyRecord models a persisted row in the idempotency_keys table.
type IdempotencyRecord struct {
	Key               string
	CallerScope       string
	Operation         string
	RequestHash       string
	ResponseStatus    int
	ResponseBody      []byte
	CreatedResourceID *string
	CreatedAt         time.Time
	ExpiresAt         time.Time
}

// IsExpired reports whether the record has reached or passed its expiration timestamp.
func (r IdempotencyRecord) IsExpired(at time.Time) bool {
	return !r.ExpiresAt.After(at)
}

// Validate verifies all invariants on an IdempotencyRecord before persistence.
func (r IdempotencyRecord) Validate() error {
	_, err := (IdempotencyInput{
		Key:         r.Key,
		CallerScope: r.CallerScope,
		Operation:   r.Operation,
		RequestHash: r.RequestHash,
	}).Normalize()
	if err != nil {
		return err
	}

	if r.ResponseStatus < 100 || r.ResponseStatus > 599 {
		return fmt.Errorf("%w: response_status must be between 100 and 599", ErrInvalidIdempotencyRecord)
	}
	if len(r.ResponseBody) == 0 || !json.Valid(r.ResponseBody) {
		return fmt.Errorf("%w: response_body must be non-empty valid JSON", ErrInvalidIdempotencyRecord)
	}
	if r.CreatedResourceID != nil && strings.TrimSpace(*r.CreatedResourceID) == "" {
		return fmt.Errorf("%w: created_resource_id cannot be blank when provided", ErrInvalidIdempotencyRecord)
	}
	if r.CreatedAt.IsZero() || r.ExpiresAt.IsZero() {
		return fmt.Errorf("%w: created_at and expires_at are required", ErrInvalidIdempotencyRecord)
	}
	if !r.ExpiresAt.After(r.CreatedAt) {
		return fmt.Errorf("%w: expires_at must be strictly after created_at", ErrInvalidIdempotencyRecord)
	}

	return nil
}

func verifyCallerScopeMatchesOwners(scope string, senderID, merchantID *string) error {
	switch {
	case strings.HasPrefix(scope, "USER:"):
		id := strings.TrimPrefix(scope, "USER:")
		if senderID == nil || *senderID != id {
			return fmt.Errorf("%w: USER caller_scope does not match booking sender_id", ErrInvalidCallerScope)
		}
		return nil
	case strings.HasPrefix(scope, "MERCHANT:"):
		id := strings.TrimPrefix(scope, "MERCHANT:")
		if merchantID == nil || *merchantID != id {
			return fmt.Errorf("%w: MERCHANT caller_scope does not match booking merchant_id", ErrInvalidCallerScope)
		}
		return nil
	default:
		return ErrInvalidCallerScope
	}
}

type bookedParcelSnapshot struct {
	Parcel        parcelSnapshotDTO        `json:"parcel"`
	StatusHistory statusHistorySnapshotDTO `json:"status_history"`
}

type parcelSnapshotDTO struct {
	ID                 string         `json:"id"`
	TrackingNumber     string         `json:"tracking_number"`
	SenderID           *string        `json:"sender_id,omitempty"`
	MerchantID         *string        `json:"merchant_id,omitempty"`
	RecipientName      string         `json:"recipient_name"`
	RecipientPhone     string         `json:"recipient_phone"`
	QuoteID            string         `json:"quote_id"`
	PricingRuleID      string         `json:"pricing_rule_id"`
	PricingRuleVersion int            `json:"pricing_rule_version"`
	DeliveryFeeCents   int64          `json:"delivery_fee_cents"`
	Currency           string         `json:"currency"`
	OriginZoneID       string         `json:"origin_zone_id"`
	DestinationZoneID  string         `json:"destination_zone_id"`
	ServiceLevel       string         `json:"service_level"`
	ActualWeightKg     float64        `json:"actual_weight_kg"`
	VolumetricWeightKg float64        `json:"volumetric_weight_kg"`
	ChargeableWeightKg float64        `json:"chargeable_weight_kg"`
	LengthCm           float64        `json:"length_cm"`
	WidthCm            float64        `json:"width_cm"`
	HeightCm           float64        `json:"height_cm"`
	PickupAddress      domain.Address `json:"pickup_address"`
	DropoffAddress     domain.Address `json:"dropoff_address"`
	Status             string         `json:"status"`
	AttemptCount       int            `json:"attempt_count"`
	Version            int            `json:"version"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type statusHistorySnapshotDTO struct {
	ID         string    `json:"id"`
	ParcelID   string    `json:"parcel_id"`
	FromStatus *string   `json:"from_status,omitempty"`
	ToStatus   string    `json:"to_status"`
	ActorID    *string   `json:"actor_id,omitempty"`
	ActorType  string    `json:"actor_type"`
	ActorRole  string    `json:"actor_role"`
	Reason     string    `json:"reason"`
	OccurredAt time.Time `json:"occurred_at"`
}

func encodeBookParcelSnapshot(result BookParcelResult) ([]byte, error) {
	var fromStatus *string
	if result.StatusHistory.FromStatus != nil {
		s := string(*result.StatusHistory.FromStatus)
		fromStatus = &s
	}

	snap := bookedParcelSnapshot{
		Parcel: parcelSnapshotDTO{
			ID:                 result.Parcel.ID,
			TrackingNumber:     result.Parcel.TrackingNumber.String(),
			SenderID:           result.Parcel.SenderID,
			MerchantID:         result.Parcel.MerchantID,
			RecipientName:      result.Parcel.RecipientName,
			RecipientPhone:     result.Parcel.RecipientPhone,
			QuoteID:            result.Parcel.QuoteID,
			PricingRuleID:      result.Parcel.PricingRuleID,
			PricingRuleVersion: result.Parcel.PricingRuleVersion,
			DeliveryFeeCents:   result.Parcel.DeliveryFee.Amount,
			Currency:           result.Parcel.DeliveryFee.Currency,
			OriginZoneID:       result.Parcel.OriginZoneID,
			DestinationZoneID:  result.Parcel.DestinationZoneID,
			ServiceLevel:       string(result.Parcel.ServiceLevel),
			ActualWeightKg:     result.Parcel.Weight.ActualKg,
			VolumetricWeightKg: result.Parcel.Weight.VolumetricKg,
			ChargeableWeightKg: result.Parcel.Weight.ChargeableKg,
			LengthCm:           result.Parcel.Dimensions.LengthCm,
			WidthCm:            result.Parcel.Dimensions.WidthCm,
			HeightCm:           result.Parcel.Dimensions.HeightCm,
			PickupAddress:      result.Parcel.PickupAddress,
			DropoffAddress:     result.Parcel.DropoffAddress,
			Status:             string(result.Parcel.Status),
			AttemptCount:       result.Parcel.AttemptCount,
			Version:            result.Parcel.Version,
			CreatedAt:          result.Parcel.CreatedAt.UTC(),
			UpdatedAt:          result.Parcel.UpdatedAt.UTC(),
		},
		StatusHistory: statusHistorySnapshotDTO{
			ID:         result.StatusHistory.ID,
			ParcelID:   result.StatusHistory.ParcelID,
			FromStatus: fromStatus,
			ToStatus:   string(result.StatusHistory.ToStatus),
			ActorID:    result.StatusHistory.ActorID,
			ActorType:  string(result.StatusHistory.ActorType),
			ActorRole:  result.StatusHistory.ActorRole,
			Reason:     result.StatusHistory.Reason,
			OccurredAt: result.StatusHistory.OccurredAt.UTC(),
		},
	}

	raw, err := json.Marshal(snap)
	if err != nil {
		return nil, fmt.Errorf("encode booking replay snapshot: %w", err)
	}
	return raw, nil
}

func decodeBookParcelSnapshot(raw []byte) (BookParcelResult, error) {
	var snap bookedParcelSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return BookParcelResult{}, fmt.Errorf("%w: failed to unmarshal cached booking snapshot: %v", ErrInvalidIdempotencyRecord, err)
	}

	tn, err := domain.NewTrackingNumber(snap.Parcel.TrackingNumber)
	if err != nil {
		return BookParcelResult{}, fmt.Errorf("%w: cached tracking number: %v", ErrInvalidIdempotencyRecord, err)
	}
	fee, err := domain.NewMoney(snap.Parcel.DeliveryFeeCents, snap.Parcel.Currency)
	if err != nil {
		return BookParcelResult{}, fmt.Errorf("%w: cached fee: %v", ErrInvalidIdempotencyRecord, err)
	}
	weight, err := domain.NewWeight(snap.Parcel.ActualWeightKg, snap.Parcel.VolumetricWeightKg, snap.Parcel.ChargeableWeightKg)
	if err != nil {
		return BookParcelResult{}, fmt.Errorf("%w: cached weight: %v", ErrInvalidIdempotencyRecord, err)
	}
	dims, err := domain.NewDimensions(snap.Parcel.LengthCm, snap.Parcel.WidthCm, snap.Parcel.HeightCm)
	if err != nil {
		return BookParcelResult{}, fmt.Errorf("%w: cached dimensions: %v", ErrInvalidIdempotencyRecord, err)
	}

	parcel, err := domain.ReconstituteParcel(domain.ReconstituteParcelParams{
		ID:                 snap.Parcel.ID,
		TrackingNumber:     tn,
		SenderID:           snap.Parcel.SenderID,
		MerchantID:         snap.Parcel.MerchantID,
		RecipientName:      snap.Parcel.RecipientName,
		RecipientPhone:     snap.Parcel.RecipientPhone,
		QuoteID:            snap.Parcel.QuoteID,
		PricingRuleID:      snap.Parcel.PricingRuleID,
		PricingRuleVersion: snap.Parcel.PricingRuleVersion,
		DeliveryFee:        fee,
		OriginZoneID:       snap.Parcel.OriginZoneID,
		DestinationZoneID:  snap.Parcel.DestinationZoneID,
		ServiceLevel:       domain.ServiceLevel(snap.Parcel.ServiceLevel),
		Weight:             weight,
		Dimensions:         dims,
		PickupAddress:      snap.Parcel.PickupAddress,
		DropoffAddress:     snap.Parcel.DropoffAddress,
		Status:             domain.ParcelStatus(snap.Parcel.Status),
		AttemptCount:       snap.Parcel.AttemptCount,
		Version:            snap.Parcel.Version,
		CreatedAt:          snap.Parcel.CreatedAt,
		UpdatedAt:          snap.Parcel.UpdatedAt,
	})
	if err != nil {
		return BookParcelResult{}, fmt.Errorf("%w: cached parcel snapshot: %v", ErrInvalidIdempotencyRecord, err)
	}

	var fromStatus *domain.ParcelStatus
	if snap.StatusHistory.FromStatus != nil {
		fs := domain.ParcelStatus(*snap.StatusHistory.FromStatus)
		fromStatus = &fs
	}
	historyEntry, err := domain.NewStatusHistoryEntry(
		snap.StatusHistory.ID,
		snap.StatusHistory.ParcelID,
		fromStatus,
		domain.ParcelStatus(snap.StatusHistory.ToStatus),
		snap.StatusHistory.ActorID,
		domain.ActorType(snap.StatusHistory.ActorType),
		snap.StatusHistory.ActorRole,
		snap.StatusHistory.Reason,
		snap.StatusHistory.OccurredAt,
		0,
	)
	if err != nil {
		return BookParcelResult{}, fmt.Errorf("%w: cached status history snapshot: %v", ErrInvalidIdempotencyRecord, err)
	}

	return BookParcelResult{
		Parcel:        parcel,
		StatusHistory: historyEntry,
	}, nil
}
