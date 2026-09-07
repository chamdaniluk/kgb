package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Districts adalah 19 kecamatan resmi Grobogan + DINAS untuk unit internal.
var Districts = []string{
	"BRATI", "GABUS", "GEYER", "GODONG", "GROBOGAN", "GUBUG",
	"KARANGRAYUNG", "KEDUNGJATI", "KLAMBU", "KRADENAN", "NGARINGAN",
	"PENAWANGAN", "PULOKULON", "PURWODADI", "TANGGUNGHARJO", "TAWANGHARJO",
	"TEGOWANU", "TOROH", "WIROSARI", "DINAS",
}

// ValidateDistrict memeriksa nama kecamatan (huruf besar); kosong = belum diisi.
func ValidateDistrict(district string) error {
	d := strings.ToUpper(strings.TrimSpace(district))
	if d == "" {
		return nil
	}
	for _, known := range Districts {
		if d == known {
			return nil
		}
	}
	return errors.New("kecamatan tidak dikenal (19 kecamatan Grobogan atau DINAS)")
}

// ListUnits mengambil semua unit kerja beserta parent Korwil dan kecamatan bila ada.
func ListUnits(ctx context.Context, pool *pgxpool.Pool) ([]Unit, error) {
	rows, err := pool.Query(ctx, `SELECT u.id, u.code, u.name, u.type, COALESCE(u.district,''), u.parent_id, COALESCE(p.name,''), u.created_at, u.updated_at FROM units u LEFT JOIN units p ON p.id=u.parent_id ORDER BY u.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Unit, 0)
	for rows.Next() {
		var u Unit
		if err := rows.Scan(&u.ID, &u.Code, &u.Name, &u.Type, &u.District, &u.ParentID, &u.ParentName, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, u)
	}
	return result, rows.Err()
}

// CreateUnit membuat unit kerja baru tanpa parent.
func CreateUnit(ctx context.Context, pool *pgxpool.Pool, code, name, unitType string) (Unit, error) {
	return CreateUnitFull(ctx, pool, code, name, unitType, "", nil)
}

// CreateUnitFull membuat unit kerja dengan kecamatan dan parent Korwil opsional.
func CreateUnitFull(ctx context.Context, pool *pgxpool.Pool, code, name, unitType, district string, parentID *int64) (Unit, error) {
	return createUnit(ctx, pool, code, name, unitType, district, parentID)
}

// UpdateUnitScope memperbarui kecamatan dan parent Korwil sebuah unit.
func UpdateUnitScope(ctx context.Context, pool *pgxpool.Pool, id int64, district string, parentID *int64) (Unit, error) {
	if err := ValidateDistrict(district); err != nil {
		return Unit{}, err
	}
	var u Unit
	err := pool.QueryRow(ctx, `
		UPDATE units SET district = NULLIF($2,''), parent_id = $3, updated_at = now() WHERE id = $1
		RETURNING id, code, name, type, COALESCE(district,''), parent_id, COALESCE((SELECT name FROM units WHERE id=units.parent_id),''), created_at, updated_at`, id, district, parentID).
		Scan(&u.ID, &u.Code, &u.Name, &u.Type, &u.District, &u.ParentID, &u.ParentName, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return Unit{}, err
	}
	return u, nil
}

func createUnit(ctx context.Context, pool *pgxpool.Pool, code, name, unitType, district string, parentID *int64) (Unit, error) {
	if err := ValidateDistrict(district); err != nil {
		return Unit{}, err
	}
	var u Unit
	err := pool.QueryRow(ctx, `
		INSERT INTO units (code, name, type, district, parent_id) VALUES ($1, $2, $3, NULLIF($4,''), $5)
		RETURNING id, code, name, type, COALESCE(district,''), parent_id, NULL::text, created_at, updated_at`, code, name, unitType, district, parentID).
		Scan(&u.ID, &u.Code, &u.Name, &u.Type, &u.District, &u.ParentID, &u.ParentName, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

// GetUnitByID mengambil unit beserta parent-nya.
func GetUnitByID(ctx context.Context, pool *pgxpool.Pool, id int64) (Unit, error) {
	var u Unit
	err := pool.QueryRow(ctx, `SELECT u.id, u.code, u.name, u.type, COALESCE(u.district,''), u.parent_id, COALESCE(p.name,''), u.created_at, u.updated_at FROM units u LEFT JOIN units p ON p.id=u.parent_id WHERE u.id=$1`, id).
		Scan(&u.ID, &u.Code, &u.Name, &u.Type, &u.District, &u.ParentID, &u.ParentName, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Unit{}, ErrNotFound
	}
	return u, err
}

// UnitInScope melaporkan apakah candidateID berada dalam kewenangan scopeID:
// unit yang sama; anak TK/SD dari Korwil (relasi parent atau kecamatan sama);
// unit Dinas hanya dalam scope unit Dinas yang sama.
func UnitInScope(ctx context.Context, pool *pgxpool.Pool, scopeID, candidateID int64) (bool, error) {
	var ok bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM units candidate JOIN units scope ON scope.id=$1
		WHERE candidate.id=$2 AND (
			candidate.id = scope.id
			OR (scope.type='korwil' AND candidate.type IN ('sd','tk') AND (
				candidate.parent_id = scope.id
				OR (candidate.district IS NOT NULL AND scope.district IS NOT NULL AND candidate.district = scope.district)
			))
			OR (scope.type='dinas' AND candidate.type='dinas' AND candidate.district IS NOT NULL AND candidate.district = scope.district)
		))`, scopeID, candidateID).Scan(&ok)
	return ok, err
}
