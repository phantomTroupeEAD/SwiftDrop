package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/merchants/application"
	parcelsapp "github.com/swiftdrop/core-api/internal/parcels/application"
	parcelsdomain "github.com/swiftdrop/core-api/internal/parcels/domain"
	pricingdomain "github.com/swiftdrop/core-api/internal/pricing/domain"
)

var _ application.ParcelBooking = (*ParcelsBookingAdapter)(nil)

// ParcelsBookingAdapter adapts Member 2's authoritative parcel booking engine
// and repository to satisfy Member 4's ParcelBooking port.
type ParcelsBookingAdapter struct {
	bookUC     *parcelsapp.BookMerchantParcelUseCase
	parcelRepo parcelsapp.ParcelRepository
}

// NewParcelsBookingAdapter constructs a ParcelsBookingAdapter.
func NewParcelsBookingAdapter(
	bookUC *parcelsapp.BookMerchantParcelUseCase,
	parcelRepo parcelsapp.ParcelRepository,
) *ParcelsBookingAdapter {
	return &ParcelsBookingAdapter{
		bookUC:     bookUC,
		parcelRepo: parcelRepo,
	}
}

// CreateForMerchant validates the incoming merchant order request, maps it to the authoritative
// merchant booking command, and delegates parcel creation to Member 2's BookMerchantParcelUseCase.
func (a *ParcelsBookingAdapter) CreateForMerchant(
	ctx context.Context,
	merchantID, idemKey, requestHash string,
	req application.OrderRequest,
) (application.OrderResult, bool, error) {
	if a.bookUC == nil {
		return application.OrderResult{}, false, errors.New("parcels booking adapter: nil book use case")
	}

	cleanMerchantID := strings.TrimSpace(merchantID)
	if cleanMerchantID == "" || !events.IsValidUUID(cleanMerchantID) {
		return application.OrderResult{}, false, application.ValidationError{
			Field: "merchant_id",
			Issue: "authenticated merchant ID is invalid or missing",
		}
	}

	// 1. Resolve parcel specification & extract weight
	parcelSpec := req.Parcel
	if parcelSpec.WeightKG <= 0 && parcelSpec.WeightGrams <= 0 && req.Package != nil {
		parcelSpec = *req.Package
	}
	weightKG := parcelSpec.ExtractWeightKG()
	if weightKG <= 0 {
		return application.OrderResult{}, false, application.ValidationError{
			Field: "parcel.weight_kg",
			Issue: "parcel weight_kg must be greater than 0",
		}
	}

	// 2. Extract and strictly validate dimensions (volumetric weight affects pricing)
	length, width, height := parcelSpec.ExtractDimensions()
	if length <= 0 || width <= 0 || height <= 0 {
		return application.OrderResult{}, false, application.ValidationError{
			Field: "parcel.dimensions",
			Issue: "parcel dimensions are required: length, width, and height in cm must be greater than 0",
		}
	}

	// 3. Resolve and validate addresses
	dropoffMap := req.Dropoff
	if len(dropoffMap) == 0 && len(req.Recipient) > 0 {
		dropoffMap = req.Recipient
	}

	pickupAddr, err := extractAddress(req.Pickup, "pickup")
	if err != nil {
		return application.OrderResult{}, false, err
	}

	dropoffAddr, err := extractAddress(dropoffMap, "dropoff")
	if err != nil {
		return application.OrderResult{}, false, err
	}

	// 4. Validate service level
	svcLevel := strings.ToUpper(strings.TrimSpace(req.ServiceLevel))
	if svcLevel != "" {
		if err := pricingdomain.ServiceLevel(svcLevel).Validate(); err != nil {
			return application.OrderResult{}, false, application.ValidationError{
				Field: "service_level",
				Issue: fmt.Sprintf("unsupported service level %q (must be STANDARD, EXPRESS, or SAME_DAY)", req.ServiceLevel),
			}
		}
	}

	// 5. Construct authoritative booking command
	cmd := parcelsapp.BookMerchantParcelCommand{
		MerchantID:     cleanMerchantID,
		ServiceLevel:   svcLevel,
		RecipientName:  dropoffAddr.ContactName,
		RecipientPhone: dropoffAddr.ContactPhone,
		WeightKg:       weightKG,
		Dimensions: parcelsapp.DimensionsDTO{
			LengthCm: length,
			WidthCm:  width,
			HeightCm: height,
		},
		PickupAddress:       pickupAddr,
		DropoffAddress:      dropoffAddr,
		ItemDescription:     parcelSpec.Description,
		SpecialInstructions: strings.TrimSpace(req.MerchantReference),
		CorrelationID:       strings.TrimSpace(req.MerchantReference),
	}

	if strings.TrimSpace(idemKey) != "" {
		cmd.Idempotency = &parcelsapp.IdempotencyInput{
			Key:         strings.TrimSpace(idemKey),
			RequestHash: strings.TrimSpace(requestHash),
			CallerScope: "MERCHANT:" + cleanMerchantID,
			Operation:   parcelsapp.DefaultMerchantBookingOperation,
		}
	}

	// 6. Execute authoritative parcel creation
	res, err := a.bookUC.Execute(ctx, cmd)
	if err != nil {
		if errors.Is(err, parcelsapp.ErrIdempotencyPayloadMismatch) {
			return application.OrderResult{}, false, application.ErrIdempotencyReused
		}
		if errors.Is(err, parcelsdomain.ErrInvalidWeight) {
			return application.OrderResult{}, false, application.ValidationError{
				Field: "parcel.weight_kg",
				Issue: err.Error(),
			}
		}
		if errors.Is(err, parcelsdomain.ErrInvalidDimensions) {
			return application.OrderResult{}, false, application.ValidationError{
				Field: "parcel.dimensions",
				Issue: err.Error(),
			}
		}
		if errors.Is(err, parcelsdomain.ErrInvalidAddress) {
			return application.OrderResult{}, false, application.ValidationError{
				Field: "address",
				Issue: err.Error(),
			}
		}
		if errors.Is(err, parcelsdomain.ErrInvalidZone) {
			return application.OrderResult{}, false, application.ValidationError{
				Field: "zone_id",
				Issue: err.Error(),
			}
		}
		if errors.Is(err, parcelsdomain.ErrInvalidServiceLevel) {
			return application.OrderResult{}, false, application.ValidationError{
				Field: "service_level",
				Issue: err.Error(),
			}
		}
		if errors.Is(err, pricingdomain.ErrTariffNotFound) {
			return application.OrderResult{}, false, application.ValidationError{
				Field: "zones",
				Issue: "no pricing tariff available for route",
			}
		}
		if errors.Is(err, parcelsdomain.ErrInvalidRecipient) {
			return application.OrderResult{}, false, application.ValidationError{
				Field: "dropoff",
				Issue: "recipient contact name and phone are required",
			}
		}
		if errors.Is(err, parcelsdomain.ErrInvalidMerchantID) {
			return application.OrderResult{}, false, application.ValidationError{
				Field: "merchant_id",
				Issue: "invalid merchant id",
			}
		}
		return application.OrderResult{}, false, err
	}

	// 7. Map to Member 4's OrderResult contract
	orderResult := application.OrderResult{
		ParcelID:          res.Parcel.ID,
		TrackingNumber:    string(res.Parcel.TrackingNumber),
		MerchantReference: req.MerchantReference,
		Status:            string(res.Parcel.Status),
		CreatedAt:         res.Parcel.CreatedAt.Format(time.RFC3339),
	}
	orderResult.Fee.Currency = res.Parcel.DeliveryFee.Currency
	orderResult.Fee.AmountMinor = res.Parcel.DeliveryFee.Amount

	return orderResult, res.Replayed, nil
}

// ListForMerchant retrieves all parcels created by the authenticated merchant tenant,
// ordered newest first, guaranteeing strict tenant isolation.
func (a *ParcelsBookingAdapter) ListForMerchant(ctx context.Context, merchantID string) ([]application.OrderResult, error) {
	if a.parcelRepo == nil {
		return nil, errors.New("parcels booking adapter: nil parcel repository")
	}

	cleanMerchantID := strings.TrimSpace(merchantID)
	if cleanMerchantID == "" || !events.IsValidUUID(cleanMerchantID) {
		return nil, application.ValidationError{
			Field: "merchant_id",
			Issue: "authenticated merchant ID is invalid or missing",
		}
	}

	parcels, err := a.parcelRepo.ListByMerchantID(ctx, nil, cleanMerchantID)
	if err != nil {
		return nil, fmt.Errorf("list parcels for merchant: %w", err)
	}

	out := make([]application.OrderResult, 0, len(parcels))
	for _, p := range parcels {
		item := application.OrderResult{
			ParcelID:          p.ID,
			TrackingNumber:    string(p.TrackingNumber),
			MerchantReference: p.SpecialInstructions,
			Status:            string(p.Status),
			CreatedAt:         p.CreatedAt.Format(time.RFC3339),
		}
		item.Fee.Currency = p.DeliveryFee.Currency
		item.Fee.AmountMinor = p.DeliveryFee.Amount
		out = append(out, item)
	}

	return out, nil
}

func extractAddress(m map[string]any, prefix string) (parcelsapp.AddressDTO, error) {
	if len(m) == 0 {
		return parcelsapp.AddressDTO{}, application.ValidationError{
			Field: prefix,
			Issue: fmt.Sprintf("%s address is required", prefix),
		}
	}

	getString := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k]; ok && v != nil {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
		if nested, ok := m["address"]; ok && nested != nil {
			if nm, ok := nested.(map[string]any); ok {
				for _, k := range keys {
					if v, ok := nm[k]; ok && v != nil {
						if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
							return strings.TrimSpace(s)
						}
					}
				}
			} else if sm, ok := nested.(map[string]string); ok {
				for _, k := range keys {
					if s, ok := sm[k]; ok && strings.TrimSpace(s) != "" {
						return strings.TrimSpace(s)
					}
				}
			}
		}
		return ""
	}

	contactName := getString("contact_name", "name")
	if contactName == "" {
		return parcelsapp.AddressDTO{}, application.ValidationError{
			Field: prefix + ".contact_name",
			Issue: "contact name is required",
		}
	}

	contactPhone := getString("contact_phone", "phone")
	if contactPhone == "" {
		return parcelsapp.AddressDTO{}, application.ValidationError{
			Field: prefix + ".contact_phone",
			Issue: "contact phone is required",
		}
	}

	streetLine1 := getString("street_line_1", "street_line1", "address_line1", "line1", "address")
	if streetLine1 == "" {
		if al, ok := m["address_lines"]; ok && al != nil {
			if arr, ok := al.([]any); ok && len(arr) > 0 {
				if s, ok := arr[0].(string); ok {
					streetLine1 = strings.TrimSpace(s)
				}
			} else if sarr, ok := al.([]string); ok && len(sarr) > 0 {
				streetLine1 = strings.TrimSpace(sarr[0])
			}
		}
	}
	if streetLine1 == "" {
		return parcelsapp.AddressDTO{}, application.ValidationError{
			Field: prefix + ".street_line_1",
			Issue: "street line 1 is required",
		}
	}

	streetLine2 := getString("street_line_2", "street_line2", "address_line2", "line2")
	if streetLine2 == "" {
		if al, ok := m["address_lines"]; ok && al != nil {
			if arr, ok := al.([]any); ok && len(arr) > 1 {
				if s, ok := arr[1].(string); ok {
					streetLine2 = strings.TrimSpace(s)
				}
			} else if sarr, ok := al.([]string); ok && len(sarr) > 1 {
				streetLine2 = strings.TrimSpace(sarr[1])
			}
		}
	}

	city := getString("city")
	if city == "" {
		return parcelsapp.AddressDTO{}, application.ValidationError{
			Field: prefix + ".city",
			Issue: "city is required",
		}
	}

	postalCode := getString("postal_code", "postalCode", "zip")
	if postalCode == "" {
		postalCode = "00100"
	}

	zoneID := getString("zone_id", "zoneId")
	if zoneID == "" {
		return parcelsapp.AddressDTO{}, application.ValidationError{
			Field: prefix + ".zone_id",
			Issue: "zone_id is required",
		}
	}
	if err := parcelsdomain.ValidateZoneID(zoneID); err != nil {
		return parcelsapp.AddressDTO{}, application.ValidationError{
			Field: prefix + ".zone_id",
			Issue: err.Error(),
		}
	}

	instructions := getString("instructions")

	return parcelsapp.AddressDTO{
		ContactName:  contactName,
		ContactPhone: contactPhone,
		StreetLine1:  streetLine1,
		StreetLine2:  streetLine2,
		City:         city,
		PostalCode:   postalCode,
		ZoneID:       zoneID,
		Instructions: instructions,
	}, nil
}
