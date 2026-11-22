package migrations_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMember03_MigrationFiles_Structure verifies the presence and structural validity of Member 3 migration files.
func TestMember03_MigrationFiles_Structure(t *testing.T) {
	migrationsDir := filepath.Join("..", "..", "migrations")

	requiredFiles := []string{
		"20261110000004_create_couriers.up.sql",
		"20261110000004_create_couriers.down.sql",
		"20261110000005_create_dispatch_and_delivery.up.sql",
		"20261110000005_create_dispatch_and_delivery.down.sql",
	}

	for _, fileName := range requiredFiles {
		fullPath := filepath.Join(migrationsDir, fileName)
		contentBytes, err := os.ReadFile(fullPath)
		require.NoError(t, err, "Migration file %s must exist", fileName)
		content := string(contentBytes)

		// Must not contain transactional control statements (managed by migrator engine)
		contentUpper := strings.ToUpper(content)
		assert.NotContains(t, contentUpper, "BEGIN;", "File %s must not contain BEGIN;", fileName)
		assert.NotContains(t, contentUpper, "COMMIT;", "File %s must not contain COMMIT;", fileName)
		assert.NotContains(t, contentUpper, "CONCURRENTLY", "File %s must not contain CONCURRENTLY", fileName)
	}

	// Verify required check constraints in 20261110000004_create_couriers.up.sql
	c06Bytes, err := os.ReadFile(filepath.Join(migrationsDir, "20261110000004_create_couriers.up.sql"))
	require.NoError(t, err)
	c06 := string(c06Bytes)
	assert.Contains(t, c06, "chk_couriers_vehicle_type")
	assert.Contains(t, c06, "chk_couriers_max_capacity")
	assert.Contains(t, c06, "chk_couriers_status")
	assert.Contains(t, c06, "idx_couriers_zone_status")

	// Verify required constraints and partial unique index in 20261110000005_create_dispatch_and_delivery.up.sql
	c07Bytes, err := os.ReadFile(filepath.Join(migrationsDir, "20261110000005_create_dispatch_and_delivery.up.sql"))
	require.NoError(t, err)
	c07 := string(c07Bytes)
	assert.Contains(t, c07, "chk_assignments_status")
	assert.Contains(t, c07, "chk_assignments_ended_at")
	assert.Contains(t, c07, "idx_assignments_parcel_single_active")
	assert.Contains(t, c07, "chk_delivery_attempts_number")
	assert.Contains(t, c07, "chk_delivery_attempts_outcome")
	assert.Contains(t, c07, "chk_delivery_attempts_reason")
	assert.Contains(t, c07, "chk_delivery_attempts_outcome_consistency")
	assert.Contains(t, c07, "chk_delivery_attempts_signature_recipient")
}

// TestMember03_Migrations_Execution tests live migration execution against a real PostgreSQL instance when configured.
func TestMember03_Migrations_Execution(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping live database execution test")
	}

	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("cannot connect to database: %v; skipping live database execution test", err)
	}

	migrationsDir := filepath.Join("..", "..", "migrations")

	// Ensure prerequisite base tables exist if running against an isolated test database
	_, _ = db.Exec(`
		CREATE EXTENSION IF NOT EXISTS "pgcrypto";
		CREATE TABLE IF NOT EXISTS users (id UUID PRIMARY KEY DEFAULT gen_random_uuid());
		CREATE TABLE IF NOT EXISTS parcels (id UUID PRIMARY KEY DEFAULT gen_random_uuid());
	`)

	// 1. Apply up migrations
	up04, err := os.ReadFile(filepath.Join(migrationsDir, "20261110000004_create_couriers.up.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(up04))
	require.NoError(t, err)

	up05, err := os.ReadFile(filepath.Join(migrationsDir, "20261110000005_create_dispatch_and_delivery.up.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(up05))
	require.NoError(t, err)

	// 2. Validate tables exist in schema
	for _, table := range []string{"couriers", "courier_assignments", "delivery_attempts"} {
		var exists bool
		err = db.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", table).Scan(&exists)
		assert.NoError(t, err)
		assert.True(t, exists, "Table %s should exist in PostgreSQL", table)
	}

	// 3. Rollback
	down05, err := os.ReadFile(filepath.Join(migrationsDir, "20261110000005_create_dispatch_and_delivery.down.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(down05))
	require.NoError(t, err)

	down04, err := os.ReadFile(filepath.Join(migrationsDir, "20261110000004_create_couriers.down.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(down04))
	require.NoError(t, err)

	// 4. Re-apply to leave schema in clean state for subsequent tests
	_, err = db.Exec(string(up04))
	require.NoError(t, err)
	_, err = db.Exec(string(up05))
	require.NoError(t, err)
}
