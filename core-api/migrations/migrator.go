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

	// 1. Ensure schema_migrations ledger table exists
	createLedgerQuery := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(50) PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.ExecContext(ctx, createLedgerQuery); err != nil {
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
		if err := db.QueryRowContext(ctx, checkQuery, version).Scan(&alreadyApplied); err != nil {
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
		tx, err := db.BeginTx(ctx, nil)
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
