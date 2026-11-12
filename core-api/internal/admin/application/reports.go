package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ReportReader reads delivery numbers from the parcels and delivery modules through
// their own read interface (Doc 3 DD-08). The admin module never runs SQL on their
// tables. Until Members 2 and 3 publish that interface, NoDeliveryData stands in.
type ReportReader interface {
	// Deliveries returns one row per Sri Lanka day in [from, to). from and to are UTC
	// instants that mark the start of a Colombo day.
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

// NoDeliveryDataSource is the "source" text that goes with NoDeliveryData.
const NoDeliveryDataSource = "stand-in: no parcel data until Members 2 and 3 connect"

const (
	defaultMaxReportDays = 92
	maxRangeDaysKey      = "reports.max_range_days"
)

var colombo = time.FixedZone("Asia/Colombo", 5*3600+1800)

// DeliveryReport serves GET /api/v1/admin/reports/deliveries?from=&to= (FR-ADM-008).
// Dates are Sri Lanka days; the range may not be longer than reports.max_range_days.
// It only reads, so it writes no audit row.
func (s *Service) DeliveryReport(ctx context.Context, r ReportReader, source, fromS, toS string) (DeliveryReport, error) {
	from, err1 := time.ParseInLocation("2006-01-02", fromS, colombo)
	to, err2 := time.ParseInLocation("2006-01-02", toS, colombo)
	if err1 != nil || err2 != nil {
		return DeliveryReport{}, fmt.Errorf("%w: from and to must be dates like 2026-10-01", ErrValidation)
	}
	if to.Before(from) {
		return DeliveryReport{}, fmt.Errorf("%w: to is before from", ErrValidation)
	}
	maxDays := s.maxReportDays(ctx)
	if days := int(to.Sub(from).Hours()/24) + 1; days > maxDays {
		return DeliveryReport{}, fmt.Errorf("%w: range is %d days, limit is %d", ErrValidation, days, maxDays)
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

// maxReportDays reads the live setting. If it cannot be read or is not a sane number,
// the built-in default applies, so a settings problem never blocks reports.
func (s *Service) maxReportDays(ctx context.Context) int {
	set, err := s.Store.ListSettings(ctx)
	if err != nil {
		return defaultMaxReportDays
	}
	for _, x := range set {
		if x.Key != maxRangeDaysKey {
			continue
		}
		var n int
		if json.Unmarshal(x.Value, &n) == nil && n >= 1 {
			return n
		}
	}
	return defaultMaxReportDays
}
