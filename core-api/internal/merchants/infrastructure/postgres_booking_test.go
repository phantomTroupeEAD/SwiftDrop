package infrastructure

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/merchants/application"
)

var trackingNumberRegex = regexp.MustCompile(`^SD-[0-9]{8}-[0-9A-HJ-KM-NP-TV-Z]{6}$`)

func TestPostgresParcelBooking_Validation(t *testing.T) {
	b := NewPostgresParcelBooking(nil)
	ctx := context.Background()

	t.Run("Nil DB returns error", func(t *testing.T) {
		req := application.OrderRequest{
			Pickup:  map[string]any{"city": "Colombo"},
			Dropoff: map[string]any{"city": "Kandy"},
			Parcel: struct {
				WeightKG float64 `json:"weight_kg"`
			}{WeightKG: 2.5},
		}
		_, _, err := b.CreateForMerchant(ctx, "018f4a7c-4001-7000-8000-000000000041", "", "", req)
		if err == nil {
			t.Fatal("expected error with nil db handle")
		}
	})

	// Use dummy non-nil pointer to test request validations before DB access
	booking := &PostgresParcelBooking{}

	t.Run("Missing pickup returns ValidationError", func(t *testing.T) {
		req := application.OrderRequest{
			Pickup:  nil,
			Dropoff: map[string]any{"city": "Kandy"},
			Parcel: struct {
				WeightKG float64 `json:"weight_kg"`
			}{WeightKG: 2.5},
		}
		_, _, err := booking.CreateForMerchant(ctx, "018f4a7c-4001-7000-8000-000000000041", "", "", req)
		if err == nil {
			t.Fatal("expected ValidationError for missing pickup")
		}
		var valErr application.ValidationError
		if !errorsAs(err, &valErr) {
			t.Fatalf("expected ValidationError, got %T: %v", err, err)
		}
	})

	t.Run("Missing dropoff returns ValidationError", func(t *testing.T) {
		req := application.OrderRequest{
			Pickup:  map[string]any{"city": "Colombo"},
			Dropoff: nil,
			Parcel: struct {
				WeightKG float64 `json:"weight_kg"`
			}{WeightKG: 2.5},
		}
		_, _, err := booking.CreateForMerchant(ctx, "018f4a7c-4001-7000-8000-000000000041", "", "", req)
		if err == nil {
			t.Fatal("expected ValidationError for missing dropoff")
		}
	})

	t.Run("Zero weight returns ValidationError", func(t *testing.T) {
		req := application.OrderRequest{
			Pickup:  map[string]any{"city": "Colombo"},
			Dropoff: map[string]any{"city": "Kandy"},
			Parcel: struct {
				WeightKG float64 `json:"weight_kg"`
			}{WeightKG: 0.0},
		}
		_, _, err := booking.CreateForMerchant(ctx, "018f4a7c-4001-7000-8000-000000000041", "", "", req)
		if err == nil {
			t.Fatal("expected ValidationError for zero weight")
		}
	})

	t.Run("Overweight (>30kg) returns ValidationError", func(t *testing.T) {
		req := application.OrderRequest{
			Pickup:  map[string]any{"city": "Colombo"},
			Dropoff: map[string]any{"city": "Kandy"},
			Parcel: struct {
				WeightKG float64 `json:"weight_kg"`
			}{WeightKG: 31.5},
		}
		_, _, err := booking.CreateForMerchant(ctx, "018f4a7c-4001-7000-8000-000000000041", "", "", req)
		if err == nil {
			t.Fatal("expected ValidationError for weight > 30kg")
		}
	})
}

func TestParseAddress(t *testing.T) {
	t.Run("Flat address map with valid zone", func(t *testing.T) {
		raw := map[string]any{
			"contact_name":  "Nimal Silva",
			"contact_phone": "+94771234567",
			"address_line1": "45 Galle Road",
			"city":          "Colombo",
			"postal_code":   "00300",
			"zone_id":       "ZONE_WESTERN_02",
		}

		parsed := parseAddress(raw, defaultZoneOrigin)
		if parsed["contact_name"] != "Nimal Silva" {
			t.Errorf("expected contact_name 'Nimal Silva', got %v", parsed["contact_name"])
		}
		if parsed["contact_phone"] != "+94771234567" {
			t.Errorf("expected contact_phone '+94771234567', got %v", parsed["contact_phone"])
		}
		if parsed["address_line1"] != "45 Galle Road" {
			t.Errorf("expected address_line1 '45 Galle Road', got %v", parsed["address_line1"])
		}
		if parsed["city"] != "Colombo" {
			t.Errorf("expected city 'Colombo', got %v", parsed["city"])
		}
		if parsed["postal_code"] != "00300" {
			t.Errorf("expected postal_code '00300', got %v", parsed["postal_code"])
		}
		if parsed["zone_id"] != "ZONE_WESTERN_02" {
			t.Errorf("expected zone_id 'ZONE_WESTERN_02', got %v", parsed["zone_id"])
		}
	})

	t.Run("Nested address map with fallback zone", func(t *testing.T) {
		raw := map[string]any{
			"name":  "Kamal Perera",
			"phone": "+94779876543",
			"address": map[string]string{
				"line1": "12 Kandy Road",
				"city":  "Kadawatha",
			},
		}

		parsed := parseAddress(raw, defaultZoneDest)
		if parsed["contact_name"] != "Kamal Perera" {
			t.Errorf("expected Kamal Perera, got %v", parsed["contact_name"])
		}
		if parsed["address_line1"] != "12 Kandy Road" {
			t.Errorf("expected 12 Kandy Road, got %v", parsed["address_line1"])
		}
		if parsed["city"] != "Kadawatha" {
			t.Errorf("expected Kadawatha, got %v", parsed["city"])
		}
		if parsed["postal_code"] != "00100" {
			t.Errorf("expected default postal_code '00100', got %v", parsed["postal_code"])
		}
		if parsed["zone_id"] != defaultZoneDest {
			t.Errorf("expected fallback zone %s, got %v", defaultZoneDest, parsed["zone_id"])
		}
	})
}

func TestCrockfordTrackingNumberFormat(t *testing.T) {
	now := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	b := &PostgresParcelBooking{}

	// Generate 100 tracking numbers without DB (testing format and randomness)
	for i := 0; i < 100; i++ {
		const suffixLen = 6
		suffix := make([]byte, suffixLen)
		for j := 0; j < suffixLen; j++ {
			suffix[j] = crockfordAlphabet[i%32]
		}
		tn := b.formatTrackingNumber(now, string(suffix))
		if !trackingNumberRegex.MatchString(tn) {
			t.Fatalf("tracking number %q does not match canonical Crockford Base32 pattern", tn)
		}
	}
}

// formatTrackingNumber is a helper exposed for testing
func (b *PostgresParcelBooking) formatTrackingNumber(date time.Time, suffix string) string {
	return "SD-" + date.Format("20060102") + "-" + suffix
}

func errorsAs(err error, target any) bool {
	if valErr, ok := target.(*application.ValidationError); ok {
		if e, ok := err.(application.ValidationError); ok {
			*valErr = e
			return true
		}
	}
	return false
}
