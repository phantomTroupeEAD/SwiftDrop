package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

// PostgresTransactionManager implements application.TransactionManager using database/sql.
type PostgresTransactionManager struct {
	db *sql.DB
}

// NewPostgresTransactionManager constructs a transaction manager wrapping a PostgreSQL connection pool.
func NewPostgresTransactionManager(db *sql.DB) *PostgresTransactionManager {
	return &PostgresTransactionManager{db: db}
}

// WithTransaction runs the provided function inside an atomic PostgreSQL transaction.
func (tm *PostgresTransactionManager) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	if tm.db == nil {
		return fmt.Errorf("transaction manager: %w", db.ErrNilDB)
	}
	tx, err := tm.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin database transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit database transaction: %w", err)
	}
	return nil
}
