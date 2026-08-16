package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate menjalankan berkas .sql di dir (urut nama alfabetis) sekali saja.
// Nama berkas dipakai sebagai versi dan tercatat di tabel schema_migrations.
// Setiap berkas dieksekusi dalam satu transaksi: gagal = rollback penuh.
func Migrate(ctx context.Context, pool *pgxpool.Pool, dir string) (int, error) {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return 0, fmt.Errorf("buat schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("baca direktori migrasi %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	applied := 0
	for _, name := range names {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, name,
		).Scan(&exists); err != nil {
			return applied, fmt.Errorf("cek versi %s: %w", name, err)
		}
		if exists {
			continue
		}
		sqlBytes, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return applied, fmt.Errorf("baca %s: %w", name, err)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return applied, err
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("eksekusi %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("catat %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}
