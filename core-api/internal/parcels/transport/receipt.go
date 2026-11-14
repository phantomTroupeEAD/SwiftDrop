package transport

import (
	"fmt"
	"time"

	"github.com/swiftdrop/core-api/internal/parcels/domain"
)

type receiptFinancialSummary struct {
	Currency           string `json:"currency"`
	PriceCents         int64  `json:"price_cents"`
	TotalFeeFormatted  string `json:"total_fee_formatted"`
	PricingRuleID      string `json:"pricing_rule_id"`
	PricingRuleVersion int    `json:"pricing_rule_version"`
}

type receiptPackageSummary struct {
	ChargeableWeightKg float64 `json:"chargeable_weight_kg"`
	OriginZone         string  `json:"origin_zone"`
	DestinationZone    string  `json:"destination_zone"`
}

type receiptResponse struct {
	ReceiptNumber    string                  `json:"receipt_number"`
	ParcelID         string                  `json:"parcel_id"`
	TrackingNumber   string                  `json:"tracking_number"`
	BookingDate      string                  `json:"booking_date"`
	CustomerName     string                  `json:"customer_name"`
	ServiceLevel     string                  `json:"service_level"`
	FinancialSummary receiptFinancialSummary `json:"financial_summary"`
	PackageSummary   receiptPackageSummary   `json:"package_summary"`
	PaymentStatus    string                  `json:"payment_status"`
}

func newReceiptResponse(parcel domain.Parcel) receiptResponse {
	price := parcel.DeliveryFee
	trackingNumber := parcel.TrackingNumber.String()
	return receiptResponse{
		ReceiptNumber:  "RCP-" + trackingNumber,
		ParcelID:       parcel.ID,
		TrackingNumber: trackingNumber,
		BookingDate:    parcel.CreatedAt.UTC().Format(time.RFC3339),
		CustomerName:   parcel.PickupAddress.ContactName,
		ServiceLevel:   string(parcel.ServiceLevel),
		FinancialSummary: receiptFinancialSummary{
			Currency:           price.Currency,
			PriceCents:         price.Amount,
			TotalFeeFormatted:  fmt.Sprintf("%s %d.%02d", price.Currency, price.Amount/100, price.Amount%100),
			PricingRuleID:      parcel.PricingRuleID,
			PricingRuleVersion: parcel.PricingRuleVersion,
		},
		PackageSummary: receiptPackageSummary{
			ChargeableWeightKg: parcel.Weight.ChargeableKg,
			OriginZone:         parcel.OriginZoneID,
			DestinationZone:    parcel.DestinationZoneID,
		},
		PaymentStatus: "NOT_TRACKED",
	}
}
