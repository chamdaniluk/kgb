package letterdata

import "testing"

func intp(v int) *int { return &v }

// TestHitungMKGAturanResmi mengunci aturan baris masa kerja naskah SK:
// poin 6 = masa SK pemenang apa adanya, poin 8 = KGB + 2 tahun 0 bulan.
func TestHitungMKGAturanResmi(t *testing.T) {
	cases := []struct {
		nama string
		in   SumberMKG
		want MKGLine
	}{
		{
			// Insiden #125: SK KGB 4 th, SK KP 5 th 7 bl menang → cetak 05/07,
			// poin 8 tetap KGB + 2 = 06/00, gaji dari grid 4.
			nama: "KP menang berbulan, poin 8 dari KGB",
			in: SumberMKG{
				KGBTahun: intp(4), KGBBulan: intp(0),
				PemenangTahun: intp(5), PemenangBulan: intp(7),
				PemenangAktif: true,
			},
			want: MKGLine{LamaTahun: 5, LamaBulan: 7, BaruTahun: 6, BaruBulan: 0, GridLama: 4, GridBaru: 6},
		},
		{
			// KP menang jauh di depan KGB (masa ganjil 7): poin 6 mentah 7/0,
			// poin 8 = grid genap poin 6 (6) + 2 = 8/0 — ikut angka cetak, bukan
			// masa KGB (4 → 6) — supaya baris masa kerja dan gajinya satu sumber.
			nama: "KP menang jauh, poin 8 dari grid poin 6",
			in: SumberMKG{
				KGBTahun: intp(4), KGBBulan: intp(0),
				PemenangTahun: intp(7), PemenangBulan: intp(0),
				PemenangAktif: true,
			},
			want: MKGLine{LamaTahun: 7, LamaBulan: 0, BaruTahun: 8, BaruBulan: 0, GridLama: 6, GridBaru: 8},
		},
		{
			// Bentuk #119/#121/#124: masa SK pemenang LEBIH BESAR dari masa SK
			// KGB (KP 26 vs KGB 24). Poin 8 harus 28 (grid 26 + 2), bukan 26 —
			// kalau dipatok ke masa KGB, gaji justru turun dan naskah jadi tidak
			// konsisten (poin 6 26 th tapi gaji dari bracket 24).
			nama: "masa SK pemenang lebih besar dari SK KGB",
			in: SumberMKG{
				KGBTahun: intp(24), KGBBulan: intp(0),
				PemenangTahun: intp(26), PemenangBulan: intp(0),
				PemenangAktif: true,
			},
			want: MKGLine{LamaTahun: 26, LamaBulan: 0, BaruTahun: 28, BaruBulan: 0, GridLama: 26, GridBaru: 28},
		},
		{
			// KP berbulan lebih kecil dari masa KGB (15/6 vs 16): poin 6 mentah
			// 15/6, grid 14, poin 8 = 16/0.
			nama: "masa SK pemenang berbulan lebih kecil dari SK KGB",
			in: SumberMKG{
				KGBTahun: intp(16), KGBBulan: intp(0),
				PemenangTahun: intp(15), PemenangBulan: intp(6),
				PemenangAktif: true,
			},
			want: MKGLine{LamaTahun: 15, LamaBulan: 6, BaruTahun: 16, BaruBulan: 0, GridLama: 14, GridBaru: 16},
		},
		{
			// KP tidak menang: poin 6 = masa SK KGB apa adanya (termasuk bulan).
			nama: "KGB menang, cetak KGB mentah",
			in: SumberMKG{
				KGBTahun: intp(7), KGBBulan: intp(3),
				PemenangTahun: intp(9), PemenangBulan: intp(5),
				PemenangAktif: false,
			},
			want: MKGLine{LamaTahun: 7, LamaBulan: 3, BaruTahun: 8, BaruBulan: 0, GridLama: 6, GridBaru: 8},
		},
		{
			// Masa KGB sudah genap: tidak ada pembulatan yang mengubah angka.
			nama: "KGB genap sederhana",
			in:   SumberMKG{KGBTahun: intp(6), KGBBulan: intp(0)},
			want: MKGLine{LamaTahun: 6, LamaBulan: 0, BaruTahun: 8, BaruBulan: 0, GridLama: 6, GridBaru: 8},
		},
		{
			// Nilai rusak (bulan di luar 0–11) tidak ikut tercetak.
			nama: "bulan rusak dinolkan",
			in:   SumberMKG{KGBTahun: intp(6), KGBBulan: intp(15)},
			want: MKGLine{LamaTahun: 6, LamaBulan: 0, BaruTahun: 8, BaruBulan: 0, GridLama: 6, GridBaru: 8},
		},
		{
			// Tanpa masa KGB: grid jatuh ke 0 dan invarian wajib tetap konsisten
			// (caller menolak lewat validasi wajib isi).
			nama: "tanpa masa KGB",
			in:   SumberMKG{},
			want: MKGLine{LamaTahun: 0, LamaBulan: 0, BaruTahun: 2, BaruBulan: 0, GridLama: 0, GridBaru: 2},
		},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			got := HitungMKG(c.in)
			if got != c.want {
				t.Fatalf("HitungMKG() = %+v, want %+v", got, c.want)
			}
		})
	}
}

// TestHitungMKGInvarianPoin8 menjaga invarian yang dipakai ValidateDraft:
// poin 8 selalu GridLama + 2 dengan bulan 0.
func TestHitungMKGInvarianPoin8(t *testing.T) {
	for kb := 0; kb <= 40; kb++ {
		for bulan := 0; bulan <= 11; bulan++ {
			line := HitungMKG(SumberMKG{
				KGBTahun: intp(kb), KGBBulan: intp(bulan),
				PemenangTahun: intp(kb + 5), PemenangBulan: intp(7),
				PemenangAktif: true,
			})
			if line.BaruBulan != 0 {
				t.Fatalf("kb=%d: poin 8 bulan = %d, want 0", kb, line.BaruBulan)
			}
			// poin 8 selalu = grid genap ANGKA CETAK poin 6 + 2 (bukan masa KGB),
			// karena poin 6 di sini berasal dari SK pemenang (kb+5).
			if line.BaruTahun != EvenYear(kb+5)+2 {
				t.Fatalf("kb=%d: poin 8 tahun = %d, want %d", kb, line.BaruTahun, EvenYear(kb+5)+2)
			}
			if line.LamaTahun != kb+5 || line.LamaBulan != 7 {
				t.Fatalf("kb=%d: poin 6 = %d/%d, want %d/7", kb, line.LamaTahun, line.LamaBulan, kb+5)
			}
		}
	}
}

// TestHitungMKGFallbackTanpaSK: saat master belum menyimpan masa kerja KGB,
// jangkar jatuh ke masa kerja dari TMT awal (keputusan 2026-09-03); poin 6
// memakai angka fallback tanpa bulan, poin 8 tetap fallback + 2.
func TestHitungMKGFallbackTanpaSK(t *testing.T) {
	line := HitungMKG(SumberMKG{
		KGBTahun:      nil,
		KGBBulan:      intp(5), // bulan tak dipakai bila jangkar bukan dari SK KGB
		FallbackTahun: intp(12),
	})
	want := MKGLine{LamaTahun: 12, LamaBulan: 0, BaruTahun: 14, BaruBulan: 0, GridLama: 12, GridBaru: 14}
	if line != want {
		t.Fatalf("HitungMKG fallback = %+v, want %+v", line, want)
	}

	// Fallback ganjil: angka cetak mentah 13, grid genap 12, poin 8 = 14.
	line = HitungMKG(SumberMKG{FallbackTahun: intp(13)})
	if line.LamaTahun != 13 || line.BaruTahun != 14 || line.GridLama != 12 {
		t.Fatalf("fallback ganjil = %+v, ingin cetak 13→14 dengan grid 12", line)
	}
}
