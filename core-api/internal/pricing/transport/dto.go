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
