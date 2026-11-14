package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed *.sql
var migrationFiles embed.FS

// RunMigrations automatically checks and executes pending .up.sql migrations against PostgreSQL.
// It is idempotent: once a migration has run, it will never re-run.
func RunMigrations(ctx context.Context, db *sql.DB) error {
	log.Println("[MIGRATOR] Checking PostgreSQL schema migrations...")

	// Acquire a session-level advisory lock so concurrent migration runners
	// (e.g. parallel go test packages) serialize cleanly instead of colliding in pg_type / DDL.
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to obtain connection for migration lock: %w", err)
	}
	defer conn.Close()

	const migrationAdvisoryLockID = 83921749
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1);", migrationAdvisoryLockID); err != nil {
		return fmt.Errorf("failed to acquire migration advisory lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1);", migrationAdvisoryLockID)
	}()

	// 1. Ensure schema_migrations ledger table exists
	createLedgerQuery := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(50) PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := conn.ExecContext(ctx, createLedgerQuery); err != nil {
		return fmt.Errorf("failed to initialize schema_migrations ledger: %w", err)
	}

	// 2. Discover all embedded .up.sql files
	entries, err := migrationFiles.ReadDir(".")
	if err != nil {
		return fmt.Errorf("failed to read embedded migration files: %w", err)
	}

	var upFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upFiles = append(upFiles, entry.Name())
		}
	}
	sort.Strings(upFiles)

	// 3. Apply pending migrations sequentially
	for _, fileName := range upFiles {
		// Version is the prefix before underscore (e.g. "20261021000001")
		version := strings.Split(fileName, "_")[0]

		var alreadyApplied bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1);`
		if err := conn.QueryRowContext(ctx, checkQuery, version).Scan(&alreadyApplied); err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", fileName, err)
		}

		if alreadyApplied {
			log.Printf("[MIGRATOR]  Already applied: %s\n", fileName)
			continue
		}

		log.Printf("[MIGRATOR] ⏳ Applying pending migration: %s ...\n", fileName)

		content, err := migrationFiles.ReadFile(fileName)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", fileName, err)
		}

		// Execute migration inside a single atomic transaction
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to start transaction for %s: %w", fileName, err)
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration failed in %s: %w", fileName, err)
		}

		recordQuery := `INSERT INTO schema_migrations (version, name, applied_at) VALUES ($1, $2, $3);`
		if _, err := tx.ExecContext(ctx, recordQuery, version, filepath.Base(fileName), time.Now().UTC()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration ledger for %s: %w", fileName, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction for %s: %w", fileName, err)
		}

		log.Printf("[MIGRATOR] ✅ Successfully applied: %s\n", fileName)
	}

	log.Println("[MIGRATOR] 🎉 All database tables and seeds are up to date.")
	return nil
}

// RollbackMigrations discovers all embedded .down.sql files and executes them in reverse numerical order.
func RollbackMigrations(ctx context.Context, db *sql.DB) error {
	log.Println("[MIGRATOR] Rolling back PostgreSQL schema migrations in reverse order...")

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to obtain connection for rollback lock: %w", err)
	}
	defer conn.Close()

	const migrationAdvisoryLockID = 83921749
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1);", migrationAdvisoryLockID); err != nil {
		return fmt.Errorf("failed to acquire migration advisory lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1);", migrationAdvisoryLockID)
	}()

	// 1. Check if schema_migrations table exists
	var tableExists bool
	checkLedgerQuery := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = 'schema_migrations'
		);
	`
	if err := conn.QueryRowContext(ctx, checkLedgerQuery).Scan(&tableExists); err != nil {
		return fmt.Errorf("failed to check schema_migrations existence: %w", err)
	}
	if !tableExists {
		log.Println("[MIGRATOR] No schema_migrations ledger found; nothing to roll back.")
		return nil
	}

	// 2. Discover all embedded .down.sql files
	entries, err := migrationFiles.ReadDir(".")
	if err != nil {
		return fmt.Errorf("failed to read embedded migration files: %w", err)
	}

	downMap := make(map[string]string) // version -> filename
	var versions []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".down.sql") {
			version := strings.Split(entry.Name(), "_")[0]
			downMap[version] = entry.Name()
			versions = append(versions, version)
		}
	}

	// Sort in reverse numerical order (newest first)
	sort.Sort(sort.Reverse(sort.StringSlice(versions)))

	// 3. Roll back applied migrations in reverse order
	for _, version := range versions {
		fileName := downMap[version]

		var applied bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1);`
		if err := conn.QueryRowContext(ctx, checkQuery, version).Scan(&applied); err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", fileName, err)
		}

		if !applied {
			continue
		}

		log.Printf("[MIGRATOR] ⏳ Rolling back migration: %s ...\n", fileName)

		content, err := migrationFiles.ReadFile(fileName)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", fileName, err)
		}

		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to start rollback transaction for %s: %w", fileName, err)
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("rollback failed in %s: %w", fileName, err)
		}

		deleteQuery := `DELETE FROM schema_migrations WHERE version = $1;`
		if _, err := tx.ExecContext(ctx, deleteQuery, version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to remove migration ledger for %s: %w", fileName, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit rollback transaction for %s: %w", fileName, err)
		}

		log.Printf("[MIGRATOR] ✅ Successfully rolled back: %s\n", fileName)
	}

	log.Println("[MIGRATOR] 🏁 Database rollback completed.")
	return nil
}

// ResetMigrations rolls back all migrations and re-applies all .up.sql migrations from scratch.
func ResetMigrations(ctx context.Context, db *sql.DB) error {
	if err := RollbackMigrations(ctx, db); err != nil {
		return fmt.Errorf("rollback phase failed: %w", err)
	}
	if err := RunMigrations(ctx, db); err != nil {
		return fmt.Errorf("re-apply phase failed: %w", err)
	}
	return nil
}
