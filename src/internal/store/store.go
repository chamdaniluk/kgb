// Package store: akses PostgreSQL SI CENDIKIA (pgx pool, tanpa ORM).
package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open membuat pgx pool dan memastikan koneksi hidup (ping dengan timeout pendek).
func Open(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, err
	}
	pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := pool.Ping(pctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
