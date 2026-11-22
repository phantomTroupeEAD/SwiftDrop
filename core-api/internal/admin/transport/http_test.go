package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/swiftdrop/core-api/internal/admin/application"
	"github.com/swiftdrop/core-api/internal/admin/domain"
	"github.com/swiftdrop/core-api/internal/platform/middleware"
)

// fakeAuth reads "X-Test-Role" instead of a JWT: no header is 401, a role not in
// the list is 403. That is the contract the wiring adapter must keep.
type fakeAuth struct{}

type principalKey struct{}

func (fakeAuth) Require(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := r.Header.Get("X-Test-Role")
			if role == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			for _, allowed := range roles {
				if role == allowed {
					ctx := context.WithValue(r.Context(), principalKey{}, Principal{UserID: "00000000-0000-4000-8000-000000000001", Role: role})
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			w.WriteHeader(http.StatusForbidden)
		})
	}
}

func (fakeAuth) Principal(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// stubStore returns canned answers; txErr makes every write transaction fail.
type stubStore struct {
	txErr     error
	listErr   error
	public    []application.PublicZone
	lookup    application.PublicZone
	lookupErr error
	gotActor  application.Actor

	// paging: the total the store reports, and the offset and limit it was asked for
	total               int
	gotOffset, gotLimit int
	gotFilter           application.AuditFilter
	auditCalls          int
	settings            []application.Setting
}

func (s *stubStore) InTx(ctx context.Context, fn func(application.Tx) error) error {
	if s.txErr != nil {
		return s.txErr
	}
	return fn(&stubTx{s: s})
}
func (s *stubStore) ListZones(_ context.Context, offset, limit int) ([]*domain.Zone, int, error) {
	s.gotOffset, s.gotLimit = offset, limit
	return nil, s.total, s.listErr
}
func (s *stubStore) GetZone(context.Context, string) (*domain.Zone, error) {
	return nil, application.ErrNotFound
}
func (s *stubStore) ListPublicZones(context.Context) ([]application.PublicZone, error) {
	return s.public, s.listErr
}
func (s *stubStore) ZoneForPostalCode(context.Context, string) (application.PublicZone, error) {
	return s.lookup, s.lookupErr
}
func (s *stubStore) ListSettings(context.Context) ([]application.Setting, error) {
	return s.settings, nil
}
func (s *stubStore) ListAudit(_ context.Context, f application.AuditFilter, offset, limit int) ([]application.AuditRow, int, error) {
	s.gotOffset, s.gotLimit, s.gotFilter = offset, limit, f
	s.auditCalls++
	return nil, s.total, s.listErr
}

type stubTx struct{ s *stubStore }

func (t *stubTx) LockZonePrefixes(context.Context) error { return nil }
func (t *stubTx) OtherZonePrefixes(context.Context, string) ([]domain.PrefixOwner, error) {
	return nil, nil
}
func (t *stubTx) InsertZone(context.Context, *domain.Zone) error { return nil }
func (t *stubTx) LoadZone(context.Context, string) (*domain.Zone, error) {
	return nil, application.ErrNotFound
}
func (t *stubTx) SaveZone(context.Context, *domain.Zone, int) error { return nil }
func (t *stubTx) LoadSetting(context.Context, string) (application.Setting, error) {
	return application.Setting{}, application.ErrNotFound
}
func (t *stubTx) SaveSetting(context.Context, string, json.RawMessage, int, string) error {
	return nil
}
func (t *stubTx) Audit(_ context.Context, a application.Actor, _ application.AuditEntry) error {
	t.s.gotActor = a
	return nil
}

func newMux(st *stubStore) http.Handler {
	h := &Handlers{Svc: &application.Service{Store: st}, Auth: fakeAuth{}}
	mux := http.NewServeMux()
	h.Register(mux)
	return middleware.RequestID(mux)
}

func do(t *testing.T, h http.Handler, method, path, role, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if role != "" {
		req.Header.Set("X-Test-Role", role)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type errorBody struct {
	Error struct {
		Code          string `json:"code"`
		Message       string `json:"message"`
		CorrelationID string `json:"correlation_id"`
	} `json:"error"`
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var b errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("not an error envelope: %s", rec.Body.String())
	}
	return b
}

var adminRoutes = [][2]string{
	{"GET", "/api/v1/admin/zones"},
	{"POST", "/api/v1/admin/zones"},
	{"GET", "/api/v1/admin/zones/ZONE_COLOMBO_01"},
	{"PATCH", "/api/v1/admin/zones/ZONE_COLOMBO_01"},
	{"PATCH", "/api/v1/admin/zones/ZONE_COLOMBO_01/status"},
	{"GET", "/api/v1/admin/settings"},
	{"PUT", "/api/v1/admin/settings/ops.board_refresh_seconds"},
	{"GET", "/api/v1/admin/audit-logs"},
}

// Doc 7 B-01: no admin route without the guard. Signed-out is 401; every other role is 403.
func TestAdminRoutesNeedAdminRole(t *testing.T) {
	mux := newMux(&stubStore{})
	for _, rt := range adminRoutes {
		if rec := do(t, mux, rt[0], rt[1], "", "{}"); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s signed out: got %d, want 401", rt[0], rt[1], rec.Code)
		}
		for _, role := range []string{"DISPATCHER", "COURIER", "MERCHANT", "CUSTOMER"} {
			if rec := do(t, mux, rt[0], rt[1], role, "{}"); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s as %s: got %d, want 403", rt[0], rt[1], role, rec.Code)
			}
		}
	}
}

func TestPublicZonesNeedNoSignInAndShowOnlyPublicFields(t *testing.T) {
	mux := newMux(&stubStore{public: []application.PublicZone{{ID: "ZONE_COLOMBO_01", Code: "CMB-01", Name: "Colombo Central"}}})
	rec := do(t, mux, "GET", "/api/v1/zones", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Fatalf("cache header: %q", rec.Header().Get("Cache-Control"))
	}
	var body struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Data) != 1 {
		t.Fatalf("body: %s", rec.Body.String())
	}
	for k := range body.Data[0] {
		if k != "id" && k != "code" && k != "name" {
			t.Fatalf("public zone shows extra field %q", k)
		}
	}
}

func TestPublicPostalLookup(t *testing.T) {
	mux := newMux(&stubStore{lookupErr: application.ErrZoneNotFound})
	rec := do(t, mux, "GET", "/api/v1/zones?postal_code=12", "", "")
	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec).Error.Code != "VALIDATION_FAILED" {
		t.Fatalf("bad postal code: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "" {
		t.Fatal("error responses must not be cached")
	}
	rec = do(t, mux, "GET", "/api/v1/zones?postal_code=99999", "", "")
	if rec.Code != http.StatusNotFound || errorCode(t, rec).Error.Code != "ZONE_NOT_FOUND" {
		t.Fatalf("unknown postal code: %d %s", rec.Code, rec.Body.String())
	}
}

const validZone = `{"id":"ZONE_COLOMBO_01","code":"CMB-01","name":"Colombo Central","postal_prefixes":["00100"]}`

func TestCreateZone(t *testing.T) {
	st := &stubStore{}
	mux := newMux(st)
	rec := do(t, mux, "POST", "/api/v1/admin/zones", "ADMIN", validZone)
	if rec.Code != http.StatusCreated || rec.Header().Get("Location") != "/api/v1/admin/zones/ZONE_COLOMBO_01" {
		t.Fatalf("got %d %q: %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("admin responses must not be cached")
	}
	if st.gotActor.Role != "ADMIN" || st.gotActor.ID == "" || st.gotActor.RequestID == "" {
		t.Fatalf("audit actor not taken from the verified principal: %+v", st.gotActor)
	}
}

func TestCreateZoneRejectsUnknownFields(t *testing.T) {
	mux := newMux(&stubStore{})
	body := `{"id":"ZONE_COLOMBO_01","code":"CMB-01","name":"C","postal_prefixes":["00100"],"base_surcharge_cents":100}`
	rec := do(t, mux, "POST", "/api/v1/admin/zones", "ADMIN", body)
	if rec.Code != http.StatusBadRequest || errorCode(t, rec).Error.Code != "BAD_REQUEST" {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{application.ErrPrefixConflict, 409, "ZONE_POSTAL_CODE_CONFLICT"},
		{application.ErrZoneCodeTaken, 409, "ZONE_CODE_ALREADY_EXISTS"},
		{application.ErrConcurrentConflict, 409, "CONCURRENT_MODIFICATION"},
		{application.ErrBusy, 503, "TRY_AGAIN"},
		{application.ErrUnavailable, 503, "SERVICE_UNAVAILABLE"},
	}
	for _, c := range cases {
		rec := do(t, newMux(&stubStore{txErr: c.err}), "POST", "/api/v1/admin/zones", "ADMIN", validZone)
		if b := errorCode(t, rec); rec.Code != c.status || b.Error.Code != c.code || b.Error.CorrelationID == "" {
			t.Errorf("%v: got %d %+v, want %d %s", c.err, rec.Code, b.Error, c.status, c.code)
		}
	}
}

// An unexpected error becomes a generic 500: no SQL, constraint names or secrets reach the client.
func TestUnexpectedErrorLeaksNothing(t *testing.T) {
	leak := errors.New(`pq: duplicate key value violates unique constraint "delivery_zones_code_key" password=hunter2`)
	rec := do(t, newMux(&stubStore{txErr: leak}), "POST", "/api/v1/admin/zones", "ADMIN", validZone)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rec.Code)
	}
	for _, s := range []string{"pq:", "delivery_zones", "hunter2", "constraint"} {
		if strings.Contains(rec.Body.String(), s) {
			t.Fatalf("response leaks %q: %s", s, rec.Body.String())
		}
	}
}

type pageBody struct {
	Data []json.RawMessage `json:"data"`
	Meta map[string]int    `json:"meta"`
}

// api-conventions 2.2: ?page=&per_page= in, {data, meta{page, per_page, total_items, total_pages}} out.
// Both lists page the same way, so every case runs against both routes.
func TestPagedListsReturnMeta(t *testing.T) {
	for _, path := range []string{"/api/v1/admin/zones", "/api/v1/admin/audit-logs"} {
		t.Run(path, func(t *testing.T) {
			st := &stubStore{total: 45}
			rec := do(t, newMux(st), "GET", path+"?page=2&per_page=10", "ADMIN", "")
			if rec.Code != http.StatusOK {
				t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
			}
			var b pageBody
			if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
				t.Fatal(err)
			}
			want := map[string]int{"page": 2, "per_page": 10, "total_items": 45, "total_pages": 5}
			for k, v := range want {
				if b.Meta[k] != v {
					t.Errorf("meta %s = %d, want %d (meta %v)", k, b.Meta[k], v, b.Meta)
				}
			}
			if b.Data == nil || len(b.Data) != 0 {
				t.Errorf("data must be an empty array, not null: %s", rec.Body.String())
			}
			if st.gotOffset != 10 || st.gotLimit != 10 {
				t.Errorf("store asked for offset %d limit %d, want 10 and 10", st.gotOffset, st.gotLimit)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Error("admin lists must not be cached")
			}
		})
	}
}

func TestPagedListsDefaults(t *testing.T) {
	for _, path := range []string{"/api/v1/admin/zones", "/api/v1/admin/audit-logs"} {
		st := &stubStore{total: 0}
		rec := do(t, newMux(st), "GET", path, "ADMIN", "")
		var b pageBody
		if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
			t.Fatal(err)
		}
		if b.Meta["page"] != 1 || b.Meta["per_page"] != 20 || b.Meta["total_items"] != 0 || b.Meta["total_pages"] != 0 {
			t.Errorf("%s defaults: %v", path, b.Meta)
		}
		if st.gotOffset != 0 || st.gotLimit != 20 {
			t.Errorf("%s: store asked for offset %d limit %d", path, st.gotOffset, st.gotLimit)
		}
	}
}

func TestPagedListsRejectBadPaging(t *testing.T) {
	for _, path := range []string{"/api/v1/admin/zones", "/api/v1/admin/audit-logs"} {
		for _, q := range []string{"page=0", "page=-1", "page=abc", "per_page=0", "per_page=101", "per_page=x", "per_page=1.5"} {
			rec := do(t, newMux(&stubStore{}), "GET", path+"?"+q, "ADMIN", "")
			if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec).Error.Code != "VALIDATION_FAILED" {
				t.Errorf("%s?%s: got %d %s", path, q, rec.Code, rec.Body.String())
			}
		}
		if rec := do(t, newMux(&stubStore{}), "GET", path+"?per_page=100", "ADMIN", ""); rec.Code != http.StatusOK {
			t.Errorf("%s?per_page=100 must pass: %d", path, rec.Code)
		}
	}
}

// The old ?limit= is gone: it is ignored, not an error, and the default page size applies.
func TestOldLimitParameterIsIgnored(t *testing.T) {
	st := &stubStore{}
	if rec := do(t, newMux(st), "GET", "/api/v1/admin/audit-logs?limit=5", "ADMIN", ""); rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if st.gotLimit != 20 {
		t.Fatalf("limit=5 must not change the page size, store got %d", st.gotLimit)
	}
}

func TestPagedStoreFailureIsMapped(t *testing.T) {
	for _, path := range []string{"/api/v1/admin/zones", "/api/v1/admin/audit-logs"} {
		rec := do(t, newMux(&stubStore{listErr: application.ErrUnavailable}), "GET", path, "ADMIN", "")
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: got %d", path, rec.Code)
		}
	}
}

func TestUnknownSettingAndMissingZone(t *testing.T) {
	mux := newMux(&stubStore{})
	rec := do(t, mux, "PUT", "/api/v1/admin/settings/jwt.secret", "ADMIN", `{"version":1,"value":"x","reason":"trying a secret key"}`)
	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec).Error.Code != "SETTING_UNKNOWN" {
		t.Fatalf("unknown setting: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "GET", "/api/v1/admin/zones/ZONE_NOPE", "ADMIN", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing zone: %d", rec.Code)
	}
}

func TestClientIPUsesRightMostForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 10.0.0.9")
	if got := clientIP(r); got != "10.0.0.9" {
		t.Fatalf("got %q", got)
	}
}
