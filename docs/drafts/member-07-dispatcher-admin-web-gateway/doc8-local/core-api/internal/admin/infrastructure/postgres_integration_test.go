package infrastructure_test

// Integration tests on a REAL PostgreSQL. They run only when TEST_DATABASE_URL is set
// (scripts/local-up.sh prints how). In CI, Testcontainers would provide the database.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"swiftdrop.local/m7/core-api/internal/admin/application"
	"swiftdrop.local/m7/core-api/internal/admin/infrastructure"
	"swiftdrop.local/m7/core-api/internal/platform/apperr"
)

const adminID = "00000000-0000-4000-8000-000000000001"

func setup(t *testing.T) (*application.Service, *pgxpool.Pool) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	clean := func() {
		// Test zones only. Audit rows stay: they are append-only, and that is the point.
		_, _ = pool.Exec(context.Background(), `DELETE FROM zone_postal_codes WHERE zone_id IN (SELECT id FROM delivery_zones WHERE code LIKE 'ZONE_T_%')`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM delivery_zones WHERE code LIKE 'ZONE_T_%'`)
	}
	clean()
	t.Cleanup(func() { clean(); pool.Close() })
	n := 0
	ids := func() string {
		n++
		return fmt.Sprintf("%08x-0000-4000-8000-%012d", time.Now().UnixNano()&0xffffffff, n)
	}
	var mu sync.Mutex
	safe := func() string { mu.Lock(); defer mu.Unlock(); return ids() }
	return &application.Service{Store: &infrastructure.Store{Pool: pool, LockTimeout: 2 * time.Second}, NewID: safe}, pool
}

var admin = application.Actor{ID: adminID, Role: "ADMIN", RequestID: "itest"}

// SC-C01 (Doc 7): 20 admins add overlapping prefixes at the same moment. Exactly one wins.
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
			_, err := svc.CreateZone(context.Background(), admin, application.CreateZoneInput{
				Code: fmt.Sprintf("ZONE_T_RACE_%s_%02d", run, i), Name: "Race", PostalPrefixes: []string{prefix}})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				results["ok"]++
			case errors.Is(err, apperr.ErrPrefixConflict):
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

// Doc 6 race test: 10 admins save the same zone with the same version. One wins.
func TestStaleVersionRace(t *testing.T) {
	svc, _ := setup(t)
	run := time.Now().Format("150405")
	z, err := svc.CreateZone(context.Background(), admin, application.CreateZoneInput{
		Code: "ZONE_T_VERSION_" + run, Name: "V", PostalPrefixes: []string{"75500"}})
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
			_, err := svc.UpdateZone(context.Background(), admin, z.ID, application.UpdateZoneInput{Version: 1, Name: &name})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				ok++
			} else if errors.Is(err, apperr.ErrConcurrentConflict) {
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

// SC-U01 (Doc 7): a change and its audit row commit together, or not at all.
func TestAuditInSameTransaction(t *testing.T) {
	svc, pool := setup(t)
	run := time.Now().Format("150405")
	code := "ZONE_T_AUDIT_" + run
	bad := admin
	bad.Role = "NOT_A_ROLE" // audit_logs CHECK refuses this, so the audit insert fails
	_, err := svc.CreateZone(context.Background(), bad, application.CreateZoneInput{Code: code, Name: "A", PostalPrefixes: []string{"85500"}})
	if err == nil {
		t.Fatal("expected the audit insert to fail")
	}
	var n int
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM delivery_zones WHERE code = $1", code).Scan(&n)
	if n != 0 {
		t.Fatal("zone was saved without its audit row")
	}
}
