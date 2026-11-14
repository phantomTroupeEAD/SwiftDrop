package infrastructure_test

// Integration tests on a REAL PostgreSQL. They run only when INTEGRATION=1, like
// core-api/tests. The database is TEST_DATABASE_URL (or DATABASE_URL). Only the
// migrations this module needs are applied (see adminMigrations), so a broken
// migration in another module cannot hide an admin failure. Test zones use ids
// starting ZONE_T_ and are deleted afterwards. Audit rows stay: audit_logs is append-only.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/swiftdrop/core-api/internal/admin/application"
	"github.com/swiftdrop/core-api/internal/admin/infrastructure"
)

// adminMigrations are applied in order: users (Member 1) for the foreign keys,
// then this module's tables.
var adminMigrations = []string{
	"20261021000001_create_identity_tables.up.sql",
	"20261109000001_create_audit_logs.up.sql",
	"20261109000100_admin_create_delivery_zones.up.sql",
	"20261109000200_admin_create_audit_logs.up.sql",
	"20261112000100_admin_add_parcel_settings.up.sql",
}

// applyMigrations runs each file once, in its own transaction, and records it in
// the same schema_migrations ledger as migrations.RunMigrations.
func applyMigrations(t *testing.T, conn *sql.DB) {
	t.Helper()
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(50) PRIMARY KEY, name TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	for _, name := range adminMigrations {
		version := strings.Split(name, "_")[0]
		var done bool
		if err := conn.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&done); err != nil {
			t.Fatal(err)
		}
		if done {
			continue
		}
		body, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		tx, err := conn.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(string(body)); err != nil {
			_ = tx.Rollback()
			t.Fatalf("migration %s: %v", name, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", version, name); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
}

func setup(t *testing.T) (*application.Service, *sql.DB) {
	t.Helper()
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("skipping admin integration test; set INTEGRATION=1 to run")
	}
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Fatal("INTEGRATION=1 needs TEST_DATABASE_URL or DATABASE_URL")
	}
	conn, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(25)
	applyMigrations(t, conn)
	clean := func() {
		// Prefixes go with the zone (ON DELETE CASCADE).
		if _, err := conn.Exec(`DELETE FROM delivery_zones WHERE id LIKE 'ZONE\_T\_%'`); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}
	clean()
	t.Cleanup(func() { clean(); _ = conn.Close() })
	return &application.Service{Store: &infrastructure.Store{DB: conn, LockTimeout: 5 * time.Second}}, conn
}

// adminActor has no user id: audit_logs.actor_id references users, and these tests
// must not depend on seeded accounts.
var adminActor = application.Actor{Role: "ADMIN", IP: "10.0.0.7", UserAgent: "itest", RequestID: "itest"}

// SC-C01: 20 admins add overlapping prefixes at the same moment. Exactly one wins.
func TestOverlappingPrefixRace(t *testing.T) {
	svc, _ := setup(t)
	run := time.Now().Format("150405")
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := map[string]int{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// "655", "6550" and "65500" all overlap each other, so only one zone may own any of them.
			prefix := []string{"655", "6550", "65500"}[i%3]
			_, err := svc.CreateZone(context.Background(), adminActor, application.CreateZoneInput{
				ID: fmt.Sprintf("ZONE_T_RACE_%s_%02d", run, i), Code: fmt.Sprintf("T-RACE-%s-%02d", run, i), Name: "Race", PostalPrefixes: []string{prefix}})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				results["ok"]++
			case errors.Is(err, application.ErrPrefixConflict):
				results["conflict"]++
			default:
				results[err.Error()]++
			}
		}(i)
	}
	wg.Wait()
	if results["ok"] != 1 || results["conflict"] != 19 {
		t.Fatalf("want 1 ok and 19 conflicts, got %v", results)
	}
}

// 10 admins save the same zone with the same version. One wins.
func TestStaleVersionRace(t *testing.T) {
	svc, _ := setup(t)
	run := time.Now().Format("150405")
	z, err := svc.CreateZone(context.Background(), adminActor, application.CreateZoneInput{
		ID: "ZONE_T_VERSION_" + run, Code: "T-VER-" + run, Name: "V", PostalPrefixes: []string{"75500"}})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, conflict := 0, 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("Name %d", i)
			_, err := svc.UpdateZone(context.Background(), adminActor, z.ID, application.UpdateZoneInput{Version: 1, Name: &name})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				ok++
			} else if errors.Is(err, application.ErrConcurrentConflict) {
				conflict++
			} else {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if ok != 1 || conflict != 9 {
		t.Fatalf("want 1 ok and 9 conflicts, got ok=%d conflict=%d", ok, conflict)
	}
}

// SC-U01: a change and its audit row commit together, or not at all.
func TestAuditInSameTransaction(t *testing.T) {
	svc, conn := setup(t)
	run := time.Now().Format("150405")
	id := "ZONE_T_AUDIT_" + run
	bad := adminActor
	bad.Role = "NOT_A_ROLE" // audit_logs CHECK refuses this, so the audit insert fails
	_, err := svc.CreateZone(context.Background(), bad, application.CreateZoneInput{ID: id, Code: "T-AUD-" + run, Name: "A", PostalPrefixes: []string{"85500"}})
	if err == nil {
		t.Fatal("expected the audit insert to fail")
	}
	var n int
	if err := conn.QueryRow("SELECT count(*) FROM delivery_zones WHERE id = $1", id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("zone was saved without its audit row")
	}
}

// SEC-015: audit rows cannot be changed or removed, even by the table owner.
func TestAuditLogsAreAppendOnly(t *testing.T) {
	svc, conn := setup(t)
	run := time.Now().Format("150405")
	if _, err := svc.CreateZone(context.Background(), adminActor, application.CreateZoneInput{
		ID: "ZONE_T_APPEND_" + run, Code: "T-APP-" + run, Name: "Append", PostalPrefixes: []string{"95500"}}); err != nil {
		t.Fatal(err)
	}
	rows, _, err := svc.Store.ListAudit(context.Background(), application.AuditFilter{}, 0, 1)
	if err != nil || len(rows) != 1 || rows[0].Action != "ZONE_CREATED" {
		t.Fatalf("latest audit row: %+v %v", rows, err)
	}
	if rows[0].RequestID == nil || *rows[0].RequestID != "itest" {
		t.Fatalf("request id not stored: %+v", rows[0])
	}
	if _, err := conn.Exec("UPDATE audit_logs SET reason = 'x' WHERE id = $1", rows[0].ID); err == nil {
		t.Fatal("UPDATE on audit_logs must fail")
	}
	if _, err := conn.Exec("DELETE FROM audit_logs WHERE id = $1", rows[0].ID); err == nil {
		t.Fatal("DELETE on audit_logs must fail")
	}
}

// Settings: the seeded keys exist, a change bumps the version, a stale version is refused.
func TestUpdateSettingRoundTrip(t *testing.T) {
	svc, _ := setup(t)
	ctx := context.Background()
	list, err := svc.Store.ListSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var cur application.Setting
	for _, s := range list {
		if s.Key == "ops.maintenance_banner" {
			cur = s
		}
	}
	if cur.Key == "" {
		t.Fatalf("seeded setting missing: %+v", list)
	}
	in := application.SettingInput{Version: cur.Version, Value: []byte(`"Integration test banner"`), Reason: "integration test round trip"}
	got, err := svc.UpdateSetting(ctx, adminActor, cur.Key, in)
	if err != nil || got.Version != cur.Version+1 {
		t.Fatalf("update: %+v %v", got, err)
	}
	if _, err := svc.UpdateSetting(ctx, adminActor, cur.Key, in); !errors.Is(err, application.ErrConcurrentConflict) {
		t.Fatalf("stale version: got %v", err)
	}
	// Put the banner back so the shared database keeps its default.
	back := application.SettingInput{Version: got.Version, Value: cur.Value, Reason: "integration test restore"}
	if _, err := svc.UpdateSetting(ctx, adminActor, cur.Key, back); err != nil {
		t.Fatal(err)
	}
}

// Public zone list and postal code lookup (Doc 1 R-18).
func TestPublicZonesAndPostalLookup(t *testing.T) {
	svc, _ := setup(t)
	run := time.Now().Format("150405")
	ctx := context.Background()
	on, err := svc.CreateZone(ctx, adminActor, application.CreateZoneInput{ID: "ZONE_T_PUB_ON_" + run, Code: "T-ON-" + run, Name: "Public On", PostalPrefixes: []string{"445"}})
	if err != nil {
		t.Fatal(err)
	}
	off, err := svc.CreateZone(ctx, adminActor, application.CreateZoneInput{ID: "ZONE_T_PUB_OFF_" + run, Code: "T-OFF-" + run, Name: "Public Off", PostalPrefixes: []string{"44600"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetZoneEnabled(ctx, adminActor, off.ID, application.StatusInput{Version: off.Version, Enabled: false, Reason: "test disable"}); err != nil {
		t.Fatal(err)
	}
	z, err := svc.Store.ZoneForPostalCode(ctx, "44512")
	if err != nil || z.ID != on.ID || z.Code != on.Code {
		t.Fatalf("lookup 44512: %+v %v", z, err)
	}
	if _, err := svc.Store.ZoneForPostalCode(ctx, "44600"); !errors.Is(err, application.ErrZoneNotFound) {
		t.Fatalf("disabled zone must not be found: %v", err)
	}
	list, err := svc.Store.ListPublicZones(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, x := range list {
		seen[x.ID] = true
	}
	if !seen[on.ID] || seen[off.ID] {
		t.Fatalf("public list must hold enabled zones only: %+v", list)
	}
	all, _, err := svc.Store.ListZones(ctx, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range all {
		if x.ID().String() == on.ID && (len(x.Prefixes()) != 1 || x.Prefixes()[0].String() != "445") {
			t.Fatalf("ListZones prefixes: %+v", application.View(x))
		}
	}
}

// Paging on a real database: pages are disjoint, ordered by id, cover every zone once,
// carry each zone's own prefixes, and the total counts all rows.
func TestZonePagingAndAuditPaging(t *testing.T) {
	svc, _ := setup(t)
	run := time.Now().Format("150405")
	ctx := context.Background()
	const n = 5
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("ZONE_T_PAGE_%s_%d", run, i)
		if _, err := svc.CreateZone(ctx, adminActor, application.CreateZoneInput{
			ID: id, Code: fmt.Sprintf("T-PG-%s-%d", run, i), Name: "Page", PostalPrefixes: []string{fmt.Sprintf("5%d%s", i, run[:2])}}); err != nil {
			t.Fatal(err)
		}
	}
	_, total, err := svc.Store.ListZones(ctx, 0, 1)
	if err != nil || total < n {
		t.Fatalf("total %d err %v, want at least %d", total, err, n)
	}
	seen := map[string]bool{}
	prev := ""
	for off := 0; off < total; off += 2 {
		page, tot, err := svc.Store.ListZones(ctx, off, 2)
		if err != nil {
			t.Fatal(err)
		}
		if tot != total {
			t.Fatalf("total changed between pages: %d vs %d", tot, total)
		}
		if len(page) > 2 {
			t.Fatalf("page of %d zones, limit was 2", len(page))
		}
		for _, z := range page {
			id := z.ID().String()
			if seen[id] {
				t.Fatalf("zone %s on two pages", id)
			}
			if id <= prev {
				t.Fatalf("zones not ordered by id: %s after %s", id, prev)
			}
			seen[id], prev = true, id
			if len(z.Prefixes()) == 0 {
				t.Fatalf("zone %s lost its prefixes on a page", id)
			}
		}
	}
	if len(seen) != total {
		t.Fatalf("pages covered %d zones, total says %d", len(seen), total)
	}
	if past, tot, err := svc.Store.ListZones(ctx, total+10, 5); err != nil || len(past) != 0 || tot != total {
		t.Fatalf("page past the end: %d zones, total %d, err %v", len(past), tot, err)
	}

	// Each create wrote one audit row, newest first.
	first, auditTotal, err := svc.Store.ListAudit(ctx, application.AuditFilter{}, 0, 3)
	if err != nil || len(first) != 3 || auditTotal < n {
		t.Fatalf("audit page 1: %d rows, total %d, err %v", len(first), auditTotal, err)
	}
	second, _, err := svc.Store.ListAudit(ctx, application.AuditFilter{}, 3, 3)
	if err != nil || len(second) == 0 {
		t.Fatalf("audit page 2: %d rows, err %v", len(second), err)
	}
	if first[0].ID == second[0].ID {
		t.Fatal("audit pages overlap")
	}
	if first[len(first)-1].CreatedAt.Before(second[0].CreatedAt) {
		t.Fatal("audit rows are not newest first across pages")
	}
}

// Seed: 7 zones, prefixes and settings load, and a second run changes nothing.
func TestSeedDemoIsRepeatable(t *testing.T) {
	_, conn := setup(t)
	ctx := context.Background()

	// Remove only zones this test creates, so a database that was already seeded keeps its rows.
	existing := map[string]bool{}
	for _, z := range infrastructure.DemoZones {
		var n int
		if err := conn.QueryRow("SELECT count(*) FROM delivery_zones WHERE id = $1", z.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		existing[z.ID] = n > 0
	}
	t.Cleanup(func() {
		for _, z := range infrastructure.DemoZones {
			if !existing[z.ID] {
				_, _ = conn.Exec("DELETE FROM delivery_zones WHERE id = $1", z.ID)
			}
		}
	})

	for run := 1; run <= 2; run++ {
		if err := infrastructure.SeedDemo(ctx, conn); err != nil {
			t.Fatalf("seed run %d: %v", run, err)
		}
	}
	if len(infrastructure.DemoZones) != 7 {
		t.Fatalf("want 7 demo zones, have %d", len(infrastructure.DemoZones))
	}
	for _, z := range infrastructure.DemoZones {
		var enabled bool
		var prefixes int
		if err := conn.QueryRow("SELECT enabled FROM delivery_zones WHERE id = $1", z.ID).Scan(&enabled); err != nil {
			t.Fatalf("zone %s missing: %v", z.ID, err)
		}
		if err := conn.QueryRow("SELECT count(*) FROM zone_postal_codes WHERE zone_id = $1", z.ID).Scan(&prefixes); err != nil {
			t.Fatal(err)
		}
		if !existing[z.ID] && (enabled != z.Enabled || prefixes != len(z.Prefixes)) {
			t.Errorf("zone %s: enabled=%v prefixes=%d, want %v/%d (a second run must not duplicate)", z.ID, enabled, prefixes, z.Enabled, len(z.Prefixes))
		}
	}
	var settings int
	if err := conn.QueryRow(`SELECT count(*) FROM system_settings WHERE key IN
		('reports.max_range_days','ops.board_refresh_seconds','ops.maintenance_banner','parcels.max_weight_kg','parcels.max_cod_cents')`).Scan(&settings); err != nil {
		t.Fatal(err)
	}
	if settings != 5 {
		t.Errorf("want 5 allowlisted settings, have %d", settings)
	}
}

// Audit filters: each one narrows the list, the total counts only the matching rows, and paging
// works inside a filtered list. Audit rows are append-only, so the test uses ids of its own.
func TestAuditFilters(t *testing.T) {
	svc, _ := setup(t)
	ctx := context.Background()
	run := time.Now().Format("150405")
	idA, idB := "ZONE_T_AUDF_A_"+run, "ZONE_T_AUDF_B_"+run
	for i, id := range []string{idA, idB} {
		if _, err := svc.CreateZone(ctx, adminActor, application.CreateZoneInput{
			ID: id, Code: fmt.Sprintf("T-AUF%d-%s", i, run), Name: "Audit filter", PostalPrefixes: []string{fmt.Sprintf("72%d20", i)}}); err != nil {
			t.Fatal(err)
		}
	}
	renamed := "Audit filter renamed"
	if _, err := svc.UpdateZone(ctx, adminActor, idA, application.UpdateZoneInput{Version: 1, Name: &renamed, Reason: "audit filter test"}); err != nil {
		t.Fatal(err)
	}

	count := func(f application.AuditFilter) (int, []application.AuditRow) {
		t.Helper()
		rows, total, err := svc.Store.ListAudit(ctx, f, 0, 50)
		if err != nil {
			t.Fatalf("ListAudit(%+v): %v", f, err)
		}
		if len(rows) > total {
			t.Fatalf("more rows (%d) than the total (%d)", len(rows), total)
		}
		return total, rows
	}

	// Target id: zone A has a create and an update row, zone B only a create row.
	if total, rows := count(application.AuditFilter{TargetID: idA}); total != 2 || len(rows) != 2 {
		t.Fatalf("target_id A: total %d, rows %d, want 2", total, len(rows))
	} else {
		for _, r := range rows {
			if r.TargetID != idA {
				t.Fatalf("a row for %s leaked into the filter for %s", r.TargetID, idA)
			}
		}
	}
	if total, _ := count(application.AuditFilter{TargetID: idB}); total != 1 {
		t.Fatalf("target_id B: total %d, want 1", total)
	}

	// Action, and action together with a target.
	if total, rows := count(application.AuditFilter{TargetID: idA, Action: "ZONE_UPDATED"}); total != 1 || rows[0].Action != "ZONE_UPDATED" {
		t.Fatalf("action filter: total %d", total)
	}
	if total, _ := count(application.AuditFilter{TargetID: idA, Action: "ZONE_ENABLED"}); total != 0 {
		t.Fatalf("an action that never happened must match nothing, got %d", total)
	}

	// Target type and actor role.
	if total, _ := count(application.AuditFilter{TargetType: "ZONE", ActorRole: "ADMIN", TargetID: idA}); total != 2 {
		t.Fatalf("target_type + actor_role: total %d, want 2", total)
	}
	if total, rows := count(application.AuditFilter{ActorRole: "COURIER", TargetID: idA}); total != 0 || rows == nil || len(rows) != 0 {
		t.Fatalf("no match must give an empty (not nil) list: total %d, rows %v", total, rows)
	}

	// Date range, as Sri Lanka days: today includes the rows, yesterday alone does not.
	colombo := time.FixedZone("Asia/Colombo", 5*3600+1800)
	today := time.Now().In(colombo).Format("2006-01-02")
	yesterday := time.Now().In(colombo).Add(-24 * time.Hour).Format("2006-01-02")
	inToday, err := application.ParseAuditFilter("", "", idA, "", today, today)
	if err != nil {
		t.Fatal(err)
	}
	if total, _ := count(inToday); total != 2 {
		t.Fatalf("today's range: total %d, want 2", total)
	}
	onlyYesterday, err := application.ParseAuditFilter("", "", idA, "", yesterday, yesterday)
	if err != nil {
		t.Fatal(err)
	}
	if total, _ := count(onlyYesterday); total != 0 {
		t.Fatalf("yesterday's range must not include today's rows, got %d", total)
	}

	// Paging inside a filtered list: the total stays the size of the filtered list.
	first, total1, err := svc.Store.ListAudit(ctx, application.AuditFilter{TargetID: idA}, 0, 1)
	if err != nil || len(first) != 1 || total1 != 2 {
		t.Fatalf("filtered page 1: %d rows, total %d, err %v", len(first), total1, err)
	}
	second, total2, err := svc.Store.ListAudit(ctx, application.AuditFilter{TargetID: idA}, 1, 1)
	if err != nil || len(second) != 1 || total2 != 2 {
		t.Fatalf("filtered page 2: %d rows, total %d, err %v", len(second), total2, err)
	}
	if first[0].ID == second[0].ID {
		t.Fatal("filtered pages overlap")
	}
	if first[0].CreatedAt.Before(second[0].CreatedAt) {
		t.Fatal("filtered rows are not newest first")
	}
	if past, total3, err := svc.Store.ListAudit(ctx, application.AuditFilter{TargetID: idA}, 10, 5); err != nil || len(past) != 0 || total3 != 2 {
		t.Fatalf("filtered page past the end: %d rows, total %d, err %v", len(past), total3, err)
	}

	// A value that looks like SQL is only ever a bound parameter: it matches nothing and breaks nothing.
	if total, _ := count(application.AuditFilter{TargetID: "x' OR '1'='1"}); total != 0 {
		t.Fatalf("an injection-looking target id must match nothing, got %d", total)
	}
	var unfiltered, all int
	unfiltered, _ = count(application.AuditFilter{})
	if all, _ = count(application.AuditFilter{ActorRole: "ADMIN"}); all > unfiltered {
		t.Fatalf("a filter cannot match more rows (%d) than no filter (%d)", all, unfiltered)
	}
}

// Regression: rows written by other modules (identity) leave target_type, target_id and the
// snapshots NULL and use entity_type, entity_id and metadata. Reading them used to fail with
// "converting NULL to string", which made the whole audit list answer 500.
func TestAuditListHandlesRowsFromOtherModules(t *testing.T) {
	svc, conn := setup(t)
	ctx := context.Background()
	userID := "USER_T_LEGACY_" + time.Now().Format("150405")
	if _, err := conn.Exec(`INSERT INTO audit_logs (actor_role, action, entity_type, entity_id, metadata)
		VALUES ('ADMIN', 'USER_ROLE_UPDATED', 'USER', $1, '{"previous_role": "ADMIN", "new_role": "COURIER"}')`, userID); err != nil {
		t.Fatalf("insert an identity-style audit row: %v", err)
	}

	rows, total, err := svc.Store.ListAudit(ctx, application.AuditFilter{}, 0, 50)
	if err != nil || total == 0 || len(rows) == 0 {
		t.Fatalf("the unfiltered list must work with such a row present: %d rows, total %d, err %v", len(rows), total, err)
	}

	for _, f := range []application.AuditFilter{{TargetID: userID}, {TargetType: "USER", TargetID: userID}, {Action: "USER_ROLE_UPDATED", TargetID: userID}} {
		got, n, err := svc.Store.ListAudit(ctx, f, 0, 10)
		if err != nil || n != 1 || len(got) != 1 {
			t.Fatalf("filter %+v: %d rows, total %d, err %v", f, len(got), n, err)
		}
		r := got[0]
		if r.TargetType != "USER" || r.TargetID != userID {
			t.Fatalf("the legacy target must show: %q %q", r.TargetType, r.TargetID)
		}
		if string(r.Before) != `{"role": "ADMIN"}` && string(r.Before) != `{"role":"ADMIN"}` {
			t.Fatalf("before = %s", r.Before)
		}
		if string(r.After) != `{"role": "COURIER"}` && string(r.After) != `{"role":"COURIER"}` {
			t.Fatalf("after = %s", r.After)
		}
	}
}
