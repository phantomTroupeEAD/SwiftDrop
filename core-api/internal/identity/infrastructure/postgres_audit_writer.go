package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/application"
)

// PostgresAuditWriter implements application.AuditWriter using PostgreSQL.
type PostgresAuditWriter struct {
	db *sql.DB
}

// NewPostgresAuditWriter constructs a PostgresAuditWriter.
func NewPostgresAuditWriter(db *sql.DB) *PostgresAuditWriter {
	return &PostgresAuditWriter{db: db}
}

// WriteAuditLog inserts a structured audit record into the audit_logs table.
func (w *PostgresAuditWriter) WriteAuditLog(ctx context.Context, entry application.AuditEntry) error {
	metaJSON, err := json.Marshal(entry.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	query := `
		INSERT INTO audit_logs (actor_id, actor_role, action, entity_type, entity_id, metadata, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
	`
	_, err = w.db.ExecContext(
		ctx,
		query,
		entry.ActorID,
		entry.ActorRole,
		entry.Action,
		entry.EntityType,
		entry.EntityID,
		metaJSON,
		entry.IPAddress,
		entry.UserAgent,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("postgres WriteAuditLog failed: %w", err)
	}
	return nil
}

// WriteAuditLogTx inserts an audit record within an existing database transaction (for atomic multi-table commits).
func (w *PostgresAuditWriter) WriteAuditLogTx(ctx context.Context, tx *sql.Tx, entry application.AuditEntry) error {
	metaJSON, err := json.Marshal(entry.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	query := `
		INSERT INTO audit_logs (actor_id, actor_role, action, entity_type, entity_id, metadata, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
	`
	_, err = tx.ExecContext(
		ctx,
		query,
		entry.ActorID,
		entry.ActorRole,
		entry.Action,
		entry.EntityType,
		entry.EntityID,
		metaJSON,
		entry.IPAddress,
		entry.UserAgent,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("postgres WriteAuditLogTx failed: %w", err)
	}
	return nil
}
