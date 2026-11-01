// Package db opens the PostgreSQL pool and runs transactions (Doc 3 §8, Doc 5 §6).
// STAND-IN for the shared platform package; explicit Querier parameter per U-20.
package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is satisfied by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Open creates a pool with statement and lock timeouts (U-30: 5 s / 2 s).
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("DATABASE_URL cannot be parsed")
	}
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "5000"
	cfg.ConnConfig.RuntimeParams["lock_timeout"] = "2000"
	cfg.ConnConfig.RuntimeParams["application_name"] = "core-api"
	return pgxpool.NewWithConfig(ctx, cfg)
}

type TxRunner struct{ Pool *pgxpool.Pool }

// WithinTx runs fn in one READ COMMITTED transaction; any error rolls back.
func (r TxRunner) WithinTx(ctx context.Context, fn func(ctx context.Context, q any) error) error {
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r TxRunner) Reader() any { return r.Pool }

// Q converts the opaque application handle back to a Querier.
func Q(q any) Querier { return q.(Querier) }

// Violation returns the SQLSTATE and constraint name of a PostgreSQL error.
func Violation(err error) (code, constraint string) {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code, pe.ConstraintName
	}
	return "", ""
}
