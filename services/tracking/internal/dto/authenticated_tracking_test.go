package dto

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAuthenticatedTrackingResponse_JSONSerialization(t *testing.T) {
	ts := time.Date(2026, 11, 10, 14, 30, 0, 0, time.UTC)
	resp := AuthenticatedTrackingResponse{
		TrackingNumber:    "SD-20261025-A8F3K2",
		ParcelID:          "7b8f9e2d-3c4a-4e5b-9d1a-8f2c3e4b5a6c",
		Status:            "ATTEMPT_FAILED",
		StatusDescription: "Delivery attempt unsuccessful. Reattempt scheduled.",
		Destination: AuthenticatedDestination{
			City:       "Colombo",
			PostalCode: "00300",
			ZoneID:     "ZONE_COLOMBO_03",
		},
		Milestones: []AuthenticatedMilestone{
			{
				Status:      "CREATED",
				Description: "Shipment booked and delivery label generated.",
				ActorRole:   "MERCHANT",
				Timestamp:   ts.Add(-6 * time.Hour),
			},
			{
				Status:      "ATTEMPT_FAILED",
				Description: "Customer unavailable at delivery premises.",
				ActorRole:   "COURIER",
				ReasonCode:  "RECIPIENT_UNAVAILABLE",
				Timestamp:   ts,
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	jsonStr := string(data)

	// Verify required snake_case keys are present
	requiredKeys := []string{
		`"tracking_number":"SD-20261025-A8F3K2"`,
		`"parcel_id":"7b8f9e2d-3c4a-4e5b-9d1a-8f2c3e4b5a6c"`,
		`"status":"ATTEMPT_FAILED"`,
		`"status_description":"Delivery attempt unsuccessful. Reattempt scheduled."`,
		`"destination":{`,
		`"city":"Colombo"`,
		`"postal_code":"00300"`,
		`"zone_id":"ZONE_COLOMBO_03"`,
		`"milestones":[`,
		`"actor_role":"MERCHANT"`,
		`"reason_code":"RECIPIENT_UNAVAILABLE"`,
	}

	for _, key := range requiredKeys {
		if !strings.Contains(jsonStr, key) {
			t.Errorf("expected JSON to contain %s, got: %s", key, jsonStr)
		}
	}

	// Verify sensitive fields (PII) are NEVER serialized
	prohibitedFields := []string{
		"recipient_phone",
		"recipient_name",
		"street_address",
		"password",
		"secret",
	}

	for _, prohibited := range prohibitedFields {
		if strings.Contains(jsonStr, prohibited) {
			t.Errorf("prohibited field %q detected in serialized authenticated response: %s", prohibited, jsonStr)
		}
	}
}
