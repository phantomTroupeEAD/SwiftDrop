package application

import "time"

// ParcelBookedPayload represents the integration event payload published
// to the transactional outbox upon successful parcel creation.
type ParcelBookedPayload struct {
	ParcelID           string    `json:"parcel_id"`
	TrackingNumber     string    `json:"tracking_number"`
	SenderID           string    `json:"sender_id"`
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

