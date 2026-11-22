package transport

import (
	"time"

	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

// DimensionsRequest captures physical package measurements in centimeters.
type DimensionsRequest struct {
	LengthCm float64 `json:"length_cm"`
	WidthCm  float64 `json:"width_cm"`
	HeightCm float64 `json:"height_cm"`
}

// CreateQuoteRequest represents the JSON payload for POST /api/v1/pricing/quotes.
// Ownership fields (requester_id, merchant_id) are intentionally excluded and resolved
// exclusively from the authenticated caller principal.
type CreateQuoteRequest struct {
	OriginZoneID      string             `json:"origin_zone_id"`
	DestinationZoneID string             `json:"destination_zone_id"`
	ActualWeightKg    float64            `json:"actual_weight_kg"`
	Dimensions        *DimensionsRequest `json:"dimensions"`
	ServiceLevel      string             `json:"service_level"`
}

// QuoteResponse represents the persisted quote returned inside the standard data envelope.
type QuoteResponse struct {
	QuoteID            string  `json:"quote_id"`
	PricingRuleID      string  `json:"pricing_rule_id"`
	PricingRuleVersion int     `json:"pricing_rule_version"`
	RequesterID        *string `json:"requester_id,omitempty"`
	MerchantID         *string `json:"merchant_id,omitempty"`
	ChargeableWeightKg float64 `json:"chargeable_weight_kg"`
	ServiceLevel       string  `json:"service_level"`
	FeeCents           int64   `json:"fee_cents"`
	Currency           string  `json:"currency"`
	CreatedAt          string  `json:"created_at"`
	ExpiresAt          string  `json:"expires_at"`
}

// NewQuoteResponse maps a domain.Quote into the authoritative HTTP response DTO.
func NewQuoteResponse(q domain.Quote) QuoteResponse {
	return QuoteResponse{
		QuoteID:            q.ID,
		PricingRuleID:      q.TariffID,
		PricingRuleVersion: q.TariffVersion,
		RequesterID:        q.RequesterID,
		MerchantID:         q.MerchantID,
		ChargeableWeightKg: q.ChargeableWeightKg,
		ServiceLevel:       string(q.ServiceLevel),
		FeeCents:           q.Fee.Amount,
		Currency:           q.Fee.Currency,
		CreatedAt:          q.CreatedAt.UTC().Format(time.RFC3339),
		ExpiresAt:          q.ExpiresAt.UTC().Format(time.RFC3339),
	}
}

// PricingRuleResponse represents the public JSON projection of a pricing rule / tariff.
type PricingRuleResponse struct {
	ID                string  `json:"id"`
	Version           int     `json:"version"`
	OriginZoneID      string  `json:"origin_zone_id"`
	DestinationZoneID string  `json:"destination_zone_id"`
	BaseWeightKg      float64 `json:"base_weight_kg"`
	MaxWeightKg       float64 `json:"max_weight_kg"`
	BaseFeeCents      int64   `json:"base_fee_cents"`
	PerKgRateCents    int64   `json:"per_kg_rate_cents"`
	Currency          string  `json:"currency"`
	EffectiveFrom     string  `json:"effective_from"`
	EffectiveUntil    *string `json:"effective_until,omitempty"`
	IsActive          bool    `json:"is_active"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

// NewPricingRuleResponse projects a domain Tariff entity into PricingRuleResponse.
func NewPricingRuleResponse(tariff domain.Tariff) PricingRuleResponse {
	var effUntil *string
	if tariff.EffectiveUntil != nil {
		formatted := tariff.EffectiveUntil.UTC().Format(time.RFC3339)
		effUntil = &formatted
	}

	updatedAt := tariff.UpdatedAt.UTC().Format(time.RFC3339)
	if tariff.UpdatedAt.IsZero() {
		updatedAt = tariff.CreatedAt.UTC().Format(time.RFC3339)
	}

	currency := string(tariff.BaseFee.Currency)
	if currency == "" {
		currency = string(domain.CurrencyLKR)
	}

	return PricingRuleResponse{
		ID:                tariff.ID,
		Version:           tariff.Version,
		OriginZoneID:      tariff.OriginZoneID,
		DestinationZoneID: tariff.DestinationZoneID,
		BaseWeightKg:      1.0,
		MaxWeightKg:       domain.MaxChargeableWeightKg,
		BaseFeeCents:      tariff.BaseFee.Amount,
		PerKgRateCents:    tariff.PerKgRate.Amount,
		Currency:          currency,
		EffectiveFrom:     tariff.EffectiveFrom.UTC().Format(time.RFC3339),
		EffectiveUntil:    effUntil,
		IsActive:          tariff.IsActive,
		CreatedAt:         tariff.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         updatedAt,
	}
}
