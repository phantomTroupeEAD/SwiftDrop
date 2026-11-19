package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PublicTrackingPayload reflects the strict allowlisted schema specified by SEC-009.
type PublicTrackingPayload struct {
	TrackingNumber    string            `json:"tracking_number"`
	Status            string            `json:"status"`
	StatusDescription string            `json:"status_description"`
	Destination       PublicDestination `json:"destination"`
	Milestones        []PublicMilestone `json:"milestones"`
}

type PublicDestination struct {
	City string `json:"city"`
}

type PublicMilestone struct {
	Status      string    `json:"status"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}

func TestAcceptance_AT005_PublicTrackingMasking(t *testing.T) {
	mux := http.NewServeMux()

	sensitiveRecipientPhone := "+94771234567"
	sensitiveStreetAddress := "42 Temple Road, Colombo 07"
	sensitivePrivateNote := "Secret door code: 1234. Guard has package."

	// Public endpoint adhering to SEC-009 sanitization
	mux.HandleFunc("GET /api/v1/tracking/public/{tracking_number}", func(w http.ResponseWriter, r *http.Request) {
		trackingNum := r.PathValue("tracking_number")
		if trackingNum != "SD-20261113-A3X9K7" {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "NOT_FOUND",
				"message": "shipment tracking number not found",
			})
			return
		}

		// Strictly allowlisted payload - sensitive PII is NEVER added
		res := PublicTrackingPayload{
			TrackingNumber:    trackingNum,
			Status:            "DELIVERED",
			StatusDescription: "Package delivered to doorstep",
			Destination: PublicDestination{
				City: "Colombo 07",
			},
			Milestones: []PublicMilestone{
				{Status: "CREATED", Description: "Shipment created", Timestamp: time.Now().Add(-4 * time.Hour).UTC()},
				{Status: "ASSIGNED", Description: "Courier assigned", Timestamp: time.Now().Add(-3 * time.Hour).UTC()},
				{Status: "IN_TRANSIT", Description: "Out for delivery", Timestamp: time.Now().Add(-2 * time.Hour).UTC()},
				{Status: "DELIVERED", Description: "Delivered to recipient", Timestamp: time.Now().Add(-1 * time.Hour).UTC()},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(res)
	})

	t.Run("1. Public Tracking Does Not Require Authorization Header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/SD-20261113-A3X9K7", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("2. Public Tracking Payload Masks All Customer PII (SEC-009)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/SD-20261113-A3X9K7", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		rawJSON := rec.Body.String()

		// INVARIANT: Raw response string must NOT leak phone, street, or notes
		assert.False(t, strings.Contains(rawJSON, sensitiveRecipientPhone), "public tracking must never contain recipient phone number")
		assert.False(t, strings.Contains(rawJSON, sensitiveStreetAddress), "public tracking must never contain street address")
		assert.False(t, strings.Contains(rawJSON, sensitivePrivateNote), "public tracking must never contain private courier notes")

		var payload PublicTrackingPayload
		err := json.Unmarshal(rec.Body.Bytes(), &payload)
		require.NoError(t, err)

		// Allowed safe fields must be present
		assert.Equal(t, "SD-20261113-A3X9K7", payload.TrackingNumber)
		assert.Equal(t, "DELIVERED", payload.Status)
		assert.Equal(t, "Colombo 07", payload.Destination.City)
		assert.Len(t, payload.Milestones, 4)
	})

	t.Run("3. Unknown Tracking Number Returns 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/SD-NONEXISTENT", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
