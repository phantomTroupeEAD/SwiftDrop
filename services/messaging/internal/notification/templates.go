package notification

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	htmltmpl "html/template"
	"strings"
	texttmpl "text/template"
)

var (
	ErrUnknownMilestone = errors.New("notification: unsupported milestone event type")
)

// MilestoneData holds sanitized customer-facing information for template rendering.
type MilestoneData struct {
	RecipientName  string
	TrackingNumber string
	CourierName    string
	Reason         string
}

// ExtractMilestoneData extracts safe, customer-facing parameters from event payload JSON.
func ExtractMilestoneData(payloadBytes []byte) MilestoneData {
	var payload map[string]interface{}
	_ = json.Unmarshal(payloadBytes, &payload)

	var data MilestoneData

	if tn, ok := payload["tracking_number"].(string); ok {
		data.TrackingNumber = tn
	}

	if notify, ok := payload["notify"].(map[string]interface{}); ok {
		if name, ok := notify["recipient_name"].(string); ok {
			data.RecipientName = strings.TrimSpace(name)
		}
	}
	if data.RecipientName == "" {
		data.RecipientName = "Valued Customer"
	}

	if cn, ok := payload["courier_name"].(string); ok {
		data.CourierName = cn
	}

	if reason, ok := payload["failure_reason"].(string); ok {
		data.Reason = reason
	} else if rReason, ok := payload["return_reason"].(string); ok {
		data.Reason = rReason
	}

	return data
}

type templateDefinition struct {
	subject  string
	textTmpl string
	htmlTmpl string
}

var milestoneTemplates = map[string]templateDefinition{
	"parcel.booked.v1": {
		subject: "SwiftDrop: Your parcel booking is confirmed ({{.TrackingNumber}})",
		textTmpl: `Hello {{.RecipientName}},

Your parcel with tracking number {{.TrackingNumber}} has been successfully booked and is being prepared for dispatch.

You can track your delivery status at any time using your tracking number.

Thank you for choosing SwiftDrop!`,
		htmlTmpl: `<p>Hello <strong>{{.RecipientName}}</strong>,</p>
<p>Your parcel with tracking number <strong>{{.TrackingNumber}}</strong> has been successfully booked and is being prepared for dispatch.</p>
<p>Thank you for choosing SwiftDrop!</p>`,
	},
	"parcel.out_for_delivery.v1": {
		subject: "SwiftDrop: Your parcel is out for delivery ({{.TrackingNumber}})",
		textTmpl: `Hello {{.RecipientName}},

Great news! Your parcel {{.TrackingNumber}} is out for delivery{{if .CourierName}} with our courier {{.CourierName}}{{end}}.

Please ensure someone is available at the delivery address to receive the parcel.

SwiftDrop Delivery Team`,
		htmlTmpl: `<p>Hello <strong>{{.RecipientName}}</strong>,</p>
<p>Great news! Your parcel <strong>{{.TrackingNumber}}</strong> is out for delivery{{if .CourierName}} with our courier <strong>{{.CourierName}}</strong>{{end}}.</p>
<p>Please ensure someone is available at the delivery address to receive the parcel.</p>`,
	},
	"parcel.delivered.v1": {
		subject: "SwiftDrop: Your parcel has been delivered ({{.TrackingNumber}})",
		textTmpl: `Hello {{.RecipientName}},

Your parcel {{.TrackingNumber}} has been successfully delivered.

If you did not receive this parcel, please contact our support team immediately.

Thank you for using SwiftDrop!`,
		htmlTmpl: `<p>Hello <strong>{{.RecipientName}}</strong>,</p>
<p>Your parcel <strong>{{.TrackingNumber}}</strong> has been successfully delivered.</p>
<p>Thank you for using SwiftDrop!</p>`,
	},
	"parcel.delivery_failed.v1": {
		subject: "SwiftDrop: Delivery attempt failed for {{.TrackingNumber}}",
		textTmpl: `Hello {{.RecipientName}},

We attempted to deliver your parcel {{.TrackingNumber}}, but were unable to complete the delivery{{if .Reason}}: {{.Reason}}{{end}}.

Our dispatcher will reschedule your delivery shortly.

SwiftDrop Support Team`,
		htmlTmpl: `<p>Hello <strong>{{.RecipientName}}</strong>,</p>
<p>We attempted to deliver your parcel <strong>{{.TrackingNumber}}</strong>, but were unable to complete the delivery{{if .Reason}}: <em>{{.Reason}}</em>{{end}}.</p>
<p>Our dispatcher will reschedule your delivery shortly.</p>`,
	},
	"parcel.returned.v1": {
		subject: "SwiftDrop: Parcel returned to sender ({{.TrackingNumber}})",
		textTmpl: `Hello {{.RecipientName}},

Your parcel {{.TrackingNumber}} has been returned to the origin sender{{if .Reason}}: {{.Reason}}{{end}}.

Please contact the merchant for further details.

SwiftDrop Team`,
		htmlTmpl: `<p>Hello <strong>{{.RecipientName}}</strong>,</p>
<p>Your parcel <strong>{{.TrackingNumber}}</strong> has been returned to the origin sender{{if .Reason}}: <em>{{.Reason}}</em>{{end}}.</p>
<p>Please contact the merchant for further details.</p>`,
	},
}

// RenderNotification renders the subject, plain text body, and HTML body for a milestone event.
func RenderNotification(eventType string, data MilestoneData) (subject, textBody, htmlBody string, err error) {
	tmplDef, ok := milestoneTemplates[eventType]
	if !ok {
		return "", "", "", fmt.Errorf("%w: %s", ErrUnknownMilestone, eventType)
	}

	// Render Subject
	subjTmpl, err := texttmpl.New("subject").Parse(tmplDef.subject)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse subject template: %w", err)
	}
	var subjBuf bytes.Buffer
	if err := subjTmpl.Execute(&subjBuf, data); err != nil {
		return "", "", "", fmt.Errorf("failed to render subject: %w", err)
	}
	subject = strings.TrimSpace(subjBuf.String())

	// Render Plain Text
	tTmpl, err := texttmpl.New("text").Parse(tmplDef.textTmpl)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse text template: %w", err)
	}
	var textBuf bytes.Buffer
	if err := tTmpl.Execute(&textBuf, data); err != nil {
		return "", "", "", fmt.Errorf("failed to render text: %w", err)
	}
	textBody = textBuf.String()

	// Render HTML
	hTmpl, err := htmltmpl.New("html").Parse(tmplDef.htmlTmpl)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse html template: %w", err)
	}
	var htmlBuf bytes.Buffer
	if err := hTmpl.Execute(&htmlBuf, data); err != nil {
		return "", "", "", fmt.Errorf("failed to render html: %w", err)
	}
	htmlBody = htmlBuf.String()

	return subject, textBody, htmlBody, nil
}
