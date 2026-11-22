package application

import (
	"context"

	"github.com/swiftdrop/core-api/internal/merchants/domain"
)

// DimensionsRequest specifies parcel physical dimensions in centimeters.
type DimensionsRequest struct {
	Length   float64 `json:"length,omitempty"`
	Width    float64 `json:"width,omitempty"`
	Height   float64 `json:"height,omitempty"`
	LengthCM float64 `json:"length_cm,omitempty"`
	WidthCM  float64 `json:"width_cm,omitempty"`
	HeightCM float64 `json:"height_cm,omitempty"`
}

// ParcelRequest carries the physical properties of the parcel.
type ParcelRequest struct {
	WeightKG     float64            `json:"weight_kg"`
	WeightGrams  float64            `json:"weight_grams,omitempty"`
	LengthCM     float64            `json:"length_cm,omitempty"`
	WidthCM      float64            `json:"width_cm,omitempty"`
	HeightCM     float64            `json:"height_cm,omitempty"`
	DimensionsCM *DimensionsRequest `json:"dimensions_cm,omitempty"`
	Dimensions   *DimensionsRequest `json:"dimensions,omitempty"`
	Description  string             `json:"description,omitempty"`
}

// ExtractDimensions resolves parcel length, width, and height in cm across supported contract structures.
func (p ParcelRequest) ExtractDimensions() (length, width, height float64) {
	if p.LengthCM > 0 && p.WidthCM > 0 && p.HeightCM > 0 {
		return p.LengthCM, p.WidthCM, p.HeightCM
	}
	if p.DimensionsCM != nil {
		l := p.DimensionsCM.Length
		if l <= 0 {
			l = p.DimensionsCM.LengthCM
		}
		w := p.DimensionsCM.Width
		if w <= 0 {
			w = p.DimensionsCM.WidthCM
		}
		h := p.DimensionsCM.Height
		if h <= 0 {
			h = p.DimensionsCM.HeightCM
		}
		if l > 0 && w > 0 && h > 0 {
			return l, w, h
		}
	}
	if p.Dimensions != nil {
		l := p.Dimensions.LengthCM
		if l <= 0 {
			l = p.Dimensions.Length
		}
		w := p.Dimensions.WidthCM
		if w <= 0 {
			w = p.Dimensions.Width
		}
		h := p.Dimensions.HeightCM
		if h <= 0 {
			h = p.Dimensions.Height
		}
		if l > 0 && w > 0 && h > 0 {
			return l, w, h
		}
	}
	return 0, 0, 0
}

// ExtractWeightKG resolves parcel weight in kg.
func (p ParcelRequest) ExtractWeightKG() float64 {
	if p.WeightKG > 0 {
		return p.WeightKG
	}
	if p.WeightGrams > 0 {
		return p.WeightGrams / 1000.0
	}
	return 0
}

// OrderRequest mirrors Member 2's booking payload (Doc 4 §5.1). Member 2 owns validation.
type OrderRequest struct {
	MerchantReference string         `json:"merchant_reference,omitempty"`
	ServiceLevel      string         `json:"service_level,omitempty"`
	Pickup            map[string]any `json:"pickup"`
	Dropoff           map[string]any `json:"dropoff"`
	Recipient         map[string]any `json:"recipient,omitempty"`
	Parcel            ParcelRequest  `json:"parcel"`
	Package           *ParcelRequest `json:"package,omitempty"`
}

type OrderResult struct {
	ParcelID          string `json:"parcel_id"`
	TrackingNumber    string `json:"tracking_number"`
	MerchantReference string `json:"merchant_reference,omitempty"`
	Status            string `json:"status"`
	Fee               struct {
		Currency    string `json:"currency"`
		AmountMinor int64  `json:"amount_minor"`
	} `json:"fee"`
	CreatedAt string `json:"created_at"`
}

// CreateOrder implements F-07: M4 authenticates and scopes; M2 creates the parcel.
func (s *Service) CreateOrder(ctx context.Context, p MerchantPrincipal, idemKey, requestHash string, req OrderRequest) (OrderResult, bool, error) {
	if !p.Scopes.Has(domain.ScopeParcelsCreate) {
		return OrderResult{}, false, ErrScopeDenied
	}
	return s.Booking.CreateForMerchant(ctx, p.MerchantID, idemKey, requestHash, req)
}

func (s *Service) ListOrders(ctx context.Context, p MerchantPrincipal) ([]OrderResult, error) {
	if !p.Scopes.Has(domain.ScopeParcelsRead) {
		return nil, ErrScopeDenied
	}
	return s.Booking.ListForMerchant(ctx, p.MerchantID)
}
