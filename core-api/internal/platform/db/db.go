package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNilDB is returned when an operation requires a database connection handle but received nil.
	ErrNilDB = errors.New("db: nil database handle provided")
)

// Options specifies connection pool configuration parameters for PostgreSQL.
type Options struct {
	// MaxOpenConns sets the maximum number of open connections to the database (default 10).
	// Must adhere to: Replicas_max × MaxOpenConns ≤ PostgreSQL max_connections.
	MaxOpenConns int

	// MaxIdleConns sets the maximum number of connections in the idle connection pool (default 5).
	MaxIdleConns int

	// ConnMaxLifetime sets the maximum amount of time a connection may be reused (default 30m).
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime sets the maximum amount of time a connection may be idle (default 5m).
	ConnMaxIdleTime time.Duration

	// ConnectTimeout sets the maximum time allowed for initial database ping (default 5s).
	ConnectTimeout time.Duration
}

// DefaultOptions returns the canonical recommended pool settings for SwiftDrop Core API.
func DefaultOptions() Options {
	return Options{
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
		ConnectTimeout:  5 * time.Second,
	}
}

// Querier abstracts query execution methods shared by both *sql.DB and *sql.Tx.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Ensure *sql.DB and *sql.Tx satisfy Querier interface.
var (
	_ Querier = (*sql.DB)(nil)
	_ Querier = (*sql.Tx)(nil)
)

// Open initializes a database handle with the configured connection pool limits and validates connectivity.
func Open(ctx context.Context, driverName, dataSourceName string, opts Options) (*sql.DB, error) {
	defaults := DefaultOptions()
	if opts.MaxOpenConns <= 0 {
		opts.MaxOpenConns = defaults.MaxOpenConns
	}
	if opts.MaxIdleConns <= 0 {
		opts.MaxIdleConns = defaults.MaxIdleConns
	}
	if opts.ConnMaxLifetime <= 0 {
		opts.ConnMaxLifetime = defaults.ConnMaxLifetime
	}
	if opts.ConnMaxIdleTime <= 0 {
		opts.ConnMaxIdleTime = defaults.ConnMaxIdleTime
	}
	if opts.ConnectTimeout <= 0 {
		opts.ConnectTimeout = defaults.ConnectTimeout
	}

	database, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database handle: %w", err)
	}

	database.SetMaxOpenConns(opts.MaxOpenConns)
	database.SetMaxIdleConns(opts.MaxIdleConns)
	database.SetConnMaxLifetime(opts.ConnMaxLifetime)
	database.SetConnMaxIdleTime(opts.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, opts.ConnectTimeout)
	defer cancel()

	if err := database.PingContext(pingCtx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return database, nil
}

// WithTx executes the provided function inside an atomic database transaction.
// If the function returns an error or panics, the transaction is automatically rolled back.
// If the function completes successfully, the transaction is committed.
func WithTx(ctx context.Context, database *sql.DB, fn func(tx *sql.Tx) error) error {
	if database == nil {
		return ErrNilDB
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	panicked := true
	defer func() {
		if panicked {
			_ = tx.Rollback()
		}
	}()

	if err := fn(tx); err != nil {
		panicked = false
		_ = tx.Rollback()
		return err
	}

	panicked = false
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
