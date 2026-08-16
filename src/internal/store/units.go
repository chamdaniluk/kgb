package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ListUnits mengambil semua unit kerja beserta parent Korwil bila ada.
func ListUnits(ctx context.Context, pool *pgxpool.Pool) ([]Unit, error) {
	rows, err := pool.Query(ctx, `SELECT u.id, u.code, u.name, u.type, u.parent_id, COALESCE(p.name,''), u.created_at, u.updated_at FROM units u LEFT JOIN units p ON p.id=u.parent_id ORDER BY u.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Unit, 0)
	for rows.Next() {
		var u Unit
		if err := rows.Scan(&u.ID, &u.Code, &u.Name, &u.Type, &u.ParentID, &u.ParentName, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, u)
	}
	return result, rows.Err()
}

// CreateUnit membuat unit kerja baru tanpa parent.
func CreateUnit(ctx context.Context, pool *pgxpool.Pool, code, name, unitType string) (Unit, error) {
	return createUnit(ctx, pool, code, name, unitType, nil)
}

func createUnit(ctx context.Context, pool *pgxpool.Pool, code, name, unitType string, parentID *int64) (Unit, error) {
	var u Unit
	err := pool.QueryRow(ctx, `
		INSERT INTO units (code, name, type, parent_id) VALUES ($1, $2, $3, $4)
		RETURNING id, code, name, type, parent_id, NULL::text, created_at, updated_at`, code, name, unitType, parentID).
		Scan(&u.ID, &u.Code, &u.Name, &u.Type, &u.ParentID, &u.ParentName, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

// GetUnitByID mengambil unit beserta parent-nya.
func GetUnitByID(ctx context.Context, pool *pgxpool.Pool, id int64) (Unit, error) {
	var u Unit
	err := pool.QueryRow(ctx, `SELECT u.id, u.code, u.name, u.type, u.parent_id, COALESCE(p.name,''), u.created_at, u.updated_at FROM units u LEFT JOIN units p ON p.id=u.parent_id WHERE u.id=$1`, id).
		Scan(&u.ID, &u.Code, &u.Name, &u.Type, &u.ParentID, &u.ParentName, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Unit{}, ErrNotFound
	}
	return u, err
}

// UnitInScope reports whether candidateID is the scope unit or a direct child.
func UnitInScope(ctx context.Context, pool *pgxpool.Pool, scopeID, candidateID int64) (bool, error) {
	var ok bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM units WHERE id=$1 AND (id=$2 OR parent_id=$1))`, scopeID, candidateID).Scan(&ok)
	return ok, err
}
