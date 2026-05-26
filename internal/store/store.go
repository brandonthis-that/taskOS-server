// Package store is the data layer. It owns the pgx pool and exposes
// typed methods for everything the HTTP layer needs.
package store

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// ErrNotFound is returned when a queried row does not exist or is not
// visible to the caller (e.g. a task belonging to another user).
var ErrNotFound = errors.New("not found")

type Store struct {
	Pool *pgxpool.Pool
}

// New connects to Postgres, pings it, and applies the schema.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("new pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{Pool: pool}, nil
}

func (s *Store) Close() { s.Pool.Close() }
