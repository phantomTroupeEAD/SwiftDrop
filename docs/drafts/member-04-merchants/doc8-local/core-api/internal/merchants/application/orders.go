package application

import (
	"context"

	"swiftdrop.local/m4/core-api/internal/merchants/domain"
)

// OrderRequest mirrors Member 2's booking payload (Doc 4 §5.1). Member 2 owns validation.
type OrderRequest struct {
	MerchantReference string         `json:"merchant_reference,omitempty"`
	ServiceLevel      string         `json:"service_level,omitempty"`
	Pickup            map[string]any `json:"pickup"`
	Dropoff           map[string]any `json:"dropoff"`
	Parcel            struct {
		WeightKG float64 `json:"weight_kg"`
	} `json:"parcel"`
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
