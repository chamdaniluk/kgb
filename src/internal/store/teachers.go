package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Teacher: baris tabel teachers (+ nama unit via join).
type Teacher struct {
	ID             int64
	UserID         *int64
	NIP            string
	Name           string
	ASNType        string
	UnitID         int64
	UnitName       string
	PangkatGol     string
	MasaKerjaTahun int
	TMTKGBLast     *time.Time
}

func GetTeacherByUserID(ctx context.Context, pool *pgxpool.Pool, userID int64) (Teacher, error) {
	var t Teacher
	err := pool.QueryRow(ctx, `
		SELECT t.id, t.user_id, t.nip, t.name, t.asn_type,
		       t.unit_id, un.name, t.pangkat_gol, t.masa_kerja_tahun, t.tmt_kgb_last
		FROM teachers t JOIN units un ON un.id = t.unit_id
		WHERE t.user_id = $1`, userID,
	).Scan(&t.ID, &t.UserID, &t.NIP, &t.Name, &t.ASNType,
		&t.UnitID, &t.UnitName, &t.PangkatGol, &t.MasaKerjaTahun, &t.TMTKGBLast)
	if err != nil {
		return Teacher{}, ErrNotFound
	}
	return t, nil
}
