package adminsec

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func call(t *testing.T, h http.Handler, p *Principal, hdr map[string]string) (*httptest.ResponseRecorder, errorBody) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/zones", strings.NewReader(`{}`))
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	if p != nil {
		r = r.WithContext(WithPrincipal(r.Context(), *p))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var b errorBody
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	return w, b
}

var ok200 = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

// SC-A01..A04: every role against an ADMIN-only endpoint (admin README rule 1).
func TestRequireRole_AdminOnlyMatrix(t *testing.T) {
	h := RequireRole(RoleAdmin)(ok200)
	cases := []struct {
		name string
		p    *Principal
		want int
		code string
	}{
		{"no token", nil, 401, "AUTH_REQUIRED"},
		{"empty principal", &Principal{}, 401, "AUTH_REQUIRED"},
		{"customer", &Principal{UserID: "u1", Role: RoleCustomer}, 403, "ACCESS_DENIED"},
		{"merchant", &Principal{UserID: "u2", Role: RoleMerchant, TenantID: "mrc_1"}, 403, "ACCESS_DENIED"},
		{"courier", &Principal{UserID: "u3", Role: RoleCourier}, 403, "ACCESS_DENIED"},
		{"dispatcher", &Principal{UserID: "u4", Role: RoleDispatcher}, 403, "ACCESS_DENIED"},
		{"lower-case admin is not admin", &Principal{UserID: "u5", Role: "admin"}, 403, "ACCESS_DENIED"},
		{"admin", &Principal{UserID: "u6", Role: RoleAdmin}, 200, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w, b := call(t, h, c.p, nil)
			if w.Code != c.want || b.Error.Code != c.code {
				t.Fatalf("got %d %q, want %d %q", w.Code, b.Error.Code, c.want, c.code)
			}
		})
	}
}

// SC-A05: role headers from the client are ignored; only the verified principal counts.
func TestRequireRole_IgnoresClientRoleHeaders(t *testing.T) {
	h := RequireRole(RoleAdmin)(ok200)
	w, _ := call(t, h, &Principal{UserID: "u1", Role: RoleCourier}, map[string]string{"X-User-Role": "ADMIN", "X-Role": "ADMIN"})
	if w.Code != 403 {
		t.Fatalf("header must not raise role, got %d", w.Code)
	}
}

// SC-A06: dispatch board allows DISPATCHER and ADMIN only.
func TestRequireRole_DispatchBoard(t *testing.T) {
	h := RequireRole(RoleDispatcher, RoleAdmin)(ok200)
	for role, want := range map[string]int{RoleDispatcher: 200, RoleAdmin: 200, RoleCourier: 403, RoleMerchant: 403} {
		if w, _ := call(t, h, &Principal{UserID: "u", Role: role}, nil); w.Code != want {
			t.Errorf("%s: got %d want %d", role, w.Code, want)
		}
	}
}

// SC-E01: an unknown error (for example a raw SQL error) never reaches the client.
func TestWriteError_HidesInternalDetails(t *testing.T) {
	sqlErr := errors.New(`pq: duplicate key value violates unique constraint "delivery_zones_code_key" password=hunter2`)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, sqlErr) })
	w, b := call(t, h, nil, nil)
	if w.Code != 500 || b.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("got %d %s", w.Code, b.Error.Code)
	}
	for _, leak := range []string{"pq:", "delivery_zones", "hunter2", "constraint"} {
		if strings.Contains(w.Body.String(), leak) {
			t.Fatalf("response leaks %q: %s", leak, w.Body.String())
		}
	}
	if b.Error.CorrelationID == "" {
		t.Fatal("500 must carry a correlation id for support")
	}
}

// SC-E02: wrapped known errors keep their safe code and status.
func TestWriteError_KnownWrapped(t *testing.T) {
	cases := map[error]int{
		fmt.Errorf("update zone z1: %w", ErrConcurrentConflict): 409,
		fmt.Errorf("add 627: %w", ErrPrefixConflict):            409,
		fmt.Errorf("%w: must be 1 to 50", ErrValidation):        422,
	}
	for err, want := range cases {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, err) })
		w, b := call(t, h, nil, nil)
		// Check the message, not the whole body: the random correlation ID could contain "627".
		if w.Code != want || strings.Contains(b.Error.Message, "z1") || strings.Contains(b.Error.Message, "627") {
			t.Errorf("%v: got %d body %s", err, w.Code, w.Body.String())
		}
	}
}

// SC-E03: a good X-Request-ID is reused; a hostile one is replaced, never echoed.
func TestCorrelationID(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, ErrNotFound) })
	_, b := call(t, h, nil, map[string]string{"X-Request-ID": "req_01HXYZ"})
	if b.Error.CorrelationID != "req_01HXYZ" {
		t.Fatalf("good id not reused: %q", b.Error.CorrelationID)
	}
	for _, bad := range []string{"abc\nlevel=ERROR forged", strings.Repeat("a", 65), "<script>"} {
		_, b := call(t, h, nil, map[string]string{"X-Request-ID": bad})
		if b.Error.CorrelationID == bad || !strings.HasPrefix(b.Error.CorrelationID, "corr_") {
			t.Fatalf("bad id echoed: %q", b.Error.CorrelationID)
		}
	}
}
