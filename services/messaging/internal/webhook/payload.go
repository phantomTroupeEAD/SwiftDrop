package webhook

import (
	"encoding/json"
	"strings"
	"time"
)

// WebhookBody represents the structured JSON payload sent to a merchant's HTTP endpoint.
type WebhookBody struct {
	EventID        string                 `json:"event_id"`
	EventType      string                 `json:"event_type"`
	AggregateID    string                 `json:"aggregate_id"`
	ParcelID       string                 `json:"parcel_id,omitempty"`
	TrackingNumber string                 `json:"tracking_number,omitempty"`
	ParcelVersion  int                    `json:"parcel_version,omitempty"`
	Status         string                 `json:"status"`
	OccurredAt     string                 `json:"occurred_at"`
	Data           map[string]interface{} `json:"data"`
	Payload        map[string]interface{} `json:"payload"`
}

// BuildWebhookBody constructs and sanitizes the outbound webhook JSON body.
// Guarantees:
// 1. notify block stripped
// 2. courier phone stripped
// 3. raw s3:// URLs stripped
// 4. private delivery notes stripped
func BuildWebhookBody(eventID, eventType, aggregateID string, aggregateVersion int, occurredAt time.Time, rawPayload []byte) ([]byte, error) {
	var payloadMap map[string]interface{}
	if err := json.Unmarshal(rawPayload, &payloadMap); err != nil {
		payloadMap = make(map[string]interface{})
	}

	// Sanitize sensitive fields
	sanitized := make(map[string]interface{}, len(payloadMap))
	for k, v := range payloadMap {
		kLower := strings.ToLower(k)
		// Exclude notify, courier_phone, dispatch_notes, s3 urls
		if kLower == "notify" || strings.Contains(kLower, "phone") || strings.Contains(kLower, "notes") {
			continue
		}
		if strVal, ok := v.(string); ok && strings.HasPrefix(strVal, "s3://") {
			continue
		}
		sanitized[k] = v
	}

	status := ""
	if s, ok := sanitized["status"].(string); ok {
		status = s
	}

	trackingNumber := ""
	if tn, ok := sanitized["tracking_number"].(string); ok {
		trackingNumber = tn
	}

	body := WebhookBody{
		EventID:        eventID,
		EventType:      eventType,
		AggregateID:    aggregateID,
		ParcelID:       aggregateID,
		TrackingNumber: trackingNumber,
		ParcelVersion:  aggregateVersion,
		Status:         status,
		OccurredAt:     occurredAt.UTC().Format(time.RFC3339),
		Data:           sanitized,
		Payload:        sanitized,
	}

	return json.Marshal(body)
}
