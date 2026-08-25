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
//
// JARAK antar-bracket dua tahunan, tetapi TITIK AWAL (offset) berbeda per
// golongan/ruang sesuai Lampiran PP 5/2024: I/a & III/IV mulai bracket genap
// (0,2,4,...), sedangkan I/b–d dan II/b mulai bracket ganjil (3,5,7,...), dan
// II/a memuat bracket 1. Karena current dipilih dengan masa_kerja_tahun <=
// masaKerja (DESC), masa kerja 5 pada golongan genap memakai bracket 4 dan KGB
// berikutnya bracket 6; pada golongan ber-offset ganjil masa kerja 4 memakai
// bracket 3 dan berikutnya bracket 5.
//
// BATAS ATAS (keputusan owner): bila masa kerja melewati bracket tertinggi
// tabel golongan tersebut (mis. masa kerja 36 th pada skala yang berhenti di
// 32), gaji mengikuti nilai tertinggi tabel — current memakai bracket puncak
// dan next sama dengan current (gaji berhenti naik di puncak skala).
// Bila masaKerja lebih kecil dari bracket terendah golongan (mis. masaKerja
// 0/2 di I/b–d), tidak ada baris current dan fungsi mengembalikan
// ErrScaleNotFound — sesuai ERD §7a, tidak ditebak.
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
		// Masa kerja sudah di/di atas bracket tertinggi: gaji berikutnya
		// mengikuti nilai tertinggi tabel golongan ini (puncak skala).
		row = pool.QueryRow(ctx, `
			SELECT gaji::text
			FROM salary_scales
			WHERE asn_type = $1 AND golongan = $2
			ORDER BY masa_kerja_tahun DESC
			LIMIT 1`, asnType, golongan)
		if err := row.Scan(&next); err != nil {
			return "", "", ErrScaleNotFound
		}
	}
	return current, next, nil
}
