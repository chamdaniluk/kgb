package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RefreshFromSIPPASN menyegarkan data induk satu pegawai dari baris SIPPASN.
// SIPPASN menang untuk: nama, jenis ASN, kategori, unit, pangkat/golongan,
// jabatan. Kolom milik alur KGB (masa kerja hasil terbit, TMT, dokumen)
// tidak disentuh. Dipakai saat login (refresh per akun) dan sinkron malam
// (refresh massal). Mengembalikan true bila ada kolom yang berubah.
//
// NIP yang belum ada di teachers dibuatkan baris + akun ASN (auto-provisi)
// memakai jalur UpsertImportedTeacher yang sama dengan impor.
func RefreshFromSIPPASN(ctx context.Context, pool *pgxpool.Pool, o SIPPASNOfficer, cfg SIPPASNSyncConfig, hashPassword func(string) (string, error)) (changed bool, err error) {
	mapped, ok := MapSIPPASNOfficer(o, cfg)
	if !ok {
		return false, errors.New("baris SIPPASN tidak layak sinkron")
	}
	var current struct {
		name, asn, kategori, gol, pangkat, jabatan string
		unitID                                     int64
	}
	qerr := pool.QueryRow(ctx, `SELECT t.name, t.asn_type, t.kategori, t.pangkat_gol, COALESCE(t.pangkat,''), COALESCE(t.jabatan,''), t.unit_id FROM teachers t WHERE t.nip=$1`, mapped.NIP).Scan(
		&current.name, &current.asn, &current.kategori, &current.gol, &current.pangkat, &current.jabatan, &current.unitID)
	if qerr != nil && !errors.Is(qerr, pgx.ErrNoRows) {
		return false, qerr
	}
	if errors.Is(qerr, pgx.ErrNoRows) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return false, err
		}
		defer tx.Rollback(ctx)
		if _, err := UpsertImportedTeacher(ctx, tx, mapped, hashPassword); err != nil {
			return false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return true, nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var unitID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO units (code, name, type, district, parent_id) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name, type=EXCLUDED.type, district=COALESCE(EXCLUDED.district, units.district), parent_id=CASE WHEN EXCLUDED.parent_id IS NULL THEN units.parent_id ELSE EXCLUDED.parent_id END, updated_at=now()
		RETURNING id`, mapped.UnitCode, mapped.UnitName, mapped.UnitType, nullIfEmpty(mapped.UnitDistrict), nil).Scan(&unitID); err != nil {
		return false, err
	}
	kategori := mapped.Kategori
	if kategori == "" {
		kategori = KategoriGuru
	}
	tag, err := tx.Exec(ctx, `UPDATE teachers SET name=$1, asn_type=$2, kategori=$3, unit_id=$4, pangkat_gol=$5, pangkat=NULLIF($6,''), jabatan=NULLIF($7,''), updated_at=now() WHERE nip=$8 AND (name IS DISTINCT FROM $1 OR asn_type IS DISTINCT FROM $2 OR kategori IS DISTINCT FROM $3 OR unit_id IS DISTINCT FROM $4 OR pangkat_gol IS DISTINCT FROM $5 OR COALESCE(pangkat,'') IS DISTINCT FROM $6 OR COALESCE(jabatan,'') IS DISTINCT FROM $7)`,
		mapped.Name, mapped.ASNType, kategori, unitID, mapped.PangkatGol, mapped.Pangkat, mapped.Jabatan, mapped.NIP)
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET name=$1, unit_id=$2, updated_at=now() WHERE username=$3 AND role='asn'`, mapped.Name, unitID, mapped.NIP); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
