package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"time"

	"swiftdrop.local/m4/core-api/internal/merchants/application"
)

// BookingStub is a STAND-IN for Member 2's parcel booking (Doc 4 §5.1) so the merchant
// order flow can be demonstrated. It keeps parcels and idempotency records in memory,
// does only minimal validation, and is replaced by the real parcels adapter at integration.
type BookingStub struct {
	mu      sync.Mutex
	seq     int
	byKey   map[string]stubEntry // merchantID|idemKey
	byMerch map[string][]application.OrderResult
}

type stubEntry struct {
	hash   string
	result application.OrderResult
}

func NewBookingStub() *BookingStub {
	return &BookingStub{byKey: map[string]stubEntry{}, byMerch: map[string][]application.OrderResult{}}
}

func (b *BookingStub) CreateForMerchant(_ context.Context, merchantID, idemKey, hash string, req application.OrderRequest) (application.OrderResult, bool, error) {
	if req.Pickup == nil || req.Dropoff == nil || req.Parcel.WeightKG <= 0 {
		return application.OrderResult{}, false, application.ValidationError{Field: "parcel", Issue: "pickup, dropoff and parcel.weight_kg > 0 are required (stand-in validation)"}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	k := merchantID + "|" + idemKey // idempotency scope MERCHANT:<id> (BR-O-03)
	if e, ok := b.byKey[k]; ok {
		if e.hash != hash {
			return application.OrderResult{}, false, application.ErrIdempotencyReused
		}
		return e.result, true, nil
	}
	b.seq++
	now := time.Now().UTC()
	r := application.OrderResult{
		ParcelID:          fmt.Sprintf("prcl_stub_%06d", b.seq),
		TrackingNumber:    fmt.Sprintf("SD-%s-%05d", now.Format("20060102"), b.seq),
		MerchantReference: req.MerchantReference,
		Status:            "CREATED",
		CreatedAt:         now.Format(time.RFC3339),
	}
	r.Fee.Currency, r.Fee.AmountMinor = "LKR", 65000
	b.byKey[k] = stubEntry{hash: hash, result: r}
	b.byMerch[merchantID] = append(b.byMerch[merchantID], r)
	return r, false, nil
}

func (b *BookingStub) ListForMerchant(_ context.Context, merchantID string) ([]application.OrderResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := append([]application.OrderResult{}, b.byMerch[merchantID]...)
	return out, nil
}
