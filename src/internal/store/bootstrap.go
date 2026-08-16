package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureBootstrapAdmin membuat atau memperbarui satu admin bootstrap yang
// diberikan melalui environment. Tidak ada password default di aplikasi.
func EnsureBootstrapAdmin(ctx context.Context, pool *pgxpool.Pool, username, passwordHash, name string) error {
	if username == "" || passwordHash == "" {
		return nil
	}
	if name == "" {
		name = "Admin Sistem"
	}
	var role string
	err := pool.QueryRow(ctx, `SELECT role FROM users WHERE username=$1`, username).Scan(&role)
	if err == nil && role != "admin" {
		return errors.New("username bootstrap sudah dipakai non-admin")
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		// QueryRow returns pgx.ErrNoRows rather than store.ErrNotFound.
		// The explicit check below handles that case without hiding DB errors.
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO users (username,password_hash,role,name,is_active)
		VALUES ($1,$2,'admin',$3,true)
		ON CONFLICT (username) DO UPDATE SET password_hash=EXCLUDED.password_hash,
		name=EXCLUDED.name, role='admin', is_active=true, updated_at=now()`, username, passwordHash, name)
	return err
}
