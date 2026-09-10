package httpapi

import "strings"

// pangkatByGol memetakan golongan PNS ke sebutan pangkat resmi (PP 11/2017 jo.
// PP 17/2020). Dipakai untuk memvalidasi pilihan dropdown pangkat/golongan dan
// menurunkan teks pangkat dari golongan yang dipilih di form usul KGB.
var pangkatByGol = map[string]string{
	"I/a":  "Juru Muda",
	"I/b":  "Juru Muda Tingkat I",
	"I/c":  "Juru",
	"I/d":  "Juru Tingkat I",
	"II/a": "Pengatur Muda",
	"II/b": "Pengatur Muda Tingkat I",
	"II/c": "Pengatur",
	"II/d": "Pengatur Tingkat I",
	"III/a": "Penata Muda",
	"III/b": "Penata Muda Tingkat I",
	"III/c": "Penata",
	"III/d": "Penata Tingkat I",
	"IV/a": "Pembina",
	"IV/b": "Pembina Tingkat I",
	"IV/c": "Pembina Utama Muda",
	"IV/d": "Pembina Utama Madya",
	"IV/e": "Pembina Utama",
}

// pangkatForGolongan mengembalikan sebutan pangkat untuk golongan PNS.
// Kosong bila golongan tidak dikenal.
func pangkatForGolongan(gol string) string {
	return pangkatByGol[strings.TrimSpace(gol)]
}

// validPNSGolongan memastikan golongan termasuk daftar resmi PNS.
func validPNSGolongan(gol string) bool {
	_, ok := pangkatByGol[strings.TrimSpace(gol)]
	return ok
}

// jabatanFungsionalGuru adalah jenjang jabatan fungsional guru yang valid
// (PermenPANRB 1/2023) ditambah Kepala Sekolah sebagai tugas tambahan. "Kepala
// Sekolah" tetap diterima di sisi server demi usulan lama, tetapi tidak lagi
// ditawarkan di dropdown form (kepala sekolah tetap mengisi jenjang fungsional
// gurunya; sebutan tugas tambahan bisa diketik lewat opsi Lainnya bila perlu).
var jabatanFungsionalGuru = map[string]bool{
	"Guru Ahli Pertama": true,
	"Guru Ahli Muda":    true,
	"Guru Ahli Madya":   true,
	"Guru Ahli Utama":   true,
	"Kepala Sekolah":    true,
}

// validJabatanGuru memeriksa jabatan terhadap daftar jenjang fungsional guru.
func validJabatanGuru(j string) bool {
	return jabatanFungsionalGuru[strings.TrimSpace(j)]
}

// validJabatanNonGuru memeriksa pola jabatan non-guru Disdik yang teramati di
// SIPPASN (probing 2026-09-03). Sejak form membuka isian manual "Lainnya",
// daftar ini hanya salah satu jalur penerimaan dan bukan lagi pembatas keras;
// bentuknya tetap dijaga agar pencocokan awalan tidak meloloskan teks nyasar
// (mis. "Guru;DROP ..." yang kebetulan berawalan "GURU").
func validJabatanNonGuru(j string) bool {
	j = strings.TrimSpace(j)
	if j == "" || !validJabatanBebas(j) {
		return false
	}
	upper := strings.ToUpper(j)
	for _, prefix := range []string{
		"PENGAWAS SEKOLAH", "PENILIK", "PAMONG BELAJAR",
		"OPERATOR LAYANAN", "PENATA LAYANAN", "PENGELOLA", "PENGADMINISTRASI",
		"PENGOLAH DATA", "PENELAAH TEKNIS", "PRANATA KOMPUTER", "PRANATA ",
		"ARSIPARIS", "ANALIS ", "PERENCANA ", "PENGEMBANG TEKNOLOGI",
		"KEPALA DINAS", "SEKRETARIS", "KEPALA BIDANG", "KEPALA SEKSI", "KEPALA SUB BAGIAN",
		"KEPALA SEKOLAH", "GURU", "TENAGA GURU",
	} {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

// validJabatanBebas menerima jabatan yang diketik manual lewat opsi "Lainnya":
// Dinas memuat jabatan di luar daftar (pengawas, penilik, pamong, pelaksana
// dengan sebutan lain), sehingga pengusul perlu mengisi sendiri. Yang dijaga
// hanya bentuknya agar nilai nyasar (kosong, satu karakter, karakter kendali,
// markup) tidak lolos; kewajiban menilai kebenaran jabatan tetap di verifikator.
func validJabatanBebas(j string) bool {
	runes := []rune(strings.TrimSpace(j))
	if len(runes) < 3 || len(runes) > 120 {
		return false
	}
	for _, r := range runes {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == ' ', r == '.', r == ',', r == '/', r == '(', r == ')', r == '-', r == '\'':
		default:
			return false
		}
	}
	return true
}

// validJabatanUntuk memeriksa jabatan yang diisi pengusul. Daftar guru dan
// daftar non-guru SIPPASN dipakai sebagai rujukan pilihan, tetapi isian manual
// ("Lainnya") tetap diterima selama bentuknya wajar — Dinas memang memuat
// jabatan yang tidak ada di kedua daftar tersebut.
func validJabatanUntuk(j, kategori string) bool {
	if kategori == "non_guru" {
		if validJabatanGuru(j) || validJabatanNonGuru(j) {
			return true
		}
		return validJabatanBebas(j)
	}
	if validJabatanGuru(j) {
		return true
	}
	return validJabatanBebas(j)
}
