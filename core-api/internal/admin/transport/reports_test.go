package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/admin/application"
	"github.com/swiftdrop/core-api/internal/platform/middleware"
)

const reportPath = "/api/v1/admin/reports/deliveries"

type stubReader struct {
	days []application.DeliveryDay
	err  error
	n    int
}

func (s *stubReader) Deliveries(context.Context, time.Time, time.Time) ([]application.DeliveryDay, error) {
	s.n++
	return s.days, s.err
}

func reportMux(st *stubStore, r application.ReportReader, source string) http.Handler {
	h := &Handlers{Svc: &application.Service{Store: st}, Auth: fakeAuth{}, Reports: r, ReportSource: source}
	mux := http.NewServeMux()
	h.Register(mux)
	return middleware.RequestID(mux)
}

// R-16: ADMIN and DISPATCHER may read the report; no token is 401; every other role is 403.
func TestReportRoleChecks(t *testing.T) {
	mux := reportMux(&stubStore{}, &stubReader{}, "s")
	url := reportPath + "?from=2026-10-01&to=2026-10-03"
	if rec := do(t, mux, "GET", url, "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: got %d, want 401", rec.Code)
	}
	for _, role := range []string{"COURIER", "MERCHANT", "CUSTOMER"} {
		if rec := do(t, mux, "GET", url, role, ""); rec.Code != http.StatusForbidden {
			t.Errorf("%s: got %d, want 403", role, rec.Code)
		}
	}
	for _, role := range []string{"ADMIN", "DISPATCHER"} {
		if rec := do(t, mux, "GET", url, role, ""); rec.Code != http.StatusOK {
			t.Errorf("%s: got %d, want 200: %s", role, rec.Code, rec.Body.String())
		}
	}
}

func TestReportOnlyAllowsGet(t *testing.T) {
	mux := reportMux(&stubStore{}, &stubReader{}, "s")
	for _, m := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		if rec := do(t, mux, m, reportPath+"?from=2026-10-01&to=2026-10-03", "ADMIN", "{}"); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: got %d, want 405", m, rec.Code)
		}
	}
}

func TestReportBodyAndSource(t *testing.T) {
	r := &stubReader{days: []application.DeliveryDay{
		{Date: "2026-10-01", Delivered: 3, Failed: 1},
		{Date: "2026-10-02", Delivered: 5, Returned: 1},
	}}
	rec := do(t, reportMux(&stubStore{}, r, "from members 2 and 3"), "GET", reportPath+"?from=2026-10-01&to=2026-10-02", "DISPATCHER", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	var b struct {
		Data application.DeliveryReport `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	d := b.Data
	if d.From != "2026-10-01" || d.To != "2026-10-02" || d.Delivered != 8 || d.Failed != 1 || len(d.Days) != 2 || d.Source != "from members 2 and 3" {
		t.Fatalf("report: %+v", d)
	}
	if d.SuccessRate == nil || *d.SuccessRate < 0.88 || *d.SuccessRate > 0.89 {
		t.Fatalf("success rate: %v", d.SuccessRate)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("report must not be cached")
	}
}

// With no reader wired, the stand-in answers and says so, so nobody mistakes zeros for real data.
func TestReportStandInIsLabelled(t *testing.T) {
	h := &Handlers{Svc: &application.Service{Store: &stubStore{}}, Auth: fakeAuth{}}
	mux := http.NewServeMux()
	h.Register(mux)
	rec := do(t, mux, "GET", reportPath+"?from=2026-10-01&to=2026-10-03", "ADMIN", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"days":[]`, `"success_rate":null`, `"delivered":0`, "stand-in"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s in %s", want, body)
		}
	}
}

func TestReportValidation(t *testing.T) {
	r := &stubReader{}
	mux := reportMux(&stubStore{}, r, "s")
	for name, q := range map[string]string{
		"no dates":      "",
		"missing to":    "?from=2026-10-01",
		"bad format":    "?from=01-10-2026&to=02-10-2026",
		"to before":     "?from=2026-10-05&to=2026-10-01",
		"over 92 days":  "?from=2026-01-01&to=2026-06-01",
		"impossible":    "?from=2026-02-30&to=2026-03-01",
		"sql in params": "?from=" + url.QueryEscape("2026-10-01';DROP TABLE x;--") + "&to=2026-10-02",
	} {
		rec := do(t, mux, "GET", reportPath+q, "ADMIN", "")
		if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec).Error.Code != "VALIDATION_FAILED" {
			t.Errorf("%s: got %d %s", name, rec.Code, rec.Body.String())
		}
	}
	if r.n != 0 {
		t.Errorf("reader called %d times for invalid input", r.n)
	}
}

func TestReportUsesTheRangeSetting(t *testing.T) {
	st := &stubStore{settings: []application.Setting{{Key: "reports.max_range_days", Value: json.RawMessage(`2`), ValueType: "INTEGER", Version: 1}}}
	mux := reportMux(st, &stubReader{}, "s")
	if rec := do(t, mux, "GET", reportPath+"?from=2026-10-01&to=2026-10-02", "ADMIN", ""); rec.Code != http.StatusOK {
		t.Fatalf("2 days: got %d", rec.Code)
	}
	if rec := do(t, mux, "GET", reportPath+"?from=2026-10-01&to=2026-10-03", "ADMIN", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("3 days over limit 2: got %d", rec.Code)
	}
}

// A failing reader must not leak its error text; it becomes the generic 500.
func TestReportReaderFailureLeaksNothing(t *testing.T) {
	r := &stubReader{err: errors.New(`pq: relation "delivery_attempts" does not exist password=hunter2`)}
	rec := do(t, reportMux(&stubStore{}, r, "s"), "GET", reportPath+"?from=2026-10-01&to=2026-10-02", "ADMIN", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rec.Code)
	}
	for _, s := range []string{"pq:", "delivery_attempts", "hunter2", "does not exist"} {
		if strings.Contains(rec.Body.String(), s) {
			t.Fatalf("response leaks %q: %s", s, rec.Body.String())
		}
	}
}
