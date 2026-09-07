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
// (PermenPANRB 1/2023) ditambah Kepala Sekolah sebagai tugas tambahan.
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

// jabatanNonGuruDisdik adalah jabatan non-guru di Dinas Pendidikan yang
// teramati di SIPPASN (probing 2026-09-03): fungsional pengawas/penilik/pamong,
// pelaksana, dan struktural. Daftar terbuka: jabatan SIPPASN di luar daftar
// guru namun satu dinas tetap diterima bila pola namanya dikenali.
func validJabatanNonGuru(j string) bool {
	j = strings.TrimSpace(j)
	if j == "" {
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

// validJabatanUntuk memeriksa jabatan sesuai kategori pegawai.
func validJabatanUntuk(j, kategori string) bool {
	if kategori == "non_guru" {
		return validJabatanGuru(j) || validJabatanNonGuru(j)
	}
	return validJabatanGuru(j)
}
