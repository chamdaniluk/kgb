package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrScaleNotFound: kombinasi asn_type+golongan+masa kerja tak ada di tabel.
// Sesuai ERD §7a: pengajuan ditolak dengan pesan jelas, TIDAK ditebak.
var ErrScaleNotFound = errors.New("kombinasi skala gaji tidak ditemukan")

// SalaryCurrentNext menghitung gaji pokok sekarang dan gaji KGB berikutnya
// menggunakan bracket skala terdekat yang tidak melebihi masa kerja aktual.
// Tabel skala memakai interval dua tahunan, sehingga masa kerja 5 memakai
// bracket 4 dan kenaikan berikutnya mengambil bracket 6.
// PNS: golongan = pangkat_gol guru. PPPK guru: golongan IX.
func SalaryCurrentNext(ctx context.Context, pool *pgxpool.Pool, asnType, golongan string, masaKerja int) (current, next string, err error) {
	if masaKerja < 0 {
		return "", "", ErrScaleNotFound
	}
	var currentBracket int
	row := pool.QueryRow(ctx, `
		SELECT masa_kerja_tahun, gaji::text
		FROM salary_scales
		WHERE asn_type = $1 AND golongan = $2 AND masa_kerja_tahun <= $3
		ORDER BY masa_kerja_tahun DESC
		LIMIT 1`, asnType, golongan, masaKerja)
	if err := row.Scan(&currentBracket, &current); err != nil {
		return "", "", ErrScaleNotFound
	}
	row = pool.QueryRow(ctx, `
		SELECT gaji::text
		FROM salary_scales
		WHERE asn_type = $1 AND golongan = $2
		  AND masa_kerja_tahun > $3 AND masa_kerja_tahun <= $4
		ORDER BY masa_kerja_tahun ASC
		LIMIT 1`, asnType, golongan, currentBracket, masaKerja+2)
	if err := row.Scan(&next); err != nil {
		return "", "", ErrScaleNotFound
	}
	return current, next, nil
}
