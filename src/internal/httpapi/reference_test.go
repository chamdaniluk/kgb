package httpapi

import "testing"

// TestValidJabatanUntukDaftarDanManual mengunci perilaku kolom Jabatan form
// usul KGB: daftar rujukan (guru/non-guru SIPPASN) diterima, dan isian manual
// lewat opsi "Lainnya" juga diterima agar pengawas/penilik/pamong serta jabatan
// lain di Dinas dapat mengisi sendiri.
func TestValidJabatanUntukDaftarDanManual(t *testing.T) {
	cases := []struct {
		jabatan  string
		kategori string
		want     bool
	}{
		// Daftar rujukan.
		{"Guru Ahli Muda", "guru", true},
		{"Guru Ahli Pertama", "guru", true},
		{"Kepala Sekolah", "guru", true}, // masih diterima server demi usulan lama
		{"Pengawas Sekolah Ahli Muda", "non_guru", true},
		{"Penilik Ahli Madya", "non_guru", true},
		{"Pamong Belajar Ahli Pertama", "non_guru", true},
		{"Operator Layanan Operasional", "non_guru", true},
		{"PENELAAH TEKNIS KEBIJAKAN", "non_guru", true},
		{"Kepala Seksi", "non_guru", true},
		// Isian manual (opsi "Lainnya"): jabatan di luar kedua daftar.
		{"Pengawas Sekolah Ahli Utama", "non_guru", true},
		{"Penilik Terampil", "non_guru", true},
		{"Guru Bimbingan Konseling Ahli Muda", "guru", true},
		{"Instruktur Diklat", "guru", true},
		{"Pengelola Sistem Informasi", "guru", true},
		{"Penyuluh Pendidikan", "non_guru", true},
		{"Tenaga Administrasi Sekolah", "non_guru", true},
		// Bentuk yang dibuang.
		{"", "guru", false},
		{"", "non_guru", false},
		{"ab", "guru", false},
		{"ab", "non_guru", false},
		{"Guru <script>", "guru", false},
		{"Guru;DROP TABLE", "non_guru", false},
		{"Guru\tAhli", "guru", false},
		{"   ", "guru", false},
	}
	for _, c := range cases {
		if got := validJabatanUntuk(c.jabatan, c.kategori); got != c.want {
			t.Errorf("validJabatanUntuk(%q, %q) = %v, ingin %v", c.jabatan, c.kategori, got, c.want)
		}
	}
}

// TestValidJabatanBebasBatasPanjang mengunci batas panjang isian manual agar
// tidak ada jabatan satu karakter maupun teks tak terbatas yang tersimpan.
func TestValidJabatanBebasBatasPanjang(t *testing.T) {
	panjang := ""
	for i := 0; i < 121; i++ {
		panjang += "a"
	}
	if validJabatanBebas(panjang) {
		t.Error("121 karakter seharusnya ditolak")
	}
	if !validJabatanBebas("abc") {
		t.Error("3 karakter seharusnya diterima")
	}
}
