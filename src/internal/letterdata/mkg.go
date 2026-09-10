package letterdata

// SumberMKG adalah bahan mentah penghitungan baris masa kerja naskah SK.
// Angka "masa kerja" pada SK boleh ganjil dan berbulan (mis. SK KP 5 th 7 bl),
// sementara siklus KGB selalu dua tahun penuh tanpa bulan.
type SumberMKG struct {
	// KGBTahun/KGBBulan: masa kerja pada SK KGB terakhir (arsip, apa adanya).
	KGBTahun *int
	KGBBulan *int
	// FallbackTahun dipakai HANYA bila masa kerja SK KGB belum tersimpan
	// (mis. master hasil SIPPASN tanpa data KGB). Nilainya masa kerja yang
	// dihitung dari TMT awal, sesuai keputusan owner 2026-09-03.
	FallbackTahun *int
	// PemenangTahun/PemenangBulan: masa kerja SK terbaru (KP pada PNS, SK
	// Pertama atau Perpanjangan Kontrak pada PPPK). Dipakai untuk angka cetak
	// poin 6, dan hanya bila PemenangAktif benar.
	PemenangTahun *int
	PemenangBulan *int
	PemenangAktif bool
}

// MKGLine adalah baris masa kerja yang dicetak ke naskah SK beserta bracket
// grid untuk pencarian gaji:
//
//	poin 6 "Masa kerja golongan pada tgl. tsb" = LamaTahun/LamaBulan
//	poin 8 "Berdasarkan Masa Kerja"             = BaruTahun/BaruBulan
//	gaji lama/baru dicari pada bracket GridLama/GridBaru
type MKGLine struct {
	LamaTahun int
	LamaBulan int
	BaruTahun int
	BaruBulan int
	GridLama  int
	GridBaru  int
}

// HitungMKG menerapkan satu aturan resmi untuk seluruh jalur (submit,
// pratinjau gaji, dan pratinjau dasbor):
//
//   - poin 6 = masa kerja SK pemenang apa adanya bila SK itu lebih baru
//     tanggalnya (mis. SK KP 5 th 7 bl → 05 Tahun 07 Bulan), selain itu masa
//     kerja SK KGB apa adanya;
//   - poin 8 = grid genap dari angka poin 6 ditambah 2 tahun 0 bulan. KGB
//     selalu berkala dua tahun, jadi baris "Berdasarkan Masa Kerja" tidak ikut
//     membawa bulan SK pemenang (KP 5 th 7 bl tetap menghasilkan 06 tahun
//     00 bulan);
//   - gaji lama/baru dicari pada bracket grid genap poin 6 dan +2, agar baris
//     "Masa kerja golongan pada tgl. tsb" dan gaji lamanya satu sumber.
//
// Bila masa SK KGB belum tersimpan, FallbackTahun (masa kerja dari TMT awal,
// keputusan 2026-09-03) menggantikan jangkar supaya pratinjau tidak nol.
func HitungMKG(s SumberMKG) MKGLine {
	kb := s.KGBTahun
	if kb == nil {
		kb = s.FallbackTahun
	}
	lamaTahun := mkgDeref(kb)
	lamaBulan := mkgBulan(mkgDeref(s.KGBBulan))
	if s.KGBTahun == nil {
		lamaBulan = 0
	}
	if s.PemenangAktif && s.PemenangTahun != nil {
		lamaTahun = *s.PemenangTahun
		lamaBulan = mkgBulan(mkgDeref(s.PemenangBulan))
	}
	if lamaTahun < 0 {
		lamaTahun = 0
	}
	grid := EvenYear(lamaTahun)
	return MKGLine{
		LamaTahun: lamaTahun,
		LamaBulan: lamaBulan,
		BaruTahun: grid + 2,
		BaruBulan: 0,
		GridLama:  grid,
		GridBaru:  grid + 2,
	}
}

func mkgDeref(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

// mkgBulan menahan bulan di rentang sah 0–11 agar nilai rusak tidak ikut tercetak.
func mkgBulan(v int) int {
	if v < 0 || v > 11 {
		return 0
	}
	return v
}
