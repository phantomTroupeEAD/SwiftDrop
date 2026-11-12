package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestPostgresTrackingRepository_RealDB_Integration verifies read-only queries against
// a real PostgreSQL database instance using authoritative migrations.
//
// Per engineering guidelines, this test never claims success unless executed
// against a real PostgreSQL instance with Member 2's authoritative migrations applied.
// If PostgreSQL or authoritative migrations are absent, the test is explicitly skipped/marked blocked.
func TestPostgresTrackingRepository_RealDB_Integration(t *testing.T) {
	dbURL := os.Getenv("SWIFTDROP_TEST_DB_URL")
	if dbURL == "" {
		dbURL = os.Getenv("TRACKING_DATABASE_URL")
	}

	if dbURL == "" {
		t.Skip("INTEGRATION BLOCKED: SWIFTDROP_TEST_DB_URL is unset and official Member 2 parcel migrations are not yet committed to core-api/migrations/.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Skipf("INTEGRATION BLOCKED: Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("INTEGRATION BLOCKED: PostgreSQL instance unreachable: %v", err)
	}

	// Verify authoritative parcels table exists
	const checkTableQuery = `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'parcels'
		);
	`
	var tableExists bool
	if err := db.QueryRowContext(ctx, checkTableQuery).Scan(&tableExists); err != nil || !tableExists {
		t.Skip("INTEGRATION BLOCKED: Authoritative 'parcels' table is absent in PostgreSQL. Member 2 migration 20261110000002_create_parcel_tables is pending.")
	}

	// Real database verified: test repository operations
	repo := NewPostgresTrackingRepository(db, DefaultPoolConfig())
	err = repo.Ping(ctx)
	if err != nil {
		t.Fatalf("unexpected ping failure on verified database: %v", err)
	}

	err = repo.CheckReadiness(ctx)
	if err != nil {
		t.Fatalf("unexpected readiness check failure on verified database: %v", err)
	}
}
