package dto

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPublicTrackingResponse_SEC009_PrivacyRedaction(t *testing.T) {
	now := time.Date(2026, 11, 6, 14, 30, 0, 0, time.UTC)

	resp := PublicTrackingResponse{
		TrackingNumber:    "SD-20261106-A8F3K2",
		Status:            "OUT_FOR_DELIVERY",
		StatusDescription: "Courier is out for final doorstep delivery.",
		Destination: PublicDestination{
			City: "Kandy",
		},
		EstimatedDelivery: nil, // Must remain omitted
		Milestones: []PublicMilestone{
			{
				Status:      "CREATED",
				Description: "Shipment booked and delivery label generated.",
				Timestamp:   now.Add(-2 * time.Hour),
			},
			{
				Status:      "OUT_FOR_DELIVERY",
				Description: "Courier is out for final doorstep delivery.",
				Timestamp:   now,
			},
		},
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	jsonStr := string(bytes)

	// List of forbidden field names and sensitive markers that must NEVER appear in public JSON
	forbiddenSubstrings := []string{
		"sender_id",
		"customer_id",
		"merchant_id",
		"recipient_name",
		"recipient_phone",
		"phone",
		"street",
		"street_line1",
		"street_line2",
		"pickup_address",
		"dropoff_address",
		"postal_code",
		"zone_id",
		"origin_zone_id",
		"destination_zone_id",
		"courier_notes",
		"internal_notes",
		"reason",
		"reason_code",
		"actor_id",
		"actor_role",
		"proof_id",
		"proof_photo_url",
		"delivery_fee",
		"delivery_fee_cents",
		"fee_cents",
		"currency",
		"chargeable_weight",
		"pricing_rule",
	}

	for _, forbidden := range forbiddenSubstrings {
		if strings.Contains(strings.ToLower(jsonStr), `"`+forbidden+`"`) {
			t.Errorf("SEC-009 VIOLATION: public JSON payload leaked sensitive field %q: %s", forbidden, jsonStr)
		}
	}

	// Verify required fields are present
	requiredKeys := []string{
		`"tracking_number":"SD-20261106-A8F3K2"`,
		`"status":"OUT_FOR_DELIVERY"`,
		`"status_description":"Courier is out for final doorstep delivery."`,
		`"destination":{"city":"Kandy"}`,
		`"milestones":[`,
	}

	for _, req := range requiredKeys {
		if !strings.Contains(jsonStr, req) {
			t.Errorf("expected public JSON payload to contain %s, got: %s", req, jsonStr)
		}
	}

	// Verify estimated_delivery is omitted when nil
	if strings.Contains(jsonStr, "estimated_delivery") {
		t.Errorf("expected estimated_delivery to be omitted when nil, got: %s", jsonStr)
	}
}
