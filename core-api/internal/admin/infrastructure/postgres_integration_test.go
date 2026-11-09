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
	"20261109000100_admin_create_delivery_zones.up.sql",
	"20261109000200_admin_create_audit_logs.up.sql",
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
	rows, err := svc.Store.ListAudit(context.Background(), 1)
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
	all, err := svc.Store.ListZones(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range all {
		if x.ID().String() == on.ID && (len(x.Prefixes()) != 1 || x.Prefixes()[0].String() != "445") {
			t.Fatalf("ListZones prefixes: %+v", application.View(x))
		}
	}
}
