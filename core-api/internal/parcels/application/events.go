package application

import (
	"time"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
)

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

// ParcelStatusChangedPayload represents the integration event payload published
// to the transactional outbox upon any successful parcel lifecycle state transition.
type ParcelStatusChangedPayload struct {
	ParcelID         string    `json:"parcel_id"`
	TrackingNumber   string    `json:"tracking_number"`
	PreviousStatus   string    `json:"previous_status"`
	NewStatus        string    `json:"new_status"`
	ActorID          string    `json:"actor_id,omitempty"`
	ActorRole        string    `json:"actor_role,omitempty"`
	Reason           string    `json:"reason,omitempty"`
	DeliveryAttempts int       `json:"delivery_attempts"`
	OccurredAt       time.Time `json:"occurred_at"`
}

// EventTypeForStatus maps a target parcel status to its authoritative versioned event type.
func EventTypeForStatus(target domain.ParcelStatus) string {
	switch target {
	case domain.StatusAssigned:
		return events.ParcelAssignedV1
	case domain.StatusPickedUp:
		return events.ParcelPickedUpV1
	case domain.StatusInTransit:
		return events.ParcelInTransitV1
	case domain.StatusOutForDelivery:
		return events.ParcelOutForDeliveryV1
	case domain.StatusDelivered:
		return events.ParcelDeliveredV1
	case domain.StatusAttemptFailed:
		return events.ParcelDeliveryFailedV1
	case domain.StatusReturned:
		return events.ParcelReturnedV1
	case domain.StatusCancelled:
		return events.ParcelCancelledV1
	default:
		return ""
	}
}
