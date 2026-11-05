// Package database is the Postgres data layer: connection pool and the
// embedded goose migrations, plus error helpers.
package database

import (
	"context"
	"embed"
	"errors"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Querier is satisfied by *pgxpool.Pool and pgx.Tx, so a query can run inside a transaction or not.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// Open connects to Postgres and checks the connection.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Migrate applies all pending goose migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrations, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		return err
	}
	conn := stdlib.OpenDBFromPool(pool)
	defer conn.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, conn, migrations)
	if err != nil {
		return err
	}
	_, err = p.Up(ctx)
	return err
}

// IsUniqueViolation reports whether err is a UNIQUE constraint failure.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// IsNotFound reports whether a single-row query found no row.
func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
