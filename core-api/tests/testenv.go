package tests

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	// PostgreSQL driver registration for integration tests.
	_ "github.com/lib/pq"
	"github.com/swiftdrop/core-api/migrations"
)

// SetupTestDB initializes a throwaway schema for integration testing.
// Skips the test unless INTEGRATION=1 is set.
func SetupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("skipping cross-module integration test; set INTEGRATION=1 to run")
	}

	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = os.Getenv("DATABASE_URL")
	}
	if testDBURL == "" {
		testDBURL = "postgres://postgres:postgres@localhost:5432/swiftdrop?sslmode=disable"
	}

	db, err := sql.Open("postgres", testDBURL)
	if err != nil {
		t.Fatalf("failed to open database for integration testing: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Fatalf("cannot reach integration database at %s: %v", testDBURL, err)
	}

	schemaName := fmt.Sprintf("itest_%d", time.Now().UnixNano())
	if _, err := db.Exec(fmt.Sprintf("CREATE SCHEMA %s;", schemaName)); err != nil {
		db.Close()
		t.Fatalf("failed to create temporary schema %s: %v", schemaName, err)
	}

	if _, err := db.Exec(fmt.Sprintf("SET search_path TO %s, public;", schemaName)); err != nil {
		_, _ = db.Exec(fmt.Sprintf("DROP SCHEMA %s CASCADE;", schemaName))
		db.Close()
		t.Fatalf("failed to set search path: %v", err)
	}

	// Run migrations against temporary schema
	if err := migrations.RunMigrations(context.Background(), db); err != nil {
		_, _ = db.Exec(fmt.Sprintf("DROP SCHEMA %s CASCADE;", schemaName))
		db.Close()
		t.Fatalf("failed to apply migrations in test schema: %v", err)
	}

	cleanup := func() {
		_, _ = db.Exec(fmt.Sprintf("DROP SCHEMA %s CASCADE;", schemaName))
		_ = db.Close()
	}

	return db, cleanup
}
