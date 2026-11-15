package transport

import (
	"time"

	"github.com/swiftdrop/core-api/internal/parcels/domain"
)

// DimensionsRequest captures physical package measurements in centimeters.
type DimensionsRequest struct {
	LengthCm float64 `json:"length_cm"`
	WidthCm  float64 `json:"width_cm"`
	HeightCm float64 `json:"height_cm"`
}

// AddressRequest captures immutable pickup or dropoff address fields submitted at booking time.
type AddressRequest struct {
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	StreetLine1  string `json:"address_line1"`
	StreetLine2  string `json:"address_line2"`
	City         string `json:"city"`
	PostalCode   string `json:"postal_code"`
	ZoneID       string `json:"zone_id"`
	Instructions string `json:"instructions"`
}

// BookParcelRequest represents the JSON payload for POST /api/v1/parcels.
// Ownership (sender_id, merchant_id), pricing snapshots, and initial status are excluded and
// resolved exclusively from the authenticated caller and consumed pricing quote.
type BookParcelRequest struct {
	QuoteID             string             `json:"quote_id"`
	RecipientName       string             `json:"recipient_name"`
	RecipientPhone      string             `json:"recipient_phone"`
	WeightKg            float64            `json:"weight_kg"`
	Dimensions          *DimensionsRequest `json:"dimensions"`
	Pickup              *AddressRequest    `json:"pickup"`
	Dropoff             *AddressRequest    `json:"dropoff"`
	ItemDescription     string             `json:"item_description"`
	SpecialInstructions string             `json:"special_instructions"`
}

// ParcelWeightResponse exposes actual, volumetric, and chargeable weight metrics in kilograms.
type ParcelWeightResponse struct {
	ActualKg     float64 `json:"actual_kg"`
	VolumetricKg float64 `json:"volumetric_kg"`
	ChargeableKg float64 `json:"chargeable_kg"`
}

// ParcelDimensionsResponse exposes package dimensions in centimeters.
type ParcelDimensionsResponse struct {
	LengthCm float64 `json:"length_cm"`
	WidthCm  float64 `json:"width_cm"`
	HeightCm float64 `json:"height_cm"`
}

// AddressResponse exposes an immutable pickup or dropoff address snapshot.
type AddressResponse struct {
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	StreetLine1  string `json:"address_line1"`
	StreetLine2  string `json:"address_line2"`
	City         string `json:"city"`
	PostalCode   string `json:"postal_code"`
	ZoneID       string `json:"zone_id"`
	Instructions string `json:"instructions,omitempty"`
}

// ParcelResponse represents the authoritative Parcel aggregate root returned inside the standard data envelope.
type ParcelResponse struct {
	ID                  string                   `json:"id"`
	TrackingNumber      string                   `json:"tracking_number"`
	SenderID            *string                  `json:"sender_id,omitempty"`
	MerchantID          *string                  `json:"merchant_id,omitempty"`
	RecipientName       string                   `json:"recipient_name"`
	RecipientPhone      string                   `json:"recipient_phone"`
	QuoteID             string                   `json:"quote_id"`
	PricingRuleID       string                   `json:"pricing_rule_id"`
	PricingRuleVersion  int                      `json:"pricing_rule_version"`
	DeliveryFeeCents    int64                    `json:"delivery_fee_cents"`
	Currency            string                   `json:"currency"`
	OriginZoneID        string                   `json:"origin_zone_id"`
	DestinationZoneID   string                   `json:"destination_zone_id"`
	ServiceLevel        string                   `json:"service_level"`
	Weight              ParcelWeightResponse     `json:"weight"`
	Dimensions          ParcelDimensionsResponse `json:"dimensions"`
	Pickup              AddressResponse          `json:"pickup"`
	Dropoff             AddressResponse          `json:"dropoff"`
	ItemDescription     string                   `json:"item_description,omitempty"`
	SpecialInstructions string                   `json:"special_instructions,omitempty"`
	Status              string                   `json:"status"`
	AttemptCount        int                      `json:"attempt_count"`
	MaxAttempts         int                      `json:"max_attempts"`
	Version             int                      `json:"version"`
	CreatedAt           string                   `json:"created_at"`
	UpdatedAt           string                   `json:"updated_at"`
}

// StatusHistoryItemResponse represents a single immutable audit record in a parcel's lifecycle.
type StatusHistoryItemResponse struct {
	ID         string  `json:"id"`
	ParcelID   string  `json:"parcel_id"`
	FromStatus *string `json:"from_status"`
	ToStatus   string  `json:"to_status"`
	ActorID    *string `json:"actor_id,omitempty"`
	ActorType  string  `json:"actor_type"`
	ActorRole  string  `json:"actor_role"`
	Reason     string  `json:"reason"`
	OccurredAt string  `json:"occurred_at"`
}

// ParcelHistoryResponse represents the parcel metadata and chronological status history entries.
type ParcelHistoryResponse struct {
	ParcelID       string                      `json:"parcel_id"`
	TrackingNumber string                      `json:"tracking_number"`
	Status         string                      `json:"status"`
	History        []StatusHistoryItemResponse `json:"history"`
}

// NewParcelResponse maps a domain.Parcel aggregate into its HTTP response DTO.
func NewParcelResponse(p domain.Parcel) ParcelResponse {
	return ParcelResponse{
		ID:                 p.ID,
		TrackingNumber:     p.TrackingNumber.String(),
		SenderID:           p.SenderID,
		MerchantID:         p.MerchantID,
		RecipientName:      p.RecipientName,
		RecipientPhone:     p.RecipientPhone,
		QuoteID:            p.QuoteID,
		PricingRuleID:      p.PricingRuleID,
		PricingRuleVersion: p.PricingRuleVersion,
		DeliveryFeeCents:   p.DeliveryFee.Amount,
		Currency:           p.DeliveryFee.Currency,
		OriginZoneID:       p.OriginZoneID,
		DestinationZoneID:  p.DestinationZoneID,
		ServiceLevel:       string(p.ServiceLevel),
		Weight: ParcelWeightResponse{
			ActualKg:     p.Weight.ActualKg,
			VolumetricKg: p.Weight.VolumetricKg,
			ChargeableKg: p.Weight.ChargeableKg,
		},
		Dimensions: ParcelDimensionsResponse{
			LengthCm: p.Dimensions.LengthCm,
			WidthCm:  p.Dimensions.WidthCm,
			HeightCm: p.Dimensions.HeightCm,
		},
		Pickup:              newAddressResponse(p.PickupAddress),
		Dropoff:             newAddressResponse(p.DropoffAddress),
		ItemDescription:     p.ItemDescription,
		SpecialInstructions: p.SpecialInstructions,
		Status:              string(p.Status),
		AttemptCount:        p.AttemptCount,
		MaxAttempts:         domain.MaxDeliveryAttempts,
		Version:             p.Version,
		CreatedAt:           p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           p.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// NewStatusHistoryItemResponse maps a domain.StatusHistoryEntry into an HTTP response DTO.
func NewStatusHistoryItemResponse(entry domain.StatusHistoryEntry) StatusHistoryItemResponse {
	var fromStatus *string
	if entry.FromStatus != nil {
		s := string(*entry.FromStatus)
		fromStatus = &s
	}

	return StatusHistoryItemResponse{
		ID:         entry.ID,
		ParcelID:   entry.ParcelID,
		FromStatus: fromStatus,
		ToStatus:   string(entry.ToStatus),
		ActorID:    entry.ActorID,
		ActorType:  string(entry.ActorType),
		ActorRole:  entry.ActorRole,
		Reason:     entry.Reason,
		OccurredAt: entry.OccurredAt.UTC().Format(time.RFC3339),
	}
}

// NewParcelHistoryResponse maps a parcel aggregate and chronological status entries into ParcelHistoryResponse.
func NewParcelHistoryResponse(p domain.Parcel, entries []domain.StatusHistoryEntry) ParcelHistoryResponse {
	history := make([]StatusHistoryItemResponse, 0, len(entries))
	for _, entry := range entries {
		history = append(history, NewStatusHistoryItemResponse(entry))
	}

	return ParcelHistoryResponse{
		ParcelID:       p.ID,
		TrackingNumber: p.TrackingNumber.String(),
		Status:         string(p.Status),
		History:        history,
	}
}

func newAddressResponse(a domain.Address) AddressResponse {
	return AddressResponse{
		ContactName:  a.ContactName,
		ContactPhone: a.ContactPhone,
		StreetLine1:  a.StreetLine1,
		StreetLine2:  a.StreetLine2,
		City:         a.City,
		PostalCode:   a.PostalCode,
		ZoneID:       a.ZoneID,
		Instructions: a.Instructions,
	}
}
