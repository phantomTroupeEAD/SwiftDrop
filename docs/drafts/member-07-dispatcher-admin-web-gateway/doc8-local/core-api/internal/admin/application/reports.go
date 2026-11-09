package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"swiftdrop.local/m7/core-api/internal/platform/apperr"
)

// ReportReader reads delivery numbers from the parcels and delivery modules through
// their own read interfaces (Doc 3 DD-08). The admin module never runs SQL on
// their tables.
type ReportReader interface {
	Deliveries(ctx context.Context, from, to time.Time) ([]DeliveryDay, error)
}

type DeliveryDay struct {
	Date      string `json:"date"` // Asia/Colombo day, YYYY-MM-DD
	Delivered int    `json:"delivered"`
	Failed    int    `json:"failed"`
	Returned  int    `json:"returned"`
}

type DeliveryReport struct {
	From        string        `json:"from"`
	To          string        `json:"to"`
	Days        []DeliveryDay `json:"days"`
	Delivered   int           `json:"delivered"`
	Failed      int           `json:"failed"`
	SuccessRate *float64      `json:"success_rate"` // null when there were no attempts
	Source      string        `json:"source"`
}

// NoDeliveryData stands in until Members 2 and 3 publish their read interfaces.
// It returns empty days and says so in "source", so nobody mistakes it for real data.
type NoDeliveryData struct{}

func (NoDeliveryData) Deliveries(context.Context, time.Time, time.Time) ([]DeliveryDay, error) {
	return nil, nil
}

var colombo = time.FixedZone("Asia/Colombo", 5*3600+1800)

// DeliveryReport serves GET /api/v1/admin/reports/deliveries?from=&to= (FR-ADM-008).
// Dates are Sri Lanka days; the range may not be longer than reports.max_range_days.
func (s *Service) DeliveryReport(ctx context.Context, r ReportReader, source, fromS, toS string) (DeliveryReport, error) {
	from, err1 := time.ParseInLocation("2006-01-02", fromS, colombo)
	to, err2 := time.ParseInLocation("2006-01-02", toS, colombo)
	if err1 != nil || err2 != nil {
		return DeliveryReport{}, fmt.Errorf("%w: from and to must be dates like 2026-10-01", apperr.ErrValidation)
	}
	if to.Before(from) {
		return DeliveryReport{}, fmt.Errorf("%w: to is before from", apperr.ErrValidation)
	}
	maxDays := 92
	if set, err := s.Store.ListSettings(ctx); err == nil {
		for _, x := range set {
			if x.Key == "reports.max_range_days" {
				_ = json.Unmarshal(x.Value, &maxDays)
			}
		}
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > maxDays {
		return DeliveryReport{}, fmt.Errorf("%w: range is %d days, limit is %d", apperr.ErrValidation, days, maxDays)
	}
	rows, err := r.Deliveries(ctx, from.UTC(), to.Add(24*time.Hour).UTC())
	if err != nil {
		return DeliveryReport{}, err
	}
	out := DeliveryReport{From: fromS, To: toS, Days: rows, Source: source}
	if out.Days == nil {
		out.Days = []DeliveryDay{}
	}
	for _, d := range rows {
		out.Delivered += d.Delivered
		out.Failed += d.Failed
	}
	if n := out.Delivered + out.Failed; n > 0 {
		rate := float64(out.Delivered) / float64(n)
		out.SuccessRate = &rate
	}
	return out, nil
}
