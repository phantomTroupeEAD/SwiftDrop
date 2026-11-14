package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/swiftdrop/core-api/internal/admin/domain"
)

// memState is the fake database. InTx works on a copy and keeps it only when fn
// succeeds, so a failed audit write also drops the change, as a real transaction does.
type memState struct {
	zones    map[string]ZoneView
	settings map[string]Setting
	audits   []AuditEntry
}

func (m memState) clone() memState {
	c := memState{zones: map[string]ZoneView{}, settings: map[string]Setting{}, audits: append([]AuditEntry(nil), m.audits...)}
	for k, v := range m.zones {
		c.zones[k] = v
	}
	for k, v := range m.settings {
		c.settings[k] = v
	}
	return c
}

type memStore struct {
	state       memState
	auditErr    error
	settingsErr error
	lockCalls   int
}

func newMemStore() *memStore {
	return &memStore{state: memState{zones: map[string]ZoneView{}, settings: map[string]Setting{
		"ops.board_refresh_seconds": {Key: "ops.board_refresh_seconds", Value: json.RawMessage(`30`), ValueType: "INTEGER", Version: 1},
	}}}
}

func (s *memStore) InTx(ctx context.Context, fn func(Tx) error) error {
	t := &memTx{s: s, st: s.state.clone()}
	if err := fn(t); err != nil {
		return err
	}
	s.state = t.st
	return nil
}

func restoreView(v ZoneView) *domain.Zone {
	id, _ := domain.NewZoneID(v.ID)
	code, _ := domain.NewZoneCode(v.Code)
	ps := make([]domain.PostalPrefix, 0, len(v.PostalPrefixes))
	for _, p := range v.PostalPrefixes {
		x, _ := domain.NewPostalPrefix(p)
		ps = append(ps, x)
	}
	return domain.Restore(id, code, v.Name, v.Description, ps, v.Enabled, v.Version)
}

func (s *memStore) ListZones(context.Context, int, int) ([]*domain.Zone, int, error) {
	return nil, 0, nil
}
func (s *memStore) GetZone(_ context.Context, id string) (*domain.Zone, error) {
	v, ok := s.state.zones[id]
	if !ok {
		return nil, ErrNotFound
	}
	return restoreView(v), nil
}
func (s *memStore) ListPublicZones(context.Context) ([]PublicZone, error) { return nil, nil }
func (s *memStore) ZoneForPostalCode(context.Context, string) (PublicZone, error) {
	return PublicZone{}, ErrZoneNotFound
}
func (s *memStore) ListSettings(context.Context) ([]Setting, error) {
	out := make([]Setting, 0, len(s.state.settings))
	for _, v := range s.state.settings {
		out = append(out, v)
	}
	return out, s.settingsErr
}
func (s *memStore) ListAudit(context.Context, AuditFilter, int, int) ([]AuditRow, int, error) {
	return nil, 0, nil
}

type memTx struct {
	s  *memStore
	st memState
}

func (t *memTx) LockZonePrefixes(context.Context) error { t.s.lockCalls++; return nil }

func (t *memTx) OtherZonePrefixes(_ context.Context, except string) ([]domain.PrefixOwner, error) {
	var out []domain.PrefixOwner
	for id, z := range t.st.zones {
		if id == except {
			continue
		}
		for _, p := range z.PostalPrefixes {
			x, _ := domain.NewPostalPrefix(p)
			out = append(out, domain.PrefixOwner{Prefix: x, ZoneID: id})
		}
	}
	return out, nil
}

func (t *memTx) InsertZone(_ context.Context, z *domain.Zone) error {
	if _, ok := t.st.zones[z.ID().String()]; ok {
		return ErrZoneCodeTaken
	}
	t.st.zones[z.ID().String()] = View(z)
	return nil
}

func (t *memTx) LoadZone(_ context.Context, id string) (*domain.Zone, error) {
	v, ok := t.st.zones[id]
	if !ok {
		return nil, ErrNotFound
	}
	return restoreView(v), nil
}

func (t *memTx) SaveZone(_ context.Context, z *domain.Zone, expected int) error {
	if t.st.zones[z.ID().String()].Version != expected {
		return ErrConcurrentConflict
	}
	t.st.zones[z.ID().String()] = View(z)
	return nil
}

func (t *memTx) LoadSetting(_ context.Context, key string) (Setting, error) {
	s, ok := t.st.settings[key]
	if !ok {
		return Setting{}, ErrNotFound
	}
	return s, nil
}

func (t *memTx) SaveSetting(_ context.Context, key string, v json.RawMessage, expected int, _ string) error {
	s := t.st.settings[key]
	if s.Version != expected {
		return ErrConcurrentConflict
	}
	s.Value, s.Version = v, s.Version+1
	t.st.settings[key] = s
	return nil
}

func (t *memTx) Audit(_ context.Context, _ Actor, e AuditEntry) error {
	if t.s.auditErr != nil {
		return t.s.auditErr
	}
	t.st.audits = append(t.st.audits, e)
	return nil
}

var testAdmin = Actor{ID: "00000000-0000-4000-8000-000000000001", Role: "ADMIN", RequestID: "utest"}

func createColombo(t *testing.T, svc *Service) ZoneView {
	t.Helper()
	z, err := svc.CreateZone(context.Background(), testAdmin, CreateZoneInput{
		ID: "ZONE_COLOMBO_01", Code: "CMB-01", Name: "Colombo Central", PostalPrefixes: []string{"00100", "00200"}})
	if err != nil {
		t.Fatal(err)
	}
	return z
}

func TestCreateZoneWritesOneAuditRowUnderLock(t *testing.T) {
	st := newMemStore()
	svc := &Service{Store: st}
	z := createColombo(t, svc)
	if z.Version != 1 || !z.Enabled {
		t.Fatalf("new zone: %+v", z)
	}
	if st.lockCalls != 1 {
		t.Fatalf("prefix lock taken %d times, want 1", st.lockCalls)
	}
	if len(st.state.audits) != 1 || st.state.audits[0].Action != "ZONE_CREATED" || st.state.audits[0].TargetID != z.ID {
		t.Fatalf("audit rows: %+v", st.state.audits)
	}
}

func TestCreateZoneRejectsInvalidInput(t *testing.T) {
	svc := &Service{Store: newMemStore()}
	for name, in := range map[string]CreateZoneInput{
		"bad id":      {ID: "colombo", Code: "CMB-01", Name: "C", PostalPrefixes: []string{"00100"}},
		"bad code":    {ID: "ZONE_C_01", Code: "c", Name: "C", PostalPrefixes: []string{"00100"}},
		"no prefix":   {ID: "ZONE_C_01", Code: "C-01", Name: "C"},
		"bad prefix":  {ID: "ZONE_C_01", Code: "C-01", Name: "C", PostalPrefixes: []string{"12"}},
		"empty name":  {ID: "ZONE_C_01", Code: "C-01", Name: "  ", PostalPrefixes: []string{"00100"}},
		"dup in zone": {ID: "ZONE_C_01", Code: "C-01", Name: "C", PostalPrefixes: []string{"00100", "00100"}},
	} {
		if _, err := svc.CreateZone(context.Background(), testAdmin, in); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: got %v, want ErrValidation", name, err)
		}
	}
}

func TestCreateZoneRefusesOverlappingPrefix(t *testing.T) {
	st := newMemStore()
	svc := &Service{Store: st}
	createColombo(t, svc)
	_, err := svc.CreateZone(context.Background(), testAdmin, CreateZoneInput{
		ID: "ZONE_COLOMBO_02", Code: "CMB-02", Name: "Overlap", PostalPrefixes: []string{"001"}})
	if !errors.Is(err, ErrPrefixConflict) {
		t.Fatalf("got %v, want ErrPrefixConflict", err)
	}
	if len(st.state.zones) != 1 || len(st.state.audits) != 1 {
		t.Fatal("refused zone must leave no row and no audit")
	}
}

// SC-U01: if the audit row cannot be written, the change is not kept either.
func TestAuditFailureDropsTheChange(t *testing.T) {
	st := newMemStore()
	st.auditErr = errors.New("audit insert failed")
	svc := &Service{Store: st}
	if _, err := svc.CreateZone(context.Background(), testAdmin, CreateZoneInput{
		ID: "ZONE_COLOMBO_01", Code: "CMB-01", Name: "C", PostalPrefixes: []string{"00100"}}); err == nil {
		t.Fatal("expected an error")
	}
	if len(st.state.zones) != 0 {
		t.Fatal("zone saved without its audit row")
	}
}

func TestUpdateZoneVersionRules(t *testing.T) {
	st := newMemStore()
	svc := &Service{Store: st}
	z := createColombo(t, svc)
	name := "Colombo Central Extended"

	if _, err := svc.UpdateZone(context.Background(), testAdmin, z.ID, UpdateZoneInput{Name: &name}); !errors.Is(err, ErrValidation) {
		t.Fatalf("missing version: got %v", err)
	}
	if _, err := svc.UpdateZone(context.Background(), testAdmin, z.ID, UpdateZoneInput{Version: 7, Name: &name}); !errors.Is(err, ErrConcurrentConflict) {
		t.Fatalf("stale version: got %v", err)
	}
	code := "CMB-1X"
	got, err := svc.UpdateZone(context.Background(), testAdmin, z.ID, UpdateZoneInput{Version: 1, Name: &name, Code: &code})
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 || got.Name != name || got.Code != code {
		t.Fatalf("two fields in one PATCH must raise the version once: %+v", got)
	}
	if len(st.state.audits) != 2 || st.state.audits[1].Action != "ZONE_UPDATED" || st.state.audits[1].Before["name"] != "Colombo Central" {
		t.Fatalf("audit rows: %+v", st.state.audits)
	}
}

func TestUpdateZoneWithoutChangesWritesNothing(t *testing.T) {
	st := newMemStore()
	svc := &Service{Store: st}
	z := createColombo(t, svc)
	same := z.Name
	got, err := svc.UpdateZone(context.Background(), testAdmin, z.ID, UpdateZoneInput{Version: 1, Name: &same})
	if err != nil || got.Version != 1 {
		t.Fatalf("got %+v %v", got, err)
	}
	if len(st.state.audits) != 1 {
		t.Fatal("a no-op PATCH must not add an audit row")
	}
}

func TestUpdateZoneUnknownID(t *testing.T) {
	svc := &Service{Store: newMemStore()}
	name := "x"
	if _, err := svc.UpdateZone(context.Background(), testAdmin, "ZONE_NOPE", UpdateZoneInput{Version: 1, Name: &name}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestSetZoneEnabled(t *testing.T) {
	st := newMemStore()
	svc := &Service{Store: st}
	z := createColombo(t, svc)
	ctx := context.Background()

	if _, err := svc.SetZoneEnabled(ctx, testAdmin, z.ID, StatusInput{Version: 1, Enabled: false, Reason: "short"}); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("short reason: got %v", err)
	}
	if _, err := svc.SetZoneEnabled(ctx, testAdmin, z.ID, StatusInput{Version: 1, Enabled: true, Reason: "already on, should refuse"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("enable an enabled zone: got %v", err)
	}
	got, err := svc.SetZoneEnabled(ctx, testAdmin, z.ID, StatusInput{Version: 1, Enabled: false, Reason: "Flooding closes the depot"})
	if err != nil || got.Enabled || got.Version != 2 {
		t.Fatalf("disable: %+v %v", got, err)
	}
	last := st.state.audits[len(st.state.audits)-1]
	if last.Action != "ZONE_DISABLED" || last.Reason != "Flooding closes the depot" {
		t.Fatalf("audit: %+v", last)
	}
}

func TestUpdateSetting(t *testing.T) {
	st := newMemStore()
	svc := &Service{Store: st}
	ctx := context.Background()
	reason := "Board polls too often at peak"

	if _, err := svc.UpdateSetting(ctx, testAdmin, "jwt.secret", SettingInput{Version: 1, Value: json.RawMessage(`"x"`), Reason: reason}); !errors.Is(err, ErrUnknownSetting) {
		t.Fatalf("unknown key: got %v", err)
	}
	if _, err := svc.UpdateSetting(ctx, testAdmin, "ops.board_refresh_seconds", SettingInput{Version: 1, Value: json.RawMessage(`60`)}); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("missing reason: got %v", err)
	}
	if _, err := svc.UpdateSetting(ctx, testAdmin, "ops.board_refresh_seconds", SettingInput{Version: 9, Value: json.RawMessage(`60`), Reason: reason}); !errors.Is(err, ErrConcurrentConflict) {
		t.Fatalf("stale version: got %v", err)
	}
	got, err := svc.UpdateSetting(ctx, testAdmin, "ops.board_refresh_seconds", SettingInput{Version: 1, Value: json.RawMessage(`60`), Reason: reason})
	if err != nil || got.Version != 2 || string(got.Value) != "60" {
		t.Fatalf("update: %+v %v", got, err)
	}
	if len(st.state.audits) != 1 || st.state.audits[0].Action != "SETTING_CHANGED" {
		t.Fatalf("audit: %+v", st.state.audits)
	}
}
