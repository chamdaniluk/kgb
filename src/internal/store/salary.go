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
// (masa kerja + 2, golongan sama) sesuai ERD §7a.
// PNS: golongan = pangkat_gol guru. PPPK guru: golongan IX.
func SalaryCurrentNext(ctx context.Context, pool *pgxpool.Pool, asnType, golongan string, masaKerja int) (current, next string, err error) {
	row := pool.QueryRow(ctx, `
		SELECT gaji::text FROM salary_scales
		WHERE asn_type = $1 AND golongan = $2 AND masa_kerja_tahun = $3`,
		asnType, golongan, masaKerja)
	if err := row.Scan(&current); err != nil {
		return "", "", ErrScaleNotFound
	}
	row = pool.QueryRow(ctx, `
		SELECT gaji::text FROM salary_scales
		WHERE asn_type = $1 AND golongan = $2 AND masa_kerja_tahun = $3`,
		asnType, golongan, masaKerja+2)
	if err := row.Scan(&next); err != nil {
		return "", "", ErrScaleNotFound
	}
	return current, next, nil
}
