package application

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

// fakeReader records the window it was asked for and returns canned days.
type fakeReader struct {
	days     []DeliveryDay
	err      error
	from, to time.Time
	calls    int
}

func (f *fakeReader) Deliveries(_ context.Context, from, to time.Time) ([]DeliveryDay, error) {
	f.calls++
	f.from, f.to = from, to
	return f.days, f.err
}

func TestDeliveryReportTotalsAndRate(t *testing.T) {
	r := &fakeReader{days: []DeliveryDay{
		{Date: "2026-10-01", Delivered: 8, Failed: 2, Returned: 1},
		{Date: "2026-10-02", Delivered: 10, Failed: 0},
	}}
	svc := &Service{Store: newMemStore()}
	got, err := svc.DeliveryReport(context.Background(), r, "test source", "2026-10-01", "2026-10-02")
	if err != nil {
		t.Fatal(err)
	}
	if got.Delivered != 18 || got.Failed != 2 || got.Source != "test source" || len(got.Days) != 2 {
		t.Fatalf("report: %+v", got)
	}
	if got.SuccessRate == nil || math.Abs(*got.SuccessRate-0.9) > 1e-9 {
		t.Fatalf("success rate: %v", got.SuccessRate)
	}
}

// The reader is asked for Sri Lanka days: midnight Colombo (UTC+5:30) is 18:30 UTC the day before.
func TestDeliveryReportAsksForColomboDays(t *testing.T) {
	r := &fakeReader{}
	svc := &Service{Store: newMemStore()}
	if _, err := svc.DeliveryReport(context.Background(), r, "s", "2026-10-01", "2026-10-03"); err != nil {
		t.Fatal(err)
	}
	wantFrom := time.Date(2026, 9, 30, 18, 30, 0, 0, time.UTC)
	wantTo := time.Date(2026, 10, 3, 18, 30, 0, 0, time.UTC)
	if !r.from.Equal(wantFrom) || !r.to.Equal(wantTo) {
		t.Fatalf("window [%v, %v), want [%v, %v)", r.from, r.to, wantFrom, wantTo)
	}
}

func TestDeliveryReportNoAttemptsHasNullRateAndEmptyDays(t *testing.T) {
	svc := &Service{Store: newMemStore()}
	got, err := svc.DeliveryReport(context.Background(), NoDeliveryData{}, NoDeliveryDataSource, "2026-10-01", "2026-10-03")
	if err != nil {
		t.Fatal(err)
	}
	if got.SuccessRate != nil || got.Days == nil || len(got.Days) != 0 || got.Source != NoDeliveryDataSource {
		t.Fatalf("stand-in report: %+v", got)
	}
	b, _ := json.Marshal(got)
	if !strings.Contains(string(b), `"days":[]`) || !strings.Contains(string(b), `"success_rate":null`) {
		t.Fatalf("json must show an empty days array and a null rate: %s", b)
	}
}

func TestDeliveryReportRejectsBadRanges(t *testing.T) {
	svc := &Service{Store: newMemStore()}
	for name, c := range map[string][2]string{
		"empty from":      {"", "2026-10-02"},
		"bad format":      {"01-10-2026", "2026-10-02"},
		"not a date":      {"2026-13-40", "2026-10-02"},
		"to before from":  {"2026-10-05", "2026-10-01"},
		"over 92 days":    {"2026-01-01", "2026-04-05"},
		"nonsense dates":  {"abc", "def"},
		"missing to date": {"2026-10-01", ""},
	} {
		r := &fakeReader{}
		_, err := svc.DeliveryReport(context.Background(), r, "s", c[0], c[1])
		if !errors.Is(err, ErrValidation) {
			t.Errorf("%s: got %v, want ErrValidation", name, err)
		}
		if r.calls != 0 {
			t.Errorf("%s: reader must not be called for a bad range", name)
		}
	}
}

func TestDeliveryReportRangeLimitFollowsTheSetting(t *testing.T) {
	st := newMemStore()
	st.state.settings["reports.max_range_days"] = Setting{Key: "reports.max_range_days", Value: json.RawMessage(`3`), Version: 1}
	svc := &Service{Store: st}
	if _, err := svc.DeliveryReport(context.Background(), &fakeReader{}, "s", "2026-10-01", "2026-10-03"); err != nil {
		t.Fatalf("3 days with limit 3: %v", err)
	}
	if _, err := svc.DeliveryReport(context.Background(), &fakeReader{}, "s", "2026-10-01", "2026-10-04"); !errors.Is(err, ErrValidation) {
		t.Fatalf("4 days with limit 3: got %v", err)
	}
}

func TestDeliveryReportFallsBackToDefaultLimit(t *testing.T) {
	st := newMemStore()
	st.settingsErr = errors.New("settings unreadable")
	svc := &Service{Store: st}
	if _, err := svc.DeliveryReport(context.Background(), &fakeReader{}, "s", "2026-10-01", "2026-12-31"); err != nil {
		t.Fatalf("92 days with unreadable settings must pass: %v", err)
	}
	if _, err := svc.DeliveryReport(context.Background(), &fakeReader{}, "s", "2026-10-01", "2027-01-05"); !errors.Is(err, ErrValidation) {
		t.Fatalf("97 days must be refused by the built-in limit: %v", err)
	}
}

func TestDeliveryReportPassesReaderErrorsThrough(t *testing.T) {
	boom := errors.New("reader down")
	svc := &Service{Store: newMemStore()}
	if _, err := svc.DeliveryReport(context.Background(), &fakeReader{err: boom}, "s", "2026-10-01", "2026-10-02"); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
}
