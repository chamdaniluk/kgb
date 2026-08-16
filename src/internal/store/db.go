package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier adalah kontrak query yang dipenuhi pool dan transaksi pgx.
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// ScanRows menjalankan callback untuk setiap row dan selalu menutup cursor.
func ScanRows(ctx context.Context, rows pgx.Rows, scan func(pgx.Row) error) error {
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
