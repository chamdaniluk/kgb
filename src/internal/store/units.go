package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ListUnits mengambil semua unit kerja.
func ListUnits(ctx context.Context, pool *pgxpool.Pool) ([]Unit, error) {
	rows, err := pool.Query(ctx, `SELECT id, code, name, type, created_at, updated_at FROM units ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Unit, 0)
	for rows.Next() {
		var u Unit
		if err := rows.Scan(&u.ID, &u.Code, &u.Name, &u.Type, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, u)
	}
	return result, rows.Err()
}

// CreateUnit membuat unit kerja baru.
func CreateUnit(ctx context.Context, pool *pgxpool.Pool, code, name, unitType string) (Unit, error) {
	var u Unit
	err := pool.QueryRow(ctx, `
		INSERT INTO units (code, name, type) VALUES ($1, $2, $3)
		RETURNING id, code, name, type, created_at, updated_at`, code, name, unitType).
		Scan(&u.ID, &u.Code, &u.Name, &u.Type, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

// GetUnitByID haalt een unit op.
func GetUnitByID(ctx context.Context, pool *pgxpool.Pool, id int64) (Unit, error) {
	var u Unit
	err := pool.QueryRow(ctx, `SELECT id, code, name, type, created_at, updated_at FROM units WHERE id=$1`, id).
		Scan(&u.ID, &u.Code, &u.Name, &u.Type, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Unit{}, ErrNotFound
	}
	return u, err
}
