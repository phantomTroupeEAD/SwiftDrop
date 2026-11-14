package transport

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

// The filters in the query string reach the store, together with the paging.
func TestAuditFiltersReachTheStore(t *testing.T) {
	st := &stubStore{}
	q := url.Values{
		"action": {"ZONE_UPDATED"}, "target_type": {"ZONE"}, "target_id": {"ZONE_COLOMBO_01"}, "actor_role": {"ADMIN"},
		"from": {"2026-11-14"}, "to": {"2026-11-15"}, "page": {"3"}, "per_page": {"10"},
	}
	rec := do(t, newMux(st), "GET", "/api/v1/admin/audit-logs?"+q.Encode(), "ADMIN", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	f := st.gotFilter
	if f.Action != "ZONE_UPDATED" || f.TargetType != "ZONE" || f.TargetID != "ZONE_COLOMBO_01" || f.ActorRole != "ADMIN" {
		t.Fatalf("text filters not passed: %+v", f)
	}
	wantFrom := time.Date(2026, 11, 13, 18, 30, 0, 0, time.UTC)
	wantTo := time.Date(2026, 11, 15, 18, 30, 0, 0, time.UTC)
	if f.From == nil || !f.From.Equal(wantFrom) || f.To == nil || !f.To.Equal(wantTo) {
		t.Fatalf("date range wrong: from %v, to %v", f.From, f.To)
	}
	if st.gotOffset != 20 || st.gotLimit != 10 {
		t.Fatalf("paging lost: offset %d, limit %d", st.gotOffset, st.gotLimit)
	}
}

func TestAuditWithoutFiltersAsksForEverything(t *testing.T) {
	st := &stubStore{}
	if rec := do(t, newMux(st), "GET", "/api/v1/admin/audit-logs", "ADMIN", ""); rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if !st.gotFilter.IsZero() {
		t.Fatalf("no query means no filter, got %+v", st.gotFilter)
	}
}

// A bad filter is 422 VALIDATION_FAILED and the store is never asked.
func TestAuditBadFiltersAreRefusedBeforeTheStore(t *testing.T) {
	bad := []string{
		"action=zone_created", "action=ZONE%20CREATED", "target_type=zone", "target_id=" + url.QueryEscape("x'; DROP TABLE audit_logs;--"),
		"actor_role=ROOT", "from=14-11-2026", "to=2026-02-30", "from=2026-11-15&to=2026-11-14",
		"action=ZONE_CREATED&action=ZONE_UPDATED", "from=2026-11-14&from=2026-11-15",
	}
	for _, q := range bad {
		st := &stubStore{}
		rec := do(t, newMux(st), "GET", "/api/v1/admin/audit-logs?"+q, "ADMIN", "")
		if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec).Error.Code != "VALIDATION_FAILED" {
			t.Errorf("?%s: got %d %s", q, rec.Code, rec.Body.String())
		}
		if st.auditCalls != 0 {
			t.Errorf("?%s: the store must not be asked for a refused filter", q)
		}
	}
}

// Filters do not change who may read the log: no token is 401, other roles are 403.
func TestAuditFiltersKeepTheRoleRules(t *testing.T) {
	path := "/api/v1/admin/audit-logs?action=ZONE_CREATED"
	if rec := do(t, newMux(&stubStore{}), "GET", path, "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: got %d", rec.Code)
	}
	for _, role := range []string{"DISPATCHER", "COURIER", "CUSTOMER", "MERCHANT"} {
		if rec := do(t, newMux(&stubStore{}), "GET", path, role, ""); rec.Code != http.StatusForbidden {
			t.Errorf("%s: got %d, want 403", role, rec.Code)
		}
	}
}

// The error text never repeats what the user typed back as SQL or markup.
func TestAuditFilterErrorDoesNotEchoSQL(t *testing.T) {
	rec := do(t, newMux(&stubStore{}), "GET", "/api/v1/admin/audit-logs?target_id="+url.QueryEscape("1' OR '1'='1"), "ADMIN", "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rec.Code)
	}
	if body := rec.Body.String(); contains(body, "OR '1'") || contains(body, "audit_logs") {
		t.Fatalf("the error leaks the input or table names: %s", body)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
